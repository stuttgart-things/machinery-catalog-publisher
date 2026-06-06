# stuttgart-things/machinery-catalog-publisher

Publishes **live Crossplane resource status** as Backstage `Resource` entities to an
S3/MinIO bucket, which a Git-side Backstage `Location` points at. It closes the loop
between [`machinery`](https://github.com/stuttgart-things/machinery) (the live-cluster
gRPC status API) and Backstage's declarative catalog.

```
crossplane cluster
   │  (dynamic informer)
   ▼
machinery  ──gRPC WatchResources──▶  machinery-catalog-publisher  ──PUT──▶  S3 / MinIO
(extracts ResourceStatus)             • ResourceStatus → Backstage Resource YAML
                                      • Cache-Control: no-cache (fresh ETag per write)
                                      • key: status/<ns>/<name>.yaml
                                              │
                  Backstage Location (in Git) ──GET──┘  (polls each processing cycle)
```

## How it works

- Opens machinery's `WatchResources` stream: the cache replays as `ADDED` (initial fill /
  placeholders), then live `ADDED`/`MODIFIED`/`DELETED` deltas stream in.
- Each status is rendered to a Backstage `Resource` named `<name>-status` (the `-status`
  suffix avoids colliding with the Git-owned `Component`), with phase / last-updated /
  info fields as `sthings.lab/*` annotations and a `dependencyOf` relation to the Component.
- A periodic **resync ticker** (`interval`) does a full `GetResources` to heal anything a
  dropped stream missed, and deletes objects whose resources have vanished.
- Health is exposed at `/metrics` (Prometheus) and `/healthz` — the deployment-mode
  stand-in for a CR status subresource.

## Layouts

- `PerResource` (default) — one object per resource at `status/<ns>/<name>.yaml`; the Git
  `Location` references each. Clean per-resource deletes.
- `Aggregate` — a single `status/all.yaml` multi-doc; one `Location`, coarser deletes.

## Configuration

Two inputs, matching the deployment model:

1. **`config.yaml`** (non-secret, mounted from a ConfigMap) — see [`examples/config.yaml`](examples/config.yaml).
2. **Connection secret** (S3/MinIO endpoint + creds + TLS trust, via `envFrom`) — see
   [`examples/connection-secret.yaml`](examples/connection-secret.yaml). Maps onto the
   `mc alias set` model: `S3_ALIAS` / `S3_ENDPOINT` / `S3_ACCESS_KEY` / `S3_SECRET_KEY`,
   plus `S3_CA_BUNDLE` **or** `S3_INSECURE_SKIP_VERIFY` (set one, not both).

| Env var | Purpose |
|---|---|
| `CONFIG_FILE` | path to config.yaml (default `/etc/publisher/config.yaml`) |
| `METRICS_ADDR` | metrics/health listen addr (default `:8080`) |
| `S3_ENDPOINT` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` | connection secret (required) |
| `S3_REGION` | SDK region (default `us-east-1`; MinIO ignores it) |
| `S3_CA_BUNDLE` / `S3_CA_BUNDLE_FILE` | custom TLS trust (PEM inline or file) |
| `S3_INSECURE_SKIP_VERIFY` | skip TLS verification (dev only) |

## Run locally

```bash
go mod tidy   # resolves deps; uses the local replace to ../machinery
go test ./...
CONFIG_FILE=examples/config.yaml \
  S3_ENDPOINT=https://minio.sthings.lab S3_ACCESS_KEY=... S3_SECRET_KEY=... \
  go run ./cmd/server
```

> The `go.mod` has `replace github.com/stuttgart-things/maschinist => ../machinery` for the
> sibling-repo layout. For image builds, tag/publish machinery or vendor its
> `resourceservice/` package and drop the replace.

## Deploy (KCL)

```bash
kcl run kcl/ \
  -D config.namespace=machinery-catalog-publisher \
  -D config.bucket=backstage-status \
  -D config.connectionSecret=minio-homerun \
  -D config.machineryAddr=machinery.machinery-system.svc:50051
```

`replicas: 0` suspends the sync. With Stakater Reloader installed (`reloaderEnabled`,
default on), editing the ConfigMap or connection Secret restarts the Pod automatically.
