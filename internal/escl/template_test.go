package escl

import (
	"strings"
	"testing"
)

func TestRenderScanSettingsXML(t *testing.T) {
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

	got, err := RenderScanSettingsXML(settings)
	if err != nil {
		t.Fatalf("RenderScanSettingsXML() error: %v", err)
	}

	xml := string(got)

	checks := []struct {
		name    string
		contain string
	}{
		{"xml declaration", `<?xml version="1.0" encoding="UTF-8"?>`},
		{"scan namespace", `xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03"`},
		{"pwg namespace", `xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm"`},
		{"height", `<pwg:Height>3508</pwg:Height>`},
		{"width", `<pwg:Width>2550</pwg:Width>`},
		{"x resolution", `<scan:XResolution>300</scan:XResolution>`},
		{"y resolution", `<scan:YResolution>300</scan:YResolution>`},
		{"color mode", `<scan:ColorMode>RGB24</scan:ColorMode>`},
		{"document format", `<scan:DocumentFormatExt>image/jpeg</scan:DocumentFormatExt>`},
		{"input source", `<pwg:InputSource>Platen</pwg:InputSource>`},
		{"compression", `<scan:CompressionFactor>25</scan:CompressionFactor>`},
		{"brightness", `<scan:Brightness>1000</scan:Brightness>`},
		{"contrast", `<scan:Contrast>1000</scan:Contrast>`},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(xml, c.contain) {
				t.Errorf("output missing %q:\n%s", c.contain, xml)
			}
		})
	}
}
