package domain

import (
	"fmt"
	"math/big"
	"strings"
)

// FraudCurrencyCatalogVersion identifies the rail-approved native-currency scale table.
const FraudCurrencyCatalogVersion = "iso4217-fraud-v1"

var fraudCurrencyScales = map[string]int{
	"PHP": 2, "USD": 2, "EUR": 2, "GBP": 2, "AUD": 2, "CAD": 2,
	"SGD": 2, "HKD": 2, "CNY": 2, "JPY": 0, "KRW": 0,
	"KWD": 3, "BHD": 3, "OMR": 3,
}

// CurrencyScale is intentionally an approved subset. Unapproved codes fail closed.
func CurrencyScale(currency string) (int, bool) {
	scale, ok := fraudCurrencyScales[currency]
	return scale, ok
}

// CanonicalAmount accepts only unsigned plain decimals at the currency scale.
// It caps the integer at 18 digits so all consumers have a bounded exact value.
func CanonicalAmount(value, currency string) (string, error) {
	return canonicalAmount(value, currency, false, 18)
}

func CanonicalNonnegativeAmount(value, currency string) (string, error) {
	return canonicalAmount(value, currency, true, 18)
}

func canonicalAmount(value, currency string, allowZero bool, maxDigits int) (string, error) {
	scale, ok := CurrencyScale(currency)
	if !ok {
		return "", fmt.Errorf("unsupported currency %q", currency)
	}
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "+-eE,_") {
		return "", fmt.Errorf("amount must be a plain unsigned decimal")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts) == 2 && (parts[1] == "" || len(parts[1]) > scale) {
		return "", fmt.Errorf("amount exceeds currency scale")
	}
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", fmt.Errorf("invalid amount digit")
			}
		}
	}
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	if len(integer) > maxDigits {
		return "", fmt.Errorf("amount overflow")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if fraction == "" {
		if integer == "0" && !allowZero {
			return "", fmt.Errorf("amount must be a positive decimal within currency scale")
		}
		return integer, nil
	}
	return integer + "." + fraction, nil
}

// CanonicalAggregateAmount permits zero and up to 36 integer digits for totals.
// Transfer amounts remain bounded to 18 integer digits.
func CanonicalAggregateAmount(value, currency string) (string, error) {
	return canonicalAmount(value, currency, true, 36)
}

// AmountMinorUnits converts a validated amount into an exact integer.
func AmountMinorUnits(amount, currency string) (*big.Int, error) {
	return amountMinorUnits(amount, currency, 18)
}

func amountMinorUnits(amount, currency string, maxDigits int) (*big.Int, error) {
	canonical, err := canonicalAmount(amount, currency, true, maxDigits)
	if err != nil {
		return nil, err
	}
	scale, _ := CurrencyScale(currency)
	parts := strings.SplitN(canonical, ".", 2)
	digits := parts[0]
	if len(parts) == 2 {
		digits += parts[1] + strings.Repeat("0", scale-len(parts[1]))
	} else {
		digits += strings.Repeat("0", scale)
	}
	value, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	return value, nil
}

func AmountFromMinorUnits(units *big.Int, currency string) (string, error) {
	scale, ok := CurrencyScale(currency)
	if !ok || units == nil || units.Sign() < 0 {
		return "", fmt.Errorf("invalid amount or currency")
	}
	digits := units.String()
	if scale == 0 {
		return CanonicalAggregateAmount(digits, currency)
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}
	value := digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	return CanonicalAggregateAmount(value, currency)
}

func CompareAmounts(a, b, currency string) (int, error) {
	left, err := amountMinorUnits(a, currency, 36)
	if err != nil {
		return 0, err
	}
	right, err := amountMinorUnits(b, currency, 36)
	if err != nil {
		return 0, err
	}
	return left.Cmp(right), nil
}

// AddAmounts sums same-currency nonnegative decimal values exactly.
func AddAmounts(currency string, amounts ...string) (string, error) {
	total := new(big.Int)
	for _, amount := range amounts {
		units, err := amountMinorUnits(amount, currency, 36)
		if err != nil {
			return "", err
		}
		total.Add(total, units)
	}
	return AmountFromMinorUnits(total, currency)
}
