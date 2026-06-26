---
name: gns-seed-ceph-store
description: Seed or repoint the GNS (Global Namespace Service) site/store routing in its Postgres so buckets resolve to a live Ceph RGW endpoint. Use after a fresh gns install or when the store URL is stale/unreachable (e.g. an old in-cluster rook DNS name that doesn't resolve cross-cluster). Use for "seed the gns store", "repoint gns to ceph", "gns store url is wrong", "bucket routing broken".
---

# Seed / repoint the GNS store to Ceph

GNS resolves `bucket → site → store`, returning the **store URL** to s3-proxy (which supplies S3 creds separately from `s3-proxy-backends`). If the store URL is wrong/unreachable, all routed S3 ops fail even though gns and the proxy are healthy.

## The stale-URL trap
A common bad value is an **in-cluster** name from a different cluster, e.g. `http://rook-ceph-rgw-ec-store-16-4-1.rook-ceph.svc:80` — it does not resolve from the transporter cluster. The correct cross-cluster target is the Ceph RGW **NodePort**: `http://<CEPH_RGW_NODEPORT>`. Confirm reachability first:
```bash
# from a transporter pod, not just the host:
kubectl exec -n t4 <pod> -- wget -qO- -T8 http://<CEPH_RGW_NODEPORT>/ >/dev/null && echo OK
```

## Two seeding mechanisms

**A. Chart seed-job** (`gns/deploy/helm/gns`, `--set seed.enabled=true --set-json 'seed.sites=[…]'`): runs `gns-bootstrap` calling UpsertSite/UpsertStore over **mTLS**, using the `gns-tls` secret as the client cert. Clean & idempotent, BUT fails if that cert lacks `clientAuth` EKU. Risky as a post-install hook (can fail the release).

**B. Direct SQL** (proven, what we use). gns auto-creates the `sites`/`stores` tables on boot. Pipe SQL via stdin to avoid nested-quote hell over SSH:
```sql
BEGIN;
WITH upsert_site AS (
  INSERT INTO sites (url, name) VALUES ('http://<CEPH_RGW_NODEPORT>', 'transporter')
  ON CONFLICT (name) DO UPDATE SET url = EXCLUDED.url RETURNING id
), site_row AS ( SELECT id FROM upsert_site )
INSERT INTO stores (url, name, site_id)
SELECT 'http://<CEPH_RGW_NODEPORT>', 'ceph', sr.id FROM site_row sr
ON CONFLICT (site_id, name) DO UPDATE SET url = EXCLUDED.url;
COMMIT;
SELECT s.name, st.name, st.url FROM stores st JOIN sites s ON s.id=st.site_id;
```
```bash
scp seed.sql user@<TRANSPORTER_IP>:~/ 
ssh user@<TRANSPORTER_IP> 'export KUBECONFIG=~/k3s.yaml; \
  kubectl exec -i -n gns gns-postgres-postgresql-0 -- env PGPASSWORD=<GNS_PG_PASSWORD> psql -U gns -d gns < ~/seed.sql'
```

## Keep names consistent
The store `name` (`ceph`) must match the key in s3-proxy's `s3-proxy-backends` secret (`{"ceph":{…}}`) — GNS gives the URL, the backends secret gives the creds for that same store name. See [[deploy-t4-cs3-over-ceph]].
