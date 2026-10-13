# Byakugan Full

## Want a ready-made solution?

If you want a ready-made solution for running fraud and AML operations, we offer
**Byakugan Full**, the commercial version built around Core. It brings the
evaluation framework together with an administration dashboard, persistent
storage, investigation tools, and integration workflows.

Byakugan Full includes:

- **Web administration:** manage accounts, rules, configuration, users,
  API keys, roles, and review approvals through a dedicated dashboard.
- **Durable data and processing:** PostgreSQL persistence, Redis caching,
  source-scoped idempotency, audit trails, exports, and retention controls.
- **Fraud investigations:** cases, analyst notes, evidence attachments,
  prioritization, saved views, metrics, and manual-review decisions.
- **Advanced fraud signals:** behavior profiles, network and link analysis,
  watchlists, and governed ingestion of external model scores.
- **AML/CFT workflows:** a separate transaction ledger, versioned scenarios and
  policies, screening evidence, monitoring jobs, alerts, investigations,
  reports with separate reviewers and approvers, legal holds, and historical imports.
- **Integration tools:** signed webhooks, delivery retries and replay, delivery
  history, dead-letter reporting, and ISO 20022 `pacs.002` payment-status correlation.
- **Deployment and licensing:** API and dashboard deployment assets, database
  migrations, readiness checks, and offline commercial licensing.

## Core or Full?

| Capability | Byakugan Core | Byakugan Full |
| --- | --- | --- |
| License and source | Apache-2.0, public | Proprietary, private distribution |
| Primary use | Embeddable fraud/AML framework and lightweight fraud HTTP service | Integrated fraud operations and AML/CFT platform |
| JSON and ISO 20022 `pacs.008` intake | Included | Private adapters with exact Core projections |
| Explainable rule DSL and synchronous decisions | Included | Shared Core evaluator with managed rule workflows |
| Velocity, beneficiary, route, remittance, and risk-tier rules | In-memory history or caller-provided metrics | Durable aggregate queries backed by PostgreSQL |
| Idempotency | Bounded in-memory replay | Durable source-scoped idempotency |
| Storage | Bring your own, or use bounded memory | PostgreSQL stores and migrations with in-memory development fallback |
| Cache and multi-replica readiness | Bring your own | Redis integration and operational readiness checks |
| Account and configuration management | Library policy inputs | Managed accounts, scoped rules, runtime configuration, and imports |
| Operator interface | Not included | Independently deployable W2UI administration dashboard |
| Fraud investigations | Decision reasons and rule hits | Cases, priorities, notes, evidence attachments, approvals, and saved views |
| Advanced fraud signals | Portable deterministic signals | Behavior profiles, network analysis, watchlists, and external/ML scores |
| Payment-status correlation | Not included | ISO 20022 `pacs.002` intake, correlation, and audit trail |
| Integrations | Compiled custom evaluator interfaces | Signed webhooks, retries, replay, delivery history, and dead-letter exports |
| Security and governance | Optional static API keys | Managed API keys, admin sessions, roles, scopes, audit events, and retention controls |
| AML/CFT | Separate pure scenarios, custom observations and exact AML contracts | Append-only ledger, ingestion, scenarios, screening, alerts, investigations, reports, legal holds, and historical imports |
| Deployment | Go binary or embedded package | API, dashboard, PostgreSQL, Redis, migrations, licensing, and deployment assets |

Choose **Core** when you want a transparent engine you can embed and extend.
Choose **Full** when you also need durable operations, analysts and casework,
governed administration, AML/CFT workflows, and production integrations.

## Contact

For a demonstration, Full edition access, integration planning, or commercial
inquiries, contact **[lynolibarra@gmail.com](mailto:lynolibarra@gmail.com)**.
