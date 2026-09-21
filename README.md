# Byakugan Core

Byakugan Core is a small, headless Go service for synchronous fraud checks on
ISO 20022-style credit transfers. It provides a canonical JSON request, a
`pacs.008` XML adapter, an in-memory idempotency store, and a portable JSON rule
evaluator. It has no external runtime dependencies.

This repository intentionally contains the public decision-engine foundation.
Operational user interfaces and commercial extensions are distributed
separately.

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

```sh
API_KEYS=local-secret go run ./cmd/byakugan-core
curl -H 'X-API-Key: local-secret' -H 'Content-Type: application/json' \
  --data @examples/transfer.json http://localhost:8080/v1/transfer-checks
```

## Rules

The built-in rule reviews transfers whose amount is at least 100,000. Provide a
JSON array of rules with `BYAKUGAN_RULES_FILE` to replace it. The public DSL
supports `all`, `any`, and comparisons over amount, currency, account IDs,
agent BICs, remittance information, and message type. Supported comparisons are
`eq`, `neq`, `contains`, `not_contains`, `in`, `not_in`, `gt`, `gte`, `lt`, and
`lte` where appropriate.

The service returns a fraud signal only. An `allow` result is not a settlement
instruction and must not be treated as one.

API details are in [docs/openapi.yaml](docs/openapi.yaml).

## Full Suite and Dashboard

The administrative dashboard and full-featured Byakugan suite are maintained
privately. The Full edition includes the operator dashboard, commercial
licensing, AML/CFT workflows, advanced fraud capabilities, integrations, and
production deployment tooling.

For private Full edition access, dashboard inquiries, or commercial support,
contact [lynolibarra@gmail.com](mailto:lynolibarra@gmail.com).

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
