package escl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCapabilities(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/capabilities.xml")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eSCL/ScannerCapabilities" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write(data)
	}))
	defer srv.Close()

	ctx := context.Background()
	client, err := New(ctx, srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	caps, err := client.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Capabilities() error: %v", err)
	}

	if caps.MakeAndModel != "HP OfficeJet Pro 9010 series" {
		t.Errorf("Capabilities().MakeAndModel = %q, want %q", caps.MakeAndModel, "HP OfficeJet Pro 9010 series")
	}
}

func TestClientStatus(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/status_idle.xml")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eSCL/ScannerStatus" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write(data)
	}))
	defer srv.Close()

	ctx := context.Background()
	client, err := New(ctx, srv.URL, ClientOptions{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if status.State != "Idle" {
		t.Errorf("Status().State = %q, want %q", status.State, "Idle")
	}
}
