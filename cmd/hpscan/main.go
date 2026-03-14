// Package main is the entry point for the hpscan CLI.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/google/subcommands"

	"github.com/AvogadroSG1/HPScanCLI/internal/discovery"
	"github.com/AvogadroSG1/HPScanCLI/internal/escl"
	"github.com/AvogadroSG1/HPScanCLI/internal/output"
)

// Build-time variables injected via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Global flags
	pretty := flag.Bool("pretty", false, "human-readable output instead of JSON")
	verbose := flag.Bool("verbose", false, "structured detail in error output")
	showVersion := flag.Bool("version", false, "print version information and exit")

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(&discoverCmd{}, "")
	subcommands.Register(&capabilitiesCmd{}, "")
	subcommands.Register(&scanCmd{}, "")

	flag.Parse()

	if *showVersion {
		resp := output.VersionResponse{Version: version, Commit: commit, Date: date}
		if *pretty {
			fmt.Fprintf(os.Stdout, "hpscan %s (commit: %s, built: %s)\n", version, commit, date)
		} else {
			output.WriteJSON(os.Stdout, resp)
		}
		os.Exit(0)
	}

	log := slog.Default()
	if *verbose {
		log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	exitStatus := subcommands.Execute(ctx, log, *pretty, *verbose)
	os.Exit(int(exitStatus))
}

func resolveIP(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("HPSCAN_IP")
}

func resolveDPI(flagVal int) int {
	if flagVal != 0 {
		return flagVal
	}
	if s := os.Getenv("HPSCAN_DPI"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	return 300
}

func resolveTimeout() time.Duration {
	if s := os.Getenv("HPSCAN_TIMEOUT_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return time.Duration(v) * time.Second
		}
	}
	return 120 * time.Second
}

func writeErr(w *os.File, code int, message string, verbose bool, detail any) {
	d := detail
	if !verbose {
		d = nil
	}
	output.WriteError(w, code, message, d)
}

func exitCode(err error) subcommands.ExitStatus {
	switch {
	case err == nil:
		return subcommands.ExitSuccess
	case errors.Is(err, escl.ErrUnreachable):
		return 2
	case errors.Is(err, escl.ErrBusy):
		return 3
	default:
		return subcommands.ExitFailure
	}
}

func makeClient(ctx context.Context, ip string, useHTTPS bool, log *slog.Logger) (*escl.Client, error) {
	scheme := "http"
	if useHTTPS {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, ip)
	return escl.New(ctx, baseURL, escl.ClientOptions{
		DownloadTimeout: resolveTimeout(),
		Logger:          log,
	})
}

// --- discover ---

type discoverCmd struct{}

func (*discoverCmd) Name() string             { return "discover" }
func (*discoverCmd) Synopsis() string         { return "find scanners on the local network" }
func (*discoverCmd) Usage() string            { return "hpscan discover\n" }
func (*discoverCmd) SetFlags(f *flag.FlagSet) {}

func (d *discoverCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	log := args[0].(*slog.Logger)
	pretty := args[1].(bool)
	verbose := args[2].(bool)

	log.Debug("starting mDNS discovery")
	scanners, err := discovery.Discover(ctx, discovery.DiscoverOptions{Timeout: 5 * time.Second})
	if err != nil {
		log.Debug("mDNS discovery failed, trying sweep", "error", err)
	}

	if len(scanners) == 0 {
		log.Debug("no scanners found via mDNS, sweeping subnet")
		scanners, err = discovery.Sweep(ctx, discovery.SweepOptions{})
		if err != nil {
			writeErr(os.Stderr, 1, fmt.Sprintf("subnet sweep failed: %v", err), verbose, nil)
			return subcommands.ExitFailure
		}
	}

	resp := output.DiscoverResponse{
		Scanners: make([]output.Scanner, len(scanners)),
	}
	for i, s := range scanners {
		resp.Scanners[i] = output.Scanner{
			IP:       s.IP,
			Hostname: s.Hostname,
			Port:     s.Port,
			Protocol: s.Protocol,
			Source:   s.Source,
		}
	}

	if pretty {
		output.WritePretty(os.Stdout, resp)
	} else {
		output.WriteJSON(os.Stdout, resp)
	}
	return subcommands.ExitSuccess
}

// --- capabilities ---

type capabilitiesCmd struct {
	ip    string
	https bool
}

func (*capabilitiesCmd) Name() string     { return "capabilities" }
func (*capabilitiesCmd) Synopsis() string { return "query and display scanner capabilities" }
func (*capabilitiesCmd) Usage() string    { return "hpscan capabilities --ip <address>\n" }

func (c *capabilitiesCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ip, "ip", "", "scanner IP address")
	f.BoolVar(&c.https, "https", false, "use HTTPS instead of HTTP")
}

func (c *capabilitiesCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	log := args[0].(*slog.Logger)
	pretty := args[1].(bool)
	verbose := args[2].(bool)

	ip := resolveIP(c.ip)
	if ip == "" {
		writeErr(os.Stderr, 4, "missing required flag: --ip (or set HPSCAN_IP)", verbose, nil)
		return 4
	}

	client, err := makeClient(ctx, ip, c.https, log)
	if err != nil {
		writeErr(os.Stderr, 1, err.Error(), verbose, nil)
		return subcommands.ExitFailure
	}

	caps, err := client.Capabilities(ctx)
	if err != nil {
		writeErr(os.Stderr, int(exitCode(err)), err.Error(), verbose, nil)
		return exitCode(err)
	}

	if pretty {
		writePrettyCapabilities(os.Stdout, caps)
	} else {
		output.WriteJSON(os.Stdout, caps)
	}
	return subcommands.ExitSuccess
}

func writePrettyCapabilities(w *os.File, caps *escl.CapabilitiesResponse) {
	fmt.Fprintf(w, "Make and Model:      %s\n", caps.MakeAndModel)
	fmt.Fprintf(w, "Serial Number:       %s\n", caps.SerialNumber)
	fmt.Fprintf(w, "Manufacturer:        %s\n", caps.Manufacturer)
	fmt.Fprintf(w, "Firmware Version:    %s\n", caps.FirmwareVersion)
	fmt.Fprintln(w)
	if caps.Platen != nil {
		printInputSource(w, "Platen", caps.Platen)
	}
	if caps.Adf != nil {
		printInputSource(w, "ADF", caps.Adf)
	}
	if caps.AdfDuplex != nil {
		printInputSource(w, "ADF Duplex", caps.AdfDuplex)
	}
}

func printInputSource(w *os.File, name string, src *escl.InputSource) {
	fmt.Fprintf(w, "[%s]\n", name)
	fmt.Fprintf(w, "  Dimensions:        %d-%d x %d-%d\n", src.MinWidth, src.MaxWidth, src.MinHeight, src.MaxHeight)
	fmt.Fprintf(w, "  Color Modes:       %s\n", joinStrings(src.ColorModes))
	fmt.Fprintf(w, "  Resolutions:       %s\n", joinInts(src.Resolutions))
	fmt.Fprintf(w, "  Document Formats:  %s\n", joinStrings(src.DocumentFormats))
	fmt.Fprintln(w)
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

func joinInts(ns []int) string {
	result := ""
	for i, n := range ns {
		if i > 0 {
			result += ", "
		}
		result += strconv.Itoa(n)
	}
	return result
}

// --- scan ---

type scanCmd struct {
	ip      string
	dpi     int
	height  int
	width   int
	color   string
	source  string
	duplex  bool
	format  string
	outName string
	https   bool
}

func (*scanCmd) Name() string     { return "scan" }
func (*scanCmd) Synopsis() string { return "perform a scan and save the result to a file" }
func (*scanCmd) Usage() string    { return "hpscan scan --ip <address> [flags]\n" }

func (s *scanCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&s.ip, "ip", "", "scanner IP address")
	f.IntVar(&s.dpi, "dpi", 0, "scan resolution")
	f.IntVar(&s.height, "height", 0, "scan region height (scanner units)")
	f.IntVar(&s.width, "width", 0, "scan region width (scanner units)")
	f.StringVar(&s.color, "color", "RGB24", "color mode")
	f.StringVar(&s.source, "source", "Platen", "input source: Platen, Adf")
	f.BoolVar(&s.duplex, "duplex", false, "enable duplex scanning (ADF only)")
	f.StringVar(&s.format, "format", "jpeg", "output format: jpeg, pdf, png")
	f.StringVar(&s.outName, "output", "", "output filename (without extension)")
	f.BoolVar(&s.https, "https", false, "use HTTPS instead of HTTP")
}

func (s *scanCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	log := args[0].(*slog.Logger)
	pretty := args[1].(bool)
	verbose := args[2].(bool)

	ip := resolveIP(s.ip)
	if ip == "" {
		writeErr(os.Stderr, 4, "missing required flag: --ip (or set HPSCAN_IP)", verbose, nil)
		return 4
	}

	dpi := resolveDPI(s.dpi)
	mime, ok := output.FormatMIME[s.format]
	if !ok {
		writeErr(os.Stderr, 4, fmt.Sprintf("unsupported format: %q (use jpeg, pdf, or png)", s.format), verbose, nil)
		return 4
	}

	client, err := makeClient(ctx, ip, s.https, log)
	if err != nil {
		writeErr(os.Stderr, 1, err.Error(), verbose, nil)
		return subcommands.ExitFailure
	}

	// Fetch capabilities to validate and fill defaults
	caps, err := client.Capabilities(ctx)
	if err != nil {
		writeErr(os.Stderr, int(exitCode(err)), err.Error(), verbose, nil)
		return exitCode(err)
	}

	// Determine which input source caps to use
	var srcCaps *escl.InputSource
	switch s.source {
	case "Platen":
		srcCaps = caps.Platen
		if srcCaps == nil {
			writeErr(os.Stderr, 4, "scanner does not have a platen", verbose, nil)
			return 4
		}
	case "Adf":
		if s.duplex {
			srcCaps = caps.AdfDuplex
			if srcCaps == nil {
				writeErr(os.Stderr, 4, "scanner does not support ADF duplex", verbose, nil)
				return 4
			}
		} else {
			srcCaps = caps.Adf
			if srcCaps == nil {
				writeErr(os.Stderr, 4, "scanner does not have an ADF", verbose, nil)
				return 4
			}
		}
	default:
		writeErr(os.Stderr, 4, fmt.Sprintf("unsupported source: %q (use Platen or Adf)", s.source), verbose, nil)
		return 4
	}

	// Validate DPI
	if !contains(srcCaps.Resolutions, dpi) && len(srcCaps.Resolutions) > 0 {
		log.Info("requested DPI not supported, using closest", "requested", dpi, "available", srcCaps.Resolutions)
		dpi = srcCaps.Resolutions[0]
		for _, r := range srcCaps.Resolutions {
			if r <= 300 {
				dpi = r
			}
		}
	}

	height := s.height
	if height == 0 {
		height = srcCaps.MaxHeight
	}
	width := s.width
	if width == 0 {
		width = srcCaps.MaxWidth
	}

	settings := escl.ScanSettings{
		Height:         height,
		Width:          width,
		XResolution:    dpi,
		YResolution:    dpi,
		ColorMode:      s.color,
		DocumentFormat: mime,
		InputSource:    s.source,
		Duplex:         s.duplex,
		Brightness:        1000,
		Contrast:          1000,
		CompressionFactor: 25,
	}

	log.Debug("starting scan", "settings", settings)
	result, err := client.Scan(ctx, settings)
	if err != nil {
		writeErr(os.Stderr, int(exitCode(err)), err.Error(), verbose, nil)
		return exitCode(err)
	}

	// PNG conversion if needed
	data := result.Data
	if s.format == "png" {
		data, err = output.ConvertToPNG(result.Data)
		if err != nil {
			writeErr(os.Stderr, 1, fmt.Sprintf("png conversion failed: %v", err), verbose, nil)
			return subcommands.ExitFailure
		}
	}

	cwd, _ := os.Getwd()
	filePath, err := output.SaveFile(ctx, data, s.outName, s.format, cwd)
	if err != nil {
		writeErr(os.Stderr, 1, fmt.Sprintf("saving file: %v", err), verbose, nil)
		return subcommands.ExitFailure
	}

	resp := output.ScanResponse{
		File:      filePath,
		Format:    mime,
		SizeBytes: int64(len(data)),
		Settings: output.ScanSettingsResponse{
			DPI:            dpi,
			Height:         height,
			Width:          width,
			ColorMode:      s.color,
			Source:         s.source,
			DocumentFormat: mime,
		},
	}

	if pretty {
		fmt.Fprintf(os.Stdout, "Scan complete. Saved to %s (%d bytes)\n", filePath, len(data))
	} else {
		output.WriteJSON(os.Stdout, resp)
	}
	return subcommands.ExitSuccess
}

func contains(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// ensure json is used (for version output in non-pretty mode)
var _ = json.Marshal
