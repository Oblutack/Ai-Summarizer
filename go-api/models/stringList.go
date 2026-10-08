package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList is a list of strings stored in a JSONB column.
type StringList []string

func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	raw, err := json.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func (l *StringList) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = StringList{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot read %T as a list of strings", src)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if out == nil {
		out = []string{}
	}
	*l = out
	return nil
}

// MarshalJSON writes an empty list as [] and never null, so clients can always iterate it.
func (l StringList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(l))
}
