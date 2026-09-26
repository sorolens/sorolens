package format

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsColorEnabled returns true when color output should be rendered.
// Color is disabled when NO_COLOR is set (any value) or when stdout is not a terminal.
func IsColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}
