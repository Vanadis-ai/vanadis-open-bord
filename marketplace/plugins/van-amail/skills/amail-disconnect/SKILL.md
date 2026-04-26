---
name: amail-disconnect
description: Close an amail connection and forget the peer (cascade-deletes pending messages). Trigger phrases — "удали connection с Тамар", "drop peer X", "отключись от Ubuntu", "disconnect from", "/amail-disconnect", "/van-amail:amail-disconnect".
user_invocable: true
allowed-tools:
  - Bash
arguments:
  - name: peer
    description: "Local name of the peer to disconnect."
    required: true
---

# amail-disconnect

Close a connection and remove the peer from the address book.

## Instructions

1. Confirm with the user once: *"This will close the connection to `<peer>` on both sides and delete any pending messages. Continue?"* — wait for any affirmative answer (`y`, `yes`, `да`, `давай`, `удаляй` — anything clearly positive).

2. On affirmative, run:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py disconnect <peer>
   ```

3. Show the result. If peer not found, show known peers and explain.

## Rules

- Irreversible. Once deleted, the other side's token also stops working. They will see 401 on their next send/read and `/amail-read` on their side will auto-prune the address-book entry.
- Do not skip the confirmation, even if the user has said "yes" to something unrelated earlier.
