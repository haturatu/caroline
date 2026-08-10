package main

import (
	"context"
	"log"
	"os"
	"strings"

	"caroline/internal/alert"
	"caroline/internal/alert/notifier"
	"caroline/internal/docker"
	"caroline/internal/explorer"
	"caroline/internal/httpserver"
	"caroline/internal/logstream"
)

func main() {
	port := os.Getenv("PORT")
	host := os.Getenv("DOCKER_HOST")
	dockerClient := docker.NewClient(host)
	explorerService := explorer.NewService(dockerClient)
	streamManager := logstream.NewManager(dockerClient)
	defer streamManager.Close()
	alertStore := strings.TrimSpace(os.Getenv("ALERTS_FILE"))
	if alertStore == "" {
		alertStore = "alerts.json"
	}
	alertEngine, err := alert.NewEngineWithPersistence(streamManager, notifier.Webhook{}, alertStore)
	if err != nil {
		log.Fatalf("load alert store %q: %v", alertStore, err)
	}
	alertContext, cancelAlerts := context.WithCancel(context.Background())
	defer cancelAlerts()
	go func() {
		if err := alertEngine.Run(alertContext); err != nil {
			log.Printf("alert engine stopped: %v", err)
		}
	}()
	server := httpserver.New(explorerService, dockerClient, streamManager, alertEngine)

	if err := server.Run(port); err != nil {
		log.Fatal(err)
	}
}
