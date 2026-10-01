# Development cache cleanup

Date: 2026-10-01 (Asia/Shanghai). Task: `cleanup-cache-20261001`.

## Scope and retention

Remove obsolete build caches and historical local executables while retaining the resources needed for continued project development. No application source, dependency manifest, lockfile, database volume, uploaded image volume, environment configuration or backup is removed. Existing harness history and downloaded verification evidence are retained.

All seven listed Docker images are required by existing containers, Compose services or integration tests, including `testcontainers/ryuk:0.13.0`. No image or container removal command was run. The old application image still referenced by worker, relay, insights and migration containers is also retained. The development database container was already stopped at inspection and remains stopped.

## Cleanup evidence

- `docker builder prune --all --force` removed unused BuildKit records; Docker reported `Total: 9.069GB`. A subsequent `docker system df` showed zero build cache.
- Removed 33 checked, untracked targets totaling 6,475,842,193 bytes (approximately 6.48 GB): repository-local `.cache/go-build`, `.cache/go-mod`, `.cache/gomod`, obsolete `.cache/web-dist` and `.cache/web-prices-v17`, Vite transient caches, and 26 historical API/worker executables.
- Each local target resolved inside `D:\FoodFlow`, contained no reparse points or tracked files, and was checked before deletion. Process inspection found no executable running from the repository.
- Retained the current global Go caches/module downloads, `web/node_modules`, current frontend output, latest API/login-fix/worker/migration executables and unversioned startup executables. Database SQL/dump backups and uploaded-image archives remain present.
- Local audit files: `.cache/harness/cleanup-manifest-20261001.json`, `.cache/harness/cleanup-result-20261001.json`, `.cache/harness/cleanup-volumes-before.txt`, and `.cache/harness/cleanup-docker-result-20261001.json`. All 33 targets were removed; no original Docker volume is missing.

## Verification and recovery

- Task start recorded with `go run ./cmd/harness -mode task -task cleanup-cache-20261001 -action start -kind maintenance -baseline unknown`.
- `go run ./cmd/harness -mode full -out .cache/harness/cleanup-full-20261001.json` passed all eight checks, including required Go/PostgreSQL integration, frontend tests and production build. Go tests took 69.893 seconds.
- During the post-cleanup verification snapshot the existing acceptance containers were stopped; the cause was not established. Original containers and volumes remained present. The existing-image acceptance command restored the stack without removing its volumes.
- `go run ./cmd/harness -mode acceptance -skip-build -out .cache/harness/cleanup-acceptance-20261001.json` passed architecture, knowledge, read-recovery regression, Redis/Kafka platform scenarios, kitchen flow, gateway replacement, desktop/mobile browser tests, runtime diagnostics and source stability.
- Verification uses deterministic model fixtures and isolated acceptance accounts; no paid model calls were authorized or made. Acceptance tests may append fictional fixture data to the acceptance database.

## Limits

The approximately 15.55 GB total combines Docker's reported cache cleanup with local file sizes. It is not a measurement of Windows host free-space increase: Docker Desktop's virtual disk may retain its allocated size. No virtual-disk compaction, global dependency-cache purge or development-data deletion was attempted. Subsequent Docker builds will recreate necessary build cache and can take longer. Application contracts and generated source inventory are unchanged.
