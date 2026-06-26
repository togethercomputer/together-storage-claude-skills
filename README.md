# claude-skills

[Claude Code](https://claude.com/claude-code) **skills** (operational runbooks) for deploying and verifying Together **T4** (GNS control-plane + s3-proxy data-plane) and **CS3** (s3-cache-proxy) on a k3s cluster backed by **Rook-Ceph** object storage.

Each skill is a self-contained `SKILL.md` (with YAML frontmatter `name` + `description`) that Claude Code can discover and invoke. Drop a skill directory into `~/.claude/skills/` (user-global) or `<repo>/.claude/skills/` (project-scoped).

## Skills

| Skill | What it does |
|-------|--------------|
| [`deploy-t4-cs3-over-ceph`](skills/deploy-t4-cs3-over-ceph/SKILL.md) | Clean-install / reinstall GNS + s3-proxy + CS3 over a Rook-Ceph RGW (helm, secrets, Postgres wipe+reseed, ECR pull secrets, both data paths). |
| [`gns-seed-ceph-store`](skills/gns-seed-ceph-store/SKILL.md) | Seed/repoint the GNS Postgres site→store routing to a live Ceph RGW endpoint. |
| [`s3-proxy-ceph-e2e`](skills/s3-proxy-ceph-e2e/SKILL.md) | End-to-end S3 round-trip (create/put/get/list) through the s3-proxy LB; diagnose auth (`keyExpired`/`AccessDenied`) from proxy logs. |
| [`create-ums-sigv4-key`](skills/create-ums-sigv4-key/SKILL.md) | Mint a SigV4 S3 access key on UMS via its gRPC Key API (includes a runnable Go client in `client/`). |

The skills cross-reference each other via `[[skill-name]]` links.

## Placeholders — fill in for your environment

These runbooks are sanitized. Replace the placeholders with your own values (e.g. via env vars or a local, untracked notes file):

| Placeholder | Meaning |
|-------------|---------|
| `<TRANSPORTER_IP>` / `<TRANSPORTER_WORKER_IP>` | k3s control-plane / worker node IPs |
| `<TRANSPORTER_HOST>` / `<TRANSPORTER_WORKER_HOST>` | k3s node hostnames |
| `<CEPH_IP>` / `<CEPH_HOST>` | Rook-Ceph node IP / hostname |
| `<CEPH_RGW_NODEPORT>` | Ceph RGW endpoint reachable cross-cluster (e.g. `http://<CEPH_IP>:30080`) |
| `<T4_ECR_ACCOUNT>` / `<CS3_ECR_ACCOUNT>` | AWS account IDs hosting the T4 / CS3 ECR repos |
| `<UMS_ENDPOINT>` | UMS gRPC endpoint (`host:50051`) |
| `<USER>` / `<CEPH_USER>` | SSH users for the compute / storage hosts |
| `<GNS_PG_PASSWORD>` | GNS Postgres password |
| `<UMS_SERVICE_KEY>` | UMS service key (`ums_v1_…`) — never commit a real one |

> **Never commit real credentials.** The Go client reads `UMS_ENDPOINT`, `UMS_SERVICE_KEY`, and `EXISTING_KEY` from the environment.

## License

Internal operational documentation. No warranty.
