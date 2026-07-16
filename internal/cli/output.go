package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/evgen2571/mangate/internal/util"
	"github.com/spf13/cobra"
)

const outputFormatVersion = "1"

type envelope struct {
	FormatVersion string `json:"formatVersion"`
	Operation     string `json:"operation"`
	Status        string `json:"status"`
	Data          any    `json:"data,omitempty"`
}

type errorData struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

func wantsJSON(cmd *cobra.Command) bool {
	value, err := cmd.Flags().GetBool("json")
	return err == nil && value
}

func isQuiet(cmd *cobra.Command) bool {
	value, err := cmd.Flags().GetBool("quiet")
	return err == nil && value
}

func writeJSON(cmd *cobra.Command, operation string, data any) error {
	return writeJSONStatus(cmd, operation, "success", data)
}

func writeJSONStatus(cmd *cobra.Command, operation, status string, data any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(envelope{FormatVersion: outputFormatVersion, Operation: operation, Status: status, Data: data})
}

// ReportedError marks an error whose structured result was already written to
// standard output. Callers use it to retain a meaningful exit status without
// corrupting JSON with a second error envelope.
type ReportedError struct {
	Cause  error
	Code   int
	Silent bool
}

func (e *ReportedError) Error() string {
	if e == nil || e.Cause == nil {
		return "operation failed"
	}
	return e.Cause.Error()
}

func (e *ReportedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// WriteError writes the documented JSON error envelope. It is used by main
// because Cobra returns command errors after the command's output phase.
func WriteError(out io.Writer, operation string, err error) error {
	message := "operation failed"
	if err != nil {
		message = err.Error()
	}
	return json.NewEncoder(out).Encode(envelope{FormatVersion: outputFormatVersion, Operation: operation, Status: "error", Data: errorData{Category: ErrorCategory(message), Message: message}})
}

// ErrorCategory provides stable categories without leaking transport details.
func ErrorCategory(message string) string {
	message = strings.ToLower(message)
	switch {
	case strings.Contains(message, "no results found"):
		return "no_results"
	case strings.Contains(message, "unknown provider"):
		return "unknown_provider"
	case strings.Contains(message, "not found"):
		return "not_found"
	case strings.Contains(message, "does not permit"), strings.Contains(message, "unsupported"):
		return "unsupported_capability"
	case strings.Contains(message, "deadline"), strings.Contains(message, "timeout"):
		return "timeout"
	case strings.Contains(message, "archive"):
		return "archive"
	case strings.Contains(message, "permission"), strings.Contains(message, "create file"), strings.Contains(message, "write file"):
		return "filesystem"
	case strings.Contains(message, "context canceled"), strings.Contains(message, "interrupted"):
		return "cancelled"
	case strings.Contains(message, "cannot be empty"), strings.Contains(message, "select chapters"), strings.Contains(message, "malformed"):
		return "invalid_input"
	default:
		return "provider_or_internal"
	}
}

// ExitCode maps stable error categories to documented process statuses.
func ExitCode(message string) int {
	if strings.Contains(strings.ToLower(message), "download title") {
		return 5
	}
	switch ErrorCategory(message) {
	case "no_results":
		return 1
	case "invalid_input":
		return 2
	case "filesystem":
		return 6
	case "archive":
		return 8
	case "cancelled":
		return 7
	case "unknown_provider", "not_found", "unsupported_capability", "timeout":
		return 4
	default:
		return 10
	}
}

// ErrorDiagnostic returns safe, stable context for verbose error reporting.
// It deliberately omits provider responses, paths, and request details.
func ErrorDiagnostic(err error) string {
	message := "operation failed"
	if err != nil {
		message = err.Error()
	}
	return fmt.Sprintf("error category: %s; exit code: %d", ErrorCategory(message), ExitCode(message))
}

func writeHuman(out io.Writer, format string, args ...any) {
	for index, value := range args {
		if text, ok := value.(string); ok {
			args[index] = util.SanitizeTerminalText(text)
		}
	}
	_, _ = fmt.Fprintf(out, format, args...)
}
