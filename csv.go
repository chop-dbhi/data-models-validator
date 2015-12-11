// Adapted from: https://github.com/gwenn/yacr/blob/b33898940948270a0198c7db28d6b7efc18b783e/reader.go
package validator

import (
	"bufio"
	"bytes"
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
	eor    bool // true when the most recent field has been terminated by a newline (not a separator).
	lineno int  // current line number (not record number)
	column int  // current column index 1-based

	Comment byte // character marking the start of a line comment. When specified (not 0), line comment appears as empty line.
}

// DefaultReader creates a "standard" CSV reader.
func DefaultCSVReader(rd io.Reader) *CSVReader {
	return NewCSVReader(rd, ',')
}

// NewReader returns a new CSV scanner.
func NewCSVReader(r io.Reader, sep byte) *CSVReader {
	s := &CSVReader{bufio.NewScanner(r), sep, true, 1, 0, 0}
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

	// No data.
	if len(data) == 0 {
		return 0, nil, nil
	}

	// Comment.
	if s.eor && s.Comment != 0 && data[0] == s.Comment {
		for i, c := range data {
			if c == '\n' {
				s.lineno++
				return i + 1, nil, nil
			}
		}

		if atEOF {
			return len(data), nil, nil
		}

		return 0, nil, nil
	}

	if data[0] == '"' {
		var c, pc, ppc byte

		startLineno := s.lineno
		escapedQuotes := 0
		strict := true

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

	} else {
		// Unquoted empty fields are allowed.
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

			// Unquoted values are not allowed.
			return 0, nil, fmt.Errorf("unquoted field at line %d, column %d", s.lineno, s.column)
		}
		// If we're at EOF, we have a final field. Return it.
		if atEOF {
			s.eor = true
			return len(data), data, nil
		}
	}

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

// greedyCSVReader attempts to read and parse all lines in a CSV file
// regardless if there are errors.
type greedyCSVReader struct {
	buf    *bytes.Buffer
	sc     *bufio.Scanner
	line   int
	record []string
}

// Read scans the line, writes to the buffer, and then reads as CSV.
// The error returned will contain the line
func (r *greedyCSVReader) Read() ([]string, error) {
	r.line++

	// Exit if the scanner is done, either an error or EOF.
	if !r.sc.Scan() {
		err := r.sc.Err()

		if err == nil {
			err = io.EOF
		}

		return nil, err
	}

	// Read the line as bytes, the newline is intact.
	line := r.sc.Bytes()

	// Error is always nil, per the docs.
	// http://golang.org/pkg/bytes/index.html#Buffer.Write
	r.buf.Write(line)

	// Attempt to read buffered line as CSV data.
	col, err := parseCSVLine(r.buf, r.record)

	// Problem parsing as CSV.
	// EOF would have been caught by the scanner.
	if err != nil {
		err = &ValidationError{
			Err:   ErrBareQuote,
			Line:  r.line,
			Value: string(line),
			Context: Context{
				"column": col,
			},
		}
	}

	// Clear the buffer for the next line.
	r.buf.Reset()

	// Return intended error.
	if err != nil {
		return nil, err
	}

	return r.record, nil
}

func newGreedyCSVReader(r io.Reader, size int) *greedyCSVReader {
	sc := bufio.NewScanner(r)

	buf := bytes.NewBuffer(nil)

	return &greedyCSVReader{
		sc:     sc,
		buf:    buf,
		record: make([]string, size),
	}
}

func parseCSVLine(r io.Reader, t []string) (int, error) {
	cr := DefaultCSVReader(r)
	cr.Comment = '#'
	i := 0
	m := len(t)

	for cr.Scan() {
		if i == m {
			return cr.Column(), fmt.Errorf("too many columns. expected %d", m)
		}

		t[i] = cr.Text()
		i++

		if cr.EndOfRecord() {
			break
		}
	}

	return cr.Column(), cr.Err()
}
