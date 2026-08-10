package httpserver

import (
	"log"
	"net/http"
	"os"
	"time"

	"caroline/internal/alert"
	"caroline/internal/docker"
	"caroline/internal/explorer"
	"caroline/internal/logstream"
)

const defaultPort = "8080"

type Server struct {
	docker   *docker.Client
	explorer *explorer.Service
	streams  *logstream.Manager
	alerts   *alert.Engine
}

func New(
	explorerService *explorer.Service,
	dockerClient *docker.Client,
	streamManager *logstream.Manager,
	alertEngine *alert.Engine,
) *Server {
	return &Server{
		docker:   dockerClient,
		explorer: explorerService,
		streams:  streamManager,
		alerts:   alertEngine,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", getOnly(s.handleHealth))
	mux.HandleFunc("/api/status", getOnly(s.handleStatus))
	mux.HandleFunc("/api/explorer", getOnly(s.handleExplorer))
	mux.HandleFunc("/api/tail", getOnly(s.handleTail))
	mux.HandleFunc("/api/alerts", s.handleAlerts)
	mux.HandleFunc("/api/alerts/", s.handleAlert)
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
