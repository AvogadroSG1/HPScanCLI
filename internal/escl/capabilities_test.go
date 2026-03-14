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
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	return data
}

func TestParseCapabilities(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/capabilities.xml")

	got, err := ParseCapabilities(data)
	if err != nil {
		t.Fatalf("ParseCapabilities() error: %v", err)
	}

	want := &CapabilitiesResponse{
		MakeAndModel:    "HP OfficeJet Pro 9010 series",
		SerialNumber:    "TH18G3737H",
		Manufacturer:    "HP",
		FirmwareVersion: "2.63",
		Platen: &InputSource{
			MinWidth:        8,
			MaxWidth:        2550,
			MinHeight:       8,
			MaxHeight:       3534,
			ColorModes:      []string{"BlackAndWhite1", "Grayscale8", "RGB24"},
			DocumentFormats: []string{"application/octet-stream", "image/jpeg", "application/pdf"},
			Resolutions:     []int{75, 100, 150, 200, 300, 400, 600, 1200},
		},
		Adf: &InputSource{
			MinWidth:        8,
			MaxWidth:        2550,
			MinHeight:       8,
			MaxHeight:       4200,
			ColorModes:      []string{"BlackAndWhite1", "Grayscale8", "RGB24"},
			DocumentFormats: []string{"application/octet-stream", "image/jpeg", "application/pdf"},
			Resolutions:     []int{75, 100, 150, 200, 300},
		},
		AdfDuplex: &InputSource{
			MinWidth:        8,
			MaxWidth:        2550,
			MinHeight:       8,
			MaxHeight:       3508,
			ColorModes:      []string{"BlackAndWhite1", "Grayscale8", "RGB24"},
			DocumentFormats: []string{"application/octet-stream", "image/jpeg", "application/pdf"},
			Resolutions:     []int{75, 100, 150, 200, 300},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseCapabilities() mismatch (-want +got):\n%s", diff)
	}
}
