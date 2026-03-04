package gateway

import (
	"testing"
	"time"

	"github.com/yahao333/myclawdbot/internal/config"
)

func TestServerStartIsNonBlocking(t *testing.T) {
	s := &Server{
		config: &config.Config{
			Gateway: config.GatewayConfig{
				Host: "127.0.0.1",
				Port: 0,
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected Start to return nil, got %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		if s.httpServer != nil {
			_ = s.Stop()
		}
		t.Fatal("expected Start to return quickly, but it blocked")
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}
}
