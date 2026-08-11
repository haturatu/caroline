package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"caroline/internal/agentproto"
)

type Identity struct {
	AgentID      string `json:"agentId"`
	PrivateKey   []byte `json:"privateKey"`
	PublicKey    []byte `json:"publicKey"`
	BootID       string `json:"bootId"`
	Fingerprint  string `json:"fingerprint"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

func LoadOrCreateIdentity(path string) (Identity, error) {
	if data, err := os.ReadFile(path); err == nil {
		var identity Identity
		if err := json.Unmarshal(data, &identity); err != nil {
			return Identity{}, fmt.Errorf("decode identity: %w", err)
		}
		if len(identity.PrivateKey) != ed25519.PrivateKeySize || len(identity.PublicKey) != ed25519.PublicKeySize || identity.AgentID == "" {
			return Identity{}, errors.New("identity file is invalid")
		}
		return identity, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Identity{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Identity{}, err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	hostname, _ := os.Hostname()
	fingerprint := machineFingerprint(hostname)
	bootID, err := agentproto.NewNonce()
	if err != nil {
		return Identity{}, err
	}
	digest := sha256.Sum256(publicKey)
	identity := Identity{
		AgentID:    "agt_" + hex.EncodeToString(digest[:])[:20],
		PrivateKey: append([]byte(nil), privateKey...), PublicKey: append([]byte(nil), publicKey...),
		BootID: bootID, Fingerprint: fingerprint, Hostname: hostname,
		OS: runtime.GOOS, Architecture: runtime.GOARCH,
	}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return Identity{}, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return LoadOrCreateIdentity(path)
		}
		return Identity{}, err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return Identity{}, err
	}
	if err := file.Close(); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func (i Identity) Private() ed25519.PrivateKey { return ed25519.PrivateKey(i.PrivateKey) }

func machineFingerprint(hostname string) string {
	machineID, _ := os.ReadFile("/etc/machine-id")
	parts := []string{strings.TrimSpace(string(machineID)), hostname, runtime.GOOS, runtime.GOARCH}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
