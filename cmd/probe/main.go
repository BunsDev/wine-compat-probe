//go:build windows

// Command probe reports the OS version, available Windows APIs, graphics
// capabilities, and basic file and network behaviour as sorted JSON so one
// binary can be compared across Windows, Wine, and Proton.
//
// Usage:
//
//	probe.exe [-out FILE] [-no-network] [-text] [-workarounds all|none|a,b]
//
// The exit code is 0 whenever the report was written; individual probe
// failures are data, not errors. Exit 2 means bad flags, exit 1 means the
// report could not be written.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const toolVersion = `probe/0.1.0`

// Workaround identifiers. See docs/FIX-VERSION-DETECTION.md.
const (
	WorkaroundRtlVersion   = `rtl-version`
	WorkaroundWarpFallback = `d3d-warp-fallback`
	WorkaroundTCPReach     = `tcp-reachability`
)

var allWorkarounds = []string{WorkaroundRtlVersion, WorkaroundWarpFallback, WorkaroundTCPReach}

type options struct {
	out         string
	noNetwork   bool
	text        bool
	workarounds map[string]bool
}

func (o *options) enabled(name string) bool {
	return o.workarounds[name]
}

func (o *options) enabledList() []string {
	var out []string
	for _, w := range allWorkarounds {
		if o.workarounds[w] {
			out = append(out, w)
		}
	}
	return out
}

func parseFlags(args []string) (*options, error) {
	o := &options{}
	fs := flag.NewFlagSet(`probe`, flag.ContinueOnError)
	fs.StringVar(&o.out, `out`, ``, `write JSON to this file (default: stdout)`)
	fs.BoolVar(&o.noNetwork, `no-network`, false, `skip probes that touch the network`)
	fs.BoolVar(&o.text, `text`, false, `also print a one-line-per-probe summary to stderr`)
	wa := fs.String(`workarounds`, `all`, `compatibility workarounds to apply: all, none, or a comma list`)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	o.workarounds = map[string]bool{}
	switch *wa {
	case `all`:
		for _, w := range allWorkarounds {
			o.workarounds[w] = true
		}
	case `none`, ``:
	default:
		for _, w := range strings.Split(*wa, `,`) {
			w = strings.TrimSpace(w)
			if !known(w) {
				return nil, fmt.Errorf(`unknown workaround %q (known: %s)`, w, strings.Join(allWorkarounds, `, `))
			}
			o.workarounds[w] = true
		}
	}
	return o, nil
}

func known(w string) bool {
	for _, k := range allWorkarounds {
		if k == w {
			return true
		}
	}
	return false
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	start := time.Now()
	doc := schema.New(toolVersion, opts.enabledList(), os.Args[1:])
	probeRuntime(doc)
	probeOS(doc, opts)
	probeAPIs(doc)
	probeGraphics(doc, opts)
	probeFiles(doc)
	probeNetwork(doc, opts)
	doc.Meta[`total_ms`] = time.Since(start).Milliseconds()

	b, err := doc.Encode()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.out == `` {
		os.Stdout.Write(b)
	} else if err := os.WriteFile(opts.out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.text {
		printText(doc)
	}
}

// printText writes a compact human-readable view to stderr.
func printText(doc *schema.Doc) {
	sections := make([]string, 0, len(doc.Sections))
	for name := range doc.Sections {
		sections = append(sections, name)
	}
	sort.Strings(sections)
	for _, name := range sections {
		sec := doc.Sections[name]
		probes := make([]string, 0, len(sec))
		for p := range sec {
			probes = append(probes, p)
		}
		sort.Strings(probes)
		for _, p := range probes {
			entry, _ := sec[p].(schema.Node)
			status, _ := entry[schema.KeyStatus].(string)
			detail := entry[schema.KeyValue]
			if status != schema.StatusOK {
				detail = firstNonNil(entry[schema.KeyError], entry[schema.KeyNote])
			}
			fmt.Fprintf(os.Stderr, `[%-11s] %s.%s = %v`+nl, status, name, p, detail)
		}
	}
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// timed runs one probe, records its duration, and converts panics to failures.
func timed(doc *schema.Doc, section, name string, fn func() schema.Node) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			doc.Set(section, name, schema.Fail(fmt.Sprint(`panic: `, r)).With(schema.KeyDurationMS, time.Since(start).Milliseconds()))
		}
	}()
	n := fn()
	if n == nil {
		n = schema.Fail(`probe returned no result`)
	}
	n[schema.KeyDurationMS] = time.Since(start).Milliseconds()
	doc.Set(section, name, n)
}

const nl = "\n"
