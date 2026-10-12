# Exact-money migration for the next Core release

These source changes prepare v0.3.0. No release tag has been published. Existing
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

## Full preparation and later cutover

Full's HTTP and persisted database contracts remain unchanged in this phase.
Full still pins v0.2.0. The private source tree prepares an exact adapter template
and verifies it with a temporary Go workspace/overlay against this Core source.
The template directly copies decimal strings in both directions; it introduces
no lossy conversion. The existing v0.2.0 projection remains solely for legacy
compatibility until the Phase 5 cutover. It is not the private fraud evaluator.

Phase 5 activates the exact adapter, removes the legacy float projection and
replaces duplicate portable evaluators after compatibility fixtures pass.
Phase 6 publishes the separately authorized tag and pins Full to the exact
released version. Do not commit a local replacement or change Full's production
module pin as part of Phase 2. Rolling back an upgrade requires restoring the
v0.2.0 adapter and JSON producer contract together, not parsing strings as floats.

## Fraud and AML extension APIs

`engine.Framework` adds stateless context-aware custom Go evaluation with
explicit registry IDs, stable ordering and aggregation/failure policies.
`engine.ReplayEngine` is the optional bounded wrapper; `engine.Engine` remains
the legacy adapter. See [fraud-framework.md](fraud-framework.md).
`aml/domain` and `aml/scenarios` provide separate pure AML contracts/observations,
bounded scenario preparation and custom evaluators; see
[aml-evaluation.md](aml-evaluation.md). None of these source additions cuts over
Full or changes its current pin.

The independent `examples/custom-fraud` and `examples/custom-aml` modules require
the future v0.3.0 tag. While it is unpublished, compile/run them with a temporary
Go workspace using the commands in [the README](../README.md). They import only
public packages, have no committed replacements, and are verified against both
the source module and history-free exported snapshots.
