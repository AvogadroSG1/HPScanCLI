package escl

import (
	"os"
	"strings"
	"testing"
)

func TestStripNamespaces(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "prefix on opening tag",
			in:   `<scan:MakeAndModel>HP</scan:MakeAndModel>`,
			want: `<MakeAndModel>HP</MakeAndModel>`,
		},
		{
			name: "prefix on self-closing tag",
			in:   `<pwg:Version/>`,
			want: `<Version/>`,
		},
		{
			name: "xmlns declaration removed",
			in:   `<Root xmlns:scan="http://example.com">text</Root>`,
			want: `<Root>text</Root>`,
		},
		{
			name: "xsi attribute removed",
			in:   `<Root xsi:schemaLocation="http://example.com foo.xsd">text</Root>`,
			want: `<Root>text</Root>`,
		},
		{
			name: "multiple prefixes",
			in:   `<scan:A><pwg:B>val</pwg:B></scan:A>`,
			want: `<A><B>val</B></A>`,
		},
		{
			name: "no namespaces unchanged",
			in:   `<Simple>value</Simple>`,
			want: `<Simple>value</Simple>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripNamespaces([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("stripNamespaces(%q) =\n  %q, want\n  %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripNamespacesRealFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/capabilities.xml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	result := string(stripNamespaces(data))

	if strings.Contains(result, "scan:") {
		t.Error("result still contains 'scan:' prefix")
	}
	if strings.Contains(result, "pwg:") {
		t.Error("result still contains 'pwg:' prefix")
	}
	if strings.Contains(result, "xmlns:") {
		t.Error("result still contains 'xmlns:' declaration")
	}
	if strings.Contains(result, "xsi:") {
		t.Error("result still contains 'xsi:' attribute")
	}

	// Verify element content is preserved.
	if !strings.Contains(result, "<MakeAndModel>HP OfficeJet Pro 9010 series</MakeAndModel>") {
		t.Error("MakeAndModel element content not preserved")
	}
	if !strings.Contains(result, "<SerialNumber>TH18G3737H</SerialNumber>") {
		t.Error("SerialNumber element content not preserved")
	}
}
