package main

import (
	"log"
	"os"

	"caroline/internal/docker"
	"caroline/internal/explorer"
	"caroline/internal/httpserver"
)

func main() {
	port := os.Getenv("PORT")
	host := os.Getenv("DOCKER_HOST")
	dockerClient := docker.NewClient(host)
	explorerService := explorer.NewService(dockerClient)
	server := httpserver.New(explorerService, dockerClient)

	if err := server.Run(port); err != nil {
		log.Fatal(err)
	}
}
