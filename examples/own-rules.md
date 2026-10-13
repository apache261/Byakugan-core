# Write your own fraud rules

Start with the application in [getting started](getting-started.md). Replace
its `engine.NewFramework` construction with the following code; the imports
already include `encoding/json`, `domain`, and `engine`:

```go
ruleSet := []domain.Rule{
    {
        ID: "my-high-amount",
        Name: "PHP high amount review",
        Priority: 10,
        Enabled: true,
        Definition: json.RawMessage(`{
          "all": [
            {"field":"currency","op":"eq","value":"PHP"},
            {"field":"amount","op":"gte","value":"1000.00"}
          ],
          "outcome":"review",
          "score":40,
          "reason":"PHP transfer meets our review threshold"
        }`),
    },
}
detector, err := engine.NewFramework(ruleSet, nil, engine.DefaultPolicy())
if err != nil {
    log.Fatal(err)
}
```

Run `go run . < transfer.json` again. The sample PHP transfer now receives
`review`, score `40`, and a hit identifying `my-high-amount`. This construction
installs only your rules. To retain the defaults as well, prepend them using
`ruleSet = append(engine.DefaultRules(), ruleSet...)` before construction.

## Understand the rule document

Each rule needs a unique ID. Lower priorities run first. `Enabled: false`
disables evaluation of that rule. A leaf condition contains `field`, `op`, and
`value`; `all` requires every child, while `any` requires at least one child.
Groups can be nested. Define `outcome`, `score`, and `reason` on the root node
to explain the matched rule.

Use quoted decimal values for amount comparisons and scope monetary rules to
the intended currency. Rule scores are numeric risk weights, separate from
money. Unknown fields, invalid operators, and malformed definitions fail
framework construction. For application-specific facts, use a
[custom evaluator](custom-fraud/README.md).

For a windowed rule, a complete definition could be:

```json
{
  "field": "velocity.debtor_transfer_count",
  "op": "gte",
  "value": 3,
  "window_minutes": 15,
  "outcome": "review",
  "score": 40,
  "reason": "three prior debtor transfers in fifteen minutes"
}
```

The count refers to prior transfers. Supply history or scoped metrics through
`engine.Context.Rules`, or reuse a replay engine. A stateless framework does
not build history across calls. See [supplied aggregates](supplied-aggregates/README.md).

## Configure thresholds and replace rules

Pass score thresholds per evaluation:

```go
result, err := detector.Evaluate(context.Background(), request, engine.Context{
    ReviewThreshold: 40,
    BlockThreshold: 80,
})
if err != nil {
    log.Fatal(err)
}
```

Scores sum across matched hits. Under the default policy, zero disables a score
threshold. Explicit `review` and `block` rule outcomes still apply; an `allow`
hit cannot downgrade them.

Your application can reload a new rule set on an existing framework:

```go
if err := detector.ReplaceRules(ruleSet); err != nil {
    log.Fatal(err)
}
```

Replacement validates the complete set and publishes it atomically. Invalid
replacement leaves the previous rules installed. IDs must also be distinct
from custom evaluator registrations. Your application owns loading and storing
rule documents. See [the framework reference](../docs/fraud-framework.md) for
ordering, error policies, and extension contracts.
