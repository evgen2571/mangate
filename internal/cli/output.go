package cli

import (
	"fmt"
	"io"
)

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer) {
	_, _ = fmt.Fprintln(w)
}
