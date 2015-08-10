package validator

import (
	"log"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/chop-dbhi/data-models-service/client"
)

const (
	DateLayout     = "2006-01-02"
	DatetimeLayout = "2006-01-02 15:04:05"
)

type Context map[string]interface{}

type ValidateFunc func(value string, cxt Context) *Error

type Validator struct {
	Name          string
	Description   string
	Validate      ValidateFunc
	RequiresValue bool
}

func (v *Validator) String() string {
	return v.Name
}

var EncodingValidator = &Validator{
	Name: "Encoding",

	Description: "Validates a string only contains utf-8 characters.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *Error {
		if !utf8.ValidString(s) {
			var bad []rune

			for i, r := range s {
				if r == utf8.RuneError {
					bs, size := utf8.DecodeRuneInString(s[i:])

					if size == 1 {
						bad = append(bad, bs)
					}
				}
			}

			return ErrBadEncoding
		}

		return nil
	},
}

// IntegerValidator validates the raw value is an integer.
var IntegerValidator = &Validator{
	Name: "Integer",

	Description: "Validates the input string is a valid integer.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *Error {
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return ErrTypeMismatch
		}

		return nil
	},
}

// NumberValidator validates the raw value is a number.
var NumberValidator = &Validator{
	Name: "Number",

	Description: "Validates the input string is a valid number (float).",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *Error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return ErrTypeMismatch
		}

		return nil
	},
}

// DateValidator validates the raw value is date.
var DateValidator = &Validator{
	Name: "Date",

	Description: "Validates the input value is a valid date.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *Error {
		if _, err := time.Parse(DateLayout, s); err != nil {
			return ErrTypeMismatch
		}

		return nil
	},
}

// DatetimeValidator validates the raw value is date.
var DatetimeValidator = &Validator{
	Name: "Datetime",

	Description: "Validates the input value is a valid date time.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *Error {
		if _, err := time.Parse(DatetimeLayout, s); err != nil {
			return ErrTypeMismatch
		}

		return nil
	},
}

// RequiredValidator validates the the raw value is not empty. This only applies
// to fields that are marked as required in the spec.
var RequiredValidator = &Validator{
	Name: "Required",

	Description: "Validates the input value is not empty.",

	Validate: func(s string, cxt Context) *Error {
		if s == "" {
			return ErrRequiredValue
		}

		return nil
	},
}

// StringLengthValidator validates the string value does not exceed a
// pre-defined length.
var StringLengthValidator = &Validator{
	Name: "String Length",

	Description: "Validates the input value is not longer than a pre-defined length.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *Error {
		length := cxt["length"].(int)

		if len(s) > length {
			return ErrLengthExceeded
		}

		return nil
	},
}

// BoundValidator binds a validator to a context.
type BoundValidator struct {
	Validator *Validator
	Context   Context
}

func (b *BoundValidator) String() string {
	return b.Validator.String()
}

func (b *BoundValidator) Validate(s string) *Error {
	return b.Validator.Validate(s, b.Context)
}

// Bind returns a bound validator given a validator and context.
func Bind(v *Validator, cxt Context) *BoundValidator {
	return &BoundValidator{
		Validator: v,
		Context:   cxt,
	}
}

// BindFieldValidators returns a set of validators for the field.
func BindFieldValidators(f *client.Field) []*BoundValidator {
	var vs []*BoundValidator

	vs = append(vs, Bind(EncodingValidator, nil))

	if f.Required {
		vs = append(vs, Bind(RequiredValidator, nil))
	}

	// Add type-specific validators.
	switch f.Type {
	case "string", "clob":
		if f.Length > 0 {
			vs = append(vs, Bind(StringLengthValidator, Context{"length": f.Length}))
		}
	case "integer":
		vs = append(vs, Bind(IntegerValidator, nil))
	case "number":
		vs = append(vs, Bind(NumberValidator, nil))
	case "date":
		vs = append(vs, Bind(DateValidator, nil))
	case "datetime":
		vs = append(vs, Bind(DatetimeValidator, nil))
	default:
		log.Printf("no validator for type '%s'", f.Type)
	}

	return vs
}
