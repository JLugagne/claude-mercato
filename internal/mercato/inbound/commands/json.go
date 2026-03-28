package commands

import (
	"encoding/json"
	"fmt"
	"io"
)

// printJSON marshals v as indented JSON and writes it to w.
func printJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
