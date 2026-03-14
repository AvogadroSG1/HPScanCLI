package escl

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		wantState string
		wantJobs  int
	}{
		{
			name:      "idle with completed jobs",
			fixture:   "../../testdata/status_idle.xml",
			wantState: "Idle",
			wantJobs:  2,
		},
		{
			name:      "processing with active job",
			fixture:   "../../testdata/status_processing.xml",
			wantState: "Processing",
			wantJobs:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mustReadFixture(t, tt.fixture)

			got, err := ParseStatus(data)
			if err != nil {
				t.Fatalf("ParseStatus() error: %v", err)
			}

			if got.State != tt.wantState {
				t.Errorf("ParseStatus().State = %q, want %q", got.State, tt.wantState)
			}
			if len(got.Jobs) != tt.wantJobs {
				t.Errorf("ParseStatus().Jobs count = %d, want %d", len(got.Jobs), tt.wantJobs)
			}
		})
	}
}

func TestFindProcessingJobURI(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/status_processing.xml")

	status, err := ParseStatus(data)
	if err != nil {
		t.Fatalf("ParseStatus() error: %v", err)
	}

	got := findProcessingJobURI(status)
	want := "/eSCL/ScanJobs/a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	if got != want {
		t.Errorf("findProcessingJobURI() = %q, want %q", got, want)
	}
}

func TestFindProcessingJobURI_NoProcessing(t *testing.T) {
	data := mustReadFixture(t, "../../testdata/status_idle.xml")

	status, err := ParseStatus(data)
	if err != nil {
		t.Fatalf("ParseStatus() error: %v", err)
	}

	got := findProcessingJobURI(status)
	if got != "" {
		t.Errorf("findProcessingJobURI() = %q, want empty string", got)
	}
}
