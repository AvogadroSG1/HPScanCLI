package escl

import (
	"bytes"
	"fmt"
	"text/template"
)

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
		return nil, fmt.Errorf("rendering scan settings xml: %w", err)
	}
	return buf.Bytes(), nil
}
