package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func LoadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("key path is required")
	}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("key file %q has invalid length", path)
		}
		return ed25519.PrivateKey(append([]byte(nil), data...)), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return LoadOrCreateKey(path)
		}
		return nil, err
	}
	if _, err := file.Write(privateKey); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return privateKey, nil
}

func KeyID(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:])[:16]
}
