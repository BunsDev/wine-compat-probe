package schema

import (
	"strings"
	"testing"
)

func TestEncodeIsDeterministic(t *testing.T) {
	d := New(`probe/test`, []string{`b`, `a`}, nil)
	d.Set(`z`, `last`, OK(1))
	d.Set(`a`, `first`, Fail(`boom`))
	d.Set(`a`, `second`, Skipped(`no network`))
	b1, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b2, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Fatal(`two encodings of the same document differ`)
	}
	s := string(b1)
	if strings.Index(s, `first`) > strings.Index(s, `second`) {
		t.Fatal(`keys are not sorted`)
	}
	if strings.Index(s, `meta`) > strings.Index(s, `sections`) {
		t.Fatal(`top-level keys are not sorted`)
	}
}

func TestSummaryCounts(t *testing.T) {
	d := New(`probe/test`, nil, nil)
	d.Set(`a`, `one`, OK(true))
	d.Set(`a`, `two`, Fail(`x`))
	d.Set(`b`, `three`, Unsupported(`missing`))
	sum := d.Summary()
	if sum[`total`] != 3 || sum[StatusOK] != 1 || sum[StatusFail] != 1 || sum[StatusUnsupported] != 1 {
		t.Fatalf(`unexpected summary: %v`, sum)
	}
}

func TestWorkaroundsAreSorted(t *testing.T) {
	w := sortedCopy([]string{`c`, `a`, `b`})
	if w[0] != `a` || w[2] != `c` {
		t.Fatalf(`not sorted: %v`, w)
	}
}
