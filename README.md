# HPScanCLI

A Go CLI for HP eSCL network scanners. Discovers scanners on your local network,
queries capabilities, and performs scans — all from the command line with
structured JSON output suitable for scripting and autonomous agent integration.

## Install

```bash
go install github.com/AvogadroSG1/HPScanCLI/cmd/hpscan@latest
```

Or build from source:

```bash
git clone https://github.com/AvogadroSG1/HPScanCLI.git
cd HPScanCLI
go build -o hpscan ./cmd/hpscan
```

## Usage

### Discover scanners

```bash
hpscan discover
# {"scanners":[{"ip":"192.168.1.198","hostname":"HP64B90A","port":443,"protocol":"https","source":"mdns"}]}

hpscan --pretty discover
# Discovered 1 scanner(s):
#   192.168.1.198  HP64B90A  :443  https  (mdns)
```

### Query capabilities

```bash
hpscan capabilities --ip 192.168.1.198
hpscan --pretty capabilities --ip 192.168.1.198
```

### Scan

```bash
# Basic scan (300 DPI JPEG from platen)
hpscan scan --ip 192.168.1.198

# Custom settings
hpscan scan --ip 192.168.1.198 --dpi 600 --format pdf --output my_document

# ADF duplex scan
hpscan scan --ip 192.168.1.198 --source Adf --duplex --format pdf
```

### Scan flags

| Flag | Default | Description |
|------|---------|-------------|
| `--ip` | `$HPSCAN_IP` | Scanner IP address (required) |
| `--dpi` | 300 / `$HPSCAN_DPI` | Scan resolution |
| `--format` | `jpeg` | Output format: `jpeg`, `pdf`, `png` |
| `--output` | timestamped | Output filename (without extension) |
| `--color` | `RGB24` | Color mode |
| `--source` | `Platen` | Input source: `Platen`, `Adf` |
| `--duplex` | `false` | Enable duplex scanning (ADF only) |
| `--https` | `false` | Use HTTPS instead of HTTP |

### Global flags

| Flag | Description |
|------|-------------|
| `--pretty` | Human-readable output instead of JSON |
| `--verbose` | Structured debug logging to stderr |
| `--version` | Print version information and exit |

## Environment variables

| Variable | Description |
|----------|-------------|
| `HPSCAN_IP` | Default scanner IP (overridden by `--ip`) |
| `HPSCAN_DPI` | Default DPI (overridden by `--dpi`) |
| `HPSCAN_TIMEOUT_SECONDS` | Download timeout in seconds (default: 120) |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General failure |
| 2 | Scanner unreachable |
| 3 | Scanner busy |
| 4 | Invalid arguments |

## Protocol

This tool implements the [eSCL](https://mopria.org/eSCL) (Embedded Scan Control Language)
protocol, an HTTP-based standard for network scanning supported by HP, Canon, Epson,
Brother, and other manufacturers.

## License

MIT
