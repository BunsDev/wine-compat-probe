// Command compare renders a Markdown comparison report from probe results.
//
// Usage:
//
//	compare [-out FILE] [-title TEXT] [-allow-missing] [-strict] label=path [label=path ...]
//
// Exit codes: 0 report written, 1 input error, 2 usage error,
// 3 (with -strict) at least one probe status differs between inputs.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/BunsDev/wine-compat-probe/internal/compare"
)

const nl = "\n"

func main() {
	fs := flag.NewFlagSet(`compare`, flag.ContinueOnError)
	out := fs.String(`out`, ``, `write the Markdown report to this file (default: stdout)`)
	title := fs.String(`title`, ``, `report title`)
	allowMissing := fs.Bool(`allow-missing`, false, `skip inputs whose file does not exist`)
	strict := fs.Bool(`strict`, false, `exit 3 when any probe status differs between inputs`)
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	inputs, err := compare.ParseInputs(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fs.Usage()
		os.Exit(2)
	}
	opts := compare.Options{Title: *title, AllowMissing: *allowMissing}
	r, err := compare.Build(inputs, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	md := compare.Markdown(r)
	diffs := compare.CountDiffers(r.StatusRows)
	if *out == `` {
		fmt.Print(md)
	} else if err := os.WriteFile(*out, []byte(md), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stderr, `compare: wrote %s (%d probes, %d status differences, %d inputs missing)`+nl, *out, len(r.StatusRows), diffs, len(r.Missing))
	}
	if *strict && diffs > 0 {
		os.Exit(3)
	}
}
