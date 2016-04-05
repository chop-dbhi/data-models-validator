package validator

import "testing"

func TestDateValidator(t *testing.T) {
	if err := DateValidator.Validate("2014-03-20", nil); err != nil {
		t.Errorf("Unexpected error when parsing date: %s", err)
	}

	if err := DateValidator.Validate("2014-03-20 15:03:01", nil); err != nil {
		t.Errorf("Unexpected error when parsing datetime: %s", err)
	}
}
