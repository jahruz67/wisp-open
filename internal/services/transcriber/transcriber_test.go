package transcriber

import "testing"

func TestRefinementPreservesTranscriptAllowsLightCleanup(t *testing.T) {
	original := "hello there this is a quick test"
	refined := "Hello there, this is a quick test."

	if !refinementPreservesTranscript(original, refined) {
		t.Fatalf("expected light punctuation cleanup to be accepted")
	}
}

func TestRefinementPreservesTranscriptRejectsUnrelatedOutput(t *testing.T) {
	original := "what time is the meeting tomorrow"
	refined := "The meeting is at 2 PM tomorrow."

	if refinementPreservesTranscript(original, refined) {
		t.Fatalf("expected answer-like rewrite to be rejected")
	}
}

func TestRefinementPreservesTranscriptRejectsPreface(t *testing.T) {
	original := "send the draft when you are done"
	refined := "Here is the corrected text: Send the draft when you are done."

	if refinementPreservesTranscript(original, refined) {
		t.Fatalf("expected assistant preface to be rejected")
	}
}
