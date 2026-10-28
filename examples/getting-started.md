# Import Core and evaluate a transfer

This example creates your own application outside the Core repository. Install
Go 1.24 or newer, then create a module and pin Core v0.3.1:

```sh
mkdir my-fraud-app
cd my-fraud-app
go mod init example.com/my-fraud-app
go get github.com/apache261/Byakugan-core@v0.3.1
```

Import packages under `github.com/apache261/Byakugan-core`, preserving the case
in the module path. No database or cache is required for this example.

## Create the application

Save this as `main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"

    "github.com/apache261/Byakugan-core/domain"
    "github.com/apache261/Byakugan-core/engine"
)

func main() {
    var request domain.TransferCheckRequest
    if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
        log.Fatal(err)
    }

    detector, err := engine.NewFramework(
        engine.DefaultRules(), nil, engine.DefaultPolicy(),
    )
    if err != nil {
        log.Fatal(err)
    }

    result, err := detector.Evaluate(context.Background(), request, engine.Context{})
    if err != nil {
        log.Fatal(err)
    }
    if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
        log.Fatal(err)
    }
}
```

Save this as `transfer.json` (also available as [the sample transfer](transfer.json)):

```json
{
  "message_type": "pacs.008",
  "idempotency_key": "example-001",
  "payload": {
    "message_id": "MSG-001",
    "creation_date_time": "2026-09-21T08:00:00Z",
    "payment_identification": { "end_to_end_id": "E2E-001" },
    "instructed_amount": { "amount": "1250.50", "currency": "PHP" },
    "debtor_account": { "id": "debtor-001" },
    "debtor_agent": { "bicfi": "DEUTPHMM" },
    "creditor_account": { "id": "creditor-001" },
    "creditor_agent": { "bicfi": "BOPIPHMM" }
  }
}
```

Amounts are decimal strings. Keep them as strings throughout JSON, application
mapping, and rule thresholds; converting through floating point loses exactness.

Run your application:

```sh
gofmt -w main.go
go run . < transfer.json
```

With no prior history and an amount below the default threshold, the result is:

```json
{"decision":"allow","score":0,"reasons":["no fraud signal detected"],"rule_hits":[]}
```

Change the amount to `"100000.00"` to match the default high-amount rule: the
decision becomes `review`, with score `50` and an explanatory rule hit.

## Choose evaluation state explicitly

`engine.Framework` is stateless. Repeating an idempotency key still evaluates
the request again, and velocity rules require caller-supplied history or metrics.
Create and reuse a framework instance in your application.

If you need local bounded history and replay, replace the `Evaluate` call above
with this code after creating `detector`:

```go
replay, err := engine.NewReplayEngine(detector, engine.Options{
    MaxHistoryEntries: 1000,
    MaxIdempotencyEntries: 1000,
})
if err != nil {
    log.Fatal(err)
}
result, err := replay.Check(context.Background(), request, engine.Context{})
if err != nil {
    log.Fatal(err)
}
```

Reuse the same `replay` instance across requests to retain history and cached
results. Restart or eviction loses that state. Durable replay and storage belong
to your application. Always handle evaluation errors before acting on a result.

Next, [write your own rules](own-rules.md) or
[add a custom Go evaluator](custom-fraud/README.md). See the
[framework reference](../docs/fraud-framework.md) for policies and concurrency.
