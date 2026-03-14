// Package escl implements an eSCL (Embedded Scan Control Language) client
// for communicating with HP network scanners over HTTP.
package escl

import (
	"errors"
	"log/slog"
	"time"
)

// ErrUnreachable indicates the scanner did not respond to a connection attempt.
var ErrUnreachable = errors.New("scanner unreachable")

// ErrBusy indicates the scanner rejected the job because it is busy.
var ErrBusy = errors.New("scanner busy")

// ErrTimeout indicates a polling or download operation exceeded its deadline.
var ErrTimeout = errors.New("operation timed out")

// ClientOptions configures the eSCL client.
type ClientOptions struct {
	ConnectTimeout  time.Duration
	DownloadTimeout time.Duration
	Logger          *slog.Logger
}

// ScanSettings configures a scan job. This is an internal config struct
// with no JSON tags; the JSON output contract uses a separate response type.
type ScanSettings struct {
	Height            int
	Width             int
	XResolution       int
	YResolution       int
	ColorMode         string
	DocumentFormat    string
	InputSource       string
	Duplex            bool
	Brightness        int
	Contrast          int
	CompressionFactor int
}

// ScanResult holds the raw scan output before file writing.
type ScanResult struct {
	Data        []byte
	ContentType string
	Settings    ScanSettings
}

// CapabilitiesResponse holds the parsed scanner capabilities for JSON output.
type CapabilitiesResponse struct {
	MakeAndModel    string       `json:"make_and_model"`
	SerialNumber    string       `json:"serial_number"`
	Manufacturer    string       `json:"manufacturer"`
	FirmwareVersion string       `json:"firmware_version"`
	Platen          *InputSource `json:"platen"`
	Adf             *InputSource `json:"adf"`
	AdfDuplex       *InputSource `json:"adf_duplex"`
}

// InputSource describes the capabilities of a single scan input source.
type InputSource struct {
	MinWidth        int      `json:"min_width"`
	MaxWidth        int      `json:"max_width"`
	MinHeight       int      `json:"min_height"`
	MaxHeight       int      `json:"max_height"`
	ColorModes      []string `json:"color_modes"`
	DocumentFormats []string `json:"document_formats"`
	Resolutions     []int    `json:"resolutions"`
}

// StatusResponse holds the parsed scanner status.
type StatusResponse struct {
	State string
	Jobs  []JobInfo
}

// JobInfo describes a single scan job reported by the scanner.
type JobInfo struct {
	URI   string
	UUID  string
	State string
}
