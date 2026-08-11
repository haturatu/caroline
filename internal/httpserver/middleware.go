package httpserver

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *statusResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusResponseWriter) Write(body []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusResponseWriter) Flush() {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusResponseWriter) status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		response := &statusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && response.status() >= http.StatusMultipleChoices {
			log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), response.status(), time.Since(started).Round(time.Millisecond))
		}
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func getOnly(handler http.HandlerFunc) http.HandlerFunc {
	return allowMethods(handler, http.MethodGet, http.MethodHead)
}

func postOnly(handler http.HandlerFunc) http.HandlerFunc {
	return allowMethods(handler, http.MethodPost)
}

func allowMethods(handler http.HandlerFunc, allowed ...string) http.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, method := range allowed {
		allowedSet[method] = struct{}{}
	}
	allow := strings.Join(allowed, ", ")
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := allowedSet[r.Method]; ok {
			handler(w, r)
			return
		}
		if isKnownHTTPMethod(r.Method) {
			w.Header().Set("Allow", allow)
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeError(w, http.StatusNotImplemented, "method not implemented")
	}
}

func isKnownHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
