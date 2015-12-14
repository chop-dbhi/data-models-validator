package validator

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

var (
	validLines = []struct {
		Len    int
		Line   string
		Output []string
	}{
		// Base
		{1, `""`, []string{""}},
		{3, `"a","b","c"`, []string{"a", "b", "c"}},

		// Unquoted empty value
		{2, `"a",`, []string{"a", ""}},
		{2, `,"a"`, []string{"", "a"}},
		{2, `,`, []string{"", ""}},
		{3, `,,`, []string{"", "", ""}},

		// Quotes
		{1, `"""b"""`, []string{`"b"`}},
		{1, `"'b'"`, []string{"'b'"}},
	}

	invalidLines = []struct {
		Len  int
		Line string
	}{
		// Unquoted value.
		{1, "a"},

		// Unescaped quote
		{1, `""a""`},

		// Missing quote
		{2, `"a,"b"`},
		{2, `a","b"`},
	}
)

func compareRows(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

func tableToCSV(t [][]string) []byte {
	buf := bytes.NewBuffer(nil)
	sep := []byte{','}
	nl := []byte{'\n'}

	for _, r := range t {
		for i, c := range r {
			if i != 0 {
				buf.Write(sep)
			}
			if c != "" {
				buf.WriteString(fmt.Sprintf(`"%s"`, c))
			}
		}

		buf.Write(nl)
	}

	return buf.Bytes()
}

func tableToToks(t [][]string) []string {
	var toks []string

	for _, r := range t {
		toks = append(toks, r...)
	}

	return toks
}

func TestCSVReader(t *testing.T) {
	table := [][]string{
		{"name", "gender", "state"},
		{"Joe", "M", "GA"},
		{"Sue", "F", "NJ"},
		{"Bob", "M", "NY"},
		{"Bill", "M", ""}, // trailing comma
	}

	buf := bytes.NewBuffer(tableToCSV(table))
	toks := tableToToks(table)

	cr := DefaultCSVReader(buf)

	var i, c, l int

	for i = 0; cr.Scan(); i++ {
		// Increment line and reset column every three tokens.
		if i%3 == 0 {
			l++
			c = 1
		} else {
			c++
		}

		if i == len(toks) {
			t.Errorf("scan exceeded %d tokens", i+1)
			break
		}

		tok := cr.Text()

		if tok != toks[i] {
			t.Errorf("line %d, column %d: expected %s, got %s", cr.LineNumber(), cr.ColumnNumber(), toks[i], tok)
		}

		if cr.LineNumber() != l {
			t.Errorf("expected line %d, got %d for %s", l, cr.LineNumber(), tok)
		}

		if cr.ColumnNumber() != c {
			t.Errorf("expected column %d, got %d for %s", c, cr.ColumnNumber(), tok)
		}
	}

	if err := cr.Err(); err != io.EOF {
		t.Errorf("unexpected error: %s", err)
	}

	if i != len(toks) {
		t.Errorf("expected %d, got %d", len(toks), i)
	}
}

func TestCSVScanLine(t *testing.T) {
	table := [][]string{
		{"name", "gender", "state"},
		{"Joe", "M", "GA"},
		{"Sue", "F", "NJ"},
		{"Bob", "M", "NY"},
		{"Bill", "M", ""},
	}

	buf := bytes.NewBuffer(tableToCSV(table))

	cr := DefaultCSVReader(buf)

	var (
		i   int
		err error
		row = make([]string, 3)
	)

	for {
		err = cr.ScanLine(row)

		if err == io.EOF {
			break
		}

		if err != nil {
			t.Errorf("%d: unexpected error: %s", i, err)
		}

		if cr.LineNumber() != i+1 {
			t.Errorf("%d: got wrong line number %d", i, cr.LineNumber())
		}

		if !compareRows(table[i], row) {
			t.Errorf("%d: wrong row, got %v", row)
		}

		i++
	}

	if i != 5 {
		t.Errorf("scanned wrong number of lines %d", i)
	}
}

func TestCSVScanLineBadInput(t *testing.T) {
	rows := []string{
		`"name","gender",state`,
		`Joe,"M", "GA"`,
		`"Sue", "F", "NJ"`,
		`"Bob",M,NY"`,
	}

	buf := bytes.NewBuffer([]byte(strings.Join(rows, "\n")))
	cr := DefaultCSVReader(buf)

	var (
		i   int
		err error
		row = make([]string, 3)
	)

	for {
		err = cr.ScanLine(row)

		if err == io.EOF {
			break
		}

		if cr.Line() != rows[i] {
			t.Errorf("%d: bad line `%s`", i, cr.Line())
		}

		if err == nil {
			t.Errorf("%d: expected error", i)
		}

		if cr.LineNumber() != i+1 {
			t.Errorf("%d: got wrong line number %d", i, cr.LineNumber())
		}

		i++
	}

	if i != 4 {
		t.Errorf("scanned wrong number of lines %d", i)
	}
}

func TestCSVReaderBadInput(t *testing.T) {
	rows := []string{
		`"name","gender",state`,
		`Joe,"M", "GA"`,
		`"Sue", "F", "NJ"`,
		`"Bob",M,NY"`,
	}

	expectedToks := []struct {
		Token  string
		Error  bool
		Line   int
		Column int
	}{
		{"name", false, 1, 1},
		{"gender", false, 1, 2},
		{"state", true, 1, 3},
		{`Joe,"M", "GA"`, true, 2, 1},
		{"Sue", false, 3, 1},
		{` "F", "NJ"`, true, 3, 2},
		{"Bob", false, 4, 1},
		{`M,NY"`, true, 4, 2},
	}

	buf := bytes.NewBuffer([]byte(strings.Join(rows, "\n")))
	cr := DefaultCSVReader(buf)

	var (
		err error
		tok string
	)

	for i := 0; cr.Scan(); i++ {
		tok = cr.Text()
		exp := expectedToks[i]

		if cr.LineNumber() != exp.Line {
			t.Errorf("%d: expected line %d, got %d", i, exp.Line, cr.LineNumber())
		}

		if cr.ColumnNumber() != exp.Column {
			t.Errorf("%d: expected column %d, got %d", i, exp.Column, cr.ColumnNumber())
		}

		if exp.Token != tok {
			t.Errorf("%d: expected token `%s`, got `%s`", i, exp.Token, tok)
		}

		err = cr.Err()

		if err == nil && exp.Error {
			t.Errorf("%d: expected error", i)
		} else if err != nil && !exp.Error {
			t.Errorf("%d: unexpected error: %s", i, err)
		}
	}
}

func BenchmarkCSVReaderScan(b *testing.B) {
	cr := DefaultCSVReader(&bytes.Buffer{})

	data := []byte(line)

	for i := 0; i < b.N; i++ {
		_, data, _, _ = cr.scanField(data)

		if len(data) == 0 {
			data = []byte(line)
		}
	}
}
