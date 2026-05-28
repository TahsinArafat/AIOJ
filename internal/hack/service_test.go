package hack

import (
	"testing"
)

func TestHackValidation_logic(t *testing.T) {
	tests := []struct {
		name         string
		testInput    string
		submissionID string
		wantErr      bool
	}{
		{"valid hack", "5\n1 2 3", "sub-123", false},
		{"empty test input", "", "sub-123", true},
		{"empty submission ID", "5\n", "", true},
		{"both empty", "", "", true},
		{"single char input", "x", "sub-456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHackRequest(tt.testInput, tt.submissionID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateHackRequest(%q, %q) error = %v, wantErr %v",
					tt.testInput, tt.submissionID, err, tt.wantErr)
			}
		})
	}
}

// Helper that mirrors the validation logic
func validateHackRequest(testInput, submissionID string) error {
	if testInput == "" {
		return errEmptyInput
	}
	if submissionID == "" {
		return errEmptySubmission
	}
	return nil
}

var (
	errEmptyInput      = fmtError("test input is required")
	errEmptySubmission = fmtError("submission ID is required")
)

type fmtError string

func (e fmtError) Error() string { return string(e) }
