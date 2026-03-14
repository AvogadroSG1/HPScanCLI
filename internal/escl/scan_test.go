package escl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClientScan(t *testing.T) {
	docBody := []byte("fake-jpeg-data")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/eSCL/ScanJobs":
			w.Header().Set("Location", "/eSCL/ScanJobs/test-job-123")
			w.WriteHeader(http.StatusCreated)

		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScanJobs/test-job-123/NextDocument":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(docBody)

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client, err := New(ctx, srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	settings := ScanSettings{
		Height:            3508,
		Width:             2550,
		XResolution:       300,
		YResolution:       300,
		ColorMode:         "RGB24",
		DocumentFormat:    "image/jpeg",
		InputSource:       "Platen",
		Brightness:        1000,
		Contrast:          1000,
		CompressionFactor: 25,
	}

	result, err := client.Scan(ctx, settings)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if string(result.Data) != string(docBody) {
		t.Errorf("Scan().Data = %q, want %q", result.Data, docBody)
	}
	if result.ContentType != "image/jpeg" {
		t.Errorf("Scan().ContentType = %q, want %q", result.ContentType, "image/jpeg")
	}
}

func TestClientScanPollFallback(t *testing.T) {
	statusData := mustReadFixture(t, "../../testdata/status_processing.xml")
	docBody := []byte("fake-pdf-data")

	var statusCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/eSCL/ScanJobs":
			// Return 201 without Location header to trigger poll fallback.
			w.WriteHeader(http.StatusCreated)

		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScannerStatus":
			statusCalls.Add(1)
			w.Header().Set("Content-Type", "text/xml")
			w.Write(statusData)

		case r.Method == http.MethodGet && r.URL.Path == "/eSCL/ScanJobs/a1b2c3d4-e5f6-7890-abcd-ef1234567890/NextDocument":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(docBody)

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client, err := New(ctx, srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	settings := ScanSettings{
		Height:            3508,
		Width:             2550,
		XResolution:       300,
		YResolution:       300,
		ColorMode:         "RGB24",
		DocumentFormat:    "application/pdf",
		InputSource:       "Platen",
		Brightness:        1000,
		Contrast:          1000,
		CompressionFactor: 25,
	}

	result, err := client.Scan(ctx, settings)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if string(result.Data) != string(docBody) {
		t.Errorf("Scan().Data = %q, want %q", result.Data, docBody)
	}
	if result.ContentType != "application/pdf" {
		t.Errorf("Scan().ContentType = %q, want %q", result.ContentType, "application/pdf")
	}
	if statusCalls.Load() < 1 {
		t.Error("expected at least one status poll call")
	}
}
