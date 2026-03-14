package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	resp := DiscoverResponse{
		Scanners: []Scanner{
			{IP: "192.168.1.42", Hostname: "printer.local", Port: 443, Protocol: "https", Source: "mdns"},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, resp); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	// Verify valid JSON.
	var decoded DiscoverResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify indentation (should contain newline + spaces).
	if !bytes.Contains(buf.Bytes(), []byte("\n  ")) {
		t.Error("expected indented JSON output")
	}

	// Verify IP roundtrips.
	if len(decoded.Scanners) == 0 || decoded.Scanners[0].IP != "192.168.1.42" {
		t.Errorf("IP did not roundtrip, got %+v", decoded.Scanners)
	}
}

func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, 404, "scanner not found", nil)

	var resp ErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if resp.Error.Code != 404 {
		t.Errorf("expected code 404, got %d", resp.Error.Code)
	}

	if resp.Error.Message != "scanner not found" {
		t.Errorf("unexpected message: %s", resp.Error.Message)
	}
}

func TestWriteErrorVerbose(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, 500, "internal error", map[string]string{"trace": "abc123"})

	var resp ErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if resp.Error.Detail == nil {
		t.Fatal("expected detail field to be populated")
	}

	detail, ok := resp.Error.Detail.(map[string]any)
	if !ok {
		t.Fatalf("expected detail to be a map, got %T", resp.Error.Detail)
	}

	if detail["trace"] != "abc123" {
		t.Errorf("expected trace abc123, got %v", detail["trace"])
	}
}
