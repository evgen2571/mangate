package cli

import (
	"errors"
	"testing"
)

func TestErrorDiagnosticUsesStableCategoryAndExitCode(t *testing.T) {
	if got, want := ErrorDiagnostic(errors.New("title cannot be empty")), "error category: invalid_input; exit code: 2"; got != want {
		t.Fatalf("ErrorDiagnostic() = %q, want %q", got, want)
	}
}
