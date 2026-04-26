---
name: amail-login
description: Store amail API token issued by the admin (enables all other amail commands). Trigger phrases — "here's my amail token", "вот токен для почты", "login to amail", "/amail-login", "/van-amail:amail-login".
user_invocable: true
allowed-tools:
  - Bash
arguments:
  - name: token
    description: "Long hex API token issued by the amail admin (64-char SHA256-hex length)."
    required: true
---

# amail-login

Store an API token and confirm it works.

## Instructions

1. Run:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py login <token>
   ```

2. Verify the token is valid by running:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py whoami
   ```
   Display the email as confirmation:
   ```
   Logged in as <email>. You can now /amail-connect, /amail-accept, /amail-send, /amail-read.
   ```

3. If whoami fails with 401 — the token is wrong or revoked. Say so plainly and ask the user to double-check they copied the full token (64 hex characters).

## Rules

- Never echo the token value back to the user in clear text once stored. The `login` command itself received it as an argument — that's fine — but don't print it on later screens.
- The token lives at `~/.Vanadis/amail-auth.json` with 0600 permissions.
