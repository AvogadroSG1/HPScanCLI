// Package discovery provides network scanner discovery via mDNS and TCP sweep.
package discovery

import "time"

// Scanner represents a scanner found on the local network.
type Scanner struct {
	IP       string
	Hostname string
	Port     int
	Protocol string
	Source   string
}

// DiscoverOptions controls mDNS-based scanner discovery.
type DiscoverOptions struct {
	Timeout time.Duration
}

// SweepOptions controls TCP sweep-based scanner discovery.
type SweepOptions struct {
	Concurrency    int
	PerHostTimeout time.Duration
	Port           int
}
