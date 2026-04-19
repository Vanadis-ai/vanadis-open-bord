# van-workflow

Session-start discipline for Claude Code. Injects engineering reliability rules into the system prompt every session so the agent doesn't have to rediscover them by reading CLAUDE.md each time.

## What it does

On every session start, van-workflow adds a system-reminder block containing:

- **Model awareness** — today's date, training cutoff, months of drift
- **Anti-patterns from anthropic/claude-code#50513** — the six documented complex-engineering failure modes the agent should watch for in itself
- **Evidence-based completion rule** — completion claims require a concrete evidence block
- **OpenSpec workflow rule** — large tasks go through spec-first
- **Version & knowledge verification** — strict source hierarchy (project files > docs > training data)
- **Temporal context** — list of high-risk libraries where training data will be stale
- **User input handling** — voice-dictation and accessibility-typo tolerance
- **ERM status** — whether Engineering Reliability Mode is active
- **Local overlay** — contents of `~/.claude/van-workflow-local.md` if present

The idea: rules that must be fresh every turn go here, so the agent's compliance doesn't depend on reading the right file at the right moment.

## Install

```bash
# 1. Install the plugin via your marketplace (or clone this repo and use a directory marketplace)
# 2. Run the init script once to seed the local overlay and register the hook
bash path/to/van-workflow/bin/init.sh
```

The init script is idempotent — safe to re-run. It will:

- Create `~/.claude/van-workflow-local.md` from the bundled template if it doesn't exist
- Register the SessionStart hook in `~/.claude/settings.json` if not already registered

## The local overlay

`~/.claude/van-workflow-local.md` is where you put things that are specific to *your* machine and shouldn't live in a public plugin — your identity, workspace paths, organization context, machine-specific reminders. It's a plain markdown file, read as-is and appended to the session-start output as "LOCAL OVERLAY".

See `templates/van-workflow-local.md.example` for what typically goes there.

## Size limit — keep it under ~10K chars

Claude Code caps SessionStart hook `additionalContext` at roughly **10,000 characters** (see `MAX_HOOK_OUTPUT_LENGTH` in the Claude Code source). Output over that is persisted to disk and replaced with a 2KB preview + file path, which defeats the whole point of keeping rules fresh every session.

The universal block this plugin emits is ~4.5KB. That leaves ~5KB for your local overlay with a safety margin. The hook checks its own output size at runtime:

- Output > 8,500 chars → warning to stderr (approaching cap)
- Output > 10,000 chars → louder warning (will be persisted by Claude Code)

If you see the warning, trim the overlay: move anything that doesn't belong in **every single session** into project CLAUDE.md or a skill file. Short, high-salience reminders only — if a line wouldn't make the top-5 list of things the agent should know on every turn, it's too much.

Quick size check from the command line:

```bash
python3 path/to/van-workflow/hooks/session-start.py | wc -c
```

## Forking

This plugin is intentionally generic. Fork it if you want to:

- Add more universal blocks (team-wide discipline)
- Change the phrasing of rules to match your org's tone
- Localize the English rules into another language

If you're just adding per-machine context, use the local overlay — no fork needed.

## What's NOT here

- Identity (which assistant, whose machine) — goes in the local overlay
- Project paths — goes in the local overlay OR the project's CLAUDE.md
- Long how-to reference material — goes in CLAUDE.md
- Secrets — goes in your secret store

## License

MIT.

## Attribution

Built by Vedissa & Pasha Gale, Vanadis.AI.
