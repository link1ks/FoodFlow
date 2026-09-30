# Harness / Redis / Kafka / microservice verification

Date: 2026-09-30. Local Docker Desktop 29.8.0, PostgreSQL 16, Redis 7.4, Kafka 3.9.1 (KRaft), Go 1.26, Windows amd64.

## Commands executed

```powershell
go run ./cmd/harness -mode full -out .cache/harness/platform-full.json
pwsh -File scripts/platform-acceptance.ps1
pwsh -File tests/smoke.ps1 -Base http://127.0.0.1:18080
sqlc generate
```

Full harness passed architecture checks, Go format, vet, required PostgreSQL integration, 8 frontend tests and production build. Structured summaries are in `validation/platform-harness.json` and `validation/platform-evidence.json`; they describe a tested local working tree based on the recorded revision, not a GitHub CI run for an uncommitted revision.

## Business and failure evidence

| Scenario | Observed result |
| --- | --- |
| Inventory transaction rollback | Rolled-back ledger insert created no extra outbox event |
| Broker publish failure | Publication marker stayed empty, retry count increased, later dispatch succeeded |
| Repeated ID | One projection fact after repeated delivery |
| Equivalent JSON encoding | Same event with canonical JSON deduplicated |
| ID reused with changed quantity | Rejected as conflicting content |
| Real Redis hit | Second authenticated catalog query returned cache hit |
| Redis stop | Catalog still returned 200 from PostgreSQL |
| Unauthenticated cache query | Returned 401 before cache access |
| Kafka stop | 10g consumption committed, authoritative 100g stock became 90g |
| Kafka recovery | Projection converged to 10g consumed |
| Consumer stop and restart | 5g waste during downtime was included after restart |
| Invalid version delivered through Kafka | Stored digest and Kafka coordinates in quarantine |
| Cross-household summary | Returned 404 |
| Existing full kitchen workflow | Registration, invite, planning, confirmation, procurement and consumption passed |
| UI | Browser showed 100g bought, 10g consumed, 5g wasted with last sync time |

Redis and Kafka were real containers, not replacement mocks. Projection has its own PostgreSQL container and database; API reaches it through an authenticated internal HTTP request. Tests use disposable accounts in the isolated acceptance stack.

## Limits

Single-node broker configuration is for local acceptance. No production HA, TLS/SASL, long-duration throughput, network partition/rebalance stress or external model accuracy result is implied. Failure tests cover stopped broker/cache/consumer and direct transactional failure; they do not assert exactly-once external delivery. The new CI job is configured but has not been observed on GitHub for these working-tree changes.

The extracted service reports snapshot-backed stock quantities. Nutrition statistics remain in the kitchen application; cost loss, financial utilization rates and long-image monthly export remain separate work. Before production use, add authenticated/encrypted transport configuration, replication, operational alerts and a tested independent projection backup/rebuild procedure.
