# Byakugan Core examples

Use these guides to import Core into your own Go application and extend it with
your own rules, data, and evaluators. Go 1.24 or newer is required. All data in
these examples is synthetic.

| Guide | What you will build |
| --- | --- |
| [Getting started](getting-started.md) | A new Go module that reads a transfer and prints a fraud result |
| [Your own rules](own-rules.md) | An exact amount rule, nested conditions, and runtime rule replacement |
| [Custom fraud evaluator](custom-fraud/README.md) | Go logic that turns an application fact into an explainable rule hit |
| [Supplied aggregates](supplied-aggregates/README.md) | Evaluation using exact totals supplied by your application |
| [Custom AML evaluator](custom-aml/README.md) | A pure scenario and a custom observation using exact AML money |

Start with getting started, then choose JSON rules for portable conditions or a
Go evaluator for application-specific logic. You can combine both in one fraud
framework. AML uses its own packages and returns separate observations.

## Run the existing examples

From the Core repository root (the `opensource/core` directory in the Full
source checkout), run these commands against the released v0.3.0 module:

```sh
(cd examples/custom-fraud && GOWORK=off go mod download github.com/apache261/Byakugan-core && GOWORK=off go run . < ../transfer.json)
(cd examples/custom-aml && GOWORK=off go mod download github.com/apache261/Byakugan-core && GOWORK=off go run .)
go run ./examples/supplied-aggregates < examples/transfer.json
```

The fraud and AML examples are independent consumer modules with their own
`go.mod`; the aggregate example belongs to the Core module.

To run the independent examples against edited local Core source instead of
the release, create a temporary workspace from the Core root:

```sh
core_source=$PWD
framework_workspace=$(mktemp -d)
(cd "$framework_workspace" && GOWORK=off go work init "$core_source" "$core_source/examples/custom-fraud" "$core_source/examples/custom-aml")
(cd examples/custom-fraud && GOWORK="$framework_workspace/go.work" go run . < ../transfer.json)
(cd examples/custom-aml && GOWORK="$framework_workspace/go.work" go run .)
rm -rf "$framework_workspace"
```

Core supplies evaluation. Your application supplies data retrieval, persistence,
authentication, payment handling, and operational workflows. A fraud `allow`
signal does not authorize or settle a payment.
