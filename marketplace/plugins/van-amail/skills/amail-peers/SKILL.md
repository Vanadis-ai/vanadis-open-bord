---
name: amail-peers
description: List all configured amail peers. Trigger phrases — "who am I connected to", "покажи peer-ов", "list amail connections", "кто в адресной книге", "/amail-peers", "/van-amail:amail-peers".
user_invocable: true
allowed-tools:
  - Bash
---

# amail-peers

Show the list of configured amail peers.

## Instructions

1. Run:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py peers
   ```

2. Display the output. If the address book is empty, tell the user they can create connections via `/amail-connect` or accept one with `/amail-accept <code>`.

## Rules

- The list shows peer name, side (a/b), and connection_id. **Never** print tokens — they are credentials. The script never prints them anyway, but if you ever read `~/.Vanadis/amail-peers.json` directly, keep the `token` field out of your output.
