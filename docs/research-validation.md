# Validate your own fraud and AML configuration

This is a proposed evaluation protocol for Core consumers. It records no
completed institutional trial or measured detection result. Its purpose is to
make future claims reviewable and reproducible. Research context and limitations
are documented in [the reference catalogue](research-references.md).

## Define the question before choosing a threshold

Specify whether you are evaluating payment fraud, a particular AML typology,
review prioritization, or a hard policy limit. Define the evaluation unit:
transfer, account, relationship, or observation. Fraud decisions and AML
observations need separate datasets, outcomes, and reports.

Define how outcomes become known and when they are sufficiently mature. An
analyst flag or rule match alone is not independent ground truth. Record the
coverage and uncertainty of investigation, customer reports, confirmed losses,
and unresolved outcomes. Do not silently label unresolved activity legitimate.

## Freeze the evaluation inputs

For a trial, retain:

- Core version and application adapter version.
- Rule/scenario definitions, enabled IDs, scores, thresholds, and failure policy.
- Dataset population, currencies, time period, inclusion criteria, and labels.
- Fact/metric provenance, query scopes, time windows, and missing-data handling.
- Replay policy and how repeated instructions are counted.
- Split boundaries, configuration changes, and evaluation scripts.

Use chronological development, calibration, and held-out evaluation periods.
Compute history and features only from information available at the evaluated
time. Do not use future transfers or eventual investigation outcomes as inputs.
Evaluate whether shared accounts across splits cause leakage for your intended
deployment question. Report separately if the trial covers familiar accounts
or previously unseen accounts.

## Measure effectiveness and operating cost

Record explicit denominators rather than reporting accuracy alone:

| Measure | Definition or reporting requirement |
| --- | --- |
| Precision | Confirmed positive outcomes divided by flagged items with sufficiently resolved outcomes; report unresolved flags separately |
| Recall | Detected positive outcomes divided by all known positives in the evaluated population; state how unflagged outcomes were ascertained |
| False-positive rate | Flagged legitimate items divided by all resolved legitimate items in the evaluated population |
| Review rate | Items sent to review divided by all eligible items |
| Detection delay | Elapsed time from the defined event or pattern onset to the signal |
| Loss coverage | Confirmed positive monetary value captured, reported separately by currency and outcome definition |
| Operating cost | Review volume, analyst effort, and the effect of intervening on legitimate customers |
| Runtime behavior | Evaluation latency and errors, distinguished from upstream data retrieval and downstream actions |

If only flagged items receive outcome investigation, precision may be
estimable for that subset; population recall and false-positive rate are not
established by that review alone. Document the sampling or independent outcome
coverage needed to estimate them. For AML, define a positive outcome precisely;
an observation is not a finding of a crime.

Compare your configuration with simple baseline rules on the same eligible
population. Report counts, uncertainty, and segment results for account type,
currency, amount band, and data availability. Group uncertainty estimates by
account or time when observations are dependent. Choose thresholds on the
calibration period, then evaluate the frozen configuration on held-out data.

## Test feature and failure behavior separately

Exercise exact monetary boundaries, currency mismatches, missing facts,
incomplete history, late data, cancellations, invalid input, and evaluator
errors. Confirm that upstream failures cannot masquerade as zero activity or
a successful evaluation. Check whether several correlated rules double-count
one behavior and inflate the aggregate score.

Use synthetic records for controlled boundary tests. SynthAML is an external
example of a documented benchmark resource, not proof that synthetic tests
predict your own deployment performance.
[SynthAML publication](https://www.nature.com/articles/s41597-023-02569-2).

## Report results at the right scope

Start with observed signals alongside existing operations where feasible,
record outcomes, and review the effect of any proposed intervention. Revisit
performance as populations, behavior, and outcome coverage change.

A useful result statement names the exact configuration, dataset period,
population, label coverage, measured counts and rates, and limitations. Keep
software correctness checks separate from detection-effectiveness results.
Until such a trial is complete, describe defaults as example policies and
research papers as contextual evidence.
