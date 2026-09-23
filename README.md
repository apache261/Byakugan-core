# Byakugan Core

Byakugan Core is the open-source fraud-decision library behind Byakugan. It can
be embedded directly in a Go application or run through the included small
`net/http` server. It provides canonical payment types, strict validation,
`pacs.008` parsing, contextual and time-window rules, score-based policies,
bounded in-memory history, and idempotent synchronous decisions. It has no
external runtime dependencies.

Rule documents are validated strictly. Unknown fields and trailing JSON are
rejected so commercial extensions cannot be mistaken for Community rules.

The fraud engine is intentionally public and useful on its own. The commercial
Full edition builds on this module for its API platform, persistence,
operations, dashboard, and private signal sources.

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

## Full Suite and Dashboard

The administrative dashboard and full-featured Byakugan suite are maintained
privately. The Full edition adds AML/CFT casework, behavior and network
analytics, external/ML scoring, watchlists, durable PostgreSQL/Redis adapters,
webhooks, audit and approval workflows, access control, licensing, and
production deployment tooling. Those features consume Core rather than being
required by it.

For private Full edition access, dashboard inquiries, or commercial support,
contact [lynolibarra@gmail.com](mailto:lynolibarra@gmail.com).

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
