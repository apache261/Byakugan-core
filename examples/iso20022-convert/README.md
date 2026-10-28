# Convert your own ISO 20022 XML to a Byakugan payload

[main.go](main.go) reads your ISO 20022 `pacs.008` XML from stdin and prints a
canonical `domain.TransferCheckRequest` as JSON. It uses Core's existing
`iso20022.ParsePacs008` adapter, then validates the mapped transfer. Conversion
does not evaluate fraud or create a payment decision.

## Try the sample

With Go 1.24 or newer, run from the Core repository root:

```sh
go run ./examples/iso20022-convert -idempotency-key xml-example-001 \
  < examples/iso20022-convert/transfer.xml > converted-transfer.json
cat converted-transfer.json
```

[transfer.xml](transfer.xml) is a synthetic single-transfer example. Output
includes `message_type: "pacs.008"`, the supplied idempotency key, and a `payload`
with message ID `MSG-XML-001`, account IDs, agent BICs, payment identifiers,
names, and remittance text. Its `instructed_amount` is:

```json
{"amount":"1250.5","currency":"PHP"}
```

The canonical amount removes insignificant trailing zeros while retaining
exact value. It remains a JSON string throughout conversion.

## Convert your own file

Replace the input path with your XML file:

```sh
go run ./examples/iso20022-convert -idempotency-key your-source-transfer-001 \
  < /path/to/your-transfer.xml > your-transfer.json
```

Choose a stable key for the same source transfer when using replay downstream.
The converter copies the key; it does not store replay state. Parsing or domain
validation failure exits with a nonzero status and writes the error to stderr.
Consume the output file only after the command succeeds.

To evaluate the converted JSON with an existing example, run:

```sh
(cd examples/custom-fraud && GOWORK=off go run . < ../../converted-transfer.json)
```

That example also supplies its own synthetic risk flag. For an application
using just your configured rules, follow [getting started](../getting-started.md).

## Import the converter into your application

Inside your Go module, install the public release:

```sh
go get github.com/apache261/Byakugan-core@v0.3.1
```

Import `github.com/apache261/Byakugan-core/iso20022` and pass an `io.Reader`
containing the XML. For example, replace the JSON stdin decoder in
[getting started](../getting-started.md) with:

```go
request, err := iso20022.ParsePacs008(os.Stdin, "your-source-transfer-001")
if err != nil {
    log.Fatal(err)
}
```

Remove the previous `var request` declaration and its unused `domain` import,
and add the `iso20022` import. The rest of that application's framework
construction, evaluation, and JSON output stays the same. Handle validation
explicitly with `domain.ValidateTransfer` for conversion-only workflows, as
the runnable converter does. Framework evaluation also validates its request.

## XML-to-payload mapping

Paths below are relative to `FIToFICstmrCdtTrf` unless otherwise specified.

| XML source | Byakugan request field |
| --- | --- |
| Caller-supplied key | `idempotency_key` |
| `GrpHdr/MsgId` (fallback: `AppHdr/BizMsgIdr`) | `payload.message_id` |
| `GrpHdr/CreDtTm` (fallback: `AppHdr/CreDt`) | `payload.creation_date_time` |
| `AppHdr/MsgDefIdr`, or transfer element namespace | `payload.message_definition_id` |
| Transfer element namespace | `payload.schema_namespace` |
| `CdtTrfTxInf/PmtId/InstrId`, `EndToEndId`, `TxId`, `ClrSysRef` | Fields in `payload.payment_identification` |
| `CdtTrfTxInf/IntrBkSttlmAmt`, fallback to `InstdAmt` if empty | `payload.instructed_amount.amount` and `.currency` from `Ccy` |
| `CdtTrfTxInf/Dbtr/Nm`, `Cdtr/Nm` | `payload.debtor.name`, `payload.creditor.name` |
| `CdtTrfTxInf/DbtrAcct/Id/IBAN`, `CdtrAcct/Id/IBAN` | Corresponding account `id` and `iban` |
| Account `Id/Othr/Id` when IBAN is absent | Corresponding account `id` and `bban` |
| `CdtTrfTxInf/DbtrAgt/FinInstnId`, `CdtrAgt/FinInstnId` | Agent `bicfi` and `name` |
| `CdtTrfTxInf/RmtInf/Ustrd` | Joined `payload.remittance_information` |
| `GrpHdr/IntrBkSttlmDt` when present | `payload.interbank_settlement_date` |

Agent fallback uses transaction `InstgAgt`/`InstdAgt`, then the corresponding
`AppHdr` party, when the primary agent has neither BIC nor name. The optional
header must occur before the transfer element for the adapter to read it.

## Supported input scope

The adapter maps the **first** `CdtTrfTxInf` only. Split multi-transfer documents
in your application if every transfer must be processed. It reads at most
2 MiB and maps supported fields rather than performing ISO XSD validation;
validate against your institution's schema/profile separately when required.
It does not preserve every XML element or provide a lossless round trip.

This example handles `pacs.008` credit transfers. For other ISO message types
or institution-specific fields, write your own adapter that maps into the
public domain contracts, then validate the resulting transfer. Keep money
exact and account, agent, and payment identity mapping explicit.
