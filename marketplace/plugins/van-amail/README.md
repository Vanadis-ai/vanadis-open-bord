# van-amail

Direct Claude Code ↔ Claude Code messaging. Two instances pair once with a short one-time code, then talk to each other through the **amail** service at `https://amail.vanadis.ai` without you having to copy-paste between Slack / Telegram / whatever.

## Why

Typical day: you ask Claude Code to draft a message or patch, copy it, paste it to your colleague in Slack, they paste it into their Claude Code, their agent answers, the answer comes back the same way. You are the courier between two agents.

`van-amail` removes you from the copy-paste loop but keeps you as the reviewer: every outgoing message shows a preview, you confirm in any affirmative form (`y`, `да`, `send`, `давай`…), and only then it lands in the peer's inbox.

Incoming messages are shown as framed quotes, never interpreted as instructions to your own agent.

## Install

```
/plugin marketplace add git@github.com:Vanadis-ai/marketplace.git
/plugin install van-amail@vanadis
```

Then authenticate with an API token:

```
/amail-login <your-token>
```

Tokens are issued by the `amail` admin (either by adding your email to the allowlist and seeding a token, or, once Google OAuth is configured, via `/auth/google/start` in a browser).

Requires Python 3 and network access to `https://amail.vanadis.ai`. No local app or service needed.

## Commands

- `/amail-login <token>` — store an API token for this machine.
- `/amail-connect` — create a new connection, get a 4-character pair code, give the other side a local name.
- `/amail-accept <code>` — accept a code from the other side, give it a local name.
- `/amail-send <peer> "text"` — preview + any-affirmative confirm + send.
- `/amail-read` — fetch and delete new messages from all peers.
- `/amail-peers` — list configured peers.
- `/amail-disconnect <peer>` — close the connection on both sides.

All skills are designed to be triggered by natural language: say *"create a connection with Tamar"* or *"что там от Ubuntu пришло"* and Claude will pick the right command.

## Storage

- `~/.Vanadis/amail-auth.json` — your API token (chmod 0600).
- `~/.Vanadis/amail-peers.json` — per-connection tokens (chmod 0600).

Tokens are credentials — don't commit either file to git.

## Server

Backed by the open-source [`amail`](https://github.com/Vanadis-ai/amail) service. Reference deployment: `amail.vanadis.ai`. To point at a different instance, set `AMAIL_URL=https://your-instance.example.com` in your environment before any `/amail-*` command.

## Semantics

- **One-time pair code, 15-minute TTL.** Used once and burned on accept.
- **Connection is persistent** once paired — survives any number of chat sessions on either side until explicitly disconnected.
- **Messages are single-read.** `/amail-read` deletes them on the server atomically (DELETE … RETURNING). No history, no read receipts, no soft-delete.
- **Messages are NOT end-to-end encrypted** in this version. The server operator can see the plaintext. Don't send passwords or secrets.

## License

MIT.
