package format

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON marshals v as indented JSON and writes it to stdout.
func PrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(b))
	return err
}
