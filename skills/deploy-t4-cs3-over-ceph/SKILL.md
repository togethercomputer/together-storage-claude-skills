---
name: deploy-t4-cs3-over-ceph
description: Deploy (or clean-reinstall) Together T4 (gns control-plane + s3-proxy data-plane) and CS3 (s3-cache-proxy) on the transporter k3s cluster, backed by a Rook-Ceph RGW running on a separate cluster. Covers the two-cluster topology, the two data paths, helm-driven install with hand-created secrets (TLS from certs1, postgres, ECR pull secrets in us-west-2), wiping+reseeding gns postgres, and pointing everything at the live Ceph RGW. Use for "deploy t4 and cs3 over ceph", "reinstall gns/s3-proxy/cs3", "redeploy T4 on the transporter".
---

# Deploy T4 + CS3 over Ceph

## Topology (two separate clusters)

- **Compute/T4** — `<USER>@<TRANSPORTER_IP>` (<TRANSPORTER_HOST>, +`.74` worker), k3s. Namespaces: `gns`, `s3-proxy`, `cs3`, `t4`. `KUBECONFIG=~/k3s.yaml` (API also reachable from a laptop at `https://<TRANSPORTER_IP>:6443`). `docker` is podman; passwordless sudo. helm installed at `/usr/local/bin/helm` (v3).
- **Storage/Ceph** — `<CEPH_USER>@<CEPH_IP>` (<CEPH_HOST>), Rook-Ceph in ns `rook-ceph` (**use sudo** for kubectl). Object store `my-store`; RGW reachable cross-cluster via **NodePort `http://<CEPH_RGW_NODEPORT>`** (in-cluster name `rook-ceph-rgw-my-store.rook-ceph.svc:80` does NOT resolve from the transporter).

## Data paths
- `client → s3-proxy (UMS auth) → GNS (bucket→site→store routing) → Ceph RGW`. s3-proxy gets the **store URL from GNS** and the **S3 creds from its `s3-proxy-backends` secret** keyed by store name `ceph`.
- `cs3 (s3-cache-proxy) → Ceph RGW directly` (`-s3-endpoint=http://<CEPH_RGW_NODEPORT>`, creds in `rgw-creds`). cs3 integrates with the T4 control plane (ENG-90005).

## Images (use `:latest`, all = origin/main — confirm with [[verify-image-matches-main]])
- gns: `<T4_ECR_ACCOUNT>.dkr.ecr.us-west-2.amazonaws.com/t4-gns:latest`
- s3-proxy: `<T4_ECR_ACCOUNT>.dkr.ecr.us-west-2.amazonaws.com/t4-s3-proxy:latest`
- cs3: `<CS3_ECR_ACCOUNT>.dkr.ecr.us-west-2.amazonaws.com/s3-cache-proxy:latest`

## Charts & certs
Charts in repo: `together-t4/gns/deploy/helm/gns`, `together-t4/s3-proxy/deploy/helm/s3-proxy`, `cs3/helm`. GNS TLS material in `together-t4/certs1/` (`ca-cert.pem`, `gns-server.crt/key`, `gns-client.crt/key`, `proxy.crt/key`). Drive helm from the host (copy charts+certs over) or locally against the kubeconfig.

## Clean reinstall sequence

**0. ECR pull secrets (us-west-2)** in `gns`, `s3-proxy` (`ecr-pull`) and `cs3` (`ecr-creds`). Generate token locally, `kubectl create secret docker-registry … --dry-run=client -o yaml | kubectl apply -f -`. ECR tokens last 12h — **refresh `ecr-creds` if cs3 pods hit `403 Forbidden`/ImagePullBackOff**.

**1. GNS (full teardown + wipe):**
```
helm uninstall gns gns-postgres -n gns ; kubectl delete ns gns ; kubectl create ns gns
# recreate: ecr-pull(usw2); gns-tls (tls.crt=gns-server.crt, tls.key=gns-server.key, ca-cert.pem=ca-cert.pem);
#           gns-postgres-secret (connection-string=postgres://gns:<GNS_PG_PASSWORD>@gns-postgres-postgresql.gns.svc.cluster.local:5432/gns?sslmode=disable)
helm repo add bitnami https://charts.bitnami.com/bitnami; helm install gns-postgres bitnami/postgresql -n gns \
  --set auth.username=gns --set auth.password=<GNS_PG_PASSWORD> --set auth.database=gns --set architecture=standalone --set primary.persistence.size=10Gi
helm install gns ./gns -n gns --set image.repository=…/t4-gns --set image.tag=latest --set image.pullPolicy=Always \
  --set 'imagePullSecrets[0].name=ecr-pull' --set config.postgres.secretName=gns-postgres-secret \
  --set tls.secretName=gns-tls --set config.tlsEnabled=true
```
gns auto-migrates the schema on boot. Then **seed the ceph store** — see [[gns-seed-ceph-store]] (chart seed-job uses mTLS bootstrap; we seed via SQL to be safe).

**2. s3-proxy** (preserve the external `s3-proxy-gns-tls` mTLS client-cert secret — don't delete the ns):
- Update `ecr-pull`→usw2 and `s3-proxy-backends` to `{"ceph":{"URL":"http://<CEPH_RGW_NODEPORT>","AWSAccessId":<ceph>,"AWSSecretId":<ceph>}}` using the **working ceph creds from cs3's `rgw-creds`** (March creds were dead).
- `helm uninstall s3-proxy -n s3-proxy && helm install s3-proxy ./s3-proxy -n s3-proxy` with image `…/t4-s3-proxy:latest`, `config.gns.serverAddress=gns.gns.svc.cluster.local:9090`, `config.gns.serverName=gns.gns.svc.cluster.local`, `config.siteName=transporter`, `config.ums.endpoint=<UMS_ENDPOINT>`, `config.ums.serviceKey=<ums service key>`, `backendsSecretName=s3-proxy-backends`, `proxyTLS.secretName=s3-proxy-tls`, `service.type=LoadBalancer`.
- Healthy log lines: `Connected to GNS via mTLS`, `defaultStore: ceph`.

**3. cs3** (already ceph-wired): `kubectl set image ds/cs3 s3-cache-proxy=…/s3-cache-proxy:latest -n cs3` then `kubectl rollout status ds/cs3 -n cs3`.

**4. Verify** end-to-end → [[s3-proxy-ceph-e2e]].

## Gotchas
- SSH to `.73` is intermittently flaky — use `-o ConnectTimeout=25 -o ServerAliveInterval=5` and retry.
- zsh does NOT word-split unquoted vars → don't put ssh flags in a `$VAR`; inline them.
- `helm get values <release>` is the source of truth for re-creating config faithfully.
- RBAC: `<USER>` can create/delete ns, patch deploy/ds, create secrets; cannot read others' helm releases without helm installed.
