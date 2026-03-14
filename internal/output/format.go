package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON encodes v as indented JSON and writes it to w.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WritePretty writes a human-readable representation of v to w.
// Known types receive special formatting; unknown types fall back to WriteJSON.
func WritePretty(w io.Writer, v any) error {
	switch t := v.(type) {
	case DiscoverResponse:
		return writeDiscoverPretty(w, t)
	case *DiscoverResponse:
		return writeDiscoverPretty(w, *t)
	default:
		return WriteJSON(w, v)
	}
}

func writeDiscoverPretty(w io.Writer, d DiscoverResponse) error {
	for _, s := range d.Scanners {
		_, err := fmt.Fprintf(w, "%s  %s  (port %d, %s, via %s)\n",
			s.IP, s.Hostname, s.Port, s.Protocol, s.Source)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteError writes a JSON-formatted error response to w.
func WriteError(w io.Writer, code int, message string, detail any) {
	resp := ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
	}
	// Best-effort write; nothing useful to do with the error here.
	_ = WriteJSON(w, resp)
}
