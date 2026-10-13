# Stateless fraud extension framework

`engine.Framework` evaluates each call using only its request, caller-supplied
context and one immutable registry snapshot. It does not read prior calls,
consult replay keys, generate transfer IDs, retrieve data or persist results.
Callers retrieve account facts, metrics and integration data before evaluation.
`engine.Context.Facts` and `PolicyValues` are raw JSON catalogs for custom logic;
`Context.Rules` supplies portable history, exact metrics and account attributes.
Supply `Rules.Now` for deterministic time-window evaluation.

```go
detector, err := engine.NewFramework(ruleSet, []engine.Registration{
    {ID: "institution-risk", Priority: 10, Evaluator: engine.EvaluatorFunc(
        func(ctx context.Context, req domain.TransferCheckRequest, data engine.Context) ([]domain.RuleHit, error) {
            if err := ctx.Err(); err != nil { return nil, err }
            // Interpret caller-supplied facts; return explainable hits or an error.
            return []domain.RuleHit{{RuleID: "institution-risk", Outcome: domain.DecisionReview,
                Score: 40, Reason: "institution policy matched"}}, nil
        }),
    },
}, engine.DefaultPolicy())
// Handle construction errors before evaluation.
result, err := detector.Evaluate(ctx, request, suppliedContext)
// Always inspect err. A failed evaluation is not successful authorization.
```

IDs are explicit, trimmed, at most 200 characters, and use letters, digits,
period, underscore, colon or hyphen. Rule IDs and custom evaluator IDs share a
namespace; duplicates, missing IDs and nil implementations are rejected.
Steps run by ascending priority, preserving rule installation order and then
custom registration order for ties. `Register` and `ReplaceRules` atomically
publish a new snapshot; invalid replacement leaves the previous snapshot intact.
DSL bytes and supplied context collections are detached, so a callback cannot
mutate another evaluator's inputs or the installed rules.

Callbacks run outside registry locks and may register or replace rules. They
must be safe for concurrent calls, treat supplied data as their own detached
snapshot, and cooperate with cancellation. Callers must not mutate input
collections while Core snapshots them. Core checks cancellation before/after
callbacks and between DSL rules; it does not terminate noncooperative Go code
or sandbox extensions. The caller owns extension side effects and error logging.

## Aggregation and errors

Each hit needs an explicit ID, a valid outcome and a finite score aggregate.
Hits retain deterministic order; scores sum in that order. A custom `allow`
cannot downgrade an existing `review` or `block`. Partial hits returned together
with an error are discarded. Invalid hits are evaluation failures, not signals.
Positive review/block thresholds escalate at equality; zero disables escalation
under the default policy. Negative/nonfinite thresholds fail explicitly.

`DefaultPolicy` returns an error and a conservative review/block result on the
first evaluation failure. `FailureBlock` escalates failures to block.
`StopOnError=false` permits subsequent evaluators while still returning all
errors and retaining at least review. Cancellation always stops evaluation and
returns its error. Neither validation failure nor cancellation can produce a
successful allow, even if a caller requests validation-failure allow.

`FullCompatibilityPolicy` provides Full's portable aggregation stage: stable
priority order, stop at the first block, stop at the first error, preserve
prior hits/scores but replace reasons with the failing evaluator's original
error, and apply configured thresholds even at zero. Full must supply already
filtered rules and valid account configuration; its private admission, account,
identity, watchlist and integration stages are not recreated by this policy.
Callbacks can occupy explicit priorities before or after DSL rules during the
later Full migration. Existing private application code is not cut over yet.

## Optional bounded convenience engine

`NewReplayEngine(detector, options)` explicitly adds bounded history and replay.
Successful identical key/payload calls share the original result, including
concurrent callers. Changed payloads receive a review conflict signal unless
already blocked. Failed/cancelled evaluations are not cached or added to history.
Callbacks run outside its locks; waiting callers can cancel independently.
Callbacks must not recursively check their own in-flight replay key.

Caches use bounded FIFO eviction and history uses the configured cap. Restart
or eviction loses replay authority. This is a local convenience, not durable,
source-scoped idempotency. Full keeps operational idempotency in its own store.

The existing `engine.Engine` and optional HTTP service remain the legacy bounded
adapter, preserving the recorded v0.2.0 decision control flow with exact money.
That adapter's legacy error-after-block behavior is recorded in compatibility
fixtures; new extensions should use `Framework` and optionally `ReplayEngine`.

## Independent consumer

`examples/custom-fraud` is an independent module with only public imports and
no committed replacement. It pins v0.3.0 and can run against the released module.
For source development, run it in a temporary workspace against Core source. From
the Full repository, `bash scripts/framework/verify-examples.sh` builds and runs
both independent fraud and AML consumers against source or an exported snapshot.

## Application DSL extensions

`rules.EvaluateWithLeaves` shares portable fields and boolean short-circuiting
with a caller-supplied leaf evaluator. Return `handled=false` to delegate a leaf
to the portable evaluator. The application owns validation of its additional
fields and registers this logic through `engine.Evaluator`. Callbacks must be
safe for concurrent evaluation; propagate errors rather than treating missing
required facts as a successful match. `rules.CompareValue` compares supplied
facts using the same operators; monetary facts require decimal strings and an
explicit currency. The strict public `rules.Validate` and default DSL intake
continue to reject unrecognized fields.

Full compatibility permits legacy score-only hits with an empty outcome. Such
hits contribute score and reasons; configured thresholds decide escalation.
Other policies reject empty outcomes.
