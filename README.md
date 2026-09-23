# Byakugan Core

**Explainable payment-fraud decisions without a heavyweight platform.**

Byakugan Core is the Apache-2.0 fraud-decision library behind Byakugan. Embed it
in a Go application or run the included small `net/http` service. Core turns
canonical JSON or ISO 20022 `pacs.008` transfers into synchronous `allow`,
`review`, or `block` signals with scores, matched rules, and human-readable
reasons.

Core has no external runtime dependencies. It is suitable for learning,
prototyping, local services, integration tests, and lightweight production
workloads where an application supplies its own durable infrastructure.

Rule documents are validated strictly. Unknown fields and trailing JSON are
rejected so commercial extensions cannot be mistaken for Community rules.

The fraud engine is intentionally public and useful on its own. **Byakugan
Full** uses the same Core contracts and evaluator, then adds durable operations,
investigation tooling, advanced signals, and a standalone administration
dashboard.

## Core Highlights

- Dependency-free Go library and optional `net/http` server.
- Canonical transfer model plus ISO 20022 `pacs.008` parsing.
- Strict JSON rule DSL with nested `all` and `any` conditions.
- Amount, currency, account, BIC, route, remittance, and execution-date rules.
- Time-window velocity, accumulated amount, repeated-amount, unique-creditor,
  and first-time-beneficiary signals.
- Account risk tiers, blocked accounts, and per-account disabled rules.
- Score-based review and block thresholds with explainable rule hits.
- Thread-safe bounded history and idempotent request replay.
- Precomputed metric injection for database-backed, high-volume integrations.
- Atomic runtime rule replacement and strict rejection of unknown rule fields.
- API-key protection for decision endpoints, plus health and readiness routes.
- Apache License 2.0 with no proprietary runtime dependency.

## Core or Full?

| Capability | Byakugan Core | Byakugan Full |
| --- | --- | --- |
| License and source | Apache-2.0, public | Proprietary, private distribution |
| Primary use | Embeddable fraud library and lightweight HTTP service | Production fraud operations and AML/CFT platform |
| JSON and ISO 20022 `pacs.008` intake | Included | Included through Core |
| Explainable rule DSL and synchronous decisions | Included | Included through Core, with managed rule workflows |
| Velocity, beneficiary, route, remittance, and risk-tier rules | In-memory history or caller-provided metrics | Durable aggregate queries backed by PostgreSQL |
| Idempotency | Bounded in-memory replay | Durable PostgreSQL/Redis-aware processing |
| Storage | Bring your own, or use bounded memory | PostgreSQL stores and migrations with in-memory development fallback |
| Cache and multi-replica readiness | Bring your own | Redis integration and operational readiness checks |
| Account and configuration management | Library policy inputs | Managed accounts, scoped rules, runtime configuration, and imports |
| Operator interface | Not included | Independently deployable W2UI administration dashboard |
| Fraud investigations | Decision reasons and rule hits | Cases, priorities, notes, evidence attachments, approvals, and saved views |
| Advanced fraud signals | Portable deterministic signals | Behavior profiles, network analysis, watchlists, and external/ML scores |
| Payment-status correlation | Not included | ISO 20022 `pacs.002` intake, correlation, and audit trail |
| Integrations | Library hooks | Signed webhooks, retries, replay, delivery history, and dead-letter exports |
| Security and governance | Optional static API keys | Managed API keys, admin sessions, roles, scopes, audit events, and retention controls |
| AML/CFT | Intentionally separate and not included | Append-only ledger, ingestion, scenarios, screening, alerts, investigations, reports, legal holds, and historical imports |
| Deployment | Go binary or embedded package | API, dashboard, PostgreSQL, Redis, migrations, licensing, and deployment assets |

Choose **Core** when you want a transparent engine you can embed and extend.
Choose **Full** when you also need durable operations, analysts and casework,
governed administration, AML/CFT workflows, and production integrations.

## Use as a Library

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/apache261/Byakugan-core/domain"
    "github.com/apache261/Byakugan-core/engine"
)

func check(request domain.TransferCheckRequest) {
    rule := domain.Rule{
        ID: "rapid-debtor-volume", Name: "Rapid debtor volume",
        Priority: 10, Enabled: true,
        Definition: json.RawMessage(`{
          "field":"velocity.debtor_transfer_count",
          "op":"gte", "value":3, "window_minutes":15,
          "outcome":"review", "score":40,
          "reason":"three prior transfers in fifteen minutes"
        }`),
    }

    detector := engine.New([]domain.Rule{rule})
    result := detector.Check(request)
    fmt.Println(result.Decision, result.Score, result.Reasons)
}
```

`Engine.Check` uses the engine's bounded in-memory decision history for
time-window rules. Applications with database aggregates can call
`Engine.CheckWithContext` and provide `rules.MetricsByWindow`. This keeps the
library independent of PostgreSQL, Redis, and any specific storage interface.

The main reusable packages are:

- `domain`: canonical transfer, rule, hit, and decision contracts.
- `iso20022`: dependency-free `pacs.008` to canonical-request adapter.
- `rules`: strict portable DSL validation and contextual evaluation.
- `engine`: thread-safe decisions, idempotency, history, thresholds, account
  blocking, per-account rule disabling, and atomic rule replacement.
- `api`: an optional minimal `net/http` adapter.

## About Byakugan

Byakugan is an explainable bank-transfer fraud detection project built for
learning, prototyping, and lightweight deployments. Its name is inspired by
the Byakugan ability from the *Naruto* series: the system is intended to help
operators clearly "see" transfer risk, account behavior, matched rules, and
decision signals.

The project was created in response to the limited number of approachable
open-source fraud detection services available to developers. Byakugan Core
provides a dependency-light foundation that can be studied, extended, and
integrated without requiring the proprietary dashboard or commercial suite.

Byakugan produces fraud signals only. It does not settle, authorize, reject,
or hold payments.

## Run

Go 1.24 or newer is required.

```sh
go test ./...
go run ./cmd/byakugan-core
```

The server listens on `:8080` by default. Set `HTTP_ADDR` to change the address.
Set `API_KEYS` to a comma-separated list to require an `X-API-Key` header on
decision endpoints. Health endpoints remain unauthenticated.
Idempotency results are held in memory and are lost at restart. The cache uses
bounded FIFO eviction and holds 10,000 key-and-payload results by default; set
`MAX_IDEMPOTENCY_ENTRIES` to a positive integer to change the cap.
Decision history is also in memory and retains 10,000 non-replayed checks by
default; set `MAX_HISTORY_ENTRIES` to change the cap.

```sh
API_KEYS=local-secret go run ./cmd/byakugan-core
curl -H 'X-API-Key: local-secret' -H 'Content-Type: application/json' \
  --data @examples/transfer.json http://localhost:8080/v1/transfer-checks
```

## Rules

The built-in rule reviews transfers whose amount is at least 100,000. Provide a
JSON array of rules with `BYAKUGAN_RULES_FILE` to replace it. The public DSL
supports nested `all` and `any` groups and the following signal families:

- transfer values, identifiers, accounts, BICs, and remittance text;
- BIC-derived debtor/creditor countries and country pairs;
- requested execution dates and days-ahead checks;
- account risk tiers and recent account metadata changes;
- debtor transfer count, amount sum, unique creditors, repeated amounts, and
  incoming/outgoing accumulated amounts over minute or hour windows;
- first-time-beneficiary checks.

Supported comparisons are `eq`, `neq`, `contains`, `not_contains`, `in`,
`not_in`, `gt`, `gte`, `lt`, and `lte` where appropriate. Rule documents reject
unknown fields, conflicting windows, invalid outcomes, and trailing JSON.

Applications can provide raw `rules.Context.History` for modest workloads or
precomputed `rules.Metrics` for high-volume stores. Use
`rules.MetricWindowsFromRules` to discover the exact aggregate windows needed by
a configured rule set.

The service returns a fraud signal only. An `allow` result is not a settlement
instruction and must not be treated as one.

API details are in [docs/openapi.yaml](docs/openapi.yaml).

## Byakugan Full

Byakugan Full is for teams that need more than a decision function. It packages
Core into an operational fraud and AML/CFT system while preserving the same
explainable rule results and canonical transfer contracts.

Full adds:

- a separate, independently deployable fraud-operations dashboard;
- PostgreSQL persistence, Redis caching, migrations, readiness gates, exports,
  audit trails, and retention workflows;
- managed accounts, rules, configurations, API keys, administrators, roles,
  sessions, and approval controls;
- fraud cases with notes, attachments, prioritization, metrics, saved views,
  and manual-review decisions;
- behavior profiling, network/link signals, watchlists, and governed external
  model-score ingestion;
- signed webhook delivery with policy controls, retries, replay, delivery
  evidence, and dead-letter reporting;
- ISO 20022 `pacs.002` payment-status correlation independent of fraud decisions;
- a separate AML/CFT ledger with transaction ingestion, immutable scenario and
  policy versions, screening evidence, monitoring jobs, alerts, investigations,
  four-eyes reports, legal holds, retention, and controlled historical imports;
- offline commercial licensing and production deployment assets.

Core remains the reusable fraud engine; Full supplies the private operational
and compliance layers around it. For Full edition access, dashboard inquiries,
integration planning, or commercial support, contact
[lynolibarra@gmail.com](mailto:lynolibarra@gmail.com).

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
