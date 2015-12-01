// Adapted from: https://github.com/gwenn/yacr/blob/b33898940948270a0198c7db28d6b7efc18b783e/reader.go
package validator

import (
	"bufio"
	"fmt"
	"io"
)

// CSVReader provides an interface for reading CSV data
// (compatible with rfc4180 and extended with the option of having a separator other than ",").
// Successive calls to the Scan method will step through the 'fields', skipping the separator/newline between the fields.
// The EndOfRecord method tells when a field is terminated by a line break.
type CSVReader struct {
	*bufio.Scanner
	sep    byte // values separator
	quoted bool // specify if values may be quoted (when they contain separator or newline)
	eor    bool // true when the most recent field has been terminated by a newline (not a separator).
	lineno int  // current line number (not record number)
	column int  // current column index 1-based

	Comment byte // character marking the start of a line comment. When specified (not 0), line comment appears as empty line.
}

// DefaultReader creates a "standard" CSV reader (separator is comma and quoted mode active)
func DefaultCSVReader(rd io.Reader) *CSVReader {
	return NewCSVReader(rd, ',', true)
}

// NewReader returns a new CSV scanner to read from r.
// When quoted is false, values must not contain a separator or newline.
func NewCSVReader(r io.Reader, sep byte, quoted bool) *CSVReader {
	s := &CSVReader{bufio.NewScanner(r), sep, quoted, true, 1, 0, 0}
	s.Split(s.ScanField)
	return s
}

// LineNumber returns current line number (not record number)
func (s *CSVReader) LineNumber() int {
	return s.lineno
}

// Column returns the column index of the current field.
func (s *CSVReader) Column() int {
	return s.column
}

// EndOfRecord returns true when the most recent field has been terminated by a newline (not a separator).
func (s *CSVReader) EndOfRecord() bool {
	return s.eor
}

// ScanField implements bufio.SplitFunc for CSV.
// Lexing is adapted from csv_read_one_field function in SQLite3 shell sources.
func (s *CSVReader) ScanField(data []byte, atEOF bool) (advance int, token []byte, err error) {
	var a int

	for {
		a, token, err = s.scanField(data, atEOF)
		advance += a

		if err != nil || a == 0 || token != nil {
			return
		}

		data = data[a:]
	}
}

func (s *CSVReader) scanField(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if s.eor {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		s.column = 0
	}

	s.column++

	if s.quoted && len(data) > 0 && data[0] == '"' { // quoted field (may contains separator, newline and escaped quote)
		startLineno := s.lineno
		escapedQuotes := 0
		strict := true

		var c, pc, ppc byte

		// Scan until the separator or newline following the closing quote (and ignore escaped quote)
		for i := 1; i < len(data); i++ {
			c = data[i]

			// Assume next line, check below whether this is valid.
			if c == '\n' {
				s.lineno++
			} else if c == '"' {
				// Successive quotes to denote an escaped quote
				if pc == c {
					// Reset previous character.
					pc = 0
					escapedQuotes++
					continue
				}
			}

			// End of field.
			if pc == '"' && c == s.sep {
				s.eor = false
				return i + 1, unescapeQuotes(data[1:i-1], escapedQuotes, strict), nil
			}

			// End of record; newline.
			if pc == '"' && c == '\n' {
				s.eor = true
				return i + 1, unescapeQuotes(data[1:i-1], escapedQuotes, strict), nil
			}

			// End of record; newline and carriage return.
			if c == '\n' && pc == '\r' && ppc == '"' {
				s.eor = true
				return i + 1, unescapeQuotes(data[1:i-2], escapedQuotes, strict), nil
			}

			//
			if pc == '"' && c != '\r' {
				return 0, nil, fmt.Errorf("unescaped %c character at line %d, column %d", pc, s.lineno, s.column)
			}

			// Shift previous characters.
			ppc = pc
			pc = c
		}

		if atEOF {
			if c == '"' {
				s.eor = true
				return len(data), unescapeQuotes(data[1:len(data)-1], escapedQuotes, strict), nil
			}

			// If we're at EOF, we have a non-terminated field.
			return 0, nil, fmt.Errorf("non-terminated quoted field at line %d, column %d", startLineno, s.column)
		}
	} else if s.eor && s.Comment != 0 && len(data) > 0 && data[0] == s.Comment { // line comment
		for i, c := range data {
			if c == '\n' {
				s.lineno++
				return i + 1, nil, nil
			}
		}
		if atEOF {
			return len(data), nil, nil
		}
	} else { // unquoted field
		// Scan until separator or newline, marking end of field.
		for i, c := range data {
			if c == s.sep {
				s.eor = false
				return i + 1, data[0:i], nil
			} else if c == '\n' {
				s.lineno++
				if i > 0 && data[i-1] == '\r' {
					s.eor = true
					return i + 1, data[0 : i-1], nil
				}
				s.eor = true
				return i + 1, data[0:i], nil
			}
		}
		// If we're at EOF, we have a final field. Return it.
		if atEOF {
			s.eor = true
			return len(data), data, nil
		}
	}

	// Request more data.
	return 0, nil, nil
}

func unescapeQuotes(b []byte, count int, strict bool) []byte {
	if count == 0 {
		return b
	}
	for i, j := 0, 0; i < len(b); i, j = i+1, j+1 {
		b[j] = b[i]

		if b[i] == '"' && (strict || i < len(b)-1 && b[i+1] == '"') {
			i++
		}
	}
	return b[:len(b)-count]
}
