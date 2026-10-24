package compare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

func writeDoc(t *testing.T, dir, name string, build func(d *schema.Doc)) string {
	t.Helper()
	d := schema.New(`probe/test`, nil, nil)
	build(d)
	b, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuildDetectsStatusAndValueDifferences(t *testing.T) {
	dir := t.TempDir()
	win := writeDoc(t, dir, `windows.json`, func(d *schema.Doc) {
		d.Set(`runtime`, `classification`, schema.OK(`windows`))
		d.Set(`os`, `version`, schema.OK(`10.0.22631`))
		d.Set(`network`, `icmp_echo`, schema.OK(0).With(`rtt_ms`, 12))
		d.Set(`files`, `symlink`, schema.Fail(`privilege not held`))
	})
	wine := writeDoc(t, dir, `wine.json`, func(d *schema.Doc) {
		d.Set(`runtime`, `classification`, schema.OK(`wine`))
		d.Set(`os`, `version`, schema.OK(`10.0.22631`))
		d.Set(`network`, `icmp_echo`, schema.Fail(`access denied`))
		d.Set(`files`, `symlink`, schema.OK(true))
	})
	inputs := []Input{
		{Label: `windows`, Path: win},
		{Label: `wine`, Path: wine},
	}
	r, err := Build(inputs, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := CountDiffers(r.StatusRows); got != 2 {
		t.Fatalf(`status differences = %d, want 2`, got)
	}
	for _, row := range r.ValueRows {
		if strings.Contains(row.Path, `rtt_ms`) || strings.Contains(row.Path, `timestamp`) {
			t.Fatalf(`volatile path leaked into report: %s`, row.Path)
		}
	}
	md := Markdown(r)
	for _, want := range []string{`## Status matrix`, `network.icmp_echo`, `**yes**`, `files.symlink`, `| windows | wine |`} {
		if !strings.Contains(md, want) {
			t.Fatalf(`markdown missing %q`, want)
		}
	}
}

func TestBuildAllowMissing(t *testing.T) {
	dir := t.TempDir()
	win := writeDoc(t, dir, `windows.json`, func(d *schema.Doc) {
		d.Set(`os`, `version`, schema.OK(`10.0.22631`))
	})
	inputs := []Input{
		{Label: `windows`, Path: win},
		{Label: `proton`, Path: filepath.Join(dir, `missing.json`)},
	}
	if _, err := Build(inputs, Options{}); err == nil {
		t.Fatal(`expected an error for a missing input without AllowMissing`)
	}
	r, err := Build(inputs, Options{AllowMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Missing) != 1 || len(r.Inputs) != 1 {
		t.Fatalf(`missing=%d inputs=%d`, len(r.Missing), len(r.Inputs))
	}
}

func TestParseInputsRejectsBadArgs(t *testing.T) {
	if _, err := ParseInputs([]string{`nolabel`}); err == nil {
		t.Fatal(`expected error for argument without =`)
	}
	if _, err := ParseInputs(nil); err == nil {
		t.Fatal(`expected error for empty input list`)
	}
	in, err := ParseInputs([]string{`wine=results/wine.json`})
	if err != nil || in[0].Label != `wine` || in[0].Path != `results/wine.json` {
		t.Fatalf(`unexpected parse result: %v %v`, in, err)
	}
}
