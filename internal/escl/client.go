package escl

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client communicates with an eSCL-compatible scanner over HTTP.
type Client struct {
	hc        *http.Client
	dlHC      *http.Client
	baseURL   string
	log       *slog.Logger
	dlTimeout time.Duration
}

// New creates a new eSCL client for the scanner at baseURL.
func New(ctx context.Context, baseURL string, opts ClientOptions) (*Client, error) {
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = 3 * time.Second
	}
	if opts.DownloadTimeout == 0 {
		opts.DownloadTimeout = 120 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ResponseHeaderTimeout: 10 * time.Second,
	}

	hc := &http.Client{
		Transport: transport,
		Timeout:   opts.ConnectTimeout,
	}

	// Separate client for downloads — no client-level timeout so the
	// context-based dlTimeout controls the full scan+transfer duration.
	dlHC := &http.Client{
		Transport: transport,
	}

	return &Client{
		hc:        hc,
		dlHC:      dlHC,
		baseURL:   baseURL,
		log:       opts.Logger,
		dlTimeout: opts.DownloadTimeout,
	}, nil
}

// Capabilities retrieves the scanner's capabilities.
func (c *Client) Capabilities(ctx context.Context) (*CapabilitiesResponse, error) {
	url := c.baseURL + "/eSCL/ScannerCapabilities"
	c.log.DebugContext(ctx, "fetching capabilities", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating capabilities request: %w", err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from ScannerCapabilities", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading capabilities response: %w", err)
	}

	return ParseCapabilities(body)
}

// Status retrieves the scanner's current status and active jobs.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	url := c.baseURL + "/eSCL/ScannerStatus"
	c.log.DebugContext(ctx, "fetching status", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating status request: %w", err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from ScannerStatus", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading status response: %w", err)
	}

	return ParseStatus(body)
}
