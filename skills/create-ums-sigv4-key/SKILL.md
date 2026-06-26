---
name: create-ums-sigv4-key
description: Mint a new SigV4 S3 access key (access-key-id + secret) on the Together User Management Service (UMS) via its gRPC Key API, so you can authenticate to the s3-proxy. Discovers the project/creator from an existing key, then calls CreateProjectKeyV1 with token_type=SIGV4 and a future expiry. Use for "create a sigv4 token on ums", "mint a new UMS key", "I need valid s3-proxy credentials", or when the s3-proxy logs show keyExpired=true.
---

# Create a SigV4 key on UMS

The s3-proxy authenticates S3 requests against **UMS** (`<UMS_ENDPOINT>`, gRPC/TLS). `AWS_ACCESS_KEY_ID` = the UMS key id (`key_…`), `AWS_SECRET_ACCESS_KEY` = the key's token (`tgs_v1_…` for SigV4). When a key's `ExpiresAt` is in the past, the proxy rejects every request with `keyExpired: true` (see [[s3-proxy-ceph-e2e]]).

## API (module `github.com/togethercomputer/user-management-service/clients/go/gen/ums`)
- Auth: gRPC metadata `authorization: Bearer <UMS_SERVICE_KEY>` (the same service key s3-proxy uses, `ums_v1_…`).
- `GetV1{KeyId}` → returns `KeyV1` incl. `ProjectId`, `CreatorId`, `Scopes`, `TokenType`. Use it to discover the project/creator of an existing key.
- `CreateProjectKeyV1{ProjectId, CreatorId, Description, ExpiresAt(*timestamppb), Scopes(*KeyScopesV1{Tcloud:true}), TokenType(*KeyTokenType = KEY_TOKEN_TYPE_SIGV4)}` → `Reply{Id, Token, ExpiresAt, TokenSuffix, …}`. `Id` is the new `key_…`; `Token` is the secret.

## Run the bundled client
`client/` here has a self-contained `main.go` + `go.mod` (Go 1.26). It reads an existing key to clone its project/creator, then creates a 1-year SigV4 key:
```bash
cd client
GOFLAGS=-mod=mod GOWORK=off \
UMS_SERVICE_KEY=<ums_v1_…> EXISTING_KEY=<key_…> go run .
# prints AWS_ACCESS_KEY_ID=key_…  and  AWS_SECRET_ACCESS_KEY=tgs_v1_…
```

## Gotchas
- `KeyV1.Scopes` (read type `*KeyV1_Scopes`) ≠ the create request's `*KeyScopesV1` — don't copy it across; construct a fresh `&pb.KeyScopesV1{Tcloud:true}` (tcloud scope is what the substrate s3-proxy needs).
- `TokenType` is a `oneof` pointer field — pass `&tt`.
- The service key must be authorized to create (the s3-proxy read key worked for create in our case). On `PermissionDenied`, you need a higher-privilege UMS key.
- This writes to **production UMS** — set a sane `Description` and expiry; it's a real credential.
- Reachable from a laptop (`<UMS_ENDPOINT>` → AWS ELB) and from the transporter.
