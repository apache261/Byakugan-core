# Build your own Go fraud evaluator

[main.go](main.go) is a complete independent consumer of the public Core module.
It combines built-in JSON rules with application-specific Go logic. Use this
approach when a condition needs your own facts or algorithm.

From the Core repository root:

```sh
cd examples/custom-fraud
GOWORK=off go mod download github.com/apache261/Byakugan-core
GOWORK=off go run . < ../transfer.json
```

The sample prints a `review` decision with the reason
`caller-supplied local risk flag`. Its custom evaluator contributes score `60`;
the sample transfer does not match the built-in high-amount rule.

## How it works

1. Decode stdin into `domain.TransferCheckRequest`.
2. Wrap a Go function with `engine.EvaluatorFunc`.
3. Register it as `institution-risk` with priority `10` using `engine.NewFramework`.
4. Supply a boolean JSON fact in `engine.Context.Facts`.
5. Call `Evaluate` and handle its error before using the result.

The callback has this signature:

```go
func(ctx context.Context, request domain.TransferCheckRequest, data engine.Context) ([]domain.RuleHit, error)
```

The example reads `data.Facts["institution.local_risk"]`. A true value returns
an explainable review hit; false returns no hits. Missing or malformed facts
return an error. With the default policy, evaluation errors produce a
conservative result and a non-nil error.

## Adapt it to your application

Copy the structure from `main.go` into your application and change the fact
name, registration ID, predicate, score, and reason. Set the example fact to
``json.RawMessage(`false`)`` to exercise the no-hit path. For multiple custom
evaluators, add more `engine.Registration` entries with unique IDs; rule IDs
and registration IDs share a namespace.

Retrieve database or service data before evaluation and put the resulting
snapshot in `Facts`, `PolicyValues`, or `Rules`. Core does not retrieve it for
you. Callback implementations must support concurrent calls and check
cancellation. A struct with an `Evaluate` method implementing `engine.Evaluator`
can replace `EvaluatorFunc` when you want reusable application logic.

To try local source edits, use [the temporary workspace commands](../README.md).
For aggregation, cancellation, and failure policies, see
[the framework reference](../../docs/fraud-framework.md).
