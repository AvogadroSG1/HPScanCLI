package escl

import "regexp"

var (
	nsPrefix = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*):`)
	nsDecl   = regexp.MustCompile(`\s+xmlns:[a-zA-Z][a-zA-Z0-9]*="[^"]*"`)
	xsiAttr  = regexp.MustCompile(`\s+[a-zA-Z][a-zA-Z0-9]*:[a-zA-Z]+=("[^"]*"|'[^']*')`)
)

// stripNamespaces removes XML namespace prefixes and xmlns declarations,
// allowing encoding/xml to unmarshal with simple element-name struct tags.
func stripNamespaces(data []byte) []byte {
	out := nsPrefix.ReplaceAll(data, []byte(`<$1`))
	out = nsDecl.ReplaceAll(out, nil)
	out = xsiAttr.ReplaceAll(out, nil)
	return out
}
