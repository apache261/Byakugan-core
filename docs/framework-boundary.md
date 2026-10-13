# Framework boundary and dependencies

The source tree implements the documentation/boundary baseline, exact fraud
money, stateless custom fraud evaluation and pure public AML evaluation.
Full uses exact adapters and the public fraud/AML evaluators; its durable
operations and private integrations remain outside Core.
Core v0.3.0 is a breaking release; v0.2.0 remains available to existing consumers.

| Public package | Approved dependencies and authority |
| --- | --- |
| `domain` | Go standard library; canonical fraud types, exact decimal validation/comparison/sums |
| `rules` | Standard library and `domain`; portable fraud DSL, caller-supplied history/metrics |
| `engine` | Standard library, `domain`, `rules`; stateless framework and optional bounded memory decisions/replay |
| `iso20022` | Standard library, `domain`; optional canonical XML intake |
| `api` | Standard library, `domain`, `engine`, `iso20022`; optional minimal HTTP adapter |
| `cmd/byakugan-core` | Standard library and public adapter/engine/rules |
| `aml/domain` | Standard library; dedicated canonical AML inputs/results and AML exact decimal policy |
| `aml/scenarios` | Standard library and public AML domain; pure scenario observations/evidence references |

AML observations never return a fraud/payment decision, mutate a fraud result,
write ledger records, or persist alerts. Fraud's approved currency scales must
not constrain AML's separate decimal policy. Only curated pure evaluation code,
tests, documentation and synthetic examples belong in the public AML packages.

Full owns HTTP ingestion/administration, auth, licenses, integrations, private
scoring, PostgreSQL, Redis, migrations, durable idempotency, operational AML
workers/alerts, investigations/reports, protected evidence and operator UI.
Public packages must not import private packages, database clients, operational
services or commercial dependencies. There are currently no third-party Go
module dependencies. No private history or operational evidence is exportable.

## Behavioral baseline

`engine/compatibility_test.go` was run against the pre-migration evaluator and
freezes stable priority ordering, equal-priority registration order, score/hit/
reason order, blocked-account early exit, threshold escalation and the legacy
error-after-block result. The existing replay fixtures freeze original/conflict
replay behavior. A custom allow hit cannot downgrade a review/block. The legacy
engine can turn a block into review when a later rule errors; this is recorded
behavior, not a proposed failure policy. The stateless framework now exposes
conservative failure policies and Full's
separate ordering/early-exit semantics; the legacy adapter retains its fixture.

Monetary validation intentionally changes under the breaking contract described
in [migration-v0.3.md](migration-v0.3.md). Behavioral fixtures retain all
nonmonetary expectations through that change.
