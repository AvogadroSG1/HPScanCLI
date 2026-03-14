# Design Spec: hpscan -- Go Rewrite of HPScanCLI

**Date:** 2026-03-14
**Status:** Draft
**Authors:** Peter O'Connor, Claude (claude-opus-4-6)

---

## 1. Overview

`hpscan` is a Go rewrite of the Python [HPScanCLI](https://github.com/prasannareddych/HPScanCLI) tool. It is a command-line eSCL (Embedded Scan Control Language) client that communicates with HP network scanners over HTTP. The rewrite targets a single static binary with JSON-first output, designed to be driven by autonomous agents as well as human operators.

The tool has been tested against an HP OfficeJet Pro 9010 series at `192.168.1.198`.

### Goals

- Single binary, zero runtime dependencies
- JSON-default stdout for agent consumption
- Proper error handling with structured errors to stderr
- mDNS-based scanner discovery with subnet sweep fallback
- Support for Platen, ADF, and Duplex input sources
- Output formats: JPEG, PDF, PNG

### Non-Goals

- GUI or interactive mode
- Bulk scan (interactive loop) -- agents can call `scan` repeatedly
- Windows service or daemon mode

---

## 2. Project Structure

```
hpscan/
├── go.mod
├── go.sum
├── Makefile
├── cmd/
│   └── hpscan/
│       └── main.go              -- entry point, flag parsing, subcommand dispatch
├── internal/
│   ├── escl/
│   │   ├── client.go            -- HTTP session, base URL management
│   │   ├── capabilities.go      -- GET /eSCL/ScannerCapabilities parsing
│   │   ├── scan.go              -- POST /eSCL/ScanJobs, poll status, download
│   │   └── types.go             -- structs: Capability, ScanSettings, JobInfo
│   ├── discovery/
│   │   ├── mdns.go              -- mDNS/DNS-SD scanner discovery
│   │   └── sweep.go             -- concurrent subnet port scan fallback
│   └── output/
│       ├── format.go            -- JSON / pretty-print formatting
│       └── file.go              -- file writing, collision avoidance, PNG conversion
├── testdata/
│   ├── capabilities.xml         -- captured from real scanner
│   └── status.xml               -- captured from real scanner
└── docs/
    └── superpowers/
        └── specs/
            └── 2026-03-14-hpscan-go-rewrite-design.md
```

---

## 3. CLI Interface

Subcommand dispatch uses `github.com/google/subcommands` per Go guidelines.

### Subcommands

| Subcommand     | Description                                      |
|----------------|--------------------------------------------------|
| `discover`     | Find scanners on the local network               |
| `capabilities` | Query and display scanner capabilities            |
| `scan`         | Perform a scan and save the result to a file      |

### Global Flags

| Flag        | Type   | Default | Description                              |
|-------------|--------|---------|------------------------------------------|
| `--pretty`  | bool   | false   | Human-readable output instead of JSON    |
| `--verbose` | bool   | false   | Structured detail in error output        |
| `--version` | bool   | false   | Print version information and exit       |

### Scan Flags

| Flag       | Type   | Default    | Env Var                  | Description                        |
|------------|--------|------------|--------------------------|------------------------------------|
| `--ip`     | string | (none)     | `HPSCAN_IP`              | Scanner IP address (required)      |
| `--dpi`    | int    | 300        | `HPSCAN_DPI`             | Scan resolution                    |
| `--height` | int    | (max)      | (none)                   | Scan region height (scanner units) |
| `--width`  | int    | (max)      | (none)                   | Scan region width (scanner units)  |
| `--color`  | string | `RGB24`    | (none)                   | Color mode                         |
| `--source` | string | `Platen`   | (none)                   | Input source: Platen, Adf          |
| `--duplex` | bool   | false      | (none)                   | Enable duplex scanning (ADF only)  |
| `--format` | string | `jpeg`     | (none)                   | Output format: jpeg, pdf, png      |
| `--output` | string | (auto)     | (none)                   | Output filename (without extension)|
| `--https`  | bool   | false      | (none)                   | Use HTTPS instead of HTTP          |

### Capabilities Flags

| Flag      | Type   | Default | Env Var      | Description                   |
|-----------|--------|---------|--------------|-------------------------------|
| `--ip`    | string | (none)  | `HPSCAN_IP`  | Scanner IP address (required) |
| `--https` | bool   | false   | (none)       | Use HTTPS instead of HTTP     |

### Environment Variables

| Variable                 | Description                                | Default |
|--------------------------|--------------------------------------------|---------|
| `HPSCAN_IP`             | Default scanner IP address                  | (none)  |
| `HPSCAN_DPI`            | Default scan resolution                     | `300`   |
| `HPSCAN_TIMEOUT_SECONDS`| Document download timeout (s); all other timeouts use fixed defaults (see Section 5.6) | `120`   |

**Precedence:** CLI flags override environment variables override built-in defaults.

### Exit Codes

| Code | Meaning              |
|------|----------------------|
| 0    | Success              |
| 1    | General error        |
| 2    | Scanner unreachable  |
| 3    | Scanner busy         |
| 4    | Invalid arguments    |

### Usage Examples

```bash
# Discover scanners on the network
hpscan discover

# Show capabilities of a specific scanner
hpscan capabilities --ip 192.168.1.198

# Scan with defaults (300 DPI, JPEG, full platen)
hpscan scan --ip 192.168.1.198

# Scan to PDF at 600 DPI with custom output name
hpscan scan --ip 192.168.1.198 --dpi 600 --format pdf --output invoice

# Scan from ADF with duplex
hpscan scan --ip 192.168.1.198 --source Adf --duplex --format pdf

# Pretty-print for human consumption
hpscan capabilities --ip 192.168.1.198 --pretty

# Using environment variables
export HPSCAN_IP=192.168.1.198
export HPSCAN_DPI=600
hpscan scan --format pdf
```

---

## 4. JSON Output Contract

All stdout output is valid JSON. Errors are written to stderr as JSON. The `--pretty` flag switches stdout to human-readable formatting.

### 4.1 Discover Response

```json
{
  "scanners": [
    {
      "ip": "192.168.1.198",
      "hostname": "HP OfficeJet Pro 9010",
      "port": 443,
      "protocol": "eSCL",
      "source": "mdns"
    },
    {
      "ip": "192.168.1.42",
      "hostname": "",
      "port": 443,
      "protocol": "eSCL",
      "source": "sweep"
    }
  ]
}
```

Go type:

```go
type DiscoverResponse struct {
    Scanners []Scanner `json:"scanners"`
}

type Scanner struct {
    IP       string `json:"ip"`
    Hostname string `json:"hostname"`
    Port     int    `json:"port"`
    Protocol string `json:"protocol"`
    Source   string `json:"source"` // "mdns" or "sweep"
}
```

### 4.2 Capabilities Response

```json
{
  "make_and_model": "HP OfficeJet Pro 9010 series",
  "serial_number": "TH12345678",
  "manufacturer": "HP",
  "firmware_version": "2.1",
  "platen": {
    "min_width": 0,
    "max_width": 2550,
    "min_height": 0,
    "max_height": 3508,
    "color_modes": ["BlackAndWhite1", "Grayscale8", "RGB24"],
    "document_formats": ["image/jpeg", "application/pdf"],
    "resolutions": [75, 100, 200, 300, 600]
  },
  "adf": null
}
```

Go type:

```go
type CapabilitiesResponse struct {
    MakeAndModel    string       `json:"make_and_model"`
    SerialNumber    string       `json:"serial_number"`
    Manufacturer    string       `json:"manufacturer"`
    FirmwareVersion string       `json:"firmware_version"`
    Platen          *InputSource `json:"platen"`
    Adf             *InputSource `json:"adf"`
}

type InputSource struct {
    MinWidth        int      `json:"min_width"`
    MaxWidth        int      `json:"max_width"`
    MinHeight       int      `json:"min_height"`
    MaxHeight       int      `json:"max_height"`
    ColorModes      []string `json:"color_modes"`
    DocumentFormats []string `json:"document_formats"`
    Resolutions     []int    `json:"resolutions"`
}
```

### 4.3 Scan Response

```json
{
  "file": "SCAN_20260314_143022.jpg",
  "format": "image/jpeg",
  "size_bytes": 2451678,
  "settings": {
    "dpi": 300,
    "height": 3508,
    "width": 2550,
    "color_mode": "RGB24",
    "source": "Platen",
    "document_format": "image/jpeg"
  }
}
```

Go types:

```go
type ScanResponse struct {
    File      string               `json:"file"`
    Format    string               `json:"format"`
    SizeBytes int64                `json:"size_bytes"`
    Settings  ScanSettingsResponse `json:"settings"`
}

// ScanSettingsResponse is the JSON output contract for scan settings.
// This is separate from the internal ScanSettings config struct (Section 5.5)
// to decouple the wire format from internal representation.
type ScanSettingsResponse struct {
    DPI            int    `json:"dpi"`
    Height         int    `json:"height"`
    Width          int    `json:"width"`
    ColorMode      string `json:"color_mode"`
    Source         string `json:"source"`
    DocumentFormat string `json:"document_format"`
}
```

### 4.4 Error Response

Standard error (stderr):

```json
{
  "error": {
    "code": 2,
    "message": "scanner unreachable at 192.168.1.198"
  }
}
```

Verbose error (`--verbose`, stderr):

```json
{
  "error": {
    "code": 2,
    "message": "scanner unreachable at 192.168.1.198",
    "detail": {
      "ip": "192.168.1.198",
      "port": 443,
      "timeout_seconds": 3,
      "underlying": "dial tcp 192.168.1.198:443: i/o timeout"
    }
  }
}
```

Go types:

```go
type ErrorResponse struct {
    Error ErrorBody `json:"error"`
}

type ErrorBody struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Detail  any    `json:"detail,omitempty"` // populated only with --verbose
}
```

---

## 5. eSCL Client (`internal/escl`)

### 5.1 Client Struct

```go
// Client communicates with an eSCL-compatible scanner over HTTP.
type Client struct {
    hc      *http.Client
    baseURL string
    log     *slog.Logger
}

// ClientOptions configures the eSCL client.
type ClientOptions struct {
    ConnectTimeout  time.Duration // default: 3s
    DownloadTimeout time.Duration // default: 120s
    Logger          *slog.Logger
}

// New creates a new eSCL client for the scanner at baseURL.
func New(ctx context.Context, baseURL string, opts ClientOptions) (*Client, error)
```

### 5.2 eSCL Protocol Endpoints

| Method | Endpoint                       | Purpose                        | Response        |
|--------|--------------------------------|--------------------------------|-----------------|
| GET    | `/eSCL/ScannerCapabilities`    | Scanner specs and limits       | XML             |
| GET    | `/eSCL/ScannerStatus`          | Current status, active jobs    | XML             |
| POST   | `/eSCL/ScanJobs`               | Submit scan job                | 201 + Location  |
| GET    | `{JobUri}/NextDocument`        | Download scanned document      | Binary (JPEG/PDF)|

### 5.3 XML Handling

The Python version uses BeautifulSoup with string-based lookups (`soup.find('pwg:MakeAndModel')`). The Go version takes two different approaches for XML generation vs. parsing, because `encoding/xml` uses full namespace URIs internally -- not prefixes -- which makes **generating** prefixed XML painful.

**Scan Settings XML Generation (request body for POST /eSCL/ScanJobs):**

Use `text/template` with a raw XML template string, similar to the Python version's `schema.py`. This avoids fighting `encoding/xml`'s namespace handling for output.

```go
var scanSettingsTemplate = template.Must(template.New("scanSettings").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<scan:ScanSettings xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03"
    xmlns:copy="http://www.hp.com/schemas/imaging/con/copy/2008/07/07"
    xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm">
    <pwg:Version>2.0</pwg:Version>
    <scan:Intent>Document</scan:Intent>
    <pwg:ScanRegions>
        <pwg:ScanRegion>
            <pwg:Height>{{.Height}}</pwg:Height>
            <pwg:Width>{{.Width}}</pwg:Width>
            <pwg:XOffset>0</pwg:XOffset>
            <pwg:YOffset>0</pwg:YOffset>
        </pwg:ScanRegion>
    </pwg:ScanRegions>
    <pwg:InputSource>{{.InputSource}}</pwg:InputSource>
    <scan:DocumentFormatExt>{{.DocumentFormat}}</scan:DocumentFormatExt>
    <scan:XResolution>{{.XResolution}}</scan:XResolution>
    <scan:YResolution>{{.YResolution}}</scan:YResolution>
    <scan:ColorMode>{{.ColorMode}}</scan:ColorMode>
    <scan:CompressionFactor>{{.CompressionFactor}}</scan:CompressionFactor>
    <scan:Brightness>{{.Brightness}}</scan:Brightness>
    <scan:Contrast>{{.Contrast}}</scan:Contrast>
</scan:ScanSettings>`))

// RenderScanSettingsXML renders a ScanSettings into the eSCL XML request body.
func RenderScanSettingsXML(s ScanSettings) ([]byte, error) {
    var buf bytes.Buffer
    if err := scanSettingsTemplate.Execute(&buf, s); err != nil {
        return nil, fmt.Errorf("rendering scan settings XML: %w", err)
    }
    return buf.Bytes(), nil
}
```

**Capabilities XML Parsing (response from GET /eSCL/ScannerCapabilities):**

Use `encoding/xml` with struct tags. Because `encoding/xml` resolves namespace prefixes to full URIs, struct tags must use the full namespace URI (not the prefix). Alternatively, a namespace-stripping pre-processor can remove namespace prefixes before unmarshaling, which simplifies the struct tags. The relevant namespace URIs are:

- **scan:** `http://schemas.hp.com/imaging/escl/2011/05/03`
- **pwg:** `http://www.pwg.org/schemas/2010/12/sm`

The approach below uses a `stripNamespaces` pre-processor to remove all namespace prefixes from the XML before unmarshaling, allowing plain element-name struct tags:

```go
// stripNamespaces removes namespace prefixes from XML element and attribute
// names, allowing encoding/xml to unmarshal with simple struct tags.
func stripNamespaces(data []byte) []byte {
    // Replace <prefix:Element with <Element and </prefix:Element with </Element
    // Also strip xmlns:prefix="..." declarations
    // Implementation: use a regex or xml.Decoder/Encoder rewrite pass
}

type capabilitiesXML struct {
    XMLName         xml.Name  `xml:"ScannerCapabilities"`
    MakeAndModel    string    `xml:"MakeAndModel"`
    SerialNumber    string    `xml:"SerialNumber"`
    Manufacturer    string    `xml:"Manufacturer"`
    Version         string    `xml:"Version"`
    Platen          platenXML `xml:"Platen"`
    Adf             *adfXML   `xml:"Adf"`
}

type platenXML struct {
    InputCaps inputCapsXML `xml:"PlatenInputCaps"`
}

type adfXML struct {
    InputCaps    inputCapsXML  `xml:"AdfSimplexInputCaps"`
    DuplexCaps   *inputCapsXML `xml:"AdfDuplexInputCaps"`
}

type inputCapsXML struct {
    MinWidth          int               `xml:"MinWidth"`
    MaxWidth          int               `xml:"MaxWidth"`
    MinHeight         int               `xml:"MinHeight"`
    MaxHeight         int               `xml:"MaxHeight"`
    SettingProfiles   []settingProfile  `xml:"SettingProfiles>SettingProfile"`
}

type settingProfile struct {
    ColorModes      []string           `xml:"ColorModes>ColorMode"`
    DocumentFormats []string           `xml:"DocumentFormats>DocumentFormat"`
    Resolutions     []discreteResXML   `xml:"SupportedResolutions>DiscreteResolutions>DiscreteResolution"`
}

type discreteResXML struct {
    XResolution int `xml:"XResolution"`
    YResolution int `xml:"YResolution"`
}
```

**Note:** The `testdata/` fixtures captured from the HP OfficeJet Pro 9010 are the source of truth for struct tag validation. The exact XML structure may vary between HP models; struct tags will be adjusted during implementation to match the real fixtures.

### 5.4 Methods

```go
// Capabilities retrieves the scanner's capabilities.
func (c *Client) Capabilities(ctx context.Context) (*CapabilitiesResponse, error)

// Status retrieves the scanner's current status and active jobs.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error)

// Scan submits a scan job, polls for completion, and downloads the result.
func (c *Client) Scan(ctx context.Context, settings ScanSettings) (*ScanResult, error)
```

### 5.5 ScanSettings

```go
// ScanSettings configures a scan job. This is an internal config struct
// with no JSON tags; the JSON output contract uses ScanSettingsResponse
// (Section 4.3).
type ScanSettings struct {
    Height            int    // scanner units; 0 means max
    Width             int    // scanner units; 0 means max
    XResolution       int    // DPI
    YResolution       int    // DPI
    ColorMode         string // e.g., "RGB24", "Grayscale8"
    DocumentFormat    string // MIME type: "image/jpeg", "application/pdf"
    InputSource       string // "Platen", "Adf"
    Duplex            bool
    Brightness        int    // default: 1000
    Contrast          int    // default: 1000
    CompressionFactor int    // default: 25
}

// ScanResult holds the raw scan output before file writing.
type ScanResult struct {
    Data           []byte
    ContentType    string
    Settings       ScanSettings
}
```

**Duplex handling:** In eSCL, duplex is not a separate XML element. Duplex scanning is handled by setting `InputSource` to `"Adf"` on a scanner that advertises `AdfDuplexInputCaps` in its capabilities. When the `--duplex` flag is set, the implementation must verify that the scanner's capabilities include `AdfDuplexInputCaps` and return `ErrInvalidArgs` if not. The scanner firmware handles the physical duplex mechanism; the client simply fetches pages via repeated `NextDocument` calls until 404.

**Memory note:** `ScanResult.Data` holds the entire scan in memory as `[]byte`. For v1 this is acceptable for single-page and small multi-page scans. For multi-page ADF jobs, this could consume significant memory (e.g., 600 DPI color ADF scans). Streaming to disk should be considered in a future version. As a safety measure, enforce a `maxPages` limit (default: 50) when looping `NextDocument` to prevent unbounded memory growth.

### 5.6 Timeouts

| Operation             | Timeout | Rationale                                          |
|-----------------------|---------|-----------------------------------------------------|
| TCP connect check     | 3s      | Reasonable for LAN, fast enough for sweep fallback  |
| Capabilities GET      | 10s     | XML response is small                               |
| Status GET            | 10s     | XML response is small                               |
| ScanJobs POST         | 10s     | Just submitting, not scanning                        |
| Job poll (total)      | 30s     | Max wait for scanner to start processing             |
| Document download     | 120s    | High-DPI scans produce large files                   |

`HPSCAN_TIMEOUT_SECONDS` controls the document download timeout only. All other timeouts (connect, capabilities, status, ScanJobs POST, job poll) use the fixed defaults shown above and are not user-configurable.

### 5.7 Job URI Resolution and Polling

The primary mechanism for obtaining the job URI is the `Location` header from the 201 response to `POST /eSCL/ScanJobs`. Status polling is a fallback only if the `Location` header is missing.

The Python version polls `ScannerStatus` for a `Processing` job with 5 retries at 200ms intervals. The Go version improves this:

- **Primary:** Parse `Location` header from 201 response (e.g., `http://192.168.1.198/eSCL/ScanJobs/1234`)
- **Fallback:** Poll `ScannerStatus` if `Location` header is absent
- **Max retries (fallback only):** 10
- **Interval:** 500ms
- **Backoff:** None (fixed interval; scanner state transitions are fast on LAN)
- **Total max poll time:** 5s (10 retries x 500ms)
- **Context cancellation:** Polling respects `ctx.Done()` for clean shutdown

```go
// submitJob posts scan settings and returns the job URI.
// It uses the Location header from the 201 response as primary,
// falling back to status polling if the header is missing.
func (c *Client) submitJob(ctx context.Context, settings ScanSettings) (string, error) {
    body, err := RenderScanSettingsXML(settings)
    if err != nil {
        return "", err
    }
    resp, err := c.postScanJobs(ctx, body)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        return "", fmt.Errorf("unexpected status %d from ScanJobs", resp.StatusCode)
    }

    // Primary: use Location header
    if loc := resp.Header.Get("Location"); loc != "" {
        return loc, nil
    }

    // Fallback: poll scanner status for the job URI
    return c.pollJob(ctx)
}

func (c *Client) pollJob(ctx context.Context) (string, error) {
    for attempt := range 10 {
        if attempt > 0 {
            select {
            case <-ctx.Done():
                return "", fmt.Errorf("polling cancelled: %w", ctx.Err())
            case <-time.After(500 * time.Millisecond):
            }
        }
        jobURI, err := c.findProcessingJob(ctx)
        if err != nil {
            return "", fmt.Errorf("polling scanner status: %w", err)
        }
        if jobURI != "" {
            return jobURI, nil
        }
    }
    return "", ErrTimeout
}
```

---

## 6. Discovery (`internal/discovery`)

### 6.1 mDNS Discovery

Primary discovery mechanism using DNS-SD (RFC 6763).

- **Library:** `github.com/hashicorp/mdns` (MPL-2.0, actively maintained, pure Go)
- **Service types:** `_uscan._tcp` and `_uscans._tcp`
- **Timeout:** 5s browse window
- **Returns:** Scanner IP, hostname, port from SRV/TXT records

```go
// DiscoverOptions configures mDNS discovery.
type DiscoverOptions struct {
    Timeout time.Duration // default: 5s
}

// Discover finds eSCL scanners on the local network using mDNS.
func Discover(ctx context.Context, opts DiscoverOptions) ([]Scanner, error)
```

### 6.2 Subnet Sweep Fallback

When mDNS yields no results, fall back to a concurrent TCP port scan of the local /24 subnet. This mirrors the Python version's `scan_network()` but replaces sequential scanning with a goroutine pool.

```go
// SweepOptions configures the subnet sweep fallback.
type SweepOptions struct {
    Concurrency int           // default: 50
    PerHostTimeout time.Duration // default: 1s
    Port        int           // default: 443 (eSCL HTTPS) with 80 (HTTP) fallback
}

// Sweep scans the local /24 subnet for eSCL scanners.
func Sweep(ctx context.Context, opts SweepOptions) ([]Scanner, error)
```

Implementation details:

- Determine local IP via UDP dial to `8.8.8.8:80` (same approach as Python version)
- Generate /24 range excluding self
- Worker pool of `Concurrency` goroutines reading from a channel of IPs
- Each worker attempts TCP connect to port 443, then 80
- On successful connect, issue GET `/eSCL/ScannerCapabilities` to confirm eSCL support
- Collect results via channel, bounded by `sync.WaitGroup`
- Respect `ctx.Done()` for cancellation

### 6.3 Combined Discovery Flow

```
discover command
    |
    v
mDNS browse (5s timeout)
    |
    +--> results found --> return
    |
    +--> no results --> subnet sweep fallback
                            |
                            v
                        return results (may be empty)
```

---

## 7. Output (`internal/output`)

### 7.1 Functions

No interfaces. Simple functions operating on `io.Writer`.

```go
// WriteJSON marshals v as JSON and writes it to w.
func WriteJSON(w io.Writer, v any) error

// WritePretty writes v in human-readable format to w.
func WritePretty(w io.Writer, v any) error
```

`WriteJSON` uses `json.NewEncoder` with `SetIndent` for readable JSON (2-space indent). Even in JSON mode, the output is indented for debuggability; agents parse it the same either way.

`WritePretty` formats output as aligned key-value pairs for terminal consumption, similar to the Python version's `print_capabilities()`.

### 7.2 File Writing

```go
// SaveFile writes data to a file with the given base name and format.
// It returns the final filename (which may differ due to collision avoidance).
// The context allows cancellation during file I/O.
func SaveFile(ctx context.Context, data []byte, baseName string, format string) (string, error)
```

**Auto-naming:** When `baseName` is empty, generate `SCAN_YYYYMMDD_HHMMSS`.

**Collision avoidance:** Matches the Python behavior. If `output.jpg` exists, try `output_1.jpg`, `output_2.jpg`, etc.

**Format mapping:**

| `--format` | MIME type for eSCL   | File extension | Notes                          |
|------------|----------------------|----------------|--------------------------------|
| `jpeg`     | `image/jpeg`         | `.jpg`         | Native scanner output          |
| `pdf`      | `application/pdf`    | `.pdf`         | Native scanner output          |
| `png`      | `image/jpeg`         | `.png`         | Request JPEG, convert to PNG   |

PNG conversion uses `image/jpeg` (decode) and `image/png` (encode) from stdlib. The scanner does not natively produce PNG, so we request JPEG and transcode.

---

## 8. Error Handling

### 8.1 Sentinel Errors

```go
package escl

var (
    // ErrUnreachable indicates the scanner did not respond to a connection attempt.
    ErrUnreachable = errors.New("scanner unreachable")

    // ErrBusy indicates the scanner rejected the job because it is busy.
    ErrBusy = errors.New("scanner busy")

    // ErrTimeout indicates a polling or download operation exceeded its deadline.
    ErrTimeout = errors.New("operation timed out")
)

package main // or a shared internal package

var (
    // ErrInvalidArgs indicates invalid or missing command-line arguments.
    ErrInvalidArgs = errors.New("invalid arguments")
)
```

### 8.2 Exit Code Mapping

```go
func exitCode(err error) int {
    switch {
    case err == nil:
        return 0
    case errors.Is(err, escl.ErrUnreachable):
        return 2
    case errors.Is(err, escl.ErrBusy):
        return 3
    case errors.Is(err, ErrInvalidArgs):
        return 4
    default:
        return 1
    }
}
```

### 8.3 Error Flow

All errors are written to stderr as JSON (see Section 4.4). The `--verbose` flag populates the `detail` field with structured diagnostic information. Stdout is never polluted with error output.

```
main.go
    |
    v
subcommand returns error
    |
    v
writeError(os.Stderr, err, verbose)  -- JSON to stderr
    |
    v
os.Exit(exitCode(err))
```

### 8.4 Signal Handling

`main.go` sets up a `signal.NotifyContext` for `SIGINT` and `SIGTERM`. The resulting context is passed to all subcommands, so in-flight HTTP requests and polling loops terminate cleanly on Ctrl-C. No special cleanup is needed beyond context cancellation since the scanner automatically times out abandoned jobs.

---

## 9. Testing Strategy

### 9.1 Unit Tests

| Package              | What is tested                                        | Approach                               |
|----------------------|-------------------------------------------------------|----------------------------------------|
| `internal/escl`      | XML parsing of capabilities                           | Table-driven, `testdata/` fixtures     |
| `internal/escl`      | XML parsing of scanner status                         | Table-driven, `testdata/` fixtures     |
| `internal/escl`      | Scan settings XML generation                          | Marshal and compare to expected XML    |
| `internal/escl`      | Job polling logic                                     | `httptest.Server` with canned responses|
| `internal/output`    | JSON formatting                                       | Compare marshaled output               |
| `internal/output`    | Pretty formatting                                     | Golden file comparison                 |
| `internal/output`    | File collision avoidance                              | `t.TempDir()`, create conflicts        |
| `internal/output`    | PNG conversion                                        | Round-trip JPEG to PNG                 |

### 9.2 Test Fixtures

Capture real XML responses from the HP OfficeJet Pro 9010 and store in `testdata/`:

- `testdata/capabilities.xml` -- full ScannerCapabilities response
- `testdata/status_idle.xml` -- ScannerStatus with no active jobs
- `testdata/status_processing.xml` -- ScannerStatus with a Processing job

### 9.3 Integration Tests

Integration tests that communicate with a real scanner are gated behind a build tag:

```go
//go:build integration

package escl_test
```

Run with:

```bash
HPSCAN_IP=192.168.1.198 go test -tags=integration ./internal/escl/
```

These tests are skipped in CI and normal development. They exist for manual validation against the physical scanner.

### 9.4 Test Conventions

- All tests use `testing.T` and table-driven patterns per Go guidelines
- Deep comparisons use `github.com/google/go-cmp/cmp`
- No assertion libraries
- `got`-before-`want` convention: `Func(%v) = %v, want %v`
- `t.Helper()` on all test helpers
- `t.TempDir()` for file system tests (auto-cleaned)

---

## 10. Dependencies

### Runtime Dependencies

| Dependency                      | Purpose                | License | Justification                                   |
|---------------------------------|------------------------|---------|-------------------------------------------------|
| `encoding/xml` (stdlib)        | XML parsing            | BSD     | eSCL protocol uses XML                          |
| `net/http` (stdlib)            | HTTP client            | BSD     | eSCL is HTTP-based                              |
| `log/slog` (stdlib)            | Structured logging     | BSD     | Go 1.21+ standard                              |
| `image/jpeg` (stdlib)          | JPEG decoding          | BSD     | PNG conversion source format                    |
| `image/png` (stdlib)           | PNG encoding           | BSD     | PNG output format                               |
| `text/template` (stdlib)       | XML template rendering | BSD     | Scan settings XML generation (Section 5.3)      |
| `encoding/json` (stdlib)       | JSON output            | BSD     | JSON-first output contract                      |
| `github.com/hashicorp/mdns`   | mDNS/DNS-SD discovery  | MPL-2.0 | No viable stdlib alternative for mDNS           |
| `github.com/google/subcommands`| CLI subcommand dispatch| Apache-2.0 | Per Go guidelines for CLI framework          |

### Test Dependencies

| Dependency                          | Purpose            | License |
|-------------------------------------|--------------------|---------|
| `testing` (stdlib)                  | Test framework     | BSD     |
| `net/http/httptest` (stdlib)        | HTTP test servers  | BSD     |
| `github.com/google/go-cmp/cmp`     | Deep comparison    | BSD     |

### Dependency Count

**Total external runtime dependencies: 2** (`hashicorp/mdns`, `google/subcommands`).
**Total external test dependencies: 1** (`go-cmp`).

This meets the least mechanism principle. The external runtime dependencies exist because (1) mDNS/DNS-SD requires multicast UDP handling that stdlib does not provide, and (2) `google/subcommands` is the prescribed CLI framework per Go guidelines.

---

## 11. Build and Distribution

### Build

```bash
# Development build
go build -o hpscan ./cmd/hpscan

# Release build with version injection
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o hpscan ./cmd/hpscan
```

### Cross-Platform Targets

| GOOS    | GOARCH | Binary Name    |
|---------|--------|----------------|
| linux   | amd64  | `hpscan`       |
| darwin  | arm64  | `hpscan`       |
| windows | amd64  | `hpscan.exe`   |

### Makefile

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o hpscan ./cmd/hpscan

test:
	go test ./...

lint:
	gofmt -l .
	go vet ./...

clean:
	rm -f hpscan hpscan.exe

release:
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-linux-amd64       ./cmd/hpscan
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-darwin-arm64       ./cmd/hpscan
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-windows-amd64.exe  ./cmd/hpscan
```

### Version Command

```bash
$ hpscan --version
{"version": "1.0.0", "commit": "abc1234", "date": "2026-03-14T12:00:00Z"}
```

---

## 12. Differences from Python Version

| Aspect                | Python (HPScanCLI)                    | Go (hpscan)                                  |
|-----------------------|---------------------------------------|-----------------------------------------------|
| XML parsing           | BeautifulSoup string lookups          | `text/template` for XML generation; `encoding/xml` with namespace stripping for parsing |
| Output                | Unstructured print statements         | JSON to stdout, errors to stderr              |
| Discovery             | Sequential port scan, 10ms per host   | mDNS first, concurrent sweep fallback (50x)   |
| Job polling           | 5 retries, 200ms interval             | 10 retries, 500ms interval                    |
| Error handling        | Bare `except:`, `exit()` calls        | Sentinel errors, structured JSON, exit codes  |
| Distribution          | pip install + Python runtime          | Single static binary                          |
| Bulk scan             | Interactive y/n loop                  | Removed; agents call `scan` repeatedly        |
| ADF/Duplex            | Not supported (Platen only)           | Platen, ADF simplex, ADF duplex               |
| PNG output            | Not supported                         | JPEG-to-PNG conversion via stdlib             |
| Config                | CLI flags only                        | Env vars (12 Factor) with flag override       |
| Printer port check    | Port 9100 (JetDirect)                | Port 443/80 (eSCL HTTP/HTTPS)                |
| Timeout               | 10ms socket timeout                   | Configurable, 3s connect / 120s download      |

---

## 13. Design Decisions (Resolved)

1. **HTTPS support:** Default to HTTP. Scanner self-signed certs add complexity (InsecureSkipVerify) without meaningful security benefit on a trusted LAN. Add `--https` flag for users who want it, with automatic InsecureSkipVerify since scanner certs are always self-signed.

2. **ADF page count:** Loop `NextDocument` until 404 to capture all pages automatically. This is how the eSCL protocol is designed to work with ADF. The scan response JSON will include a `pages` count.

3. **Multi-page PDF from ADF:** For PDF format, output one file containing all pages (scanners handle this natively via eSCL). For JPEG/PNG format, output one file per page with `_page1`, `_page2` suffixes.

---

*Authored By Peter O'Connor with Assistance from Claude Code (claude-opus-4-6) -- 2026-03-14 -- hpscan Go rewrite design spec*
