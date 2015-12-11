package validator

import (
	"bytes"
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

func TestParseCSVLineValid(t *testing.T) {
	// Shared array.
	buf := [3]string{}
	r := bytes.NewBuffer(nil)

	for i, test := range validLines {
		r.Reset()
		r.Write([]byte(test.Line + "\n"))

		row := buf[:test.Len]
		col, err := parseCSVLine(r, row)

		if err != nil {
			t.Errorf("line %d: unexpected error: %s at column %d", i+1, err, col)
			continue
		}

		if !compareRows(row, test.Output) {
			t.Errorf("line %d: expected %v, got %v", i+1, test.Output, row)
		}
	}
}

func TestParseCSVLineInvalid(t *testing.T) {
	// Shared array.
	buf := [3]string{}
	r := bytes.NewBuffer(nil)

	for i, test := range invalidLines {
		r.Reset()
		r.Write([]byte(test.Line + "\n"))

		row := buf[:test.Len]
		_, err := parseCSVLine(r, row)

		if err == nil {
			t.Errorf("line %d: expected error, got %v", i+1, row)
		}
	}
}

func BenchmarkParseCSVLine(b *testing.B) {
	buf := []byte(line)
	r := bytes.NewBuffer(buf)
	rec := [25]string{}

	for i := 0; i < b.N; i++ {
		b.StartTimer()
		parseCSVLine(r, rec[:0])

		b.StopTimer()
		r = bytes.NewBuffer(buf)
	}
}
