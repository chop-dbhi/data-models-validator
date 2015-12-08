package validator

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/chop-dbhi/data-models-service/client"
)

// Plan is composed of the set of validators used to evaluate
// the field values.
type Plan struct {
	FieldValidators map[string][]*BoundValidator
}

type TableValidator struct {
	Fields *client.Fields
	Header []string

	Plan   *Plan
	result *Result

	errs   int
	length int
	reader io.Reader
	csv    *greedyCSVReader

	// Mapped field index to field.
	fields map[int]*client.Field
}

func (t *TableValidator) validateRow(row []string) error {
	var (
		i    int
		verr *ValidationError
		v    string
		bv   *BoundValidator
		f    *client.Field
	)

	// Line level error, individual fields are not inspected since they
	// may be shifted relative to the header.
	if len(row) != t.length {
		t.result.LogError(&ValidationError{
			Line: t.csv.line,
			Err:  ErrExtraColumns,
			Context: Context{
				"expected": t.length,
				"actual":   len(row),
			},
		})

		return nil
	}

	// Validate each value mapped to the respective field in the line.
	for i, v = range row {
		f = t.fields[i]

		// Run through all the validators.
		for _, bv = range t.Plan.FieldValidators[f.Name] {
			if bv.Validator.RequiresValue && v == "" {
				continue
			}

			if verr = bv.Validate(v); verr != nil {
				t.result.LogError(&ValidationError{
					Err:     verr.Err,
					Line:    t.csv.line,
					Field:   f.Name,
					Value:   v,
					Context: verr.Context,
				})

				t.errs++
				break
			}
		}
	}

	return nil
}

// Init initializes the validator by checking the header and compiling
// a set of validators for each field.
func (t *TableValidator) Init() error {
	var (
		err       error
		head      []string
		lengthErr bool
		matchErr  bool
	)

	if head, err = t.csv.Read(); err != nil {
		return err
	}

	if len(head) != t.length {
		lengthErr = true
	}

	t.Header = head
	for i, s := range t.Header {
		t.Header[i] = strings.ToLower(s)
	}

	valid := make(map[string]int)
	fields := make(map[int]*client.Field)
	unknown := make([]string, 0)
	missing := make([]string, 0)

	// Check if all fields in the header are expected.
	for i, name := range t.Header {
		if f := t.Fields.Get(name); f != nil {
			valid[name] = i
			fields[i] = f
		} else {
			unknown = append(unknown, name)
		}
	}

	// Check if any fields are missing from the header.
	for _, f := range t.Fields.List() {
		if _, ok := valid[f.Name]; !ok {
			missing = append(missing, f.Name)
		}
	}

	// Set of fields by position mapped to schema.
	t.fields = fields

	if len(unknown) > 0 || len(missing) > 0 {
		matchErr = true
	}

	if lengthErr || matchErr {
		return &ValidationError{
			Err: ErrBadHeader,
			Context: Context{
				"expectedLength": t.length,
				"actualLength":   len(head),
				"unknownFields":  unknown,
				"missingFields":  missing,
			},
		}
	}

	// Compile a list of validators per field.
	t.Plan.FieldValidators = make(map[string][]*BoundValidator, len(t.fields))

	for _, f := range t.fields {
		t.Plan.FieldValidators[f.Name] = BindFieldValidators(f)
	}

	return nil
}

// Next reads the next row and validates it.
func (t *TableValidator) Next() error {
	row, err := t.csv.Read()

	if err != nil {
		// Log and ignore.
		if verr, ok := err.(*ValidationError); ok {
			t.result.LogError(verr)
			return nil
		}

		// EOF or unexpected error.
		return err
	}

	return t.validateRow(row)
}

// Run executes all of the validators for the input. All parse and validation
// errors are handled so the only error that should stop the validator is EOF.
func (t *TableValidator) Run() error {
	var err error

	for {
		if err = t.Next(); err != nil {
			break
		}
	}

	if err == nil || err == io.EOF {
		return nil
	}

	return err
}

// Result returns the result of the validation.
func (t *TableValidator) Result() *Result {
	return t.result
}

// New takes an io.Reader and validates it against a data model table.
func New(reader io.Reader, table *client.Table) *TableValidator {
	cr := newGreedyCSVReader(reader, table.Fields.Len())

	return &TableValidator{
		Fields: table.Fields,
		Plan:   new(Plan),
		length: table.Fields.Len(),
		reader: reader,
		csv:    cr,
		result: NewResult(),
	}
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
