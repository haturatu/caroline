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

const oldestLogRefreshInterval = 5 * time.Minute

type containerRetentionState struct {
	checkedAt   time.Time
	oldestLogAt time.Time
}

type Agent struct {
	config             Config
	identity           Identity
	bootID             string
	sender             *Sender
	spool              *Spool
	startedAt          time.Time
	sequence           uint64
	entryID            uint64
	containerRetention map[string]containerRetentionState
	containerInfos     []explorer.ContainerInfo
}

func New(config Config, identity Identity) (*Agent, error) {
	bootID, err := agentproto.NewNonce()
	if err != nil {
		return nil, fmt.Errorf("create agent boot id: %w", err)
	}
	sender, err := NewSender(config, identity, bootID)
	if err != nil {
		return nil, err
	}
	spool, err := OpenSpool(config.SpoolDir(), config.SpoolMaxBytes, config.SpoolMaxAge)
	if err != nil {
		return nil, err
	}
	return &Agent{
		config: config, identity: identity, bootID: bootID, sender: sender, spool: spool,
		startedAt: time.Now().UTC(), containerRetention: make(map[string]containerRetentionState),
	}, nil
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
	queue := NewBatchQueue(a.identity.AgentID, a.bootID, a.config.MaxBatchEntries, a.config.MaxBatchBytes, a.config.QueueCapacity, a.config.FlushInterval, a.sequence)
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
			entry.InsertID = fmt.Sprintf("%s/%s/%020d", a.identity.AgentID, a.bootID, a.entryID)
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
			if err := a.sendHeartbeat(ctx, queue); err != nil {
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
	now := time.Now().UTC()
	for index := range containers {
		container := &containers[index]
		if !docker.ShouldCollect(*container) {
			continue
		}
		logConfig, inspectErr := source.InspectContainer(ctx, container.ID)
		if inspectErr != nil {
			log.Printf("agent Docker inspect failed for %s: %v", explorer.ContainerName(*container), inspectErr)
		} else {
			container.LoggingDriver = logConfig.Type
			container.LoggingOptions = logConfig.Config
		}

		state := a.containerRetention[container.ID]
		if state.checkedAt.IsZero() || now.Sub(state.checkedAt) >= oldestLogRefreshInterval {
			oldestLogAt, oldestErr := source.OldestLogTime(ctx, container.ID)
			state.checkedAt = now
			if oldestErr != nil {
				log.Printf("agent Docker oldest log lookup failed for %s: %v", explorer.ContainerName(*container), oldestErr)
			} else {
				state.oldestLogAt = oldestLogAt
			}
			a.containerRetention[container.ID] = state
		}
		container.OldestLogAt = state.oldestLogAt
		infos = append(infos, explorer.ToContainerInfoForNode(*container, a.identity.AgentID, a.identity.Hostname))
	}
	a.containerInfos = append(a.containerInfos[:0], infos...)
	queue.SetContainers(infos)
	return nil
}

func (a *Agent) sendHeartbeat(ctx context.Context, queue *BatchQueue) error {
	spoolBytes, err := a.spool.Bytes()
	if err != nil {
		return err
	}
	containerMetadata := append([]explorer.ContainerInfo(nil), a.containerInfos...)
	return a.sender.Heartbeat(ctx, agentproto.Heartbeat{
		ProtocolVersion: agentproto.ProtocolVersion, AgentID: a.identity.AgentID, BootID: a.bootID,
		AgentTime: time.Now().UTC(), UptimeSeconds: int64(time.Since(a.startedAt).Seconds()),
		Containers: len(containerMetadata), ContainerMetadata: containerMetadata,
		QueueDepth: queue.Len(), SpoolBytes: spoolBytes,
	})
}
