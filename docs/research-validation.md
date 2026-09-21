# Testing your fraud and AML rules

A rule can pass its tests and still send too many ordinary payments to review.
Once you know that it evaluates correctly, you need to find out how it behaves
with your data and what happens when people act on its signals.

This guide walks through that evaluation. It does not report a completed
institutional trial or measured detection results. For the studies behind the
approach, see the [reference catalogue](research-references.md).

## Decide what you need to learn

Start with the problem you are trying to address: payment fraud, an AML typology,
review prioritization, or a hard policy limit. Your choice determines what you
count—a transfer, an account, a relationship, or an observation. Keep fraud
and AML evaluations separate, with their own datasets, outcomes, and reports.

Then consider how you will learn what actually happened. A rule match or an
analyst flag does not establish ground truth on its own. Investigations, customer
reports, and confirmed losses may cover only part of your population, and some
outcomes take time to become clear. Record that coverage and the time allowed
for outcomes to mature. Keep unresolved cases visible in the results.

## Leave enough detail to repeat the trial

Months later, someone should be able to run the same evaluation and understand
why it produced those results. Keep these details with the trial:

- The Core version and application adapter version.
- Rule and scenario definitions, enabled IDs, scores, thresholds, and failure policy.
- Dataset population, currencies, period, inclusion criteria, and labels.
- Fact and metric provenance, query scopes, windows, and missing-data handling.
- Replay policy and the way repeated instructions are counted.
- Split boundaries, configuration changes, and evaluation scripts.

Use separate chronological periods for development, threshold calibration, and
the held-out evaluation. When you replay an event, give it only the history and
features available at that time. Future transfers or later investigation outcomes
can make a replay look more successful than it would have been in operation.

Check which accounts appear across those periods. You may be testing behavior
on familiar accounts, accounts the configuration has never seen, or both. Report
those groups separately and explain any information leakage from shared history.

## Measure detection and the work it creates

An accuracy number alone tells you little about the review queue or the customers
affected. These measures help you see what the configuration catches and what
it costs to act on it. State the denominator for each rate so readers can tell
which items were counted.

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

If you investigate only flagged items, you may be able to estimate precision
for that subset. You still need sampling or independent outcome coverage to
estimate population recall and the false-positive rate. Explain how you obtained
that coverage. For AML, define the positive outcome carefully: an observation
does not establish that a crime occurred.

Compare your configuration with a simple baseline on the same eligible
population. Report counts and uncertainty, then break down the results by account
type, currency, amount band, and data availability. Where observations depend on
one another, group uncertainty estimates by account or time. Choose thresholds
during calibration and freeze the configuration before the held-out evaluation.

## Check the cases that are easy to miss

Population results can hide problems at the edges. Test exact monetary boundaries,
currency mismatches, missing facts, incomplete history, late data, cancellations,
invalid input, and evaluator errors. Keep monetary values exact and report
currencies separately. An upstream failure needs to remain visible as a failure;
it should not appear as zero activity or a successful evaluation. Check for
correlated rules too: several rules may count the same behavior and inflate
the score.

Synthetic records let you control these cases precisely. The
[SynthAML publication](https://www.nature.com/articles/s41597-023-02569-2)
describes one benchmark resource. Results on synthetic data still need validation
against your own deployment before you use them to make performance claims.

## Share what you actually learned

Where feasible, start by observing signals alongside existing operations. Record
outcomes and review the likely effect of an intervention before putting it into
practice. Revisit the evaluation as your population, behavior, or outcome coverage
changes.

When you publish results, include the exact configuration, dataset period,
population, label coverage, measured counts and rates, and limitations. Readers
need that context to judge whether the findings apply to their situation.
Report software correctness and detection effectiveness separately. Until an
institutional trial is complete, treat defaults as example policies and research
papers as background evidence.
