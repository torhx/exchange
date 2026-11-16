package exchange

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
)

const (
	// A code may be shorter or longer than the four characters of an ISO
	// 10383 market identifier, because not every venue has one. See Code.
	minCodeLen = 2
	maxCodeLen = 12
)

// A Code identifies an exchange: its ISO 10383 market identifier where it
// has one, such as XNYS, and a name of the same kind where it has none, such
// as COINBASE. It is 2 to 12 upper-case letters or digits.
//
// A Code is deliberately not a market identifier code (MIC) type, which is
// what a reader may expect here. A MIC is always four characters, and a type
// held to that could not name a venue that ISO 10383 does not list. Several
// large crypto exchanges are not listed, or are listed only for one legal
// entity or for their derivatives arm, so that their identifier does not
// mean the venue people trade on. Such a venue would need an invented
// four-character code, which a later ISO assignment could collide with.
//
// The four-character rule would also promise little. It checks the shape of
// a code and cannot tell a real identifier from a made-up one without the
// whole ISO list. So a Code accepts any well-formed name, every real MIC is
// a valid Code, and the built-in exchanges use their MIC wherever they have
// one.
//
// A Code comes only from a constructor, so it is always well formed. Codes
// are comparable with == and usable as map keys, which makes a Code the way
// to tell whether two exchanges are the same venue.
//
// The zero value is not a code. It stands for a missing one, IsValid reports
// false for it, and other methods are meaningless on it.
type Code struct {
	// value is the code in upper case.
	value string
}

// NewCode returns the code written in s. The code may be in any letter case,
// and white space around it is ignored: "xnys" and " XNYS " both give XNYS.
//
// It returns an error if s is not 2 to 12 letters or digits. It does not
// check that an exchange with the code exists, or that a four-character
// code is a registered market identifier.
func NewCode(s string) (Code, error) {
	c := Code{value: normalizeCode(s)}
	if err := c.validate(); err != nil {
		return Code{}, fmt.Errorf("invalid exchange code %q: %w", s, err)
	}

	return c, nil
}

// IsValid reports whether c is a code. It is false only for the zero value,
// which stands for a missing one; every Code from a constructor is valid.
func (c Code) IsValid() bool { return c.value != "" }

// String returns the code as text, such as "XNYS".
func (c Code) String() string { return c.value }

// MarshalText returns the code as text. It never fails. It makes Code an
// encoding.TextMarshaler, which encoding/json uses for values and for map
// keys.
func (c Code) MarshalText() ([]byte, error) {
	return []byte(c.value), nil
}

// UnmarshalText sets c to the code that NewCode reads from text. If the text
// is not a code, it returns the error and leaves c as it was. It makes *Code
// an encoding.TextUnmarshaler.
func (c *Code) UnmarshalText(text []byte) error {
	parsed, err := NewCode(string(text))
	if err != nil {
		return err
	}

	*c = parsed

	return nil
}

// Value returns the code as text for use as a query argument. It makes Code
// a driver.Valuer.
func (c Code) Value() (driver.Value, error) {
	return c.value, nil
}

// Scan sets c from a database column value. It makes *Code a sql.Scanner.
//
// A string or []byte must be text that NewCode accepts. NULL is an error;
// scan a nullable column into a sql.Null[Code]. On error c is unchanged.
func (c *Code) Scan(src any) error {
	switch value := src.(type) {
	case string:
		return c.UnmarshalText([]byte(value))
	case []byte:
		return c.UnmarshalText(value)
	case nil:
		return errors.New("invalid exchange code: NULL")
	default:
		return fmt.Errorf("invalid exchange code: unsupported type %T", src)
	}
}

func (c Code) validate() error {
	if len(c.value) < minCodeLen || len(c.value) > maxCodeLen {
		return fmt.Errorf("must be %d to %d letters or digits", minCodeLen, maxCodeLen)
	}

	for i := range len(c.value) {
		isLetter := c.value[i] >= 'A' && c.value[i] <= 'Z'
		isDigit := c.value[i] >= '0' && c.value[i] <= '9'

		if !isLetter && !isDigit {
			return fmt.Errorf("must be %d to %d letters or digits", minCodeLen, maxCodeLen)
		}
	}

	return nil
}

// normalizeCode returns s as a code is stored: without surrounding white
// space and with its letters in upper case. Only ASCII letters change, so
// that no other letter turns into one a code may hold.
func normalizeCode(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}

		return r
	}, strings.TrimSpace(s))
}
