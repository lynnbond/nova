package nova

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Seal is a signed, serializable payload that carries
// the current state of a workflow interaction.
// Conceptually equivalent to NovaSealingWax (火漆).
type Seal struct {
	EntityCode   string              `json:"ec"`
	HandlerUID   string              `json:"hu"`
	TaskID       string              `json:"ti"`
	NextActNames []string            `json:"na"`
	ActUIDsMap   map[string][]string `json:"am,omitempty"`
	TS           int64               `json:"ts"` // unix millis
	Sign         string              `json:"s,omitempty"`
}

// SealConfig configures the seal signer.
type SealConfig struct {
	SecretKey []byte
	TTL       time.Duration // default 30s
}

// SealSigner creates and verifies Seals.
type SealSigner struct {
	key []byte
	ttl time.Duration
}

// NewSealSigner creates a new SealSigner.
func NewSealSigner(cfg SealConfig) *SealSigner {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &SealSigner{key: cfg.SecretKey, ttl: ttl}
}

// Sign creates a signed Seal.
func (ss *SealSigner) Sign(entityCode, handlerUID string, taskID string, nextActs []string, actUIDs map[string][]string) (string, error) {
	seal := &Seal{
		EntityCode:   entityCode,
		HandlerUID:   handlerUID,
		TaskID:       taskID,
		NextActNames: nextActs,
		ActUIDsMap:   actUIDs,
		TS:           time.Now().UnixMilli(),
	}
	data, err := json.Marshal(seal)
	if err != nil {
		return "", fmt.Errorf("seal marshal: %w", err)
	}
	mac := hmac.New(sha256.New, ss.key)
	mac.Write(data)
	seal.Sign = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	signed, _ := json.Marshal(seal)
	return base64.RawURLEncoding.EncodeToString(signed), nil
}

// Verify checks the seal's signature, TTL, and ownership.
func (ss *SealSigner) Verify(token, entityCode, handlerUID string, taskID string) (*Seal, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("seal decode: %w", err)
	}
	var seal Seal
	if err := json.Unmarshal(raw, &seal); err != nil {
		return nil, fmt.Errorf("seal unmarshal: %w", err)
	}
	if time.Since(time.UnixMilli(seal.TS)) > ss.ttl {
		return nil, fmt.Errorf("seal expired")
	}
	stored := seal.Sign
	seal.Sign = ""
	data, _ := json.Marshal(seal)
	mac := hmac.New(sha256.New, ss.key)
	mac.Write(data)
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(stored), []byte(expected)) {
		return nil, fmt.Errorf("seal signature mismatch")
	}
	if seal.EntityCode != entityCode {
		return nil, fmt.Errorf("seal entity mismatch")
	}
	if seal.HandlerUID != handlerUID {
		return nil, fmt.Errorf("seal handler mismatch")
	}
	if seal.TaskID != taskID {
		return nil, fmt.Errorf("seal task mismatch")
	}
	return &seal, nil
}

// Compile-time check: SealConfig is usable
var _ = (*SealSigner)(nil)
