// Package compare loads probe result documents and renders a Markdown
// report that highlights where environments disagree.
package compare

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Input names one result file and the label used for it in the report.
type Input struct {
	Label string
	Path  string
}

// Row is one comparison line: a path plus one value per input.
type Row struct {
	Path    string
	Values  []string
	Differs bool
}

// Report is the comparison result ready for rendering.
type Report struct {
	Title      string
	Inputs     []Input
	Missing    []Input
	Meta       []Row
	StatusRows []Row
	ValueRows  []Row
	Summary    []Row
}

// Options controls how a report is built.
type Options struct {
	Title        string
	AllowMissing bool
	MaxValueLen  int
}

const (
	missingMark = `-`
	statusLeaf  = `status`
	defaultMax  = 96
)

// Leaves that legitimately differ between runs and are excluded from diffs.
var volatileLeaves = map[string]bool{
	`duration_ms`: true,
	`rtt_ms`:      true,
	`timestamp`:   true,
	`total_ms`:    true,
	`args`:        true,
}

// Paths (prefix match) shown in the environment table instead of the diff.
var metaPaths = []string{
	`meta.schema`,
	`meta.tool`,
	`meta.workarounds`,
	`sections.runtime.classification.value`,
	`sections.runtime.wine_version.value`,
	`sections.os.version.value.string`,
}

// ParseInputs turns label=path arguments into Inputs.
func ParseInputs(args []string) ([]Input, error) {
	var out []Input
	for _, a := range args {
		label, path, ok := strings.Cut(a, `=`)
		if !ok || label == `` || path == `` {
			return nil, fmt.Errorf(`expected label=path, got %q`, a)
		}
		out = append(out, Input{Label: label, Path: path})
	}
	if len(out) < 1 {
		return nil, errors.New(`at least one label=path input is required`)
	}
	return out, nil
}

// Load reads one result document.
func Load(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf(`%s: %w`, path, err)
	}
	return doc, nil
}

// Build loads every input and computes the comparison rows.
func Build(inputs []Input, opts Options) (*Report, error) {
	r := &Report{Title: opts.Title}
	if r.Title == `` {
		r.Title = `Wine Compatibility Probe: comparison report`
	}
	maxLen := opts.MaxValueLen
	if maxLen <= 0 {
		maxLen = defaultMax
	}
	var flats []map[string]string
	for _, in := range inputs {
		doc, err := Load(in.Path)
		if err != nil {
			if opts.AllowMissing && errors.Is(err, os.ErrNotExist) {
				r.Missing = append(r.Missing, in)
				continue
			}
			return nil, err
		}
		r.Inputs = append(r.Inputs, in)
		flat := map[string]string{}
		flatten(``, doc, flat)
		flats = append(flats, flat)
	}
	if len(flats) == 0 {
		return nil, errors.New(`no result files could be loaded`)
	}
	for _, p := range unionPaths(flats) {
		if isVolatile(p) {
			continue
		}
		row := Row{Path: p}
		seen := map[string]bool{}
		for _, f := range flats {
			v, ok := f[p]
			if !ok {
				v = missingMark
			}
			seen[v] = true
			row.Values = append(row.Values, truncate(v, maxLen))
		}
		row.Differs = len(seen) > 1
		switch {
		case isMetaPath(p):
			r.Meta = append(r.Meta, row)
		case strings.HasPrefix(p, `summary.`):
			r.Summary = append(r.Summary, row)
		case strings.HasPrefix(p, `sections.`) && strings.HasSuffix(p, `.`+statusLeaf):
			row.Path = strings.TrimSuffix(strings.TrimPrefix(p, `sections.`), `.`+statusLeaf)
			r.StatusRows = append(r.StatusRows, row)
		case row.Differs && strings.HasPrefix(p, `sections.`):
			r.ValueRows = append(r.ValueRows, row)
		}
	}
	return r, nil
}

// CountDiffers returns how many rows differ between inputs.
func CountDiffers(rows []Row) int {
	n := 0
	for _, r := range rows {
		if r.Differs {
			n++
		}
	}
	return n
}

func unionPaths(flats []map[string]string) []string {
	set := map[string]bool{}
	for _, f := range flats {
		for k := range f {
			set[k] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isMetaPath(p string) bool {
	for _, m := range metaPaths {
		if strings.HasPrefix(p, m) {
			return true
		}
	}
	return false
}

func isVolatile(path string) bool {
	for _, part := range strings.Split(path, `.`) {
		if volatileLeaves[part] {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + `...`
}

func flatten(prefix string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			out[prefix] = `(empty object)`
			return
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			flatten(join(prefix, k), t[k], out)
		}
	case []any:
		if len(t) == 0 {
			out[prefix] = `(empty list)`
			return
		}
		for i, item := range t {
			flatten(join(prefix, strconv.Itoa(i)), item, out)
		}
	case string:
		out[prefix] = t
	case nil:
		out[prefix] = `null`
	default:
		b, err := json.Marshal(v)
		if err != nil {
			out[prefix] = fmt.Sprint(v)
			return
		}
		out[prefix] = string(b)
	}
}

func join(prefix, key string) string {
	if prefix == `` {
		return key
	}
	return prefix + `.` + key
}
