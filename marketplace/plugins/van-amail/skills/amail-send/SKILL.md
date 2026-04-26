---
name: amail-send
description: Send a message to a paired Claude Code peer with preview + confirmation gate. Trigger phrases — "напиши Тамар ...", "передай на ubuntu", "send a message to X", "tell the other machine", "/amail-send", "/van-amail:amail-send".
user_invocable: true
allowed-tools:
  - Bash
  - Read
arguments:
  - name: peer
    description: "Local name of the peer (from the address book). List via /amail-peers if unsure."
    required: true
  - name: text
    description: "Message body. Can be multi-line. The assistant may distill this from the current chat context on the user's explicit request."
    required: true
---

# amail-send

Send a single message to an amail peer. Always preview, always wait for an affirmative confirmation.

## Instructions

1. **Compose the draft.** If the user asked to "prepare a message from this conversation" or similar, distill the relevant bit of the current chat into `text`. Do NOT auto-include raw chat dumps, file contents, environment variables, or anything the user did not explicitly ask to include. The draft should read like a message a human would compose — short, clear, aimed at the recipient.

2. **Show the preview.** Format it as:
   ```
   → Draft to <peer>:

     <text, indented by 2 spaces>

   Send? ([y]es / [e]dit / [n]o — or просто "да" / "нет" / "правь")
   ```
   Do not abbreviate the body. Do not auto-answer; wait for the user.

3. **Interpret the response loosely.** Any of the following (or their close variants in Russian/English) counts as affirmative: `y`, `yes`, `ok`, `send`, `да`, `давай`, `отправь`, `пиши`, `шли`, `go`. Any of these counts as cancel: `n`, `no`, `stop`, `нет`, `отмена`, `стоп`, `не надо`. Anything that asks to change the text (`e`, `edit`, `правь`, `переделай`, "`change X to Y`") means edit.

   - Affirmative → step 4.
   - Edit → ask for the revision (or apply it inline if the user directly proposed a change), then loop back to step 2 with the new text.
   - Cancel → say "cancelled, nothing sent" and STOP.
   - Anything else (unclear) → re-show the preview one more time with a gentler prompt; still do not send without affirmative.

4. **Send.** Run exactly once:
   ```
   python3 ${CLAUDE_PLUGIN_ROOT}/bin/amail.py send <peer> "<text>" --json
   ```
   Pass the text as one argument. If the text contains dangerous shell characters, write it to a temp file and read via `$(cat /tmp/draft.txt)` — the bytes on the wire must still equal the bytes in the preview.

5. **Confirm.** Show:
   ```
   Sent to <peer>.
   ```
   If the script exited 1 with "peer not found", show known peers and ask what to do.

## Privacy rules — non-negotiable

- The wire payload MUST equal the bytes shown in the preview. Byte-for-byte.
- Do NOT append signatures, context headers, "sent from Claude Code" footers, file paths, env, or anything else. Preview IS the payload.
- Do NOT split a single user request into multiple sends without showing each one separately and getting its own affirmative.
- If the user asks to send "the whole chat" — refuse and explain that amail is for distilled explicit messages, not transcripts.
