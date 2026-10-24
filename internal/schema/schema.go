// Package schema defines the probe result document and small builders for it.
//
// The document is a tree of maps so that encoding/json emits keys in sorted
// order. Sorted keys make two runs byte-comparable without extra tooling.
package schema

import (
	"encoding/json"
	"sort"
	"time"
)

// Version is the document schema identifier. Bump it when the shape changes.
const Version = `wine-compat-probe/1`

// Probe status values.
const (
	StatusOK          = `ok`
	StatusFail        = `fail`
	StatusSkipped     = `skipped`
	StatusUnsupported = `unsupported`
)

// Keys shared by every probe entry.
const (
	KeyStatus     = `status`
	KeyValue      = `value`
	KeyError      = `error`
	KeyNote       = `note`
	KeySource     = `source`
	KeyDurationMS = `duration_ms`
)

// Top-level document keys.
const (
	KeyMeta     = `meta`
	KeySummary  = `summary`
	KeySections = `sections`
)

// Node is a JSON object. Nested objects are also Node values.
type Node map[string]any

// Doc is the full probe document.
type Doc struct {
	Meta     Node
	Sections map[string]Node
}

// New returns an empty document with meta filled in.
func New(tool string, workarounds []string, args []string) *Doc {
	d := &Doc{}
	d.Meta = Node{}
	d.Sections = map[string]Node{}
	d.Meta[`schema`] = Version
	d.Meta[`tool`] = tool
	d.Meta[`timestamp`] = time.Now().UTC().Format(time.RFC3339)
	d.Meta[`workarounds`] = sortedCopy(workarounds)
	d.Meta[`args`] = args
	return d
}

// Section returns the named section, creating it when needed.
func (d *Doc) Section(name string) Node {
	s, ok := d.Sections[name]
	if !ok {
		s = Node{}
		d.Sections[name] = s
	}
	return s
}

// Set stores a probe entry in a section.
func (d *Doc) Set(section, probe string, entry Node) {
	d.Section(section)[probe] = entry
}

// OK builds a successful probe entry.
func OK(value any) Node {
	n := Node{}
	n[KeyStatus] = StatusOK
	n[KeyValue] = value
	return n
}

// Fail builds a failed probe entry with an error string.
func Fail(err string) Node {
	n := Node{}
	n[KeyStatus] = StatusFail
	n[KeyError] = err
	return n
}

// Skipped builds a skipped probe entry with a reason.
func Skipped(reason string) Node {
	n := Node{}
	n[KeyStatus] = StatusSkipped
	n[KeyNote] = reason
	return n
}

// Unsupported builds an entry for an API that is absent in this environment.
func Unsupported(reason string) Node {
	n := Node{}
	n[KeyStatus] = StatusUnsupported
	n[KeyNote] = reason
	return n
}

// With adds a key to an entry and returns it, for chaining.
func (n Node) With(key string, value any) Node {
	n[key] = value
	return n
}

// Summary recomputes status counts over all sections.
func (d *Doc) Summary() Node {
	counts := map[string]int{}
	total := 0
	for _, sec := range d.Sections {
		for _, raw := range sec {
			entry, ok := raw.(Node)
			if !ok {
				continue
			}
			st, _ := entry[KeyStatus].(string)
			counts[st]++
			total++
		}
	}
	s := Node{}
	s[`total`] = total
	for k, v := range counts {
		s[k] = v
	}
	return s
}

// MarshalJSON renders the document as one object with sorted keys.
func (d *Doc) MarshalJSON() ([]byte, error) {
	root := Node{}
	root[KeyMeta] = d.Meta
	root[KeySummary] = d.Summary()
	root[KeySections] = d.Sections
	return json.Marshal(root)
}

// Encode returns indented JSON with a trailing newline.
func (d *Doc) Encode() ([]byte, error) {
	b, err := json.MarshalIndent(d, ``, `  `)
	if err != nil {
		return nil, err
	}
	return append(b, newline), nil
}

const newline = 0x0a

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
