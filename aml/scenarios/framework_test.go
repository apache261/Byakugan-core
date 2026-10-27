package scenarios

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/apache261/Byakugan-core/aml/domain"
)

func scalarVersion(id string, version int) domain.ScenarioVersion {
	return domain.ScenarioVersion{ScenarioID: id, Version: version, SchemaVersion: 1, Typology: "synthetic", ExecutionModes: []string{"simulation"}, SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600, LateDataBehavior: "observe", Condition: domain.ScenarioNode{Field: "transaction.amount", Operator: "gte", Value: json.RawMessage(`"0.1"`)}, ReasonTemplate: "{{left}} {{operator}} {{right}}", Severity: "medium", RiskScore: domain.MustDecimal("50"), Author: "synthetic-test"}
}
func customObservation(id string) Observation {
	return Observation{ScenarioID: id, ScenarioVersion: 2, Typology: "synthetic", Severity: "low", RiskScore: domain.MustDecimal("1"), Reasons: []string{"supplied context"}, EvidenceIDs: []string{"z", "a"}}
}
func customRegistration(id string) Registration {
	return Registration{ID: id, Evaluator: EvaluatorFunc(func(context.Context, Input) (Observation, error) { return customObservation(id), nil })}
}
func amlFramework(t *testing.T, versions []domain.ScenarioVersion, registrations []Registration) *Framework {
	t.Helper()
	f, err := NewFramework(versions, registrations)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func exactInput() Input {
	return Input{Transaction: domain.Transaction{Money: domain.Money{Amount: domain.MustDecimal("0.1"), AssetCode: "USD"}}}
}

func TestPublicScenarioVersionEvidenceAndExactBoundaries(t *testing.T) {
	version := scalarVersion("exact", 7)
	version.Condition.Value = json.RawMessage(`"99999999999999999999.000000000000000001"`)
	input := Input{Transaction: domain.Transaction{Money: domain.Money{Amount: domain.MustDecimal("99999999999999999999.000000000000000001"), AssetCode: "SYNTHETIC"}}, EvidenceIDs: []string{"z", "a", "z"}}
	got, err := EvaluateContext(context.Background(), version, input)
	if err != nil || !got.Matched || got.ScenarioID != "exact" || got.ScenarioVersion != 7 || !reflect.DeepEqual(got.EvidenceIDs, []string{"a", "z", "z"}) {
		t.Fatalf("observation changed: %+v %v", got, err)
	}
	if !reflect.DeepEqual(input.EvidenceIDs, []string{"z", "a", "z"}) {
		t.Fatal("caller evidence mutated")
	}
	input.Transaction.Money.Amount = domain.MustDecimal("99999999999999999999")
	got, err = Evaluate(version, input)
	if err != nil || got.Matched {
		t.Fatalf("fractional boundary rounded: %+v %v", got, err)
	}
	version.Condition.Value = json.RawMessage(`"0.1"`)
	got, err = Evaluate(version, Input{})
	if err != nil || got.Matched || !got.InsufficientData {
		t.Fatalf("missing money was not unknown: %+v %v", got, err)
	}
	version.LateDataBehavior = "ignore"
	input.LateData = true
	got, err = Evaluate(version, input)
	if err != nil || got.Matched || got.InsufficientData || !reflect.DeepEqual(got.Reasons, []string{"late data ignored by scenario version"}) {
		t.Fatalf("late behavior changed: %+v %v", got, err)
	}
}

func TestPublicScenarioScreeningPolicyAndUnknownBehavior(t *testing.T) {
	version := scalarVersion("screening", 3)
	version.Condition = domain.ScenarioNode{Screening: &domain.ScreeningPredicate{SubjectBasis: domain.SubjectCustomer, Kind: "sanctions", DatasetRef: "synthetic", Disposition: "confirmed"}}
	got, err := Evaluate(version, Input{})
	if err != nil || !got.InsufficientData || got.Matched {
		t.Fatalf("missing screening fact defaulted: %+v %v", got, err)
	}
	got, err = Evaluate(version, Input{Screening: map[string]bool{"customer:sanctions:synthetic:confirmed": true}})
	if err != nil || !got.Matched {
		t.Fatalf("supplied screening fact ignored: %+v %v", got, err)
	}
	version.Condition = domain.ScenarioNode{Metric: "sum", Operator: "gte", PolicyRef: "policy.threshold", SubjectBasis: domain.SubjectCustomer, WindowSeconds: 3600}
	input := Input{Metrics: map[string]domain.Decimal{"sum:customer:3600": domain.MustDecimal("0.3")}}
	got, err = Evaluate(version, input)
	if err != nil || !got.InsufficientData {
		t.Fatalf("missing policy defaulted: %+v %v", got, err)
	}
	input.PolicyValues = map[string]json.RawMessage{"policy.threshold": json.RawMessage(`"0.30"`)}
	got, err = Evaluate(version, input)
	if err != nil || !got.Matched {
		t.Fatalf("supplied exact policy ignored: %+v %v", got, err)
	}
}

func TestPublicScenarioRejectsInvalidExactInputsAndTrailingJSON(t *testing.T) {
	for _, value := range []string{`"1/2"`, `"1e2"`, `"-1"`, `"0.1" "0.2"`} {
		version := scalarVersion("bad", 1)
		version.Condition.Value = json.RawMessage(value)
		if _, err := Evaluate(version, exactInput()); err == nil {
			t.Fatalf("invalid literal accepted: %s", value)
		}
	}
	version := scalarVersion("bad", 1)
	version.Condition = domain.ScenarioNode{Field: "transaction.amount", Operator: "gte", PolicyRef: "threshold"}
	for _, value := range []string{`"1/2"`, `"0.1" true`, `0.1`} {
		input := exactInput()
		input.PolicyValues = map[string]json.RawMessage{"threshold": json.RawMessage(value)}
		if _, err := Evaluate(version, input); err == nil {
			t.Fatalf("invalid policy accepted: %s", value)
		}
	}
	version.Condition = metric("sum", "1")
	if _, err := Evaluate(version, Input{Metrics: map[string]domain.Decimal{"sum:customer:3600": domain.Decimal("invalid")}}); err == nil {
		t.Fatal("invalid metric silently treated as unavailable")
	}
}

func TestAMLRegistryDeterminismAndDetachedObservations(t *testing.T) {
	returned := customObservation("custom")
	f := amlFramework(t, []domain.ScenarioVersion{scalarVersion("first", 1), scalarVersion("second", 2)}, []Registration{{ID: "custom", Evaluator: EvaluatorFunc(func(context.Context, Input) (Observation, error) { return returned, nil })}})
	got, err := f.Evaluate(context.Background(), exactInput())
	if err != nil || !got.Complete || len(got.Observations) != 3 {
		t.Fatalf("evaluation incomplete: %+v %v", got, err)
	}
	ids := []string{}
	for _, observation := range got.Observations {
		ids = append(ids, observation.ScenarioID)
	}
	if !reflect.DeepEqual(ids, []string{"first", "second", "custom"}) || !reflect.DeepEqual(got.Observations[2].EvidenceIDs, []string{"a", "z"}) {
		t.Fatalf("order changed: %+v", got)
	}
	got.Observations[2].Reasons[0] = "changed"
	if returned.Reasons[0] != "supplied context" {
		t.Fatal("custom output storage was retained")
	}
	if !reflect.DeepEqual(returned.EvidenceIDs, []string{"z", "a"}) {
		t.Fatal("custom evidence was reordered in place")
	}
	if err := f.Register(customRegistration("first")); err == nil {
		t.Fatal("scenario/custom collision accepted")
	}
	for _, registrations := range [][]Registration{{customRegistration("same"), customRegistration("same")}, {{ID: "nil", Evaluator: EvaluatorFunc(nil)}}, {{ID: "", Evaluator: customRegistration("x").Evaluator}}} {
		if _, err := NewFramework(nil, registrations); err == nil {
			t.Fatal("invalid registry accepted")
		}
	}
	if err := f.ReplaceScenarios([]domain.ScenarioVersion{scalarVersion("custom", 1)}); err == nil {
		t.Fatal("colliding replacement accepted")
	}
	got, err = f.Evaluate(context.Background(), exactInput())
	if err != nil || len(got.Observations) != 3 {
		t.Fatalf("failed replacement changed registry: %+v %v", got, err)
	}
}

func TestAMLErrorsAndCancellationLeaveResultIncomplete(t *testing.T) {
	failed := errors.New("facts unavailable")
	f := amlFramework(t, []domain.ScenarioVersion{scalarVersion("first", 1)}, []Registration{{ID: "failure", Evaluator: EvaluatorFunc(func(context.Context, Input) (Observation, error) { return customObservation("failure"), failed })}, customRegistration("later")})
	got, err := f.Evaluate(context.Background(), exactInput())
	if !errors.Is(err, failed) || got.Complete || len(got.Observations) != 1 {
		t.Fatalf("error treated as success: %+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = f.Evaluate(ctx, exactInput())
	if !errors.Is(err, context.Canceled) || got.Complete || len(got.Observations) != 0 {
		t.Fatalf("cancellation treated as success: %+v %v", got, err)
	}
	if _, err := EvaluateContext(ctx, scalarVersion("cancel", 1), exactInput()); !errors.Is(err, context.Canceled) {
		t.Fatalf("pure DSL ignored cancellation: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	f = amlFramework(t, nil, []Registration{{ID: "cancel", Evaluator: EvaluatorFunc(func(context.Context, Input) (Observation, error) { cancel(); return customObservation("cancel"), nil })}})
	got, err = f.Evaluate(ctx, exactInput())
	if !errors.Is(err, context.Canceled) || got.Complete || len(got.Observations) != 0 {
		t.Fatalf("callback cancellation treated as success: %+v %v", got, err)
	}
	for _, observation := range []Observation{{ScenarioID: "mismatch"}, func() Observation {
		value := customObservation("bad")
		value.Matched = true
		value.InsufficientData = true
		return value
	}(), func() Observation {
		value := customObservation("bad")
		value.RiskScore = domain.MustDecimal("100.01")
		return value
	}()} {
		f = amlFramework(t, nil, []Registration{{ID: "bad", Evaluator: EvaluatorFunc(func(context.Context, Input) (Observation, error) { return observation, nil })}})
		got, err = f.Evaluate(context.Background(), exactInput())
		if err == nil || got.Complete {
			t.Fatalf("invalid custom result accepted: %+v %v", got, err)
		}
	}
}

func TestAMLRegistrySnapshotAndCallerInputIsolation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	callback := Registration{ID: "gate", Evaluator: EvaluatorFunc(func(ctx context.Context, input Input) (Observation, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		case <-ctx.Done():
			return Observation{}, ctx.Err()
		}
		input.Facts["risk"][0] = 'f'
		delete(input.Metrics, "synthetic.count")
		delete(input.Screening, "synthetic.match")
		input.Transaction.Metadata["synthetic.context"].(map[string]any)["risk"] = "changed"
		return customObservation("gate"), nil
	})}
	f := amlFramework(t, []domain.ScenarioVersion{scalarVersion("old", 1)}, []Registration{callback})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	input := exactInput()
	input.Facts = map[string]json.RawMessage{"risk": json.RawMessage(`true`)}
	input.Metrics = map[string]domain.Decimal{"synthetic.count": domain.MustDecimal("1")}
	input.Screening = map[string]bool{"synthetic.match": true}
	input.Transaction.Metadata = map[string]any{"synthetic.context": map[string]any{"risk": "original"}}
	done := make(chan Result, 1)
	go func() { got, _ := f.Evaluate(ctx, input); done <- got }()
	<-entered
	replaced := make(chan error, 1)
	go func() { replaced <- f.ReplaceScenarios([]domain.ScenarioVersion{scalarVersion("new", 2)}) }()
	select {
	case err := <-replaced:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("AML callback held registry lock")
	}
	close(release)
	got := <-done
	if !got.Complete || got.Observations[0].ScenarioID != "old" {
		t.Fatalf("in-flight registry changed: %+v", got)
	}
	got, err := f.Evaluate(context.Background(), input)
	if err != nil || got.Observations[0].ScenarioID != "new" {
		t.Fatalf("replacement not visible: %+v %v", got, err)
	}
	if string(input.Facts["risk"]) != "true" || input.Transaction.Metadata["synthetic.context"].(map[string]any)["risk"] != "original" {
		t.Fatal("callback mutated caller input")
	}
	if input.Metrics["synthetic.count"] != domain.MustDecimal("1") || !input.Screening["synthetic.match"] {
		t.Fatal("callback mutated caller metrics or screening facts")
	}
}

func TestAMLConcurrentEvaluationAndReplacement(t *testing.T) {
	version := scalarVersion("scenario", 1)
	f := amlFramework(t, []domain.ScenarioVersion{version}, nil)
	// Mutate the caller-owned rule after registration; registry remains detached.
	version.Condition.Value[1] = '9'
	original, err := f.Evaluate(context.Background(), exactInput())
	if err != nil || !original.Observations[0].Matched {
		t.Fatalf("caller mutated installed version: %+v %v", original, err)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 6; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				if i%3 == 0 {
					if err := f.ReplaceScenarios([]domain.ScenarioVersion{scalarVersion("scenario", 2)}); err != nil {
						t.Error(err)
					}
				}
				got, err := f.Evaluate(context.Background(), exactInput())
				if err != nil || !got.Complete || len(got.Observations) != 1 || !got.Observations[0].Matched {
					t.Errorf("concurrent result %+v %v", got, err)
				}
			}
		}()
	}
	wg.Wait()
}
