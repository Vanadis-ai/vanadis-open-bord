---
name: amail-accept
description: Accept an amail 4-character pair code from another Claude Code (handshake completion). Trigger phrases — "here's the code K7F3", "прими соединение TJRS", "вот код XXXX", "accept pair code", "/amail-accept", "/van-amail:amail-accept".
user_invocable: true
allowed-tools:
  - Bash
arguments:
  - name: pair_code
    description: "The 4-character code produced by /amail-connect on the other side"
    required: true
  - name: peer_name
    description: "Local label for the other side (e.g. 'mac', 'ubuntu-vanadis'). If omitted, asked."
    required: false
---

# amail-accept

Accept a pair code and store the resulting connection.

## Instructions

1. Normalise `pair_code`: uppercase, strip whitespace. Don't reject on length — let the server decide.

2. Run:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py accept <pair_code> --json
   ```
   If it prints "no API token" — tell the user to run `/amail-login <token>` first.

3. Ask: **"What name do you want to give the other side locally?"** If `peer_name` was passed, use it with a one-sentence confirmation.

4. Store the peer:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py set-peer-name \
     --name <peer_name> --connection-id <connection_id> --token <token> --side b
   ```

5. Confirm:
   ```
   Accepted connection. Peer saved as <peer_name>.
   Send with /amail-send <peer_name> "text"
   Check for new messages with /amail-read
   ```

## Rules

- Never print the token.
- On "invalid or expired pair code" — say it plainly and ask the user to get a fresh code via `/amail-connect` on the other side.
