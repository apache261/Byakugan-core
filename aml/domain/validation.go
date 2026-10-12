package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var assetCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{0,15}$`)
var extensionTypePattern = regexp.MustCompile(`^x-[a-z0-9][a-z0-9_-]*$`)

func (s SourceIdentity) Validate() error {
	if err := requiredBounded("source_system", s.SourceSystem, 100); err != nil {
		return err
	}
	if err := requiredBounded("external_id", s.ExternalID, 200); err != nil {
		return err
	}
	if s.SourceEventAt.IsZero() {
		return fmt.Errorf("source_event_at is required")
	}
	return nil
}

func (s SubjectReference) Validate() error {
	if !validSubjectType(s.Type) {
		return fmt.Errorf("unsupported subject type %q", s.Type)
	}
	return requiredBounded("subject id", s.ID, 200)
}

func (m Money) Validate() error {
	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("amount: %w", err)
	}
	if !assetCodePattern.MatchString(m.AssetCode) {
		return fmt.Errorf("invalid asset_code")
	}
	if m.AssetType != "" && !oneOf(string(m.AssetType), "fiat", "virtual_asset", "security", "commodity", "other") {
		return fmt.Errorf("invalid asset_type")
	}
	return nil
}

func fingerprint(value any) string {
	raw, _ := json.Marshal(value)
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&document)
	delete(document, "id")
	delete(document, "content_fingerprint")
	delete(document, "ingested_at")
	delete(document, "created_at")
	canonical, _ := json.Marshal(document)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func requiredBounded(name, value string, max int) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s is required and must be trimmed", name)
	}
	if len(value) > max {
		return fmt.Errorf("%s exceeds %d bytes", name, max)
	}
	return nil
}

func validSubjectType(value SubjectType) bool {
	switch value {
	case SubjectCustomer, SubjectAccount, SubjectBeneficialOwner, SubjectRelatedParty, SubjectCounterparty, SubjectInstitution:
		return true
	default:
		return extensionTypePattern.MatchString(string(value))
	}
}

func validateCountryOptional(value string) error {
	if value == "" {
		return nil
	}
	return validateCountry(value)
}

func validateCountry(value string) error {
	if len(value) != 2 || value != strings.ToUpper(value) || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
		return fmt.Errorf("country must be an uppercase ISO alpha-2 code")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func decimalGreaterThan(value Decimal, limit string) bool {
	parts := strings.SplitN(string(value), ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	limitInteger := strings.TrimLeft(limit, "0")
	if len(integer) != len(limitInteger) {
		return len(integer) > len(limitInteger)
	}
	if integer != limitInteger {
		return integer > limitInteger
	}
	return len(parts) == 2 && strings.Trim(parts[1], "0") != ""
}

func validateMetadata(metadata map[string]any) error {
	if len(metadata) == 0 {
		return nil
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("metadata is not valid JSON: %w", err)
	}
	if len(raw) > 64*1024 {
		return fmt.Errorf("metadata exceeds 65536 bytes")
	}
	var normalized map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return fmt.Errorf("metadata is not valid JSON: %w", err)
	}
	if len(normalized) > 100 {
		return fmt.Errorf("metadata has too many keys")
	}
	keys := make([]string, 0, len(normalized))
	for key := range normalized {
		if len(key) > 100 || !strings.Contains(key, ".") {
			return fmt.Errorf("metadata keys must be bounded and namespaced")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := validateMetadataValue(normalized[key], 1); err != nil {
			return fmt.Errorf("metadata %s: %w", key, err)
		}
	}
	return nil
}

func validateMetadataValue(value any, depth int) error {
	if depth > 3 {
		return fmt.Errorf("metadata nesting exceeds 3")
	}
	switch typed := value.(type) {
	case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return nil
	case string:
		if len(typed) > 4096 {
			return fmt.Errorf("metadata string exceeds 4096 bytes")
		}
		return nil
	case []any:
		if len(typed) > 100 {
			return fmt.Errorf("metadata array exceeds 100 items")
		}
		for _, item := range typed {
			if err := validateMetadataValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(typed) > 50 {
			return fmt.Errorf("metadata object exceeds 50 keys")
		}
		for key, item := range typed {
			if len(key) > 100 {
				return fmt.Errorf("metadata nested key exceeds 100 bytes")
			}
			if err := validateMetadataValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported metadata value type")
	}
}

func (t Transaction) Validate() error {
	if err := requiredBounded("id", t.ID, 200); err != nil {
		return err
	}
	if err := t.Source.Validate(); err != nil {
		return err
	}
	if t.EventAt.IsZero() {
		return fmt.Errorf("event_at is required")
	}
	if !oneOf(string(t.Kind), "payment", "cash", "card", "trade", "security", "virtual_asset", "adjustment", "reversal", "other") {
		return fmt.Errorf("invalid transaction kind")
	}
	if !oneOf(string(t.Direction), "inbound", "outbound", "internal", "unknown") {
		return fmt.Errorf("invalid transaction direction")
	}
	if !oneOf(string(t.Status), "pending", "posted", "settled", "rejected", "cancelled", "reversed") {
		return fmt.Errorf("invalid transaction status")
	}
	if err := t.Money.Validate(); err != nil {
		return err
	}
	if len(t.Subjects) == 0 || len(t.Subjects) > 100 {
		return fmt.Errorf("subjects must contain between 1 and 100 entries")
	}
	seen := map[SubjectReference]bool{}
	for _, subject := range t.Subjects {
		if err := subject.Validate(); err != nil {
			return err
		}
		if seen[subject] {
			return fmt.Errorf("duplicate transaction subject")
		}
		seen[subject] = true
	}
	if t.RelatedTransaction != nil {
		if err := requiredBounded("related transaction id", t.RelatedTransaction.TransactionID, 200); err != nil {
			return err
		}
		if t.RelatedTransaction.TransactionID == t.ID {
			return fmt.Errorf("transaction cannot link to itself")
		}
		switch t.RelatedTransaction.Type {
		case LinkCorrection:
		case LinkReversal:
			if t.Kind != TransactionReversal {
				return fmt.Errorf("reversal_of requires reversal kind")
			}
		case LinkAdjustment:
			if t.Kind != TransactionAdjustment {
				return fmt.Errorf("adjustment_of requires adjustment kind")
			}
		default:
			return fmt.Errorf("invalid related transaction type")
		}
	} else if t.Kind == TransactionReversal || t.Kind == TransactionAdjustment {
		return fmt.Errorf("reversal and adjustment transactions require lineage")
	}
	if err := validateCountryOptional(t.OriginCountry); err != nil {
		return err
	}
	if err := validateCountryOptional(t.DestinationCountry); err != nil {
		return err
	}
	if err := validateMetadata(t.Metadata); err != nil {
		return err
	}
	return nil
}
