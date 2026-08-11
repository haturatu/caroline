package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"caroline/internal/agentproto"
	"caroline/internal/docker"
	"caroline/internal/explorer"
	"caroline/internal/logstream"
)

type Agent struct {
	config    Config
	identity  Identity
	sender    *Sender
	spool     *Spool
	startedAt time.Time
	sequence  uint64
	entryID   uint64
}

func New(config Config, identity Identity) (*Agent, error) {
	sender := NewSender(config, identity)
	spool, err := OpenSpool(config.SpoolDir(), config.SpoolMaxBytes, config.SpoolMaxAge)
	if err != nil {
		return nil, err
	}
	return &Agent{config: config, identity: identity, sender: sender, spool: spool, startedAt: time.Now().UTC()}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	dockerClient := docker.NewClient(a.config.DockerHost)
	manager := logstream.NewManagerForNode(dockerClient, a.identity.AgentID, a.identity.Hostname)
	defer manager.Close()
	go func() {
		if err := a.sender.ControlLoop(ctx); err != nil && ctx.Err() == nil {
			log.Printf("agent control stream stopped: %v", err)
		}
	}()
	for {
		if err := a.runCollection(ctx, manager, dockerClient); err != nil && ctx.Err() == nil {
			log.Printf("agent collection stopped: %v", err)
		}
		if ctx.Err() != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}

func (a *Agent) runCollection(ctx context.Context, manager *logstream.Manager, source *docker.Client) error {
	subscription, err := manager.Subscribe(ctx, nil, time.Now().UTC(), 0)
	if err != nil {
		return err
	}
	defer subscription.Close()
	queue := NewBatchQueue(a.identity.AgentID, a.identity.BootID, a.config.MaxBatchEntries, a.config.MaxBatchBytes, a.config.QueueCapacity, a.config.FlushInterval, a.sequence)
	defer func() { a.sequence = queue.Sequence() }()
	if err := a.refreshContainers(ctx, source, queue); err != nil {
		log.Printf("agent container discovery failed: %v", err)
	}
	if err := a.replaySpool(ctx); err != nil {
		log.Printf("agent spool replay failed: %v", err)
	}
	flushTicker := time.NewTicker(a.config.FlushInterval)
	discoveryTicker := time.NewTicker(a.config.DiscoveryInterval)
	heartbeatTicker := time.NewTicker(a.config.HeartbeatInterval)
	defer flushTicker.Stop()
	defer discoveryTicker.Stop()
	defer heartbeatTicker.Stop()
	for {
		select {
		case entry, open := <-subscription.Entries:
			if !open {
				return nil
			}
			a.entryID++
			entry.InsertID = fmt.Sprintf("%s/%s/%020d", a.identity.AgentID, a.identity.BootID, a.entryID)
			if err := queue.Add(entry); err != nil {
				if batch, ok := queue.Flush(); ok {
					if spoolErr := a.spool.Write(batch); spoolErr != nil {
						return spoolErr
					}
				}
				if err := queue.Add(entry); err != nil {
					return err
				}
			}
			if queue.Ready() {
				if err := a.flushQueue(ctx, queue); err != nil {
					log.Printf("agent batch delivery deferred to spool: %v", err)
				}
			}
		case streamErr, open := <-subscription.Errors:
			if !open {
				return nil
			}
			log.Printf("agent Docker stream error for %s: %v", explorer.ContainerName(streamErr.Container), streamErr.Err)
		case <-flushTicker.C:
			if err := a.flushQueue(ctx, queue); err != nil {
				log.Printf("agent batch delivery deferred to spool: %v", err)
			}
		case <-discoveryTicker.C:
			if err := manager.Refresh(ctx); err != nil {
				log.Printf("agent refresh failed: %v", err)
			}
			if err := a.refreshContainers(ctx, source, queue); err != nil {
				log.Printf("agent container discovery failed: %v", err)
			}
		case <-heartbeatTicker.C:
			if err := a.sendHeartbeat(ctx, source, queue); err != nil {
				log.Printf("agent heartbeat failed: %v", err)
			}
		case <-ctx.Done():
			_ = a.flushQueue(context.Background(), queue)
			return nil
		}
	}
}

func (a *Agent) flushQueue(ctx context.Context, queue *BatchQueue) error {
	batch, ok := queue.Flush()
	if !ok {
		return a.replaySpool(ctx)
	}
	if err := a.replaySpool(ctx); err != nil {
		_ = a.spool.Write(batch)
		return err
	}
	if err := a.sender.SendBatch(ctx, batch); err != nil {
		if spoolErr := a.spool.Write(batch); spoolErr != nil {
			return spoolErr
		}
		return err
	}
	return nil
}

func (a *Agent) replaySpool(ctx context.Context) error {
	items, err := a.spool.Items()
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := a.sender.SendBatch(ctx, item.Batch); err != nil {
			return err
		}
		if err := a.spool.Remove(item.Path); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) refreshContainers(ctx context.Context, source *docker.Client, queue *BatchQueue) error {
	containers, err := source.ListRunning(ctx)
	if err != nil {
		return err
	}
	infos := make([]explorer.ContainerInfo, 0, len(containers))
	for _, container := range containers {
		infos = append(infos, explorer.ToContainerInfoForNode(container, a.identity.AgentID, a.identity.Hostname))
	}
	queue.SetContainers(infos)
	return nil
}

func (a *Agent) sendHeartbeat(ctx context.Context, source *docker.Client, queue *BatchQueue) error {
	containers, err := source.ListRunning(ctx)
	if err != nil {
		return err
	}
	spoolBytes, err := a.spool.Bytes()
	if err != nil {
		return err
	}
	return a.sender.Heartbeat(ctx, agentproto.Heartbeat{
		ProtocolVersion: agentproto.ProtocolVersion, AgentID: a.identity.AgentID, BootID: a.identity.BootID,
		AgentTime: time.Now().UTC(), UptimeSeconds: int64(time.Since(a.startedAt).Seconds()),
		Containers: len(containers), QueueDepth: queue.Len(), SpoolBytes: spoolBytes,
	})
}
