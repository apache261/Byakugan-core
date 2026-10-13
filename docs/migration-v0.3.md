# Exact-money migration for the next Core release

v0.3.0 introduces the breaking contracts below. Existing
v0.2.0 consumers keep their existing Go/JSON contracts until they upgrade.

- `domain.Money.Amount` changes from `float64` to `string`.
- JSON intake requires `"amount":"1250.50"`; JSON numbers are rejected. XML
  decimal text is validated and canonicalized directly, without float parsing.
- `rules.Metrics.DebtorAmountSum` and `CreditorAmountSum` change to decimal
  strings. `Metrics.Currency` must match the transfer currency when a monetary
  metric is used. Empty loaded sums represent zero; invalid/mismatched sums
  return evaluation errors. Nonmonetary counts, scores and thresholds retain
  their existing representations.
- Monetary DSL `value` must be a decimal string, including nested conditions:
  `{"field":"amount","op":"gte","value":"100000","outcome":"review"}`.
  Monetary comparisons use exact numeric order and support `eq`, `neq`, `gt`,
  `gte`, `lt`, `lte`. Numeric JSON thresholds are rejected rather than rounded.
- Rules are validated before a transfer currency is available: monetary
  thresholds permit up to three fractional digits at installation; evaluation
  applies the actual currency scale and returns an error on mismatch.

The approved fraud catalog `iso4217-fraud-v1` uses scale 2 for PHP, USD, EUR,
GBP, AUD, CAD, SGD, HKD, CNY; scale 0 for JPY, KRW; scale 3 for KWD, BHD, OMR.
Currency codes must be uppercase. Transfer amounts must be positive unsigned
plain decimals with at most 18 significant integer digits. Signs, exponents,
whitespace, separators, unknown currencies and excess fractional digits fail.
Zero is allowed only for aggregates and comparison thresholds. Canonicalization
removes leading integer zeros and trailing fractional zeros; it never rounds.

`domain.AddAmounts` and `CompareAmounts` operate exactly on currency-scaled
integers. Aggregate values allow up to 36 integer digits, so sums can exceed the
individual transfer bound; aggregate overflow fails explicitly. History sums
include only the current currency and compare repeated amounts canonically.
Caller-supplied sums must already be filtered by currency, account and window.
AML's separate exact-decimal policy is unchanged and must not use this fraud
currency catalog as an AML validation policy.

Run the supplied aggregate example from the module root:

```sh
go run ./examples/supplied-aggregates < examples/transfer.json
```

It prints `review [exact sum meets threshold]` for the supplied example, where
1250.5 plus 0.1 reaches the exact 1250.6 threshold.

## Full cutover and rollback

Full's HTTP/JSON and persisted database contracts remain unchanged. Its exact
adapter copies monetary strings directly; Core handles portable rules and
aggregation through `engine.FullCompatibilityPolicy`. Full prepares account
facts, currency-bound aggregates and private behavior/network/ML signals.
`rules.EvaluateWithLeaves` lets a private evaluator supply custom leaf conditions
while Core retains boolean traversal and portable comparison semantics.
`rules.CompareValue` accepts exact monetary facts with an explicit currency.
Legacy Full numeric monetary rule thresholds are converted directly from
`json.Number` to decimal text by its private adapter; public DSL intake still
requires decimal strings.

Full's compatibility policy preserves stable priorities, first block/error,
replacement error reasons, zero thresholds and legacy score-only hits with an
empty outcome. Conservative default policies continue to reject empty outcomes.
AML simulation and workers use Core's context-aware pure scenario evaluator;
ledger operations, alert persistence and authorization stay private.

The release operator must authorize publication separately, publish the reviewed
history-free snapshot as v0.3.0, and then pin Full to that exact tag. Validate
Full with `GOWORK=off` against the downloaded release before declaring cutover
complete. No committed local replacement is required.

Rollback restores the pre-cutover Full commit and its v0.2.0 pin together.
Library consumers reverting to v0.2.0 must restore their Go/JSON amount producers
at the same time. Database rollback is unnecessary for this migration.

## Fraud and AML extension APIs

`engine.Framework` adds stateless context-aware custom Go evaluation with
explicit registry IDs, stable ordering and aggregation/failure policies.
`engine.ReplayEngine` is the optional bounded wrapper; `engine.Engine` remains
the legacy adapter. See [fraud-framework.md](fraud-framework.md).
`aml/domain` and `aml/scenarios` provide separate pure AML contracts/observations,
bounded scenario preparation and custom evaluators; see
[aml-evaluation.md](aml-evaluation.md). Full consumes these public contracts without invoking Core replay.

The independent `examples/custom-fraud` and `examples/custom-aml` modules require
the v0.3.0 tag. Run them against the released module, or use a temporary
Go workspace for source development, with the commands in [the README](../README.md). They import only
public packages, have no committed replacements, and are verified against both
the source module and history-free exported snapshots.
