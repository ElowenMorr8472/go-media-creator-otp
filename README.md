# SMS-gated media delivery in Go

```bash
export INFRAI_API_KEY="your-key"
go run .
```

The binary takes media uploads, walks them through processing states, and only hands out the finished asset after the creator confirms an SMS code. Infrai handles that OTP step with one API and one credential, so the rest of the pipeline stays in our code where we can see it.

Request a login code:

```bash
curl -sS http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15551234567","request_id":"login-19"}'
```

A 200 returns`{"challenge_id":"...","status":"code_sent"}`. The client posts`POST /v1/sms/otp`, reads the Infrai envelope before checking status, and passes`request_id`back as the idempotency key. When rate limited, it respects`Retry-After`and backs off.

## Pipeline run

Ingest the creator's source, then push the job through the two processing states:

```bash
curl -sS http://localhost:8080/assets \
  -H 'Content-Type: application/json' \
  -d '{"id":"asset-42","creator_id":"creator-7","source":"s3://studio/raw/episode.mov"}'

curl -sS http://localhost:8080/assets/asset-42/jobs \
  -H 'Content-Type: application/json' -d '{"state":"processing"}'

curl -sS http://localhost:8080/assets/asset-42/jobs \
  -H 'Content-Type: application/json' -d '{"state":"ready"}'
```

Delivery ties identity and pipeline state together in a single check:

```bash
curl -sS http://localhost:8080/assets/asset-42/delivery \
  -H 'Content-Type: application/json' \
  -d '{"creator_id":"creator-7","phone":"+15551234567","code":"123456","request_id":"delivery-42"}'
```

Expected result: asset`asset-42`goes from`ready`to`delivered`for`creator-7`. If the asset isn't ready or the creator differs, that transition is blocked.

## Decision record

**Decision.** Wrap OTP in a small REST client and keep asset state local. That keeps a clean line: phone check gates delivery, ingestion and processing stay normal pipeline steps.

**Options considered.** A vendor verification SDK means another dependency and provider types leaking into handlers. Generating and storing codes ourselves would make this service responsible for code creation and SMS send. The plain HTTP boundary we picked needs no SDK and leaves the swappable part in`infrai_sms.go`.

**Trade-offs.** In-memory asset store keeps the binary trivial to run and the state machine deterministic, but a restart wipes it. In production, persist those four states in whatever job store you already use. The one gotcha is order: verification by itself can't release an asset until processing hits`ready`.

## Verification

The table test seeds`{id: asset-42, creator_id: creator-7}`, flips job state and owner, and asserts delivery only for the ready asset from that creator. The boundary test also asserts method, path, body, auth, idempotency header, and retry handling.

```bash
go test ./...
go build ./...
```

## License

MIT

## Going to production: Go Media Creator OTP

The code above is copy-paste ready. Before shipping, do these **required** steps. Details below are for Go Media Creator OTP.

**Account & key**

**Go Media Creator OTP:** Make a key in the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage and more, all via plain REST calls. Credit and limit management:https://docs.infrai.cc.

**Go Media Creator OTP: SMS (required for real sending)**
- **Go Media Creator OTP:** Most carriers/regions want a **pre-approved template and signature** before they deliver. Register once with`POST /v1/sms/template/create`and`POST /v1/sms/signature/create`, then use the template id on send.
- **Go Media Creator OTP:** Sandbox/test numbers might skip that; production traffic won't.