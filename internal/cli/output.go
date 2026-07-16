package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(envelope{FormatVersion: outputFormatVersion, Operation: operation, Status: "success", Data: data})
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
	case strings.Contains(message, "unknown provider"):
		return "unknown_provider"
	case strings.Contains(message, "not found"):
		return "not_found"
	case strings.Contains(message, "does not permit"), strings.Contains(message, "unsupported"):
		return "unsupported_capability"
	case strings.Contains(message, "deadline"), strings.Contains(message, "timeout"):
		return "timeout"
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
	case "invalid_input":
		return 2
	case "filesystem":
		return 6
	case "cancelled":
		return 7
	case "unknown_provider", "not_found", "unsupported_capability", "timeout":
		return 4
	default:
		return 10
	}
}

func writeHuman(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}
