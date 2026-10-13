# Supply your own aggregate data

[main.go](main.go) demonstrates evaluating an exact accumulated amount using
data supplied by your application. It is part of the Core module, so run it
from the Core repository root:

```sh
go run ./examples/supplied-aggregates < examples/transfer.json
```

The result is `review` with the reason `exact sum meets threshold`.

## Understand the calculation

The rule compares `velocity.debtor_amount_sum_including_current` against
`"1250.6"`. The caller supplies a prior debtor sum of `"0.1"` in PHP, and the
sample request contributes `"1250.50"`. Core adds them exactly to get `1250.60`.
The prior sum must exclude the current transfer.

The context in the example is:

```go
engine.Context{
    Rules: rules.Context{
        Metrics: rules.Metrics{
            Loaded: true,
            Currency: request.Payload.InstructedAmount.Currency,
            DebtorAmountSum: "0.1",
        },
    },
}
```

In `main.go`, the request variable is named `req`. `Loaded: true` marks supplied
metrics as available. The example uses the legacy `engine.New` convenience
adapter; the same context can be passed to `Framework.Evaluate` from
[getting started](../getting-started.md), with explicit error handling.

## Connect your own database

Query your data source before evaluation. Scope the results to the relevant
debtor/creditor, currency, time range, and rule semantics. Keep monetary totals
as exact decimal strings. Only mark metrics loaded when the required query
results are available; a database failure must not become a fabricated zero.

The sample uses a rule without an explicit window. For windowed rules, use
`rules.MetricWindowsFromRules(ruleSet)` to discover the required windows and
populate `rules.Context.MetricsByWindow` with matching aggregates. Supply
`rules.Context.Now` when you need a fixed evaluation time. Avoid combining
different currencies into one monetary total.

Core does not connect to a database, choose your query scope, or store these
metrics. For broader integration contracts, see
[the fraud framework reference](../../docs/fraud-framework.md).
