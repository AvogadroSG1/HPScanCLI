package discovery

import (
	"context"
	"testing"
	"time"
)

func TestDiscoverReturnsWithoutError(t *testing.T) {
	ctx := context.Background()
	opts := DiscoverOptions{Timeout: 500 * time.Millisecond}

	scanners, err := Discover(ctx, opts)
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	// Results may be empty if no scanners are on the network; that is acceptable.
	t.Logf("discovered %d scanner(s)", len(scanners))
}
