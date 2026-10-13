# Create your own AML observations

[main.go](main.go) is a complete independent consumer of `aml/domain` and
`aml/scenarios`. It combines a declarative exact-amount scenario with a custom
Go evaluator. AML results are observations, separate from fraud decisions.

From the Core repository root:

```sh
cd examples/custom-aml
GOWORK=off go mod download github.com/apache261/Byakugan-core
GOWORK=off go run .
```

Expected output:

```text
exact-amount true [synthetic-tx-a synthetic-tx-b]
institution-observation true [synthetic-tx-a synthetic-tx-b]
```

The evidence references are sorted. The example supplies synthetic inputs
directly and needs no ledger, database, or monitoring worker.

## How it works

The declarative `domain.ScenarioVersion` compares `transaction.amount` with
the policy value `institution.threshold`. The amount `1250.5` and threshold
`1250.50` compare equally using exact decimals. Use `domain.MustDecimal` for
known valid literals; for external strings use `domain.ParseDecimal` and
handle its error.

The custom callback is wrapped in `scenarios.EvaluatorFunc` and registered as
`institution-observation`. Its signature is:

```go
func(ctx context.Context, input scenarios.Input) (scenarios.Observation, error)
```

It reads `input.Facts["institution.local_risk"]`. True sets `Matched`; false
leaves the observation unmatched. A missing fact sets `InsufficientData`
with an explanation. Malformed data returns an error. Each observation
identifies the registered scenario, a positive version, typology, severity,
exact risk score, reasons, and evidence references.

## Adapt it to your application

Copy `main.go` into your application, choose your scenario IDs and versions,
and supply your own facts, policy values, and transaction projection through
`scenarios.Input`. Change the threshold to `"2000"` to make the declarative
scenario unmatched; remove the local-risk fact to exercise insufficient data.

The example supplies a partial transaction containing the money consumed by
its scenario. For a complete canonical transaction, populate and validate the
full public projection. Your application prepares correctly scoped aggregates,
screening facts, and evidence before evaluation.

Handle `Evaluate` errors and inspect `result.Complete` before consuming an
observation batch. Callbacks must support concurrent calls and cooperate with
cancellation. Core retains no evaluation state and creates no ledger records,
alerts, investigations, or payment decisions; your application owns those
workflows.

For local source edits, use [the temporary workspace commands](../README.md).
See [the AML evaluation reference](../../docs/aml-evaluation.md) for supported
inputs, decimal limits, and custom observation validation.
