package agent

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hubPin struct {
	KeyID     string            `json:"keyId"`
	PublicKey ed25519.PublicKey `json:"publicKey"`
	HubURL    string            `json:"hubUrl,omitempty"`
}

func loadHubPublicKey(config Config) (ed25519.PublicKey, error) {
	if len(config.HubPublicKey) > 0 {
		if len(config.HubPublicKey) != ed25519.PublicKeySize {
			return nil, errors.New("CAROLINE_HUB_PUBLIC_KEY has an invalid Ed25519 key length")
		}
		key := append(ed25519.PublicKey(nil), config.HubPublicKey...)
		if err := saveHubPin(config.HubPinPath(), hubKeyID(key), key, config.HubURL); err != nil {
			return nil, err
		}
		return key, nil
	}
	pin, err := readHubPin(config.HubPinPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return append(ed25519.PublicKey(nil), pin.PublicKey...), nil
}

func loadPinnedHubURL(path string) (string, error) {
	pin, err := readHubPin(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(strings.TrimSpace(pin.HubURL), "/"), nil
}

func readHubPin(path string) (hubPin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return hubPin{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return hubPin{}, err
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(path, 0o600); err != nil {
			return hubPin{}, err
		}
	}
	var pin hubPin
	if err := json.Unmarshal(data, &pin); err != nil {
		return hubPin{}, fmt.Errorf("decode Hub pin: %w", err)
	}
	if len(pin.PublicKey) != ed25519.PublicKeySize {
		return hubPin{}, errors.New("persisted Hub pin has an invalid Ed25519 key length")
	}
	return pin, nil
}

func saveHubPin(path, keyID string, publicKey ed25519.PublicKey, hubURL string) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("cannot persist an invalid Hub public key")
	}
	if keyID == "" {
		keyID = hubKeyID(publicKey)
	}
	data, err := json.MarshalIndent(hubPin{KeyID: keyID, PublicKey: publicKey, HubURL: strings.TrimRight(strings.TrimSpace(hubURL), "/")}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if directory != "." && directory != "" {
		if err := os.Chmod(directory, 0o700); err != nil {
			return err
		}
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".hub-pin-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func hubKeyID(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:])[:16]
}
