package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/yahao333/myclawdbot/internal/config"
)

func TestServer_isValidAPIKey_HashOnly(t *testing.T) {
	rawKey := "gateway-secret-key"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := hex.EncodeToString(hash[:])

	s := &Server{
		config: &config.Config{
			Gateway: config.GatewayConfig{
				APIKeys: []string{hashedKey},
			},
		},
	}

	if !s.isValidAPIKey(rawKey) {
		t.Error("expected raw key to pass when configured hash exists")
	}
	if s.isValidAPIKey(hashedKey) {
		t.Error("expected hash text itself to fail")
	}
	if s.isValidAPIKey("wrong-key") {
		t.Error("expected wrong key to fail")
	}
}
