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

Core provides reusable fraud and AML evaluation contracts. Applications supply
their own data, integrations, persistence, and operational workflows.

The v0.3.0 release introduces exact fraud money, stateless fraud extensions and
pure AML evaluation. v0.2.0 remains available for existing consumers. Read the
[breaking migration guide](docs/migration-v0.3.md),
[package boundary](docs/framework-boundary.md),
[fraud framework](docs/fraud-framework.md) and [AML evaluation](docs/aml-evaluation.md).

## Core Highlights

- Dependency-free Go library and optional `net/http` server.
- Canonical transfer model plus ISO 20022 `pacs.008` parsing.
- Strict JSON rule DSL with nested `all` and `any` conditions.
- Amount, currency, account, BIC, route, remittance, and execution-date rules.
- Time-window velocity, accumulated amount, repeated-amount, unique-creditor,
  and first-time-beneficiary signals.
- Account risk tiers, blocked accounts, and per-account disabled rules.
- Score-based review and block thresholds with explainable rule hits.
- Stateless, context-aware custom Go fraud and AML evaluators.
- Explicit aggregation/failure policies and immutable evaluator registries.
- Optional thread-safe bounded history and idempotent request replay.
- Precomputed metric injection for database-backed, high-volume integrations.
- Atomic runtime rule replacement and strict rejection of unknown rule fields.
- API-key protection for decision endpoints, plus health and readiness routes.
- Apache License 2.0 with no proprietary runtime dependency.

## Use as a Library

Start with the [step-by-step examples](examples/README.md):
[import and evaluate a transfer](examples/getting-started.md),
[convert your own ISO 20022 XML](examples/iso20022-convert/README.md),
[write your own rules](examples/own-rules.md),
[implement a custom Go evaluator](examples/custom-fraud/README.md),
[supply database aggregates](examples/supplied-aggregates/README.md), or
[create separate AML observations](examples/custom-aml/README.md).

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
- `engine`: stateless fraud framework, explicit policies, optional replay/history,
  account blocking, rule disabling and atomic rule replacement.
- `aml/domain`: evaluation transaction/subject contracts, exact decimals and bounded DSL.
- `aml/scenarios`: pure AML observations and custom evaluator registries.
- `api`: an optional minimal `net/http` adapter.

For new extensions, use `engine.NewFramework` and its context-aware `Evaluate`
method; add `engine.NewReplayEngine` only when local history/replay is wanted.
AML uses separate `aml/scenarios` evaluation and returns observations.
The independent consumer modules pin v0.3.0. Run them against the released
module from the Core repository root:

```sh
(cd examples/custom-fraud && GOWORK=off go mod download github.com/apache261/Byakugan-core && GOWORK=off go run . < ../transfer.json)
(cd examples/custom-aml && GOWORK=off go mod download github.com/apache261/Byakugan-core && GOWORK=off go run .)
go run ./examples/supplied-aggregates < examples/transfer.json
```

To test changes against local source, use a temporary workspace:

```sh
core_source=$PWD
framework_workspace=$(mktemp -d)
(cd "$framework_workspace" && GOWORK=off go work init "$core_source" "$core_source/examples/custom-fraud" "$core_source/examples/custom-aml")
(cd examples/custom-fraud && GOWORK="$framework_workspace/go.work" go run . < ../transfer.json)
(cd examples/custom-aml && GOWORK="$framework_workspace/go.work" go run .)
rm -rf "$framework_workspace"
```

## About Byakugan

Byakugan is an explainable bank-transfer fraud detection project built for
learning, prototyping, and lightweight deployments. Its name is inspired by
the Byakugan ability from the *Naruto* series: the system is intended to help
operators clearly "see" transfer risk, account behavior, matched rules, and
decision signals.

The project was created in response to the limited number of approachable
open-source fraud detection services available to developers. Byakugan Core
provides a dependency-light foundation that can be studied, extended, and
integrated into applications with caller-supplied infrastructure.

Byakugan fraud evaluation produces signals; AML evaluation produces separate
observations. Neither settles, authorizes, rejects or holds payments.

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

Set `CORE_API_KEY` to your local test key before running:

```sh
API_KEYS="$CORE_API_KEY" go run ./cmd/byakugan-core
curl -H "X-API-Key: $CORE_API_KEY" -H 'Content-Type: application/json' \
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

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

## Core or Full?

[Compare editions and explore Byakugan Full](readmepro.md).
