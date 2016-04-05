package validator

import (
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
	csv    *CSVReader

	// Mapped field index to field.
	fields map[int]*client.Field
	record []string
}

func (t *TableValidator) validateRow(row []string) error {
	// Line level error, individual fields are not inspected since they
	// may be shifted relative to the header.
	if len(row) != t.length {
		t.result.LogError(&ValidationError{
			Value: t.csv.Line(),
			Line:  t.csv.LineNumber(),
			Err:   ErrExtraColumns,
			Context: Context{
				"expected": t.length,
				"actual":   len(row),
			},
		})

		return nil
	}

	// Validate each value mapped to the respective field in the line.
	for i, v := range row {
		f := t.fields[i]

		// Run through all the validators.
		for _, bv := range t.Plan.FieldValidators[f.Name] {
			if bv.Validator.RequiresValue && v == "" {
				continue
			}

			if verr := bv.Validate(v); verr != nil {
				t.result.LogError(&ValidationError{
					Err:     verr.Err,
					Line:    t.csv.LineNumber(),
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
	t.record = make([]string, len(head))

	if len(unknown) > 0 || len(missing) > 0 {
		matchErr = true
	}

	if lengthErr || matchErr {
		return &ValidationError{
			Err:   ErrBadHeader,
			Value: t.csv.Line(),
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

// Next reads the next row and validates it. Row and field level errors are logged and
// not returned. Errors that are returned are EOF and unexpected errors.
func (t *TableValidator) Next() error {
	err := t.csv.ScanLine(t.record)

	if err != nil {
		switch err {
		case csvErrUnquotedField:
			err = ErrUnquotedColumn
		case csvErrUnterminatedField:
			err = ErrUnterminatedColumn
		case csvErrUnescapedQuote:
			err = ErrBareQuote
		case csvErrExtraColumns:
			err = ErrExtraColumns
		}

		switch x := err.(type) {
		case *Error:
			t.result.LogError(&ValidationError{
				Err:   x,
				Value: t.csv.Line(),
				Line:  t.csv.LineNumber(),
				Context: Context{
					"column": t.csv.ColumnNumber(),
				},
			})

			// Return nil so caller knows to continue.
			return nil
		}

		// EOF or unexpected error.
		return err
	}

	return t.validateRow(t.record)
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
	cr := DefaultCSVReader(reader)

	return &TableValidator{
		Fields: table.Fields,
		Plan:   new(Plan),
		length: table.Fields.Len(),
		reader: reader,
		csv:    cr,
		result: NewResult(),
	}
}
