package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/mdns"
)

const defaultDiscoverTimeout = 5 * time.Second

// Discover finds eSCL scanners on the local network using mDNS service browsing.
// It queries for both _uscan._tcp and _uscans._tcp services and returns a
// deduplicated list of discovered scanners.
func Discover(ctx context.Context, opts DiscoverOptions) ([]Scanner, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultDiscoverTimeout
	}

	entryCh := make(chan *mdns.ServiceEntry, 16)

	var scanners []Scanner
	done := make(chan struct{})
	go func() {
		for entry := range entryCh {
			s := Scanner{
				IP:       entry.AddrV4.String(),
				Hostname: entry.Host,
				Port:     entry.Port,
				Protocol: "eSCL",
				Source:   "mdns",
			}
			scanners = append(scanners, s)
		}
		close(done)
	}()

	services := []string{"_uscan._tcp", "_uscans._tcp"}
	for _, svc := range services {
		params := mdns.DefaultParams(svc)
		params.Entries = entryCh
		params.Timeout = timeout
		params.DisableIPv6 = true

		if err := mdns.Query(params); err != nil {
			close(entryCh)
			<-done
			return nil, fmt.Errorf("mdns query for %s: %w", svc, err)
		}
	}

	close(entryCh)
	<-done

	return dedup(scanners), nil
}

// dedup removes duplicate scanners by IP address, keeping the first occurrence.
func dedup(scanners []Scanner) []Scanner {
	seen := make(map[string]struct{}, len(scanners))
	out := make([]Scanner, 0, len(scanners))
	for _, s := range scanners {
		if _, ok := seen[s.IP]; ok {
			continue
		}
		seen[s.IP] = struct{}{}
		out = append(out, s)
	}
	return out
}
