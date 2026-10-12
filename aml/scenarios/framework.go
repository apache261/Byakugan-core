package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/apache261/Byakugan-core/aml/domain"
)

// Evaluator returns an AML observation, never a fraud or payment decision.
// It must be safe for concurrent calls and cooperate with cancellation.
type Evaluator interface {
	Evaluate(context.Context, Input) (Observation, error)
}
type EvaluatorFunc func(context.Context, Input) (Observation, error)

func (f EvaluatorFunc) Evaluate(ctx context.Context, input Input) (Observation, error) {
	return f(ctx, input)
}

type Registration struct {
	ID        string
	Evaluator Evaluator
}

// Result is incomplete on any error. The caller owns every persistence decision.
type Result struct {
	Observations []Observation `json:"observations"`
	Complete     bool          `json:"complete"`
}

type Framework struct {
	mu            sync.RWMutex
	versions      []domain.ScenarioVersion
	registrations []Registration
}

func NewFramework(versions []domain.ScenarioVersion, registrations []Registration) (*Framework, error) {
	prepared, err := prepareVersions(versions)
	if err != nil {
		return nil, err
	}
	if err := validateRegistry(prepared, registrations); err != nil {
		return nil, err
	}
	return &Framework{versions: prepared, registrations: append([]Registration(nil), registrations...)}, nil
}
func validID(id string) bool {
	if id == "" || len(id) > 200 || strings.TrimSpace(id) != id {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r)) {
			return false
		}
	}
	return true
}
func validateRegistry(versions []domain.ScenarioVersion, registrations []Registration) error {
	seen := map[string]bool{}
	for _, version := range versions {
		if !validID(version.ScenarioID) || seen[version.ScenarioID] {
			return fmt.Errorf("invalid or duplicate evaluator ID %q", version.ScenarioID)
		}
		seen[version.ScenarioID] = true
	}
	for _, registration := range registrations {
		if !validID(registration.ID) || seen[registration.ID] || nilEvaluator(registration.Evaluator) {
			return fmt.Errorf("invalid or duplicate evaluator %q", registration.ID)
		}
		seen[registration.ID] = true
	}
	return nil
}
func nilEvaluator(value Evaluator) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
func prepareVersions(versions []domain.ScenarioVersion) ([]domain.ScenarioVersion, error) {
	prepared := make([]domain.ScenarioVersion, len(versions))
	for i, version := range versions {
		value, err := domain.PrepareScenarioVersion(version)
		if err != nil {
			return nil, err
		}
		// The validated DSL is bounded and JSON-safe. Detach all nested nodes/values.
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &prepared[i]); err != nil {
			return nil, err
		}
	}
	return prepared, nil
}
func (f *Framework) Register(registration Registration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	next := append(append([]Registration(nil), f.registrations...), registration)
	if err := validateRegistry(f.versions, next); err != nil {
		return err
	}
	f.registrations = next
	return nil
}
func (f *Framework) ReplaceScenarios(versions []domain.ScenarioVersion) error {
	prepared, err := prepareVersions(versions)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := validateRegistry(prepared, f.registrations); err != nil {
		return err
	}
	f.versions = prepared
	return nil
}

// Evaluate uses one immutable registry snapshot. Scenarios run in installation
// order, then custom evaluators in registration order, with no locks held during
// callbacks. Errors stop evaluation and cancellation never yields Complete=true.
func (f *Framework) Evaluate(ctx context.Context, input Input) (Result, error) {
	result := Result{Observations: []Observation{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	f.mu.RLock()
	versions := f.versions // owned immutable values, replaced rather than mutated
	registrations := append([]Registration(nil), f.registrations...)
	f.mu.RUnlock()
	snapshot, err := cloneInput(input)
	if err != nil {
		return result, err
	}
	for _, version := range versions {
		observation, err := EvaluateContext(ctx, version, snapshot)
		if err != nil {
			return result, fmt.Errorf("scenario %s: %w", version.ScenarioID, err)
		}
		result.Observations = append(result.Observations, observation)
	}
	for _, registration := range registrations {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		detached, err := cloneInput(snapshot)
		if err != nil {
			return result, err
		}
		observation, err := registration.Evaluator.Evaluate(ctx, detached)
		if cancelErr := ctx.Err(); cancelErr != nil {
			return result, cancelErr
		}
		if err == nil {
			err = validateObservation(registration.ID, observation)
		}
		if err != nil {
			return result, fmt.Errorf("evaluator %s: %w", registration.ID, err)
		}
		observation = cloneObservation(observation)
		result.Observations = append(result.Observations, observation)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.Complete = true
	return result, nil
}
func validateObservation(id string, observation Observation) error {
	if observation.ScenarioID != id || observation.ScenarioVersion < 1 {
		return fmt.Errorf("observation requires registered scenario identity and positive version")
	}
	if observation.Matched && observation.InsufficientData {
		return fmt.Errorf("an insufficient-data observation cannot match")
	}
	if observation.Typology == "" {
		return fmt.Errorf("observation typology is required")
	}
	switch observation.Severity {
	case "low", "medium", "high", "critical":
	default:
		return fmt.Errorf("invalid observation severity")
	}
	if err := observation.RiskScore.Validate(); err != nil {
		return err
	}
	// Risk scores follow the scenario contract even for caller-defined evaluators.
	score, err := domain.ParseDecimal(observation.RiskScore.String())
	if err != nil {
		return err
	}
	if len(strings.SplitN(score.String(), ".", 2)[0]) > 3 {
		return fmt.Errorf("risk score exceeds 100")
	}
	cmp, err := compareExact(score.String(), "100")
	if err != nil || cmp > 0 {
		return fmt.Errorf("risk score exceeds 100")
	}
	return nil
}
func cloneObservation(value Observation) Observation {
	value.Reasons = append([]string(nil), value.Reasons...)
	value.EvidenceIDs = append([]string(nil), value.EvidenceIDs...)
	sort.Strings(value.EvidenceIDs)
	if value.Values != nil {
		values := make(map[string]string, len(value.Values))
		for key, v := range value.Values {
			values[key] = v
		}
		value.Values = values
	}
	return value
}
