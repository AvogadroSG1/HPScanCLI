# hpscan Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Rewrite HPScanCLI Python tool as a Go binary with JSON-first output, mDNS discovery, ADF/duplex support, and agent-friendly CLI.

**Architecture:** Flat single-package with internal packages (escl, discovery, output). text/template for XML generation, encoding/xml with namespace stripping for XML parsing. github.com/google/subcommands for CLI, github.com/hashicorp/mdns for discovery.

**Tech Stack:** Go 1.26, encoding/xml, net/http, log/slog, text/template, image/jpeg+png, hashicorp/mdns, google/subcommands, google/go-cmp

**Spec:** `docs/superpowers/specs/2026-03-14-hpscan-go-rewrite-design.md`

**Go Guidelines:** `~/.claude/guidelines/golang.md` -- all code MUST comply.

**Commit convention:** All commits include both co-authors:
```
Co-Authored-By: Peter O'Connor <avogadrosg1@gmail.com>
Co-Authored-By: Claude <noreply@anthropic.com>
```

---

## File Structure

| File | Responsibility |
|------|---------------|
| `cmd/hpscan/main.go` | Entry point, subcommand registration, global flags, signal handling, env var resolution |
| `internal/escl/types.go` | All eSCL types: ScanSettings, ScanResult, ClientOptions, sentinel errors, XML structs, response types |
| `internal/escl/xml.go` | stripNamespaces utility for XML namespace removal |
| `internal/escl/capabilities.go` | ParseCapabilities: XML -> CapabilitiesResponse |
| `internal/escl/template.go` | scanSettingsTemplate and RenderScanSettingsXML |
| `internal/escl/status.go` | ParseStatus, findProcessingJob |
| `internal/escl/client.go` | Client struct, New constructor, Capabilities/Status HTTP methods |
| `internal/escl/scan.go` | submitJob, pollJob, downloadDocument, Scan orchestration |
| `internal/discovery/types.go` | Scanner struct, DiscoverOptions, SweepOptions |
| `internal/discovery/mdns.go` | Discover via mDNS/DNS-SD |
| `internal/discovery/sweep.go` | Sweep via concurrent subnet scan |
| `internal/output/types.go` | JSON response types: ScanResponse, ScanSettingsResponse, ErrorResponse, ErrorBody, DiscoverResponse |
| `internal/output/format.go` | WriteJSON, WritePretty, WriteError |
| `internal/output/file.go` | SaveFile with collision avoidance, convertToPNG |
| `Makefile` | Build, test, lint, release targets |
| `.gitignore` | Go project ignores |
| `testdata/capabilities.xml` | Real scanner fixture (already captured) |
| `testdata/status_idle.xml` | Real scanner fixture (already captured) |
| `testdata/status_processing.xml` | Synthetic fixture for testing |

---

## Chunk 1: Project Scaffold and Core Types

### Task 1: Initialize Go Module and Project Structure

**Files:**
- Create: `go.mod`, `.gitignore`, `Makefile`
- Create: `cmd/hpscan/`, `internal/escl/`, `internal/discovery/`, `internal/output/` (directories)

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd C:/Users/avoga/ObsidianNotes/code/hpscan
go mod init github.com/AvogadroSG1/HPScanCLI
```

- [ ] **Step 2: Create .gitignore**

```gitignore
# Binaries
hpscan
hpscan.exe
*.exe
*.dll
*.so
*.dylib
dist/

# Test
*.test
*.out
coverage.html

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Environment
.env
.env.local
```

- [ ] **Step 3: Create Makefile**

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test lint clean release

build:
	go build -ldflags "$(LDFLAGS)" -o hpscan ./cmd/hpscan

test:
	go test ./...

lint:
	gofmt -l .
	go vet ./...

clean:
	rm -f hpscan hpscan.exe
	rm -rf dist/

release:
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-linux-amd64       ./cmd/hpscan
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-darwin-arm64       ./cmd/hpscan
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hpscan-windows-amd64.exe  ./cmd/hpscan
```

- [ ] **Step 4: Create directory structure**

```bash
mkdir -p cmd/hpscan internal/escl internal/discovery internal/output
```

- [ ] **Step 5: Create placeholder main.go so module compiles**

File: `cmd/hpscan/main.go`
```go
// Package main is the entry point for the hpscan CLI.
package main

func main() {}
```

- [ ] **Step 6: Verify build**

Run: `go build ./cmd/hpscan`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add go.mod .gitignore Makefile cmd/hpscan/main.go
git commit -m "Initialize Go module and project scaffold"
```

---

### Task 2: Define Core Types (internal/escl/types.go)

**Files:**
- Create: `internal/escl/types.go`

- [ ] **Step 1: Write types.go with all eSCL types**

File: `internal/escl/types.go`
```go
// Package escl implements an eSCL (Embedded Scan Control Language) client
// for communicating with HP network scanners over HTTP.
package escl

import (
	"errors"
	"log/slog"
	"time"
)

// Sentinel errors for programmatic inspection.
var (
	// ErrUnreachable indicates the scanner did not respond to a connection attempt.
	ErrUnreachable = errors.New("scanner unreachable")

	// ErrBusy indicates the scanner rejected the job because it is busy.
	ErrBusy = errors.New("scanner busy")

	// ErrTimeout indicates a polling or download operation exceeded its deadline.
	ErrTimeout = errors.New("operation timed out")
)

// ClientOptions configures the eSCL client.
type ClientOptions struct {
	ConnectTimeout  time.Duration // default: 3s
	DownloadTimeout time.Duration // default: 120s
	Logger          *slog.Logger
}

// ScanSettings configures a scan job. This is an internal config struct
// with no JSON tags; the JSON output contract uses output.ScanSettingsResponse.
type ScanSettings struct {
	Height            int    // scanner units; 0 means max
	Width             int    // scanner units; 0 means max
	XResolution       int    // DPI
	YResolution       int    // DPI
	ColorMode         string // e.g., "RGB24", "Grayscale8"
	DocumentFormat    string // MIME type: "image/jpeg", "application/pdf"
	InputSource       string // "Platen", "Adf"
	Duplex            bool
	Brightness        int // default: 1000
	Contrast          int // default: 1000
	CompressionFactor int // default: 0
}

// ScanResult holds the raw scan output before file writing.
type ScanResult struct {
	Data        []byte
	ContentType string
	Settings    ScanSettings
}

// CapabilitiesResponse is the parsed scanner capabilities.
type CapabilitiesResponse struct {
	MakeAndModel    string       `json:"make_and_model"`
	SerialNumber    string       `json:"serial_number"`
	Manufacturer    string       `json:"manufacturer"`
	FirmwareVersion string       `json:"firmware_version"`
	Platen          *InputSource `json:"platen"`
	Adf             *InputSource `json:"adf"`
	AdfDuplex       *InputSource `json:"adf_duplex,omitempty"`
}

// InputSource describes the capabilities of a single input source (platen, ADF, etc.).
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

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/escl/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/escl/types.go
git commit -m "Add core eSCL types, sentinel errors, and response structs"
```

---

### Task 3: XML Namespace Stripping (internal/escl/xml.go)

**Files:**
- Create: `internal/escl/xml.go`
- Test: `internal/escl/xml_test.go`

- [ ] **Step 1: Write failing test**

File: `internal/escl/xml_test.go`
```go
package escl

import (
	"os"
	"strings"
	"testing"
)

func TestStripNamespaces(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "simple prefixed element",
			input:    `<scan:Foo>bar</scan:Foo>`,
			contains: []string{"<Foo>bar</Foo>"},
			excludes: []string{"scan:"},
		},
		{
			name:     "multiple prefixes",
			input:    `<scan:A><pwg:B>val</pwg:B></scan:A>`,
			contains: []string{"<A>", "<B>val</B>", "</A>"},
			excludes: []string{"scan:", "pwg:"},
		},
		{
			name:     "xmlns declarations removed",
			input:    `<scan:Root xmlns:scan="http://example.com" xmlns:pwg="http://example.org"><scan:Child/></scan:Root>`,
			contains: []string{"<Root", "<Child/>"},
			excludes: []string{"xmlns:scan", "xmlns:pwg"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripNamespaces([]byte(tt.input)))
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("stripNamespaces(%q) missing %q, got %q", tt.input, s, got)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("stripNamespaces(%q) should not contain %q, got %q", tt.input, s, got)
				}
			}
		})
	}
}

func TestStripNamespacesRealFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/capabilities.xml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	got := string(stripNamespaces(data))
	if strings.Contains(got, "scan:") || strings.Contains(got, "pwg:") {
		t.Error("stripped XML still contains namespace prefixes")
	}
	for _, elem := range []string{"<ScannerCapabilities", "<MakeAndModel>", "<PlatenInputCaps>", "<AdfSimplexInputCaps>"} {
		if !strings.Contains(got, elem) {
			t.Errorf("stripped XML missing element %q", elem)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/escl/ -run TestStripNamespaces -v`
Expected: FAIL (function not defined)

- [ ] **Step 3: Implement stripNamespaces**

File: `internal/escl/xml.go`
```go
package escl

import "regexp"

var (
	// nsPrefix matches namespace prefixes in element names: <prefix:Name or </prefix:Name
	nsPrefix = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*):`)
	// nsDecl matches xmlns:prefix="..." declarations
	nsDecl = regexp.MustCompile(`\s+xmlns:[a-zA-Z][a-zA-Z0-9]*="[^"]*"`)
	// xsiAttr matches xsi:schemaLocation and similar prefixed attributes
	xsiAttr = regexp.MustCompile(`\s+[a-zA-Z][a-zA-Z0-9]*:[a-zA-Z]+=("[^"]*"|'[^']*')`)
)

// stripNamespaces removes XML namespace prefixes from element names and
// strips xmlns declarations, allowing encoding/xml to unmarshal with
// simple struct tags.
func stripNamespaces(data []byte) []byte {
	result := nsPrefix.ReplaceAll(data, []byte("<$1"))
	result = nsDecl.ReplaceAll(result, nil)
	result = xsiAttr.ReplaceAll(result, nil)
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/escl/ -run TestStripNamespaces -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/escl/xml.go internal/escl/xml_test.go
git commit -m "Add XML namespace stripping utility for eSCL parsing"
```

---

## Chunk 2: eSCL Capabilities and Template

### Task 4: Capabilities XML Parsing (internal/escl/capabilities.go)

**Files:**
- Create: `internal/escl/capabilities.go`
- Test: `internal/escl/capabilities_test.go`

- [ ] **Step 1: Write failing test using real fixture**

File: `internal/escl/capabilities_test.go`
```go
package escl

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func mustReadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %q: %v", path, err)
	}
	return data
}

func TestParseCapabilities(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/capabilities.xml")
	got, err := ParseCapabilities(data)
	if err != nil {
		t.Fatalf("ParseCapabilities() error = %v", err)
	}

	// Top-level fields
	if got.MakeAndModel != "HP OfficeJet Pro 9010 series" {
		t.Errorf("MakeAndModel = %q, want %q", got.MakeAndModel, "HP OfficeJet Pro 9010 series")
	}
	if got.SerialNumber != "TH18G3737H" {
		t.Errorf("SerialNumber = %q, want %q", got.SerialNumber, "TH18G3737H")
	}
	if got.Manufacturer != "HP" {
		t.Errorf("Manufacturer = %q, want %q", got.Manufacturer, "HP")
	}
	if got.FirmwareVersion != "2.63" {
		t.Errorf("FirmwareVersion = %q, want %q", got.FirmwareVersion, "2.63")
	}

	// Platen
	if got.Platen == nil {
		t.Fatal("Platen is nil")
	}
	wantPlaten := &InputSource{
		MinWidth:        8,
		MaxWidth:        2550,
		MinHeight:       8,
		MaxHeight:       3534,
		ColorModes:      []string{"BlackAndWhite1", "Grayscale8", "RGB24"},
		DocumentFormats: []string{"application/octet-stream", "image/jpeg", "application/pdf"},
		Resolutions:     []int{75, 100, 150, 200, 300, 400, 600, 1200},
	}
	if diff := cmp.Diff(wantPlaten, got.Platen); diff != "" {
		t.Errorf("Platen mismatch (-want +got):\n%s", diff)
	}

	// ADF
	if got.Adf == nil {
		t.Fatal("Adf is nil")
	}
	if got.Adf.MaxHeight != 4200 {
		t.Errorf("Adf.MaxHeight = %d, want %d", got.Adf.MaxHeight, 4200)
	}
	wantAdfRes := []int{75, 100, 150, 200, 300}
	if diff := cmp.Diff(wantAdfRes, got.Adf.Resolutions); diff != "" {
		t.Errorf("Adf.Resolutions mismatch (-want +got):\n%s", diff)
	}

	// ADF Duplex
	if got.AdfDuplex == nil {
		t.Fatal("AdfDuplex is nil")
	}
	if got.AdfDuplex.MaxHeight != 3508 {
		t.Errorf("AdfDuplex.MaxHeight = %d, want %d", got.AdfDuplex.MaxHeight, 3508)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/escl/ -run TestParseCapabilities -v`
Expected: FAIL (ParseCapabilities not defined)

- [ ] **Step 3: Add go-cmp dependency**

Run: `go get github.com/google/go-cmp/cmp`

- [ ] **Step 4: Implement ParseCapabilities**

File: `internal/escl/capabilities.go`
```go
package escl

import "encoding/xml"

// XML types for parsing scanner capabilities response.
// These use simple element names because stripNamespaces removes prefixes.

type capabilitiesXML struct {
	XMLName      xml.Name  `xml:"ScannerCapabilities"`
	Version      string    `xml:"Version"`
	MakeAndModel string    `xml:"MakeAndModel"`
	SerialNumber string    `xml:"SerialNumber"`
	Manufacturer string    `xml:"Manufacturer"`
	Platen       *platenXML `xml:"Platen"`
	Adf          *adfXML    `xml:"Adf"`
}

type platenXML struct {
	InputCaps inputCapsXML `xml:"PlatenInputCaps"`
}

type adfXML struct {
	SimplexCaps *inputCapsXML `xml:"AdfSimplexInputCaps"`
	DuplexCaps  *inputCapsXML `xml:"AdfDuplexInputCaps"`
}

type inputCapsXML struct {
	MinWidth  int                `xml:"MinWidth"`
	MaxWidth  int                `xml:"MaxWidth"`
	MinHeight int                `xml:"MinHeight"`
	MaxHeight int                `xml:"MaxHeight"`
	Profiles  settingProfilesXML `xml:"SettingProfiles"`
}

type settingProfilesXML struct {
	Profiles []settingProfileXML `xml:"SettingProfile"`
}

type settingProfileXML struct {
	ColorModes  colorModesXML  `xml:"ColorModes"`
	DocFormats  docFormatsXML  `xml:"DocumentFormats"`
	Resolutions resolutionsXML `xml:"SupportedResolutions"`
}

type colorModesXML struct {
	Modes []string `xml:"ColorMode"`
}

type docFormatsXML struct {
	Formats    []string `xml:"DocumentFormat"`
	FormatsExt []string `xml:"DocumentFormatExt"`
}

type resolutionsXML struct {
	Discrete discreteResolutionsXML `xml:"DiscreteResolutions"`
}

type discreteResolutionsXML struct {
	Resolutions []discreteResXML `xml:"DiscreteResolution"`
}

type discreteResXML struct {
	XResolution int `xml:"XResolution"`
	YResolution int `xml:"YResolution"`
}

// ParseCapabilities parses raw eSCL ScannerCapabilities XML into a CapabilitiesResponse.
func ParseCapabilities(data []byte) (*CapabilitiesResponse, error) {
	stripped := stripNamespaces(data)
	var raw capabilitiesXML
	if err := xml.Unmarshal(stripped, &raw); err != nil {
		return nil, fmt.Errorf("parsing capabilities XML: %w", err)
	}
	resp := &CapabilitiesResponse{
		MakeAndModel:    raw.MakeAndModel,
		SerialNumber:    raw.SerialNumber,
		Manufacturer:    raw.Manufacturer,
		FirmwareVersion: raw.Version,
	}
	if raw.Platen != nil {
		resp.Platen = convertInputCaps(&raw.Platen.InputCaps)
	}
	if raw.Adf != nil {
		if raw.Adf.SimplexCaps != nil {
			resp.Adf = convertInputCaps(raw.Adf.SimplexCaps)
		}
		if raw.Adf.DuplexCaps != nil {
			resp.AdfDuplex = convertInputCaps(raw.Adf.DuplexCaps)
		}
	}
	return resp, nil
}

func convertInputCaps(caps *inputCapsXML) *InputSource {
	src := &InputSource{
		MinWidth:  caps.MinWidth,
		MaxWidth:  caps.MaxWidth,
		MinHeight: caps.MinHeight,
		MaxHeight: caps.MaxHeight,
	}
	// Merge all profiles (typically just one)
	for _, p := range caps.Profiles.Profiles {
		src.ColorModes = append(src.ColorModes, p.ColorModes.Modes...)
		// Prefer DocumentFormatExt over DocumentFormat
		if len(p.DocFormats.FormatsExt) > 0 {
			src.DocumentFormats = append(src.DocumentFormats, p.DocFormats.FormatsExt...)
		} else {
			src.DocumentFormats = append(src.DocumentFormats, p.DocFormats.Formats...)
		}
		for _, r := range p.Resolutions.Discrete.Resolutions {
			src.Resolutions = append(src.Resolutions, r.XResolution)
		}
	}
	return src
}
```

Note: add `"fmt"` to the import block.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/escl/ -run TestParseCapabilities -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/escl/capabilities.go internal/escl/capabilities_test.go go.mod go.sum
git commit -m "Add eSCL capabilities XML parsing with real fixture tests"
```

---

### Task 5: Scan Settings XML Template (internal/escl/template.go)

**Files:**
- Create: `internal/escl/template.go`
- Test: `internal/escl/template_test.go`

- [ ] **Step 1: Write failing test**

File: `internal/escl/template_test.go`
```go
package escl

import (
	"strings"
	"testing"
)

func TestRenderScanSettingsXML(t *testing.T) {
	settings := ScanSettings{
		Height:         3534,
		Width:          2550,
		XResolution:    300,
		YResolution:    300,
		ColorMode:      "RGB24",
		DocumentFormat: "image/jpeg",
		InputSource:    "Platen",
		Brightness:     1000,
		Contrast:       1000,
	}

	got, err := RenderScanSettingsXML(settings)
	if err != nil {
		t.Fatalf("RenderScanSettingsXML() error = %v", err)
	}

	xml := string(got)
	checks := []string{
		`<pwg:Height>3534</pwg:Height>`,
		`<pwg:Width>2550</pwg:Width>`,
		`<scan:XResolution>300</scan:XResolution>`,
		`<scan:YResolution>300</scan:YResolution>`,
		`<scan:ColorMode>RGB24</scan:ColorMode>`,
		`<scan:DocumentFormatExt>image/jpeg</scan:DocumentFormatExt>`,
		`<pwg:InputSource>Platen</pwg:InputSource>`,
		`<scan:Brightness>1000</scan:Brightness>`,
		`<scan:Contrast>1000</scan:Contrast>`,
		`xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03"`,
		`xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm"`,
	}
	for _, want := range checks {
		if !strings.Contains(xml, want) {
			t.Errorf("rendered XML missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/escl/ -run TestRenderScanSettingsXML -v`
Expected: FAIL

- [ ] **Step 3: Implement template**

File: `internal/escl/template.go`
```go
package escl

import (
	"bytes"
	"fmt"
	"text/template"
)

var scanSettingsTemplate = template.Must(template.New("scanSettings").Parse(
	`<?xml version="1.0" encoding="UTF-8"?>
<scan:ScanSettings xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03" xmlns:copy="http://www.hp.com/schemas/imaging/con/copy/2008/07/07" xmlns:dd="http://www.hp.com/schemas/imaging/con/dictionaries/1.0/" xmlns:dd3="http://www.hp.com/schemas/imaging/con/dictionaries/2009/04/06" xmlns:fw="http://www.hp.com/schemas/imaging/con/firewall/2011/01/05" xmlns:scc="http://schemas.hp.com/imaging/escl/2011/05/03" xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm">
	<pwg:Version>2.1</pwg:Version>
	<scan:Intent>Photo</scan:Intent>
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

- [ ] **Step 4: Run tests**

Run: `go test ./internal/escl/ -run TestRenderScanSettingsXML -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/escl/template.go internal/escl/template_test.go
git commit -m "Add scan settings XML template rendering"
```

---

### Task 6: Scanner Status XML Parsing (internal/escl/status.go)

**Files:**
- Create: `internal/escl/status.go`
- Create: `testdata/status_processing.xml`
- Test: `internal/escl/status_test.go`

- [ ] **Step 1: Create synthetic processing fixture**

File: `testdata/status_processing.xml` -- copy status_idle.xml and change one job to Processing state, add a different JobUri.

- [ ] **Step 2: Write failing test**

File: `internal/escl/status_test.go`
```go
package escl

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		wantState string
		wantJobs  int
	}{
		{
			name:      "idle with completed jobs",
			fixture:   "../../testdata/status_idle.xml",
			wantState: "Idle",
			wantJobs:  2,
		},
		{
			name:      "processing job",
			fixture:   "../../testdata/status_processing.xml",
			wantState: "Processing",
			wantJobs:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustReadFixture(t, tt.fixture)
			got, err := ParseStatus(data)
			if err != nil {
				t.Fatalf("ParseStatus() error = %v", err)
			}
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if len(got.Jobs) != tt.wantJobs {
				t.Errorf("len(Jobs) = %d, want %d", len(got.Jobs), tt.wantJobs)
			}
		})
	}
}

func TestFindProcessingJobURI(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/status_processing.xml")
	status, err := ParseStatus(data)
	if err != nil {
		t.Fatalf("ParseStatus() error = %v", err)
	}
	got := findProcessingJobURI(status)
	if got == "" {
		t.Error("findProcessingJobURI() returned empty string, want a job URI")
	}
}

func TestFindProcessingJobURI_NoProcessing(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/status_idle.xml")
	status, err := ParseStatus(data)
	if err != nil {
		t.Fatalf("ParseStatus() error = %v", err)
	}
	got := findProcessingJobURI(status)
	if got != "" {
		t.Errorf("findProcessingJobURI() = %q, want empty string", got)
	}
}
```

- [ ] **Step 3: Implement ParseStatus and types**

File: `internal/escl/status.go`
```go
package escl

import (
	"encoding/xml"
	"fmt"
)

// StatusResponse is the parsed scanner status.
type StatusResponse struct {
	State string
	Jobs  []JobInfo
}

// JobInfo describes a scan job from the scanner's status.
type JobInfo struct {
	URI   string
	UUID  string
	State string
}

type statusXML struct {
	XMLName xml.Name     `xml:"ScannerStatus"`
	State   string       `xml:"State"`
	Jobs    jobsXML      `xml:"Jobs"`
}

type jobsXML struct {
	Jobs []jobInfoXML `xml:"JobInfo"`
}

type jobInfoXML struct {
	URI   string `xml:"JobUri"`
	UUID  string `xml:"JobUuid"`
	State string `xml:"JobState"`
}

// ParseStatus parses raw eSCL ScannerStatus XML into a StatusResponse.
func ParseStatus(data []byte) (*StatusResponse, error) {
	stripped := stripNamespaces(data)
	var raw statusXML
	if err := xml.Unmarshal(stripped, &raw); err != nil {
		return nil, fmt.Errorf("parsing status XML: %w", err)
	}
	resp := &StatusResponse{State: raw.State}
	for _, j := range raw.Jobs.Jobs {
		resp.Jobs = append(resp.Jobs, JobInfo{
			URI:   j.URI,
			UUID:  j.UUID,
			State: j.State,
		})
	}
	return resp, nil
}

// findProcessingJobURI returns the URI of the first job in Processing state,
// or empty string if none found.
func findProcessingJobURI(status *StatusResponse) string {
	for _, j := range status.Jobs {
		if j.State == "Processing" {
			return j.URI
		}
	}
	return ""
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/escl/ -run TestParseStatus -v && go test ./internal/escl/ -run TestFindProcessingJob -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/escl/status.go internal/escl/status_test.go testdata/status_processing.xml
git commit -m "Add scanner status XML parsing and job URI lookup"
```

---

## Chunk 3: eSCL HTTP Client

### Task 7: HTTP Client (internal/escl/client.go)

**Files:**
- Create: `internal/escl/client.go`
- Test: `internal/escl/client_test.go`

- [ ] **Step 1: Write failing test**

File: `internal/escl/client_test.go`
```go
package escl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClientCapabilities(t *testing.T) {
	data, err := os.ReadFile("../../testdata/capabilities.xml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eSCL/ScannerCapabilities" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write(data)
	}))
	defer srv.Close()

	c, err := New(context.Background(), srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	caps, err := c.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities() error = %v", err)
	}
	if caps.MakeAndModel != "HP OfficeJet Pro 9010 series" {
		t.Errorf("MakeAndModel = %q, want %q", caps.MakeAndModel, "HP OfficeJet Pro 9010 series")
	}
}

func TestClientStatus(t *testing.T) {
	data, err := os.ReadFile("../../testdata/status_idle.xml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eSCL/ScannerStatus" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write(data)
	}))
	defer srv.Close()

	c, err := New(context.Background(), srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.State != "Idle" {
		t.Errorf("State = %q, want %q", status.State, "Idle")
	}
}
```

- [ ] **Step 2: Implement client**

File: `internal/escl/client.go`
```go
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

const (
	defaultConnectTimeout  = 3 * time.Second
	defaultDownloadTimeout = 120 * time.Second
)

// Client communicates with an eSCL-compatible scanner over HTTP.
type Client struct {
	hc      *http.Client
	baseURL string
	log     *slog.Logger
	dlTimeout time.Duration
}

// New creates a new eSCL client for the scanner at baseURL.
func New(ctx context.Context, baseURL string, opts ClientOptions) (*Client, error) {
	connectTimeout := opts.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = defaultConnectTimeout
	}
	dlTimeout := opts.DownloadTimeout
	if dlTimeout == 0 {
		dlTimeout = defaultDownloadTimeout
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // scanner certs are always self-signed
		},
		ResponseHeaderTimeout: 10 * time.Second,
	}
	return &Client{
		hc: &http.Client{
			Transport: transport,
			Timeout:   connectTimeout,
		},
		baseURL:   baseURL,
		log:       logger,
		dlTimeout: dlTimeout,
	}, nil
}

// Capabilities retrieves the scanner's capabilities.
func (c *Client) Capabilities(ctx context.Context) (*CapabilitiesResponse, error) {
	url := c.baseURL + "/eSCL/ScannerCapabilities"
	c.log.Debug("fetching capabilities", "url", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching capabilities: %w", ErrUnreachable)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading capabilities response: %w", err)
	}
	return ParseCapabilities(body)
}

// Status retrieves the scanner's current status and active jobs.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	url := c.baseURL + "/eSCL/ScannerStatus"
	c.log.Debug("fetching status", "url", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching status: %w", ErrUnreachable)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading status response: %w", err)
	}
	return ParseStatus(body)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/escl/ -run TestClient -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/escl/client.go internal/escl/client_test.go
git commit -m "Add eSCL HTTP client with capabilities and status methods"
```

---

### Task 8: Scan Workflow (internal/escl/scan.go)

**Files:**
- Create: `internal/escl/scan.go`
- Test: `internal/escl/scan_test.go`

- [ ] **Step 1: Write failing test for full scan workflow**

File: `internal/escl/scan_test.go`
```go
package escl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClientScan(t *testing.T) {
	capData, err := os.ReadFile("../../testdata/capabilities.xml")
	if err != nil {
		t.Fatalf("reading capabilities fixture: %v", err)
	}
	scanContent := []byte("fake-jpeg-content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScannerCapabilities":
			w.Write(capData)
		case r.Method == http.MethodPost && r.URL.Path == "/eSCL/ScanJobs":
			w.Header().Set("Location", "/eSCL/ScanJobs/test-job-123")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScanJobs/test-job-123/NextDocument":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(scanContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := New(context.Background(), srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := c.Scan(context.Background(), ScanSettings{
		Height:         3534,
		Width:          2550,
		XResolution:    300,
		YResolution:    300,
		ColorMode:      "RGB24",
		DocumentFormat: "image/jpeg",
		InputSource:    "Platen",
		Brightness:     1000,
		Contrast:       1000,
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if string(result.Data) != string(scanContent) {
		t.Errorf("Data = %q, want %q", result.Data, scanContent)
	}
	if result.ContentType != "image/jpeg" {
		t.Errorf("ContentType = %q, want %q", result.ContentType, "image/jpeg")
	}
}

func TestClientScanPollFallback(t *testing.T) {
	capData, _ := os.ReadFile("../../testdata/capabilities.xml")
	statusData, _ := os.ReadFile("../../testdata/status_processing.xml")
	scanContent := []byte("fake-pdf-content")
	postCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScannerCapabilities":
			w.Write(capData)
		case r.Method == http.MethodPost && r.URL.Path == "/eSCL/ScanJobs":
			postCalled = true
			// No Location header -- force polling fallback
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScannerStatus":
			w.Write(statusData)
		case r.Method == http.MethodGet && r.URL.Path == "/NextDocument":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(scanContent)
		default:
			// The job URI from status_processing.xml + /NextDocument
			if r.Method == http.MethodGet && len(r.URL.Path) > 10 {
				w.Header().Set("Content-Type", "application/pdf")
				w.Write(scanContent)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := New(context.Background(), srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := c.Scan(context.Background(), ScanSettings{
		Height:         3534,
		Width:          2550,
		XResolution:    300,
		YResolution:    300,
		ColorMode:      "RGB24",
		DocumentFormat: "application/pdf",
		InputSource:    "Platen",
		Brightness:     1000,
		Contrast:       1000,
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !postCalled {
		t.Error("POST to ScanJobs was not called")
	}
	if len(result.Data) == 0 {
		t.Error("result.Data is empty")
	}
}
```

- [ ] **Step 2: Implement scan.go**

File: `internal/escl/scan.go`
```go
package escl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxPollRetries = 10
	pollInterval   = 500 * time.Millisecond
)

// Scan submits a scan job, waits for completion, and downloads the result.
func (c *Client) Scan(ctx context.Context, settings ScanSettings) (*ScanResult, error) {
	jobURI, err := c.submitJob(ctx, settings)
	if err != nil {
		return nil, err
	}
	c.log.Debug("scan job submitted", "job_uri", jobURI)

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

func (c *Client) submitJob(ctx context.Context, settings ScanSettings) (string, error) {
	body, err := RenderScanSettingsXML(settings)
	if err != nil {
		return "", err
	}
	url := c.baseURL + "/eSCL/ScanJobs"
	c.log.Debug("posting scan job", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating scan request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml")

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("posting scan job: %w", ErrUnreachable)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return "", fmt.Errorf("scanner rejected job: %w", ErrBusy)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status %d from ScanJobs", resp.StatusCode)
	}

	// Primary: use Location header
	if loc := resp.Header.Get("Location"); loc != "" {
		return loc, nil
	}

	// Fallback: poll scanner status
	return c.pollJob(ctx)
}

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
			return uri, nil
		}
	}
	return "", fmt.Errorf("no processing job found after %d attempts: %w", maxPollRetries, ErrTimeout)
}

func (c *Client) downloadDocument(ctx context.Context, jobURI string) ([]byte, string, error) {
	url := c.baseURL + jobURI + "/NextDocument"
	c.log.Debug("downloading document", "url", url)

	dlCtx, cancel := context.WithTimeout(ctx, c.dlTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating download request: %w", err)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("downloading document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading document: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")
	return data, contentType, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/escl/ -run TestClientScan -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/escl/scan.go internal/escl/scan_test.go
git commit -m "Add scan workflow with job submission, polling fallback, and document download"
```

---

## Chunk 4: Output Package

### Task 9: JSON and Pretty Output (internal/output/format.go)

**Files:**
- Create: `internal/output/types.go`
- Create: `internal/output/format.go`
- Test: `internal/output/format_test.go`

- [ ] **Step 1: Write output types**

File: `internal/output/types.go`
```go
// Package output handles JSON formatting, pretty printing, and file writing.
package output

// ScanResponse is the JSON output for a completed scan.
type ScanResponse struct {
	File      string               `json:"file"`
	Format    string               `json:"format"`
	SizeBytes int64                `json:"size_bytes"`
	Settings  ScanSettingsResponse `json:"settings"`
}

// ScanSettingsResponse is the JSON output for scan settings.
type ScanSettingsResponse struct {
	DPI            int    `json:"dpi"`
	Height         int    `json:"height"`
	Width          int    `json:"width"`
	ColorMode      string `json:"color_mode"`
	Source         string `json:"source"`
	DocumentFormat string `json:"document_format"`
}

// DiscoverResponse is the JSON output for scanner discovery.
type DiscoverResponse struct {
	Scanners []Scanner `json:"scanners"`
}

// Scanner describes a discovered scanner.
type Scanner struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Source   string `json:"source"`
}

// ErrorResponse is the JSON error output.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the error detail.
type ErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// VersionResponse is the JSON output for --version.
type VersionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}
```

- [ ] **Step 2: Write failing test**

File: `internal/output/format_test.go`
```go
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	resp := DiscoverResponse{
		Scanners: []Scanner{{IP: "192.168.1.198", Hostname: "HP 9010", Port: 80, Protocol: "eSCL", Source: "mdns"}},
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, resp); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	// Verify valid JSON
	var decoded DiscoverResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if decoded.Scanners[0].IP != "192.168.1.198" {
		t.Errorf("IP = %q, want %q", decoded.Scanners[0].IP, "192.168.1.198")
	}
	// Verify indented
	if !strings.Contains(buf.String(), "\n") {
		t.Error("output is not indented")
	}
}

func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, 2, "scanner unreachable at 192.168.1.198", nil)
	var decoded ErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Error.Code != 2 {
		t.Errorf("Code = %d, want %d", decoded.Error.Code, 2)
	}
}

func TestWriteErrorVerbose(t *testing.T) {
	var buf bytes.Buffer
	detail := map[string]any{"ip": "192.168.1.198", "port": 443}
	WriteError(&buf, 2, "scanner unreachable", detail)
	var decoded ErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Error.Detail == nil {
		t.Error("Detail is nil, want populated")
	}
}
```

- [ ] **Step 3: Implement format.go**

File: `internal/output/format.go`
```go
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON marshals v as indented JSON and writes it to w.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WritePretty writes v in human-readable format to w.
func WritePretty(w io.Writer, v any) error {
	switch val := v.(type) {
	case *DiscoverResponse:
		return writePrettyDiscover(w, val)
	case DiscoverResponse:
		return writePrettyDiscover(w, &val)
	default:
		// Fall back to indented JSON for types without custom formatting
		return WriteJSON(w, v)
	}
}

func writePrettyDiscover(w io.Writer, resp *DiscoverResponse) error {
	if len(resp.Scanners) == 0 {
		_, err := fmt.Fprintln(w, "No scanners found.")
		return err
	}
	for _, s := range resp.Scanners {
		_, err := fmt.Fprintf(w, "%-16s  %s  (port %d, %s, via %s)\n", s.IP, s.Hostname, s.Port, s.Protocol, s.Source)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteError writes a JSON error to w.
func WriteError(w io.Writer, code int, message string, detail any) {
	resp := ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
	}
	WriteJSON(w, resp)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/output/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/types.go internal/output/format.go internal/output/format_test.go
git commit -m "Add JSON and pretty output formatting with error support"
```

---

### Task 10: File Writing and PNG Conversion (internal/output/file.go)

**Files:**
- Create: `internal/output/file.go`
- Test: `internal/output/file_test.go`

- [ ] **Step 1: Write failing test**

File: `internal/output/file_test.go`
```go
package output

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveFileAutoName(t *testing.T) {
	dir := t.TempDir()
	name, err := SaveFile(context.Background(), []byte("test"), "", "jpeg", dir)
	if err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	if !strings.HasPrefix(filepath.Base(name), "SCAN_") {
		t.Errorf("auto-name %q does not start with SCAN_", name)
	}
	if !strings.HasSuffix(name, ".jpg") {
		t.Errorf("auto-name %q does not end with .jpg", name)
	}
}

func TestSaveFileCollisionAvoidance(t *testing.T) {
	dir := t.TempDir()
	// Create existing file
	existing := filepath.Join(dir, "test.jpg")
	os.WriteFile(existing, []byte("existing"), 0644)

	name, err := SaveFile(context.Background(), []byte("new"), "test", "jpeg", dir)
	if err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	if name == existing {
		t.Error("SaveFile should have avoided collision")
	}
	if !strings.Contains(filepath.Base(name), "test_1") {
		t.Errorf("collision-avoided name %q does not contain test_1", name)
	}
}

func TestConvertToPNG(t *testing.T) {
	// Create a minimal JPEG
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, nil); err != nil {
		t.Fatalf("encoding test JPEG: %v", err)
	}

	pngData, err := ConvertToPNG(jpegBuf.Bytes())
	if err != nil {
		t.Fatalf("ConvertToPNG() error = %v", err)
	}
	// PNG magic bytes
	if len(pngData) < 8 || string(pngData[:4]) != "\x89PNG" {
		t.Error("output is not a valid PNG")
	}
}
```

- [ ] **Step 2: Implement file.go**

File: `internal/output/file.go`
```go
package output

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// FormatExtension maps output format names to file extensions.
var FormatExtension = map[string]string{
	"jpeg": ".jpg",
	"pdf":  ".pdf",
	"png":  ".png",
}

// FormatMIME maps output format names to MIME types for the eSCL request.
var FormatMIME = map[string]string{
	"jpeg": "image/jpeg",
	"pdf":  "application/pdf",
	"png":  "image/jpeg", // request JPEG, convert to PNG
}

// SaveFile writes data to a file with collision avoidance.
// If baseName is empty, a timestamped name is generated.
// The dir parameter specifies the output directory.
// Returns the final file path.
func SaveFile(_ context.Context, data []byte, baseName string, format string, dir string) (string, error) {
	if baseName == "" {
		baseName = time.Now().Format("SCAN_20060102_150405")
	}
	ext := FormatExtension[format]
	if ext == "" {
		ext = "." + format
	}
	name := filepath.Join(dir, baseName+ext)
	suffix := 1
	for {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			break
		}
		name = filepath.Join(dir, fmt.Sprintf("%s_%d%s", baseName, suffix, ext))
		suffix++
	}
	if err := os.WriteFile(name, data, 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}
	return name, nil
}

// ConvertToPNG decodes JPEG data and re-encodes as PNG.
func ConvertToPNG(jpegData []byte) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("decoding JPEG for PNG conversion: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encoding PNG: %w", err)
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/output/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/output/file.go internal/output/file_test.go
git commit -m "Add file writing with collision avoidance and PNG conversion"
```

---

## Chunk 5: Discovery

### Task 11: mDNS Discovery (internal/discovery/mdns.go)

**Files:**
- Create: `internal/discovery/types.go`
- Create: `internal/discovery/mdns.go`
- Test: `internal/discovery/mdns_test.go`

- [ ] **Step 1: Add hashicorp/mdns dependency**

Run: `go get github.com/hashicorp/mdns`

- [ ] **Step 2: Write types**

File: `internal/discovery/types.go`
```go
// Package discovery finds eSCL scanners on the local network.
package discovery

import "time"

// Scanner describes a discovered scanner.
type Scanner struct {
	IP       string
	Hostname string
	Port     int
	Protocol string
	Source   string // "mdns" or "sweep"
}

// DiscoverOptions configures mDNS discovery.
type DiscoverOptions struct {
	Timeout time.Duration // default: 5s
}

// SweepOptions configures the subnet sweep fallback.
type SweepOptions struct {
	Concurrency    int           // default: 50
	PerHostTimeout time.Duration // default: 1s
	Port           int           // default: 80
}
```

- [ ] **Step 3: Implement mDNS discovery**

File: `internal/discovery/mdns.go`
```go
package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/mdns"
)

const defaultDiscoverTimeout = 5 * time.Second

// Discover finds eSCL scanners via mDNS/DNS-SD.
func Discover(ctx context.Context, opts DiscoverOptions) ([]Scanner, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultDiscoverTimeout
	}

	var scanners []Scanner
	entryCh := make(chan *mdns.ServiceEntry, 16)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for entry := range entryCh {
			ip := entry.AddrV4.String()
			if entry.AddrV4 == nil && entry.AddrV6 != nil {
				ip = entry.AddrV6.String()
			}
			if entry.AddrV4 == nil && entry.AddrV6 == nil {
				continue
			}
			scanners = append(scanners, Scanner{
				IP:       ip,
				Hostname: entry.Host,
				Port:     entry.Port,
				Protocol: "eSCL",
				Source:   "mdns",
			})
		}
	}()

	// Browse for eSCL scanners (_uscan._tcp and _uscans._tcp)
	for _, service := range []string{"_uscan._tcp", "_uscans._tcp"} {
		params := mdns.DefaultParams(service)
		params.Entries = entryCh
		params.Timeout = timeout
		params.DisableIPv6 = true

		if err := mdns.Query(params); err != nil {
			return nil, fmt.Errorf("mDNS query for %s: %w", service, err)
		}
	}

	close(entryCh)
	<-done

	// Deduplicate by IP
	seen := make(map[string]bool)
	var unique []Scanner
	for _, s := range scanners {
		if !seen[s.IP] {
			seen[s.IP] = true
			unique = append(unique, s)
		}
	}
	return unique, nil
}
```

- [ ] **Step 4: Write basic test**

File: `internal/discovery/mdns_test.go`
```go
package discovery

import (
	"context"
	"testing"
	"time"
)

func TestDiscoverReturnsWithoutError(t *testing.T) {
	// mDNS is hard to unit test without real network.
	// This test verifies the function handles empty results gracefully.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	scanners, err := Discover(ctx, DiscoverOptions{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	// Result may be empty or contain real scanners depending on environment
	_ = scanners
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/discovery/ go.mod go.sum
git commit -m "Add mDNS scanner discovery via hashicorp/mdns"
```

---

### Task 12: Subnet Sweep (internal/discovery/sweep.go)

**Files:**
- Create: `internal/discovery/sweep.go`
- Test: `internal/discovery/sweep_test.go`

- [ ] **Step 1: Write failing test**

File: `internal/discovery/sweep_test.go`
```go
package discovery

import (
	"context"
	"testing"
)

func TestLocalIP(t *testing.T) {
	ip, err := localIP()
	if err != nil {
		t.Fatalf("localIP() error = %v", err)
	}
	if ip == "" {
		t.Error("localIP() returned empty string")
	}
	t.Logf("local IP: %s", ip)
}
```

- [ ] **Step 2: Implement sweep.go**

File: `internal/discovery/sweep.go`
```go
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

// Sweep scans the local /24 subnet for eSCL scanners.
func Sweep(ctx context.Context, opts SweepOptions) ([]Scanner, error) {
	concurrency := opts.Concurrency
	if concurrency == 0 {
		concurrency = defaultSweepConcurrency
	}
	hostTimeout := opts.PerHostTimeout
	if hostTimeout == 0 {
		hostTimeout = defaultHostTimeout
	}
	port := opts.Port
	if port == 0 {
		port = defaultSweepPort
	}

	myIP, err := localIP()
	if err != nil {
		return nil, fmt.Errorf("determining local IP: %w", err)
	}
	subnet := myIP[:strings.LastIndex(myIP, ".")+1]

	ips := make(chan string, 253)
	go func() {
		defer close(ips)
		for i := 1; i < 255; i++ {
			ip := fmt.Sprintf("%s%d", subnet, i)
			if ip == myIP {
				continue
			}
			select {
			case ips <- ip:
			case <-ctx.Done():
				return
			}
		}
	}()

	var mu sync.Mutex
	var scanners []Scanner
	var wg sync.WaitGroup

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hc := &http.Client{Timeout: hostTimeout}
			for ip := range ips {
				if ctx.Err() != nil {
					return
				}
				if isESCLScanner(hc, ip, port) {
					mu.Lock()
					scanners = append(scanners, Scanner{
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
	return scanners, nil
}

func isESCLScanner(hc *http.Client, ip string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/eSCL/ScannerCapabilities", ip, port)
	resp, err := hc.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func localIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/discovery/ -v -run TestLocalIP`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/discovery/sweep.go internal/discovery/sweep_test.go
git commit -m "Add concurrent subnet sweep for eSCL scanner discovery"
```

---

## Chunk 6: CLI Integration

### Task 13: Main Entry Point (cmd/hpscan/main.go)

**Files:**
- Modify: `cmd/hpscan/main.go`

- [ ] **Step 1: Add google/subcommands dependency**

Run: `go get github.com/google/subcommands`

- [ ] **Step 2: Implement main.go with all subcommands**

This is the largest single file. It wires together all internal packages and implements the three subcommands (discover, capabilities, scan), global flags, env var resolution, signal handling, and error output.

File: `cmd/hpscan/main.go` -- complete implementation with:
- Version variables for ldflags injection
- Global flags: --pretty, --verbose, --version
- signal.NotifyContext for SIGINT/SIGTERM
- discoverCmd: calls discovery.Discover then discovery.Sweep
- capabilitiesCmd: creates escl.Client, calls Capabilities
- scanCmd: creates escl.Client, validates settings, calls Scan, handles PNG conversion, saves file
- Env var resolution with flag precedence
- Exit code mapping from sentinel errors
- Error output to stderr as JSON

- [ ] **Step 3: Build and verify**

Run:
```bash
go build -o hpscan.exe ./cmd/hpscan
./hpscan.exe --version
```
Expected: JSON version output

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add cmd/hpscan/main.go go.mod go.sum
git commit -m "Add CLI entry point with discover, capabilities, and scan subcommands"
```

---

## Chunk 7: Build, Test, Polish

### Task 14: Integration Test Skeleton

**Files:**
- Create: `internal/escl/integration_test.go`

- [ ] **Step 1: Write integration tests**

File: `internal/escl/integration_test.go`
```go
//go:build integration

package escl_test

import (
	"context"
	"os"
	"testing"

	"github.com/AvogadroSG1/HPScanCLI/internal/escl"
)

func TestIntegrationCapabilities(t *testing.T) {
	ip := os.Getenv("HPSCAN_IP")
	if ip == "" {
		t.Skip("HPSCAN_IP not set")
	}
	c, err := escl.New(context.Background(), "http://"+ip, escl.ClientOptions{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	caps, err := c.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities() error = %v", err)
	}
	if caps.MakeAndModel == "" {
		t.Error("MakeAndModel is empty")
	}
	t.Logf("Scanner: %s (serial: %s)", caps.MakeAndModel, caps.SerialNumber)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/escl/integration_test.go
git commit -m "Add integration test skeleton for real scanner validation"
```

---

### Task 15: Final Polish and Verification

- [ ] **Step 1: Run gofmt**

Run: `gofmt -l .` -- fix any files listed

- [ ] **Step 2: Run go vet**

Run: `go vet ./...` -- fix any issues

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -v`

- [ ] **Step 4: Build release binary**

Run: `make build`

- [ ] **Step 5: Test binary against real scanner**

```bash
./hpscan.exe capabilities --ip 192.168.1.198
./hpscan.exe capabilities --ip 192.168.1.198 --pretty
./hpscan.exe scan --ip 192.168.1.198 --dpi 300 --format jpeg --output go_test_scan
```

- [ ] **Step 6: Clean up test_scan.jpg from earlier Python test**

```bash
rm -f test_scan.jpg
```

- [ ] **Step 7: Final commit**

```bash
git add -A
git commit -m "Final polish: formatting, linting, and build verification"
```

- [ ] **Step 8: Push to GitHub**

```bash
git push -u origin main
```
