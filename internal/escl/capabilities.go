package escl

import "encoding/xml"

type capabilitiesXML struct {
	XMLName      xml.Name   `xml:"ScannerCapabilities"`
	MakeAndModel string     `xml:"MakeAndModel"`
	SerialNumber string     `xml:"SerialNumber"`
	Manufacturer string     `xml:"Manufacturer"`
	Version      string     `xml:"Version"`
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

// ParseCapabilities parses an eSCL ScannerCapabilities XML response into
// a CapabilitiesResponse suitable for JSON output.
func ParseCapabilities(data []byte) (*CapabilitiesResponse, error) {
	cleaned := stripNamespaces(data)

	var raw capabilitiesXML
	if err := xml.Unmarshal(cleaned, &raw); err != nil {
		return nil, err
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

	for _, p := range caps.Profiles.Profiles {
		src.ColorModes = append(src.ColorModes, p.ColorModes.Modes...)

		// Prefer DocumentFormatExt if available, fall back to DocumentFormat.
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
