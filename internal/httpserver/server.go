package httpserver

import (
	"log"
	"net/http"
	"os"
	"time"

	"caroline/internal/docker"
	"caroline/internal/explorer"
)

const defaultPort = "8080"

type Server struct {
	docker   *docker.Client
	explorer *explorer.Service
}

func New(explorerService *explorer.Service, dockerClient *docker.Client) *Server {
	return &Server{docker: dockerClient, explorer: explorerService}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", getOnly(s.handleHealth))
	mux.HandleFunc("/api/status", getOnly(s.handleStatus))
	mux.HandleFunc("/api/explorer", getOnly(s.handleExplorer))
	mux.HandleFunc("/api/tail", getOnly(s.handleTail))
	mux.Handle("/", http.FileServer(http.Dir("static")))
	return securityHeaders(loggingMiddleware(mux))
}

func (s *Server) Run(port string) error {
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = defaultPort
	}
	address := ":" + port
	log.Printf("Caroline listening on http://localhost:%s", port)
	log.Printf("Docker host: %s", docker.HostDescription(os.Getenv("DOCKER_HOST")))

	server := &http.Server{
		Addr:              address,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// /api/tail is a long-lived SSE response. API handlers enforce their own
		// context deadlines, while the stream ends when the client disconnects.
		WriteTimeout:   0,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}
