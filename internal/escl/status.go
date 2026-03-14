package escl

import "encoding/xml"

type statusXML struct {
	XMLName xml.Name `xml:"ScannerStatus"`
	State   string   `xml:"State"`
	Jobs    jobsXML  `xml:"Jobs"`
}

type jobsXML struct {
	Jobs []jobInfoXML `xml:"JobInfo"`
}

type jobInfoXML struct {
	URI   string `xml:"JobUri"`
	UUID  string `xml:"JobUuid"`
	State string `xml:"JobState"`
}

// ParseStatus parses an eSCL ScannerStatus XML response.
func ParseStatus(data []byte) (*StatusResponse, error) {
	cleaned := stripNamespaces(data)

	var raw statusXML
	if err := xml.Unmarshal(cleaned, &raw); err != nil {
		return nil, err
	}

	resp := &StatusResponse{
		State: raw.State,
	}

	for _, j := range raw.Jobs.Jobs {
		resp.Jobs = append(resp.Jobs, JobInfo{
			URI:   j.URI,
			UUID:  j.UUID,
			State: j.State,
		})
	}

	return resp, nil
}

// findProcessingJobURI returns the URI of the first job in Processing state,
// or an empty string if no such job exists.
func findProcessingJobURI(status *StatusResponse) string {
	for _, j := range status.Jobs {
		if j.State == "Processing" {
			return j.URI
		}
	}
	return ""
}
