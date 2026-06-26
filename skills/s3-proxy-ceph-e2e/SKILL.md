---
name: s3-proxy-ceph-e2e
description: Run an end-to-end S3 round-trip (create-bucket, put, get, list) through the Together s3-proxy LoadBalancer to verify the full T4 path — UMS auth → GNS routing → Ceph RGW — actually serves data. Includes how to diagnose auth failures from the proxy logs (keyExpired, AccessDenied) and how to confirm the object landed on Ceph. Use for "verify s3-proxy end to end", "test the t4 data path", "confirm data lands in ceph", "is the proxy serving".
---

# End-to-end S3 test through s3-proxy → Ceph

Endpoint: `https://<TRANSPORTER_IP>:9001` (s3-proxy LoadBalancer, self-signed → `--no-verify-ssl`). Needs a **valid SigV4 UMS key** (`key_…`/`tgs_v1_…`) — mint one with [[create-ums-sigv4-key]].

```bash
export AWS_REGION=us-west-2 AWS_ACCESS_KEY_ID=key_… AWS_SECRET_ACCESS_KEY=tgs_v1_…
AWS=(aws --region us-west-2 --endpoint-url https://<TRANSPORTER_IP>:9001 --no-verify-ssl)
B="t4ceph-e2e-$(date +%s)"; echo hello > /tmp/e2e.txt
"${AWS[@]}" s3api create-bucket --bucket "$B"
"${AWS[@]}" s3api put-object --bucket "$B" --key hello.txt --body /tmp/e2e.txt   # → ETag
"${AWS[@]}" s3api get-object  --bucket "$B" --key hello.txt /tmp/out.txt          # content matches
"${AWS[@]}" s3api list-objects-v2 --bucket "$B"
```
Success = ETag on put, matching content on get, key listed. That proves auth + GNS routing + Ceph write/read all work.

## Diagnosing failures (read the proxy log)
```bash
ssh <USER>@<TRANSPORTER_IP> 'export KUBECONFIG=~/k3s.yaml; kubectl logs -n s3-proxy deploy/s3-proxy --since=60s | grep -iE "authenticate|denied|gns|store" | grep -v chalk'
```
- **`AccessDenied` + `keyExpired: true`** → the UMS key's `ExpiresAt` is in the past. Mint a fresh key. (Note: `ListAllMyBuckets` may be denied even for a valid key — test bucket-scoped ops instead.)
- **`AccessDenied` only on data ops** → the `s3-proxy-backends` ceph creds are wrong/stale for the current RGW. Refresh from cs3's `rgw-creds`.
- **5xx / store errors** → GNS store URL unreachable; re-check [[gns-seed-ceph-store]].

## Confirm on the Ceph side (optional, definitive)
```bash
ssh <CEPH_USER>@<CEPH_IP> 'sudo kubectl -n rook-ceph exec deploy/rook-ceph-tools -- radosgw-admin bucket list' | grep t4ceph-e2e
```

See [[deploy-t4-cs3-over-ceph]] for the topology this exercises.
