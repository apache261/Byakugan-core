package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MaxDecimalPrecision = 38
	MaxDecimalScale     = 18
)

// Decimal is an exact, unsigned, canonical decimal value. It deliberately
// marshals as a JSON string and never passes through floating point.
type Decimal string

func ParseDecimal(value string) (Decimal, error) {
	if value == "" {
		return "", fmt.Errorf("decimal is required")
	}
	if strings.TrimSpace(value) != value {
		return "", fmt.Errorf("decimal must not contain surrounding whitespace")
	}
	if strings.ContainsAny(value, "+-eE,_") {
		return "", fmt.Errorf("decimal must be an unsigned plain decimal")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts) == 2 && parts[1] == "" {
		return "", fmt.Errorf("invalid decimal format")
	}
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", fmt.Errorf("invalid decimal digit")
			}
		}
	}
	precision := len(parts[0])
	scale := 0
	if len(parts) == 2 {
		precision += len(parts[1])
		scale = len(parts[1])
	}
	if precision > MaxDecimalPrecision {
		return "", fmt.Errorf("decimal precision exceeds %d", MaxDecimalPrecision)
	}
	if scale > MaxDecimalScale {
		return "", fmt.Errorf("decimal scale exceeds %d", MaxDecimalScale)
	}
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if fraction == "" {
		return Decimal(integer), nil
	}
	return Decimal(integer + "." + fraction), nil
}

func MustDecimal(value string) Decimal {
	decimal, err := ParseDecimal(value)
	if err != nil {
		panic(err)
	}
	return decimal
}

func (d Decimal) Validate() error {
	parsed, err := ParseDecimal(string(d))
	if err != nil {
		return err
	}
	if parsed != d {
		return fmt.Errorf("decimal is not canonical")
	}
	return nil
}

func (d Decimal) String() string { return string(d) }

func (d Decimal) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(string(d))
}

func (d *Decimal) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("decimal cannot be null")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("decimal must be a JSON string: %w", err)
	}
	parsed, err := ParseDecimal(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
