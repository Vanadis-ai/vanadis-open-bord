# amail — agent mail

Pre-authorized cross-machine mailbox. Two clients pair with a short one-time code, then exchange single-read messages until one side disconnects. Authentication is Google OAuth + an allowlist: only pre-approved emails can open or accept connections.

Built as a small self-hostable Go service (single binary, Postgres, optional Google OAuth). The reference deployment is `amail.vanadis.ai`; the client plugin is `van-amail` in the Vanadis marketplace.

## Why

Typical day: Claude Code on your laptop generates a patch, you copy it to Slack, your colleague pastes it into *their* Claude Code, their agent responds, and the cycle repeats. You are the courier between two agents.

`amail` is the mailbox that lets the agents talk directly. You stay in the loop as the reviewer (every outgoing message is previewed before it leaves), but you stop being the bytes-between-clipboards middleman.

## Architecture

- **Auth**: Google OAuth (loopback / installed-app flow) → email checked against `amail_allowlist` → long-lived API token issued.
- **Pairing**: `POST /amail/connection/create` with a valid API token mints a 4-character `pair_code` (15-minute TTL). `POST /amail/connection/accept` consumes it and returns the second side's token.
- **Messaging**: `POST /amail/send` / `GET /amail/inbox`. Inbox is `DELETE ... RETURNING` in a single SQL statement — **single-read** is a hard property.
- **Disconnect**: `DELETE /amail/connection` — cascades through both sides via the FK.

### Why not just email?

Email is rich mail for humans with spam filters, rich text, attachments. `amail` is the opposite: plain-text, per-connection tokens, delete-on-read, no history. Designed for agent↔agent hand-off, not for inbox-zero.

## Self-hosting

### Prerequisites

- Postgres 13+ (any provider — Supabase, AWS RDS, your own).
- Docker or Kubernetes.
- Optional: a Google Cloud project with an OAuth 2.0 "Desktop app" client (needed if you want Google login; skip for admin-issued tokens only).

### Run locally (Docker)

```bash
docker run -d --name amail \
  -e DATABASE_URL="postgresql://user:pass@host:5432/amail" \
  -e PUBLIC_URL="https://amail.example.com" \
  -e GOOGLE_CLIENT_ID="..." \
  -e GOOGLE_CLIENT_SECRET="..." \
  -p 8080:8080 \
  ghcr.io/vanadis-ai/amail:latest
```

### Run on Kubernetes

```bash
# Create the secret with DB URL + Google creds:
kubectl -n vanadis create secret generic amail-secrets \
  --from-literal=DATABASE_URL="postgresql://..." \
  --from-literal=GOOGLE_CLIENT_ID="..." \
  --from-literal=GOOGLE_CLIENT_SECRET="..."

# Apply deployment + service + ingress:
kubectl apply -f deploy/k8s.yaml
```

Edit `deploy/k8s.yaml` to set your own host (replace `amail.vanadis.ai`) and cluster-issuer name if you're not using cert-manager with LetsEncrypt.

### Seed the allowlist + initial tokens

```bash
DATABASE_URL="postgresql://..." go run ./cmd/seed \
  admin@example.com alice@example.com bob@example.com
```

Prints `email<tab>token` pairs. Store tokens somewhere safe and hand them out to users — they plug into `~/.Vanadis/amail-auth.json` on the client side.

Once Google OAuth is configured, users can also self-serve by running `/amail-auth` in their Claude Code; the allowlist still gates who actually gets a token.

### Admin

There is no built-in admin UI in this repo (by design — one service, one job). Hit the admin endpoints directly with an admin API token (an admin is an email listed in `isAdminEmail` in `server.go`; adjust at build time or via fork):

```bash
# list
curl -H "Authorization: Bearer $ADMIN_TOKEN" https://amail.example.com/admin/allowlist
# add
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{"email": "new@example.com"}' https://amail.example.com/admin/allowlist
# remove
curl -X DELETE -H "Authorization: Bearer $ADMIN_TOKEN" \
  https://amail.example.com/admin/allowlist/old@example.com
```

For a web admin UI, see `Vanadis Jarl` in the upstream reference deployment.

## API

All endpoints except `/auth/google/*` and `/health` require `Authorization: Bearer <token>`.

Tokens come in two flavours: **API tokens** (issued via OAuth login or seed script, bound to an email, used to create/accept connections) and **connection tokens** (issued by the pair-create / pair-accept flow, bound to a specific connection, used to send/read messages). The server figures out which kind by looking the hash up in the right table.

| Method | Path                           | Token kind   | Body                   | Response                                            |
|--------|--------------------------------|--------------|------------------------|-----------------------------------------------------|
| GET    | `/health`                      | —            | —                      | `ok`                                                |
| GET    | `/auth/google/start`           | —            | `?redirect_uri=<loop>` | 302 to Google consent screen                        |
| GET    | `/auth/google/callback`        | —            | `?code=&state=`        | 302 to loopback with `?token=&email=`               |
| POST   | `/api/tokens`                  | API          | `{label}`              | `{token}`                                           |
| DELETE | `/api/tokens/current`          | API          | —                      | `{revoked: true}`                                   |
| GET    | `/admin/allowlist`             | API (admin)  | —                      | `{emails: [...]}`                                   |
| POST   | `/admin/allowlist`             | API (admin)  | `{email}`              | `{ok: true}`                                        |
| DELETE | `/admin/allowlist/{email}`     | API (admin)  | —                      | `{ok: true}`                                        |
| POST   | `/amail/connection/create`     | API          | —                      | `{connection_id, pair_code, token, expires_in_sec}` |
| POST   | `/amail/connection/accept`     | API          | `{pair_code}`          | `{connection_id, token}`                            |
| GET    | `/amail/connection/status`     | Connection   | —                      | `{connection_id, paired, my_email, peer_email?}`    |
| DELETE | `/amail/connection`            | Connection   | —                      | `{deleted: true}`                                   |
| POST   | `/amail/send`                  | Connection   | `{text}`               | `{message_id}`                                      |
| GET    | `/amail/inbox`                 | Connection   | —                      | `{messages: [...]}`                                 |

## Schema

See `internal/amail/db.go`. Six tables, all idempotent `CREATE TABLE IF NOT EXISTS` on startup. Tokens stored as SHA256 hex (safe for opaque random 256-bit values, not appropriate for password-like secrets). Pair codes are 4 characters from a confusion-resistant alphabet, Postgres `gen_random_uuid()` for row IDs.

## Client

The reference client is the `van-amail` Claude Code plugin (see the Vanadis marketplace). Any HTTP client that can carry a Bearer token will work — the API is plain REST.

## License

MIT.
