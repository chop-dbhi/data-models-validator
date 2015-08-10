package validator

import (
	"encoding/csv"
	"io"

	"github.com/chop-dbhi/data-models-service/client"
)

const DefaultMaxErrors = 10

// Plan is composed of the set of validators used to evaluate
// the field values.
type Plan struct {
	FieldValidators map[string][]*BoundValidator
}

type TableValidator struct {
	Fields *client.Fields
	Header []string

	MaxErrors int

	Plan   *Plan
	result *Result

	errs   int
	length int
	reader io.Reader
	csv    *csv.Reader
	line   int

	// Mapped field index to field.
	fields map[int]*client.Field
}

func (t *TableValidator) validateRow(row []string) error {
	var (
		i   int
		err *Error
		v   string
		bv  *BoundValidator
		f   *client.Field
	)

	if len(row) != t.length {
		return &ValidationError{
			Line: t.line,
			Err:  ErrExtraColumns,
			Context: Context{
				"expected": t.length,
				"actual":   len(row),
			},
		}
	}

	// Validate each value mapped to the respective field in the line.
	for i, v = range row {
		if t.errs >= t.MaxErrors {
			return ErrTooManyErrors
		}

		f = t.fields[i]

		// Run through all the validators.
		for _, bv = range t.Plan.FieldValidators[f.Name] {
			if bv.Validator.RequiresValue && v == "" {
				continue
			}

			if err = bv.Validate(v); err != nil {
				t.result.LogError(&ValidationError{
					Err:   err,
					Line:  t.line,
					Field: f.Name,
					Value: v,
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
		err  error
		head []string
	)

	if head, err = t.csv.Read(); err != nil {
		return err
	}

	if len(head) != t.length {
		return &ValidationError{
			Err: ErrExtraColumns,
			Context: Context{
				"expected": t.length,
				"actual":   len(head),
			},
		}
	}

	t.Header = head

	valid := make(map[string]int)
	fields := make(map[int]*client.Field)
	extra := make(map[string]int)
	missing := make([]string, 0)

	// Check if all fields in the header are expected.
	for i, name := range t.Header {
		if f := t.Fields.Get(name); f != nil {
			valid[name] = i
			fields[i] = f
		} else {
			extra[name] = i
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

	if len(extra) > 0 || len(missing) > 0 {
		return &ValidationError{
			Err: ErrExtraColumns,
			Context: Context{
				"extra":   extra,
				"missing": missing,
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
		return err
	}

	t.line++

	return t.validateRow(row)
}

// Run executes all of the validators for the input.
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

func New(reader io.Reader, table *client.Table) *TableValidator {
	cr := csv.NewReader(reader)

	cr.Comment = '#'
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	return &TableValidator{
		Fields:    table.Fields,
		Plan:      new(Plan),
		MaxErrors: DefaultMaxErrors,
		length:    table.Fields.Len(),
		reader:    reader,
		csv:       cr,
		result:    NewResult(),
	}
}
