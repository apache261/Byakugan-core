# Research basis for Byakugan Core

Reference review date: 2026-10-13.

This guide records the published studies and standards cited in the project's
research notes and relates them to the public Core implementation. Read the
[annotated references](research-references.md) for authors, dates, evidence,
and limitations, and the [validation guide](research-validation.md) for how to
evaluate your own configuration.

## What the evidence establishes

The cited studies report evidence for particular methods on their own datasets.
That evidence supports investigating contextual and relationship-based signals.
It does not establish Byakugan's detection accuracy or validate its default
thresholds. Core does not implement the cited research algorithms merely by
supporting similar input features.

This is an engineering interpretation of the references, not a reproduction
study or a claim that every feature originated in a particular paper. The
reference review checks primary publication pages and their reported summaries;
the experiments and datasets have not been independently reproduced here.

## Research-to-implementation map

The connections below are project interpretations. Follow each reference for
the published findings and each code link for the actual implementation.

| Design topic | Reference context | Implemented public Core behavior | Evidence boundary |
| --- | --- | --- | --- |
| Structured payment input | ISO 20022 catalogue, reference S1 | [pacs.008 adapter](../iso20022/pacs008.go) maps payment identity, money, accounts, agents, and remittance | Maps selected fields from the first transfer; no full XSD validation or lossless XML round trip |
| Account context and explicit policies | FATF risk-based banking guidance, S2 | [Fraud context and defaults](../engine/engine.go) accept caller-provided account restrictions, risk context, and thresholds | Configurable engineering inputs; no compliance certification or mandated fraud threshold |
| Recent transaction activity | MonLAD, R1 | [Portable rules](../rules/rules.go) support transfer count, exact amount sums, repeated amounts, and unique creditors | Related feature families; MonLAD's residual tracking and AnoScore algorithm are not bundled |
| Transaction flows and relationships | CubeFlow and network studies, R2–R5 | [Custom fraud framework](../engine/framework.go) accepts caller-supplied facts and evaluators | Core does not compute coupled tensors, network communities, cycles, or centrality rankings |
| Limits of static rules | Accounting autoencoder study, R6 | Custom evaluators allow an application to contribute additional explainable hits | Core ships no autoencoder or trained model; journal-entry evidence cannot establish payment-fraud performance |
| Separate AML observations | AML studies R1–R5 and the Core evaluation contract | [AML evaluator](../aml/scenarios/evaluator.go) evaluates supplied scenarios, metrics, policy values, and evidence references | AML observations are separate from fraud decisions; operational investigation remains with the caller |
| Reproducible evaluation | SynthAML, R7 | Pure evaluation contracts and [synthetic examples](../examples/README.md) make controlled integration exercises possible | The examples are not a labeled effectiveness benchmark; Core reports no SynthAML benchmark result |
| Multiple signal sources | Emerging banking-agent study, R8 | A custom evaluator can interpret application-prepared context | No LSTM, session detector, autonomous banking agent, or published fusion model is included |

References S1–S2 and R1–R8 are documented in
[the reference catalogue](research-references.md).

## Interpreting existing signals

Recent transfer count, repeated amounts, and accumulated amounts describe
activity. A match alone does not establish fraud, structuring, or laundering.
Payroll, merchant settlements, and treasury activity can produce similar
patterns. Select the relevant account population, currency, and time window
before interpreting a signal.

First-time-creditor rules identify an unfamiliar relationship in the supplied
history or metrics. They do not establish account takeover or a scam. Risk-tier
and recent-metadata-change rules consume context supplied by the application;
Core does not independently verify its source or meaning.

BIC-derived countries describe institution identifiers. They are not verified
customer residence, payment destination, or a sanctions result. Currency alone
also does not establish jurisdictional risk.

These interpretations follow the contracts in [rules.go](../rules/rules.go) and
[the framework reference](fraud-framework.md). Production usefulness requires
evaluation against your own outcomes and operating context.

## Defaults are engineering examples

[DefaultRules](../engine/engine.go) installs:

- A review signal for at least five prior debtor transfers in fifteen minutes,
  with score 40.
- A review signal for an amount at least `"100000"`, with score 50.

Neither threshold or score is derived from a cited study. The amount rule has
no currency predicate; adapt it to the currencies and populations you support
before treating it as an institution policy. Scores are additive risk points,
not calibrated probabilities. A score of 50 does not mean a 50% fraud likelihood.

Stateless evaluation needs caller-provided history or metrics for contextual
rules. Local replay/history is optional and bounded. See
[supplied aggregates](../examples/supplied-aggregates/README.md) and
[your own rules](../examples/own-rules.md).

## Research evidence and software evidence

Unit tests establish behavior such as exact money comparisons, rule matching,
error handling, and observation construction. They do not measure how well a
rule distinguishes real fraud from legitimate transactions. A paper's reported
result is evidence about that paper's method and dataset, not about a different
implementation or institution.

To support a future effectiveness claim, retain the dataset definition,
independent outcomes, evaluation period, configuration, error policy, and
measured operational results described in [the validation guide](research-validation.md).
