package validator

import (
	"errors"
	"fmt"
)

// ErrTooManyErrors is returned when the maximum errors have been reached
// during validation. This is done to prevent overloading the output with
// so many errors that it makes it difficult to understand, iterate, and fix
// the first set of the problems.
var ErrTooManyErrors = errors.New("too many errors")

// Error defines a specific type of error denoted by the description. A code is
// defined as a shorthand for the error and to act as a lookup for the error itself.
// Errors are classified by code:
//     - 1xx: encoding related issues
//     - 2xx: parse related issues
//     - 3xx: value related issues
type Error struct {
	Code        int
	Description string
}

func (e Error) Error() string {
	return fmt.Sprintf("[code: %d] %s", e.Code, e.Description)
}

var ErrBadEncoding = &Error{
	Code:        100,
	Description: "UTF-8 encoding required",
}

var ErrBadHeader = &Error{
	Code:        201,
	Description: "Header does not contain the correct set of fields",
}

var ErrExtraColumns = &Error{
	Code:        202,
	Description: "Extra columns were detected in line",
}

var ErrRequiredValue = &Error{
	Code:        300,
	Description: "Value is required",
}

var ErrTypeMismatch = &Error{
	Code:        301,
	Description: "Value is not the correct type",
}

var ErrLengthExceeded = &Error{
	Code:        302,
	Description: "Value exceeds the maximum length",
}

var ErrPrecisionExceeded = &Error{
	Code:        303,
	Description: "Numeric precision exceeded",
}

var ErrScaleExceeded = &Error{
	Code:        304,
	Description: "Numeric scale exceeded",
}

// Map of errors by code.
var Errors = map[int]*Error{
	100: ErrBadEncoding,

	201: ErrBadHeader,
	202: ErrExtraColumns,

	300: ErrRequiredValue,
	301: ErrTypeMismatch,
	302: ErrLengthExceeded,
	303: ErrPrecisionExceeded,
	304: ErrScaleExceeded,
}

// ValidationError is composed of an error with an optional line and
// and field the error is specific to. Additional context can be supplied
// in the context field, however it is not currently included in the error
// message.
type ValidationError struct {
	Err     *Error
	Line    int
	Field   string
	Value   string
	Context Context
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Err)
	}

	return fmt.Sprintf("line %d, field %s: %s", e.Line, e.Field, e.Err)
}

// Result maintains the validation results currently consisting of
// validation errors.
type Result struct {
	Errors []*ValidationError

	// Index by field, grouped error code.
	fieldIndex map[string]map[*Error][]*ValidationError
}

// LogError logs an error to the result.
func (r *Result) LogError(verr *ValidationError) {
	r.Errors = append(r.Errors, verr)

	if verr.Field != "" {
		errs, ok := r.fieldIndex[verr.Field]

		if !ok {
			errs = make(map[*Error][]*ValidationError)
			r.fieldIndex[verr.Field] = errs
		}

		errs[verr.Err] = append(errs[verr.Err], verr)
	}
}

// FieldErrors returns errors for field grouped by error code.
func (r *Result) FieldErrors(f string) map[*Error][]*ValidationError {
	return r.fieldIndex[f]
}

func NewResult() *Result {
	return &Result{
		fieldIndex: make(map[string]map[*Error][]*ValidationError),
	}
}
