// Package output defines JSON response types and formatting helpers for CLI output.
package output

// ScanResponse represents the result of a completed scan operation.
type ScanResponse struct {
	File      string               `json:"file"`
	Format    string               `json:"format"`
	SizeBytes int64                `json:"size_bytes"`
	Settings  ScanSettingsResponse `json:"settings"`
}

// ScanSettingsResponse describes the scanner settings used for a scan.
type ScanSettingsResponse struct {
	DPI            int    `json:"dpi"`
	Height         int    `json:"height"`
	Width          int    `json:"width"`
	ColorMode      string `json:"color_mode"`
	Source         string `json:"source"`
	DocumentFormat string `json:"document_format"`
}

// DiscoverResponse holds a list of discovered scanners.
type DiscoverResponse struct {
	Scanners []Scanner `json:"scanners"`
}

// Scanner represents a single discovered scanner on the network.
type Scanner struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Source   string `json:"source"`
}

// ErrorResponse wraps an error body for JSON error output.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the details of an error.
type ErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// VersionResponse contains build version metadata.
type VersionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}
