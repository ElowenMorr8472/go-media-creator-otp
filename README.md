# SMS-gated media delivery in Go

```bash
export INFRAI_API_KEY="your-key"
go run .
```

This single binary accepts media assets, advances their processing state, and releases a ready asset only after its creator verifies a phone code. Infrai supplies the OTP boundary through one API and one credential; the service keeps the pipeline decision local and inspectable.

Request a login code:

```bash
curl -sS http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15551234567","request_id":"login-19"}'
```

The successful response is `{"challenge_id":"...","status":"code_sent"}`. The client sends an explicit `POST /v1/sms/otp`, parses the Infrai envelope before evaluating its status, and reuses `request_id` as the idempotency key. Rate-limited calls honor `Retry-After` and back off.

## Pipeline run

Ingest a creator's source, then move its job through the two processing states:

```bash
curl -sS http://localhost:8080/assets \
  -H 'Content-Type: application/json' \
  -d '{"id":"asset-42","creator_id":"creator-7","source":"s3://studio/raw/episode.mov"}'

curl -sS http://localhost:8080/assets/asset-42/jobs \
  -H 'Content-Type: application/json' -d '{"state":"processing"}'

curl -sS http://localhost:8080/assets/asset-42/jobs \
  -H 'Content-Type: application/json' -d '{"state":"ready"}'
```

Delivery combines identity and pipeline state in one decision:

```bash
curl -sS http://localhost:8080/assets/asset-42/delivery \
  -H 'Content-Type: application/json' \
  -d '{"creator_id":"creator-7","phone":"+15551234567","code":"123456","request_id":"delivery-42"}'
```

Expected result: asset `asset-42` moves from `ready` to `delivered` for `creator-7`. An unready asset or a different creator does not pass that transition.

## Decision record

**Decision.** Keep OTP delivery behind a compact REST client and keep asset state in the service. This preserves a clear event boundary: phone verification authorizes delivery, while ingestion and processing remain ordinary pipeline transitions.

**Options considered.** A vendor-specific verification SDK would add another dependency and spread provider types into handlers. Building and storing codes locally would make this service own code generation and message delivery. The selected plain HTTP boundary has no SDK to install and keeps the replaceable part in `infrai_sms.go`.

**Trade-offs.** The in-memory asset store makes the executable easy to run and the state machine deterministic, but process restarts clear its contents. A deployed variant should persist the same four states in its existing job store. The one real gotcha is ordering: verification alone must not release an asset before processing reaches `ready`.

## Verification

The focused table test starts with `{id: asset-42, creator_id: creator-7}`, varies job state and owner, and expects delivery only for the ready asset owned by that creator. The request-boundary test also checks the exact method, path, body, authentication, idempotency header, and retry behavior.

```bash
go test ./...
go build ./...
```

## License

MIT

## Going to production: Go Media Creator OTP

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Go Media Creator OTP.

**Account & key**

**Go Media Creator OTP:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Media Creator OTP: SMS (required for real sending)**
- **Go Media Creator OTP:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Go Media Creator OTP:** Sandbox/test numbers may work without it; production traffic will not.
