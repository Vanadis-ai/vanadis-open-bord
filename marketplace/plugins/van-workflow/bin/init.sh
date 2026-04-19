#!/usr/bin/env bash
# van-workflow init — idempotent local setup.
# Creates ~/.claude/van-workflow-local.md from the bundled template if missing.
# Registers the plugin's SessionStart hook in ~/.claude/settings.json if missing.
#
# Safe to run multiple times: every step is a no-op when already done.

set -euo pipefail

PLUGIN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE="${PLUGIN_ROOT}/templates/van-workflow-local.md.example"
LOCAL="${HOME}/.claude/van-workflow-local.md"
SETTINGS="${HOME}/.claude/settings.json"

mkdir -p "${HOME}/.claude"

if [[ ! -f "${LOCAL}" ]]; then
  cp "${TEMPLATE}" "${LOCAL}"
  echo "created ${LOCAL} — edit it to add your identity / workspace context"
else
  echo "${LOCAL} already exists — leaving as is"
fi

if [[ -f "${SETTINGS}" ]]; then
  # Idempotent hook registration via python (keeps JSON clean, preserves comments-free structure).
  python3 - <<PY
import json, pathlib
p = pathlib.Path(r"${SETTINGS}")
s = json.loads(p.read_text())
hooks = s.setdefault("hooks", {})
ss = hooks.setdefault("SessionStart", [])
cmd = "python3 \${CLAUDE_PLUGIN_ROOT}/hooks/session-start.py"
# Only register if no entry already references session-start.py
flat = [h.get("command","") for e in ss for h in e.get("hooks",[])]
if not any("session-start.py" in c for c in flat):
    ss.append({"hooks":[{"type":"command","command": cmd}]})
    p.write_text(json.dumps(s, indent=2, ensure_ascii=False) + "\n")
    print(f"registered van-workflow SessionStart hook in {p}")
else:
    print("van-workflow SessionStart hook already present")
PY
else
  echo "warning: ${SETTINGS} not found — plugin hook will activate once Claude Code creates it"
fi

echo "van-workflow init complete"
