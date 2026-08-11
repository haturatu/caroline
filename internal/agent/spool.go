package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"caroline/internal/explorer"
)

type Spool struct {
	dir     string
	maxSize int64
	maxAge  time.Duration
	mu      sync.Mutex
}

type SpoolItem struct {
	Path  string
	Batch explorer.EntryBatch
	Size  int64
}

func OpenSpool(dir string, maxSize int64, maxAge time.Duration) (*Spool, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Spool{dir: dir, maxSize: maxSize, maxAge: maxAge}, nil
}

func (s *Spool) Write(batch explorer.EntryBatch) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := fmt.Sprintf("%020d-%s.json", batch.Sequence, safeName(batch.BootID))
	tempPath := filepath.Join(s.dir, "."+name+".tmp")
	path := filepath.Join(s.dir, name)
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return s.pruneLocked(time.Now().UTC())
}

func (s *Spool) Items() ([]SpoolItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.itemsLocked()
}

func (s *Spool) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(path)
}

func (s *Spool) Bytes() (int64, error) {
	items, err := s.Items()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, item := range items {
		total += item.Size
	}
	return total, nil
}

func (s *Spool) pruneLocked(now time.Time) error {
	items, err := s.itemsLocked()
	if err != nil {
		return err
	}
	for _, item := range items {
		info, statErr := os.Stat(item.Path)
		if statErr != nil {
			continue
		}
		if s.maxAge > 0 && now.Sub(info.ModTime()) > s.maxAge {
			if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	items, err = s.itemsLocked()
	if err != nil {
		return err
	}
	var total int64
	for _, item := range items {
		total += item.Size
	}
	for _, item := range items {
		if s.maxSize <= 0 || total <= s.maxSize {
			break
		}
		if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		total -= item.Size
	}
	return nil
}

func (s *Spool) itemsLocked() ([]SpoolItem, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	items := make([]SpoolItem, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var batch explorer.EntryBatch
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("decode spool item %q: %w", path, err)
		}
		sequence := strings.SplitN(entry.Name(), "-", 2)[0]
		if _, err := strconv.ParseUint(sequence, 10, 64); err != nil {
			continue
		}
		items = append(items, SpoolItem{Path: path, Batch: batch, Size: int64(len(data))})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Batch.Sequence == items[j].Batch.Sequence {
			return items[i].Path < items[j].Path
		}
		return items[i].Batch.Sequence < items[j].Batch.Sequence
	})
	return items, nil
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, value)
}
