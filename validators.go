package validator

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chop-dbhi/data-models-service/client"
)

const (
	DateLayout     = "2006-01-02"
	DatetimeLayout = "2006-01-02 15:04:05"
)

// isZeroValue returns true if the value is equal to the type's respective
// zero value or is empty in the case of a slice, map, or array.
func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}

	t := reflect.TypeOf(v)

	switch t.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return reflect.ValueOf(v).Len() == 0

	default:
		return v == reflect.Zero(t).Interface()
	}
}

type Context map[string]interface{}

func (c Context) String() string {
	var i int
	toks := make([]string, len(c))

	for k, v := range c {
		if !isZeroValue(v) {
			toks[i] = fmt.Sprintf("%s = %v", k, v)
			i++
		}
	}

	return fmt.Sprintf("{%s}", strings.Join(toks, ", "))
}

type ValidateFunc func(value string, cxt Context) *ValidationError

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

	Validate: func(s string, cxt Context) *ValidationError {
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

			return &ValidationError{
				Err: ErrBadEncoding,
				Context: Context{
					"badRunes": bad,
				},
			}
		}

		return nil
	},
}

var EscapedQuotesValidator = &Validator{
	Name: "EscapedQoutes",

	Description: "Validates any quote characters in a string are escaped.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *ValidationError {
		i := strings.Index(s, `"`)

		for i != -1 {
			if i == len(s)-1 || s[i+1] != '"' {
				return &ValidationError{
					Err: ErrBareQuote,
				}
			} else {
				s = s[i+2:]
			}

			i = strings.Index(s, `"`)
		}

		return nil
	},
}

// IntegerValidator validates the raw value is an integer.
var IntegerValidator = &Validator{
	Name: "Integer",

	Description: "Validates the input string is a valid integer.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *ValidationError {
		if _, err := strconv.ParseInt(s, 10, 32); err != nil {
			return &ValidationError{
				Err: ErrTypeMismatchInt,
			}
		}

		return nil
	},
}

// BigIntegerValidator validates the raw value is an integer.
var BigIntegerValidator = &Validator{
	Name: "BigInteger",

	Description: "Validates the input string is a valid BigInteger.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *ValidationError {
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return &ValidationError{
				Err: ErrTypeMismatchInt,
			}
		}

		return nil
	},
}

// NumberValidator validates the raw value is a number.
var NumberValidator = &Validator{
	Name: "Number",

	Description: "Validates the input string is a valid number (float).",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *ValidationError {
		if _, err := strconv.ParseFloat(s, 32); err != nil {
			return &ValidationError{
				Err: ErrTypeMismatchNum,
			}
		}

		return nil
	},
}

// DateValidator validates the raw value is date.
var DateValidator = &Validator{
	Name: "Date",

	Description: "Validates the input value is a valid date.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *ValidationError {
		if _, err := time.Parse(DateLayout, s); err != nil {
			// Since dates are a subset of datetimes, a datetime is also
			// a valid date. The consumer will need to handle using only
			// the date portion.
			if err := DatetimeValidator.Validate(s, cxt); err == nil {
				return nil
			}

			return &ValidationError{
				Err: ErrTypeMismatchDate,
			}
		}

		return nil
	},
}

// DatetimeValidator validates the raw value is date.
var DatetimeValidator = &Validator{
	Name: "Datetime",

	Description: "Validates the input value is a valid date time.",

	RequiresValue: true,

	Validate: func(s string, cxt Context) *ValidationError {
		if _, err := time.Parse(DatetimeLayout, s); err != nil {
			return &ValidationError{
				Err: ErrTypeMismatchDateTime,
			}
		}

		return nil
	},
}

// RequiredValidator validates the the raw value is not empty. This only applies
// to fields that are marked as required in the spec.
var RequiredValidator = &Validator{
	Name: "Required",

	Description: "Validates the input value is not empty.",

	Validate: func(s string, cxt Context) *ValidationError {
		if s == "" {
			return &ValidationError{
				Err: ErrRequiredValue,
			}
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

	Validate: func(s string, cxt Context) *ValidationError {
		length := cxt["length"].(int)

		if len(s) > length {
			return &ValidationError{
				Err: ErrLengthExceeded,
				Context: Context{
					"maxLength": length,
				},
			}
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

func (b *BoundValidator) Validate(s string) *ValidationError {
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
	// vs = append(vs, Bind(EscapedQuotesValidator, nil))

	if f.Required {
		vs = append(vs, Bind(RequiredValidator, nil))
	}

	// Add type-specific validators.
	switch f.Type {
	case "string", "clob", "text":
		if f.Length > 0 {
			vs = append(vs, Bind(StringLengthValidator, Context{"length": f.Length}))
		}
	case "integer":
		vs = append(vs, Bind(IntegerValidator, nil))
	case "biginteger":
		vs = append(vs, Bind(BigIntegerValidator, nil))	
	case "number", "float", "decimal":
		vs = append(vs, Bind(NumberValidator, nil))
	case "date":
		vs = append(vs, Bind(DateValidator, nil))
	case "datetime", "timestamp":
		vs = append(vs, Bind(DatetimeValidator, nil))
	default:
		log.Printf("no validator for type '%s'", f.Type)
	}

	return vs
}
