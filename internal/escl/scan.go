package escl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxPollRetries = 10
	pollInterval   = 500 * time.Millisecond
)

// Scan submits a scan job, polls for completion, and downloads the result.
func (c *Client) Scan(ctx context.Context, settings ScanSettings) (*ScanResult, error) {
	jobURI, err := c.submitJob(ctx, settings)
	if err != nil {
		return nil, err
	}

	data, contentType, err := c.downloadDocument(ctx, jobURI)
	if err != nil {
		return nil, err
	}

	return &ScanResult{
		Data:        data,
		ContentType: contentType,
		Settings:    settings,
	}, nil
}

// submitJob posts scan settings and returns the job URI. It uses the Location
// header from the 201 response as primary, falling back to status polling if
// the header is missing.
func (c *Client) submitJob(ctx context.Context, settings ScanSettings) (string, error) {
	body, err := RenderScanSettingsXML(settings)
	if err != nil {
		return "", err
	}

	url := c.baseURL + "/eSCL/ScanJobs"
	c.log.DebugContext(ctx, "submitting scan job", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating scan job request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml")

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return "", ErrBusy
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status %d from ScanJobs", resp.StatusCode)
	}

	if loc := resp.Header.Get("Location"); loc != "" {
		c.log.DebugContext(ctx, "job created", "location", loc)
		return loc, nil
	}

	c.log.DebugContext(ctx, "no Location header, falling back to polling")
	return c.pollJob(ctx)
}

// pollJob polls the scanner status until a Processing job is found.
func (c *Client) pollJob(ctx context.Context) (string, error) {
	for attempt := range maxPollRetries {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("polling cancelled: %w", ctx.Err())
			case <-time.After(pollInterval):
			}
		}

		status, err := c.Status(ctx)
		if err != nil {
			return "", fmt.Errorf("polling scanner status: %w", err)
		}

		if uri := findProcessingJobURI(status); uri != "" {
			c.log.DebugContext(ctx, "found processing job", "uri", uri, "attempt", attempt)
			return uri, nil
		}
	}

	return "", ErrTimeout
}

// downloadDocument retrieves the scanned document from the scanner using
// the download timeout.
func (c *Client) downloadDocument(ctx context.Context, jobURI string) ([]byte, string, error) {
	// jobURI may be absolute (from Location header) or relative (from status polling)
	var url string
	if strings.HasPrefix(jobURI, "http://") || strings.HasPrefix(jobURI, "https://") {
		url = jobURI + "/NextDocument"
	} else {
		url = c.baseURL + jobURI + "/NextDocument"
	}
	c.log.DebugContext(ctx, "downloading document", "url", url)

	dlCtx, cancel := context.WithTimeout(ctx, c.dlTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating download request: %w", err)
	}

	resp, err := c.dlHC.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("downloading document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %d from NextDocument", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading document body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	return data, contentType, nil
}
