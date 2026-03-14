package discovery

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultSweepConcurrency = 50
	defaultHostTimeout      = 1 * time.Second
	defaultSweepPort        = 80
)

// localIP returns the preferred outbound IP address of this machine.
func localIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("determining local ip: %w", err)
	}
	defer conn.Close()

	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}

// Sweep performs a TCP sweep of the local /24 subnet looking for eSCL scanners.
// It probes each host in parallel and returns all discovered scanners.
func Sweep(ctx context.Context, opts SweepOptions) ([]Scanner, error) {
	concurrency := opts.Concurrency
	if concurrency == 0 {
		concurrency = defaultSweepConcurrency
	}
	perHostTimeout := opts.PerHostTimeout
	if perHostTimeout == 0 {
		perHostTimeout = defaultHostTimeout
	}
	port := opts.Port
	if port == 0 {
		port = defaultSweepPort
	}

	selfIP, err := localIP()
	if err != nil {
		return nil, err
	}

	// Extract the first three octets to form the subnet prefix.
	lastDot := strings.LastIndex(selfIP, ".")
	if lastDot == -1 {
		return nil, fmt.Errorf("unexpected ip format: %s", selfIP)
	}
	subnet := selfIP[:lastDot+1]

	ipCh := make(chan string, concurrency)
	go func() {
		for i := 1; i <= 254; i++ {
			ip := fmt.Sprintf("%s%d", subnet, i)
			if ip == selfIP {
				continue
			}
			ipCh <- ip
		}
		close(ipCh)
	}()

	var (
		mu      sync.Mutex
		results []Scanner
		wg      sync.WaitGroup
	)

	hc := &http.Client{Timeout: perHostTimeout}

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ipCh {
				if ctx.Err() != nil {
					return
				}
				if isESCLScanner(hc, ip, port) {
					mu.Lock()
					results = append(results, Scanner{
						IP:       ip,
						Port:     port,
						Protocol: "eSCL",
						Source:   "sweep",
					})
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return results, nil
}

// isESCLScanner checks whether the host at ip:port exposes an eSCL endpoint.
func isESCLScanner(hc *http.Client, ip string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/eSCL/ScannerCapabilities", ip, port)
	resp, err := hc.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
