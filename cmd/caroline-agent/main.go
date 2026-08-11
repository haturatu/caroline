package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"caroline/internal/agent"
	"caroline/internal/agentproto"
)

func main() {
	config, err := agent.ConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	identity, err := agent.LoadOrCreateIdentity(config.IdentityPath())
	if err != nil {
		log.Fatalf("load agent identity: %v", err)
	}
	if identity.BootID, err = agentproto.NewNonce(); err != nil {
		log.Fatalf("create agent boot id: %v", err)
	}
	runner, err := agent.New(config, identity)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("Caroline Agent %s collecting Docker logs for %s", identity.AgentID, config.HubURL)
	if err := runner.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
