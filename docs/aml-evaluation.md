# Public AML evaluation

`aml/domain` owns the canonical transaction/typed subject projection, exact
AML decimals, immutable scenario version and bounded DSL preparation.
`aml/scenarios` owns pure observations and context-aware evaluation. These
packages depend only on the Go standard library and the AML domain. They do
not import fraud decisions, persistence, ledger services, screening providers,
workers, alerts, investigations, reports, authorization or protected storage.

AML evaluation produces observations and evidence references. It never returns
`allow`, `review` or `block`, writes a ledger/alert, or changes a fraud result.
Full continues using its private AML evaluator until framework Phase 5.

## Canonical inputs

`domain.Transaction` contains evaluation fields: ID, source identity, semantic
timestamps, kind/subtype/direction/status, exact money, typed subjects, lineage,
channel/purpose/countries and bounded metadata. Import bookkeeping, database
fingerprints, ingestion timestamps and workflow records are excluded.
`Transaction.Validate` validates a complete canonical projection without
requiring private persistence timestamps. Evaluating a partial scenario input
only requires the fields consumed by that scenario; unavailable amount, facts,
metrics or policy inputs produce insufficient-data observations.

AML `Decimal` is an unsigned canonical JSON string with precision at most 38
and scale at most 18. It permits zero and arbitrary valid asset codes, independent
of fraud's approved fiat currency scale table. Exponents, signs, rational text,
excess precision/scale, JSON numbers and null fail decimal intake. Relational
DSL comparisons use exact decimal strings, never floats. Equality retains the
existing type-sensitive JSON comparison; callers should supply canonical values.

`scenarios.Input` contains:

- a canonical transaction projection;
- raw JSON facts keyed by allowed field names;
- exact metrics keyed `metric:subject_basis:window_seconds`;
- raw policy values keyed by the scenario's `policy_ref`;
- precomputed screening booleans keyed
  `subject_basis:kind:dataset_ref:disposition`;
- evidence transaction IDs; and
- a late-data flag supplied by the caller's event-time policy.

Metrics, policy values and screening facts arrive already scoped to the correct
subject, asset, window, dataset and scenario/policy version. Callers apply
screening confidence/minimum-score rules when preparing the boolean fact; Core
never retrieves or discloses provider evidence. Missing screening facts remain
unknown. The private monitoring layer still owns minimum-history eligibility,
late-data watermarks, policy activation and durable evidence authority.

`Evaluate(version, input)` retains the pure evaluator's existing API.
`EvaluateContext(ctx, version, input)` adds cooperative cancellation at every
DSL node. Callers keep these pure-function inputs read-only for the duration of
evaluation; the registry API separately snapshots callback inputs. DSL limits remain depth 12, 200 nodes, 50 boolean children, bounded
windows and allowlisted fields/operators; compiled cyclic Go nodes are rejected.
Only one JSON value is accepted per literal/fact/policy operand.

Observations preserve scenario ID/version, typology, severity, exact risk score,
matched/insufficient-data flags, reasons, values and sorted evidence IDs.
Duplicates in evidence are retained for compatibility. Unknown boolean/late-data
behavior matches the private evaluator's fixtures. Late-data `ignore` returns
an unmatched observation with its explicit reason. Evaluation does not mutate
caller evidence or source data. Invalid exact operands return errors.

## Custom Go evaluators

```go
detector, err := scenarios.NewFramework(versions, []scenarios.Registration{
    {ID: "institution-observation", Evaluator: scenarios.EvaluatorFunc(
        func(ctx context.Context, input scenarios.Input) (scenarios.Observation, error) {
            if err := ctx.Err(); err != nil { return scenarios.Observation{}, err }
            return scenarios.Observation{ScenarioID: "institution-observation",
                ScenarioVersion: 1, Typology: "institution-context", Severity: "low",
                RiskScore: domain.MustDecimal("10"), InsufficientData: true,
                Reasons: []string{"caller context unavailable"}}, nil
        }),
    },
})
result, err := detector.Evaluate(ctx, suppliedInput)
// Persist nothing from an incomplete result without an explicit caller policy.
```

Scenario IDs and custom evaluator IDs share an explicit namespace. Missing,
duplicate or nil registrations fail. Scenario installation order runs first,
then custom registration order. Registry replacement is atomic and immutable
for in-flight evaluations. A custom observation must identify its registered
scenario ID, a positive version, valid severity/typology and exact score 0–100;
it cannot simultaneously match and report insufficient data.

Callbacks receive detached inputs outside registry locks. They must be safe for
concurrent calls and cooperate with cancellation. Callers must not mutate input
collections during snapshotting. Results are detached from custom callback
storage and evidence is sorted. The first error stops execution and returns
`Complete=false` with any prior observations; cancelled callbacks cannot produce
a complete result. The framework retains no evaluation/replay/persistence state.

`examples/custom-aml` is an independent public-only module targeting the future
v0.3.0 release. Before publication, use a temporary Go workspace against source
as described in [the migration guide](migration-v0.3.md). The example supplies
synthetic facts, exact policy values and evidence without private integrations.

The public suite includes the existing pure scenario family fixtures. The Full
repository's `scripts/framework/verify-aml-parity.sh` additionally compares
complete observations and prepared version hashes through public contracts for
exact amounts, missing inputs, boolean conditions, late data and evidence order.
The private implementation remains intact for the later cutover.
