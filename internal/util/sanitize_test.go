package util

import "testing"

func TestSanitizeStringRemovesDotDotPathTraversal(t *testing.T) {
	got := SanitizeString("../evil")
	if got == "..evil" || got == "../evil" || got == ".." {
		t.Fatalf("SanitizeString() = %q, want traversal segments removed", got)
	}
}
