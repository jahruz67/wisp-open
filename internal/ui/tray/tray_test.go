package tray

import "testing"

func TestIsProcessingStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "Processing...", want: true},
		{status: "Transcribing...", want: true},
		{status: "Typing...", want: true},
		{status: "Recording...", want: false},
		{status: "Ready", want: false},
	}

	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			if got := isProcessingStatus(test.status); got != test.want {
				t.Fatalf("isProcessingStatus(%q) = %v, want %v", test.status, got, test.want)
			}
		})
	}
}
