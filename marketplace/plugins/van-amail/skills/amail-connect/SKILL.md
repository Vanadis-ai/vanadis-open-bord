---
name: amail-connect
description: Create amail connection — issue 4-char pair code for the other side. Trigger phrases — "создай соединение с X", "дай код для Тамар", "соедини меня с Ubuntu", "pair with another machine", "/amail-connect", "/van-amail:amail-connect".
user_invocable: true
allowed-tools:
  - Bash
  - Read
arguments:
  - name: peer_name
    description: "Local label for the other side (e.g. 'tamar', 'ubuntu-vanadis', 'windows'). If omitted, asked after the code is issued."
    required: false
---

# amail-connect

Create a new connection and walk the user through pairing.

## Instructions

1. Run:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py create --json
   ```
   The script prints a JSON object with `connection_id`, `pair_code`, `token`, `expires_in_sec`. If it prints "no API token" — tell the user to run `/amail-login <token>` first.

2. Show the user the pair code prominently + the instruction for the other side:

   ```
   Pair code: XXXX  (valid 15 minutes)

   On the other Claude Code instance, ask it to run:
       /amail-accept XXXX
   then come back here.
   ```

3. Ask for a peer name: **"What name do you want to give the other side locally?"** (e.g. `tamar`, `ubuntu-vanadis`). If `peer_name` argument was passed, use it and confirm in one sentence.

4. Poll `GET /amail/connection/status` with the connection token (from step 1) every 3 seconds, up to 15 minutes:
   ```
   curl -s -H "Authorization: Bearer <TOKEN>" https://amail.vanadis.ai/amail/connection/status
   ```
   Stop early if the user says "cancel" or similar.

5. When `paired: true` appears, store the peer:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py set-peer-name \
     --name <peer_name> --connection-id <connection_id> --token <token> --side a
   ```

6. Confirm:
   ```
   Connected to <peer_name>. You can now send with: /amail-send <peer_name> "text"
   ```

## Rules

- Never print the `token` value to the user — only the `pair_code`. The token is a credential.
- If 15 minutes pass without pairing, say the code expired and offer to restart. Do not auto-restart.
