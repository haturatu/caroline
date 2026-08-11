package agentproto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidSignature = errors.New("invalid agent signature")
	ErrStaleRequest     = errors.New("agent request timestamp is outside the allowed window")
	ErrReplay           = errors.New("agent request nonce was already used")
)

const RequestClockSkew = time.Minute

func NewNonce() (string, error) {
	value := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func BodyDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func CanonicalRequest(method, path, timestamp, nonce string, body []byte) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		path,
		timestamp,
		nonce,
		BodyDigest(body),
	}, "\n")
}

func SignRequest(privateKey ed25519.PrivateKey, method, path, timestamp, nonce string, body []byte) []byte {
	return ed25519.Sign(privateKey, []byte(CanonicalRequest(method, path, timestamp, nonce, body)))
}

func VerifyRequest(publicKey ed25519.PublicKey, method, path, timestamp, nonce string, body, signature []byte, now time.Time) error {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || parsed.IsZero() || now.Sub(parsed) > RequestClockSkew || parsed.Sub(now) > RequestClockSkew {
		return ErrStaleRequest
	}
	if !ed25519.Verify(publicKey, []byte(CanonicalRequest(method, path, timestamp, nonce, body)), signature) {
		return ErrInvalidSignature
	}
	return nil
}

func ChallengePayload(protocolVersion int, agentID, agentNonce, challengeID string, expiresAt time.Time) string {
	return fmt.Sprintf("%d\n%s\n%s\n%s\n%s", protocolVersion, agentID, agentNonce, challengeID, expiresAt.UTC().Format(time.RFC3339Nano))
}

func SignChallenge(privateKey ed25519.PrivateKey, protocolVersion int, agentID, agentNonce, challengeID string, expiresAt time.Time) []byte {
	return ed25519.Sign(privateKey, []byte(ChallengePayload(protocolVersion, agentID, agentNonce, challengeID, expiresAt)))
}

func VerifyChallenge(publicKey ed25519.PublicKey, signature []byte, protocolVersion int, agentID, agentNonce, challengeID string, expiresAt time.Time) bool {
	return ed25519.Verify(publicKey, []byte(ChallengePayload(protocolVersion, agentID, agentNonce, challengeID, expiresAt)), signature)
}

func ApplyRequestHeaders(request *http.Request, privateKey ed25519.PrivateKey, body []byte, now time.Time) error {
	nonce, err := NewNonce()
	if err != nil {
		return err
	}
	timestamp := now.UTC().Format(time.RFC3339Nano)
	request.Header.Set(ProtocolHeader, strconv.Itoa(ProtocolVersion))
	request.Header.Set("Caroline-Agent-Timestamp", timestamp)
	request.Header.Set("Caroline-Agent-Nonce", nonce)
	request.Header.Set("Caroline-Agent-Signature", base64.RawStdEncoding.EncodeToString(SignRequest(privateKey, request.Method, request.URL.Path, timestamp, nonce, body)))
	return nil
}

func ParseSignatureHeaders(request *http.Request) (timestamp, nonce string, signature []byte, err error) {
	if request.Header.Get(ProtocolHeader) != strconv.Itoa(ProtocolVersion) {
		return "", "", nil, fmt.Errorf("unsupported agent protocol")
	}
	timestamp = request.Header.Get("Caroline-Agent-Timestamp")
	nonce = request.Header.Get("Caroline-Agent-Nonce")
	encoded := request.Header.Get("Caroline-Agent-Signature")
	if timestamp == "" || nonce == "" || encoded == "" {
		return "", "", nil, fmt.Errorf("agent signature headers are required")
	}
	signature, err = base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid agent signature encoding: %w", err)
	}
	return timestamp, nonce, signature, nil
}
