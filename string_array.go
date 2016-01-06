package pq_types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// StringArray type compatible with varchar[] in PostgreSQL
// FIXME doesn't work for any string (for example, doesn't work with spaces)
type StringArray []string

// Value implements database/sql/driver Valuer interface
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}

	res := make([]string, len(a))
	for i, e := range a {
		r := e
		r = strings.Replace(r, `\`, `\\`, -1)
		r = strings.Replace(r, `"`, `\"`, -1)
		res[i] = `"` + r + `"`
	}
	return []byte("{" + strings.Join(res, ",") + "}"), nil
}

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}

	v, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("StringArray.Scan: expected []byte, got %T (%q)", value, value)
	}

	if len(v) < 2 || v[0] != '{' || v[len(v)-1] != '}' {
		return fmt.Errorf("StringArray.Scan: unexpected data %q", v)
	}

	*a = (*a)[:0]
	if len(v) == 2 { // '{}'
		return nil
	}

	reader := bytes.NewReader(v[1 : len(v)-1]) // skip '{' and '}'

	// helper function to read next rune and check if it valid
	readRune := func() (rune, error) {
		r, _, err := reader.ReadRune()
		if err != nil {
			return 0, err
		}
		if r == unicode.ReplacementChar {
			return 0, fmt.Errorf("StringArray.Scan: invalid rune")
		}
		return r, nil
	}

	var q bool
	var e []rune
	for {
		// read next rune and check if we are done
		r, err := readRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch r {
		case '"':
			// enter or leave quotes
			q = !q
			continue
		case ',':
			// end of element unless in we are in quotes
			if !q {
				*a = append(*a, string(e))
				e = e[:0]
				continue
			}
		case '\\':
			// skip to next rune, it should be present
			n, err := readRune()
			if err != nil {
				return err
			}
			r = n
		}

		e = append(e, r)
	}

	// we should not be in quotes at this point
	if q {
		panic("StringArray.Scan bug")
	}

	// add last element
	*a = append(*a, string(e))
	return nil
}

// check interfaces
var (
	_ driver.Valuer = StringArray{}
	_ sql.Scanner   = &StringArray{}
)
