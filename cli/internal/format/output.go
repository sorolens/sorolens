package format

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Output formats understood by the shared output layer.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
	FormatJSONL = "jsonl"
)

// ValidFormat reports whether f is a supported --output value.
func ValidFormat(f string) bool {
	switch f {
	case FormatTable, FormatJSON, FormatYAML, FormatJSONL:
		return true
	}
	return false
}

// Options controls how structured values are rendered. The zero value means
// indented JSON without a query, which keeps the historical behaviour.
type Options struct {
	Format string
	Query  *Query
}

var current Options

// SetOptions installs the process-wide output options (called once from the
// root command after flags and config are resolved).
func SetOptions(o Options) { current = o }

// Structured reports whether commands should emit structured output instead
// of a human-readable table.
func Structured() bool {
	return current.Format != "" && current.Format != FormatTable
}

// PrintJSON renders v in the configured structured format (json by default)
// and writes it to stdout. The name is kept for existing call sites; it now
// honours --output (json, yaml, jsonl) and --query.
func PrintJSON(v any) error {
	return Fprint(os.Stdout, v, current)
}

// Fprint renders v to w according to o.
func Fprint(w io.Writer, v any, o Options) error {
	f := o.Format
	if f == "" || f == FormatTable {
		f = FormatJSON
	}
	if o.Query != nil {
		generic, err := normalize(v)
		if err != nil {
			return err
		}
		results, err := o.Query.Run(generic)
		if err != nil {
			return err
		}
		for i, r := range results {
			if err := emit(w, r, f, i > 0); err != nil {
				return err
			}
		}
		return nil
	}
	if f == FormatJSONL {
		return emitJSONL(w, v)
	}
	return emit(w, v, f, false)
}

// emit writes one value. YAML values after the first are separated by a
// document marker so a multi-result query stays valid YAML.
func emit(w io.Writer, v any, f string, sep bool) error {
	switch f {
	case FormatYAML:
		generic, err := normalize(v)
		if err != nil {
			return err
		}
		if sep {
			if _, err := fmt.Fprintln(w, "---"); err != nil {
				return err
			}
		}
		b, err := yaml.Marshal(generic)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, err = w.Write(b)
		return err
	case FormatJSONL:
		return emitJSONL(w, v)
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	}
}

// emitJSONL writes one compact JSON document per line. Slices and arrays are
// written element by element straight to w, so a large result is never held
// as one marshalled buffer.
func emitJSONL(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	rv := reflect.ValueOf(v)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		for i := 0; i < rv.Len(); i++ {
			if err := enc.Encode(rv.Index(i).Interface()); err != nil {
				return fmt.Errorf("marshal jsonl: %w", err)
			}
		}
		return nil
	}
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("marshal jsonl: %w", err)
	}
	return nil
}

// normalize converts v to generic JSON data (maps, slices, scalars) so json
// struct tags are honoured by every format and by queries.
func normalize(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	var out any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return convertNumbers(out), nil
}

// convertNumbers turns json.Number into int64 (or float64) so YAML renders
// numbers as numbers while large integers keep full precision.
func convertNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, x := range t {
			t[k] = convertNumbers(x)
		}
	case []any:
		for i, x := range t {
			t[i] = convertNumbers(x)
		}
	}
	return v
}
