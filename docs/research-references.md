# Annotated research and standards references

Primary source pages reviewed on 2026-10-13. These references were cited in
the project's existing research notes; this catalogue makes their scope and
relationship to public Core explicit. Findings below summarize publisher or
author-reported descriptions. They are not independently replicated results.

An arXiv record is an author-hosted research source; its existence alone does
not establish peer review. Standards and guidance describe expectations or
data contracts rather than experimentally measured detector accuracy.

## Standards and guidance

### S1. ISO 20022 message definitions

**Source:** ISO 20022 Registration Authority,
[ISO 20022 Message Definitions](https://www.iso20022.org/iso-20022-message-definitions).

The catalogue provides message definitions and access to schemas, Message
Definition Reports, and usage guidelines. It is the structural reference for
the project's `pacs.008` mapping, rather than evidence of fraud-detection
effectiveness. Use the exact message version and payment-system profile you
integrate with; parsing supported fields does not demonstrate schema compliance.
[Official catalogue](https://www.iso20022.org/iso-20022-message-definitions).

### S2. FATF risk-based approach

**Sources:** FATF,
[Risk-Based Approach for the Banking Sector](https://www.fatf-gafi.org/en/publications/Fatfrecommendations/Risk%2Dbased-approach-banking-sector.html)
(October 2014), and
[the FATF Recommendations](https://www.fatf-gafi.org/en/publications/Fatfrecommendations/Fatf-recommendations.html).

The banking guidance describes assessing and understanding AML/CFT exposure
and applying mitigation proportionate to risk. This is context for configurable
policies and caller-supplied risk facts, not an endorsement of particular fraud
scores. FATF explicitly notes that the 2014 guidance does not incorporate later
standards revisions, including 2025 changes; read it alongside current
Recommendations. [Banking guidance](https://www.fatf-gafi.org/en/publications/Fatfrecommendations/Risk%2Dbased-approach-banking-sector.html).

## Empirical studies and reported experiments

### R1. MonLAD: Money Laundering Agents Detection in Transaction Streams

**Citation:** Xiaobing Sun et al. (2022), arXiv:2201.10051.
[Author record](https://arxiv.org/abs/2201.10051);
[related publication DOI](https://doi.org/10.1145/3488560.3498418).

**Reported evidence:** MonLAD and MonLAD-W track transaction-stream residuals
and other account features; AnoScore identifies statistical deviations. The
authors report comparisons on real-world data and manual verification of some
suspected laundering agents. [Study](https://arxiv.org/abs/2201.10051).

**Project interpretation:** Motivates examining account activity across time,
instead of evaluating only an isolated amount. Core supplies simpler velocity
metrics and extension contracts, not this algorithm. Reported agent detection
does not validate Core's fifteen-minute transfer-count threshold.

### R2. CubeFlow: Money Laundering Detection with Coupled Tensors

**Citation:** Xiaobing Sun et al. (2021), arXiv:2103.12411.
[Author record](https://arxiv.org/abs/2103.12411).

**Reported evidence:** Models transaction flows using coupled tensors to identify
chains through intermediary accounts. The authors report comparisons with
baseline methods using synthetic and real data. [Study](https://arxiv.org/abs/2103.12411).

**Project interpretation:** Supports investigating linked incoming/outgoing
activity. A caller may prepare related facts for evaluation, but Core does not
ship CubeFlow, tensor construction, or its optimization procedure. An amount
sum alone does not reproduce the study's method.

### R3. Anomaly detection in cross-country money transfer temporal networks

**Citation:** Salvatore Vilella et al. (2023; revised 2024),
arXiv:2311.14778v2. [Author record](https://arxiv.org/abs/2311.14778v2).

**Reported evidence:** Examines changes in centrality-based node rankings in a
dataset of more than 80 million cross-country wire transfers, using abrupt
changes to identify outliers for expert investigation.
[Study](https://arxiv.org/abs/2311.14778v2).

**Project interpretation:** Provides context for temporal relationship features
supplied by an application. Core does not calculate network rankings. An outlier
or changed network role is a lead for investigation, not independently confirmed
fraud or laundering.

### R4. Enhancing Anti-Money Laundering Efforts with Network-Based Algorithms

**Citation:** Anthony Bonato, Juan Sebastian Chavez Palan, and Adam Szava
(2024), arXiv:2409.00823. [Author record](https://arxiv.org/abs/2409.00823).

**Reported evidence:** Uses anonymized Rabobank transactions, Louvain community
detection, and small-cycle detection to identify suspicious patterns below
reporting thresholds. [Study](https://arxiv.org/abs/2409.00823).

**Project interpretation:** Explains why relationships and chains can complement
amount rules. Core has no bundled community or cycle detector. Suspicious cycles
are not equivalent to adjudicated criminal activity.

### R5. Network Analysis of Global Banking Systems and Detection of Suspicious Transactions

**Citation:** Anthony Bonato, Juan Chavez Palan, and Adam Szava (2025),
arXiv:2503.08456. [Author record](https://arxiv.org/abs/2503.08456).

**Reported evidence:** Separately examines international banking networks using
BIS data and suspicious transaction patterns in anonymized Rabobank data using
network partitioning and cycle detection. [Study](https://arxiv.org/abs/2503.08456).

**Project interpretation:** Further context for relationship-based extensions.
The authors and transaction source overlap with R4; do not count the two records
as independent replications. Macro-level banking exposure is also different
from identifying fraud in an individual payment.

### R6. Detection of Anomalies in Large Scale Accounting Data using Deep Autoencoder Networks

**Citation:** Marco Schreyer et al. (2017; revised 2018),
arXiv:1709.05254v2. [Author record](https://arxiv.org/abs/1709.05254v2).

**Reported evidence:** Evaluates autoencoder-based anomaly assessment on two
real-world journal-entry datasets and reports comparisons with baselines and
feedback from accounting specialists. [Study](https://arxiv.org/abs/1709.05254v2).

**Project interpretation:** Illustrates research into signals beyond handcrafted
rules. This is accounting-anomaly research, not a `pacs.008` payment-fraud trial.
Core includes extension interfaces, but no trained autoencoder or demonstrated
transfer of this result to payment screening.

### R7. A synthetic data set to benchmark anti-money laundering methods

**Citation:** Rasmus Ingemann Tuffveson Jensen et al. (2023),
*Scientific Data* 10, article 661; DOI:10.1038/s41597-023-02569-2.
[Publisher article](https://www.nature.com/articles/s41597-023-02569-2).

**Reported contribution:** Introduces SynthAML as a synthetic benchmark resource
for statistical and machine-learning AML methods, addressing the difficulty of
sharing confidential banking data. It discusses evaluation issues including
imbalance, drift, and interpretability.
[Article](https://www.nature.com/articles/s41597-023-02569-2).

**Project interpretation:** Supports reproducible evaluation using documented
data generation and labels. Core's synthetic examples are integration fixtures,
not SynthAML or an effectiveness study. Synthetic benchmark success still needs
validation against the deployment population.

### R8. An Autonomous AI Security Agent for Banking: Multi-Vector Fraud and AML Detection Across Retail and Corporate Accounts

**Citation:** Joseph Walusimbi and Joshua Benjamin Ssentongo (2026),
arXiv:2606.17555v4, revised 8 October 2026.
[Author record](https://arxiv.org/abs/2606.17555v4).

**Reported evidence:** Combines sequence, statistical velocity, and graph
components across transaction and session streams. The authors report results
on synthetic transactions and sessions, including comparisons with component
baselines. [Study](https://arxiv.org/abs/2606.17555v4).

**Project interpretation:** An emerging architecture reference for combining
application-provided signals. Synthetic experiments and a preprint do not prove
deployment effectiveness. Core does not include the proposed autonomous agent,
session models, chatbot, or fusion implementation.

## How to cite these documents

For a published finding, cite the original study above. For Core behavior,
cite the corresponding source or [implementation map](research.md). Keep the
study version, dataset scope, and evidence type attached to any performance
claim. No cited publication establishes Byakugan's own precision, recall,
latency, or compliance status.
