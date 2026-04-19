#!/usr/bin/env python3
"""
SessionStart hook for van-workflow.

Emits a single system-reminder block with engineering-discipline rules that
should be fresh in the agent's working memory every session. Universal blocks
come first (generic to any developer). Optional local overlay is appended
from ~/.claude/van-workflow-local.md if present — that's where the user puts
identity, workspace context, or machine-specific reminders.

SIZE LIMIT: Claude Code caps SessionStart hook additionalContext at
MAX_HOOK_OUTPUT_CHARS (~10,000 chars in current releases — see
MAX_HOOK_OUTPUT_LENGTH in src/utils/processUserInput/processUserInput.ts).
Output over that is persisted to disk and replaced with a 2KB preview +
file path, which DEFEATS the purpose of keeping rules fresh every session.
The hook prints a warning to stderr when output exceeds SOFT_WARN_CHARS so
you can trim the overlay before hitting the hard cap.

Graceful: on any failure emits an empty context rather than blocking session start.
"""

import json
import os
import sys
from datetime import date
from pathlib import Path

LOCAL_OVERLAY = Path.home() / ".claude" / "van-workflow-local.md"
ERM_FLAG = Path.home() / ".Vanadis" / "erm.flag"  # well-known but harmless on machines without it

# Upper bound on hook additionalContext before Claude Code persists it to disk.
MAX_HOOK_OUTPUT_CHARS = 10_000
# Warn above this threshold so users notice they are approaching the cap.
SOFT_WARN_CHARS = 8_500

CUTOFF_YEAR = 2025
CUTOFF_MONTH = 5

HIGH_RISK_LIBS = [
    "Next.js", "React", "React Native", "Expo SDK", "Wails",
    "LangChain", "OpenAI SDK", "Anthropic SDK", "Claude Code CLI",
    "Bun", "Deno", "SvelteKit", "Nuxt",
    "Prisma", "Drizzle", "tRPC",
    "Tailwind CSS", "Vitest", "Playwright", "Maestro",
]


def months_since_cutoff() -> int:
    today = date.today()
    return (today.year - CUTOFF_YEAR) * 12 + (today.month - CUTOFF_MONTH)


def env(name: str, default: str = "unknown") -> str:
    return os.environ.get(name, default) or default


def read_local_overlay() -> str:
    if not LOCAL_OVERLAY.exists():
        return ""
    try:
        return LOCAL_OVERLAY.read_text().strip()
    except Exception:
        return ""


def erm_line() -> str:
    if ERM_FLAG.exists():
        return "ERM: **ON** — stricter discipline gates active (commit blocked without vet+test; completion claims strictly require evidence)."
    return "ERM: off — baseline Tier A+B rules active. Toggle with /erm-on."


def build_context() -> str:
    today = date.today().isoformat()
    cwd = os.getcwd()
    ide_line = (
        f"IDE: {env('TERMINAL_EMULATOR')} | "
        f"Bundle: {env('__CFBundleIdentifier')} | "
        f"Term: {env('TERM_PROGRAM')} | "
        f"PWD: {cwd}"
    )
    delta = months_since_cutoff()
    high_risk = ", ".join(HIGH_RISK_LIBS)

    blocks = [
        # Identity / when / where
        f"{ide_line}\ncurrentDate: {today}",

        # Model self-awareness
        (
            f"MODEL AWARENESS: You are Claude. Your training cutoff is May 2025. "
            f"Today is {today} — approximately {delta} months after cutoff. "
            f"For any version-sensitive work, your memory is legacy archive, not truth."
        ),

        # Anti-patterns from anthropic/claude-code#50513
        (
            "ANTI-PATTERNS TO WATCH IN YOURSELF (aggregate report anthropic/claude-code#50513):\n"
            "1. Task compression — narrowing the task into a cheaper proxy instead of preserving the real objective.\n"
            "2. Symptom patching — editing the first plausible spot before identifying root cause.\n"
            "3. Green tests as proof — treating passing nearby tests as evidence of correctness.\n"
            "4. Edit before understand — making changes before reading the relevant call path, tests, and configuration.\n"
            "5. Cargo-cult copying — replicating a nearby pattern without demonstrating why it is correct here.\n"
            "6. Done without evidence — claiming 'fixed', 'verified', 'done' without a concrete check.\n"
            "These are not hypothetical. They are the documented failure class for complex engineering tasks. Catch yourself."
        ),

        # Evidence-Based Completion (full rule)
        (
            "EVIDENCE-BASED COMPLETION (mandatory):\n"
            "Never claim 'done', 'fixed', 'verified', 'works', 'ready', 'complete' without a concrete evidence block "
            "adjacent to the claim. Every completion claim MUST carry at least one of:\n"
            "- Command run + output excerpt (e.g. `go vet ./... — ok`)\n"
            "- Test result with suite name + duration\n"
            "- Artifact path (binary, installer, file produced)\n"
            "- Visual verification (screenshot path, DevTools result)\n"
            "- Reproduction before + after for bug fixes\n"
            "- Explicit 'not yet verified' disclaimer when evidence is unavailable\n"
            "Green tests alone are NOT evidence of correctness — they are evidence that what the tests checked still passes. "
            "Name what the tests did NOT cover for any non-trivial change."
        ),

        # OpenSpec Workflow (large-task criteria)
        (
            "OPENSPEC WORKFLOW (large tasks):\n"
            "Before starting implementation on any large task, run /v-spec-write first to produce an OpenSpec specification. "
            "A task is 'large' when ANY of:\n"
            "- Touches 3+ files\n"
            "- Adds or removes a capability (not just fixes behavior)\n"
            "- Affects a public API / wire format / data schema\n"
            "- Expected implementation >100 LoC\n"
            "- Crosses subsystem boundaries\n"
            "Spec uses RFC 2119 (SHALL/MUST/SHOULD) requirements + Given/When/Then scenarios. "
            "Save to in-repo `specs/<domain>/spec.md` when the project uses in-repo specs, otherwise ask the user "
            "where specs should live. Skip only if the user explicitly says 'skip spec'."
        ),

        # Version & Knowledge Verification
        (
            "VERSION & KNOWLEDGE VERIFICATION (strict source hierarchy):\n"
            "1. Project files (package.json, requirements.txt, go.mod, pyproject.toml, Cargo.toml) — absolute authority.\n"
            "2. External verification (official docs, Context7, web search) — overrides training data.\n"
            "3. Training data — reliable for syntax and concepts, UNRELIABLE for versions, API signatures, CLI flags, defaults.\n"
            "Rules:\n"
            "- NEVER state a library version from memory as fact — verify via project files or web search first.\n"
            "- NEVER 'fix' modern code to match older syntax you remember from training.\n"
            "- NEVER claim a function/parameter doesn't exist without verification.\n"
            "- If uncertain about a version or API, say so explicitly, then verify.\n"
            "- When providing version-sensitive information, mark the source: (from package.json), (from web search), (from training data — may be outdated)."
        ),

        # TEMPORAL CONTEXT — high-risk libraries
        (
            f"TEMPORAL CONTEXT: In {delta} months since training cutoff, fast-moving libraries ship multiple major versions. "
            f"HIGH-RISK libraries — always verify versions before writing code:\n"
            f"{high_risk}"
        ),

        # User input handling — voice + hand injuries
        (
            "USER INPUT HANDLING:\n"
            "The user dictates via voice-to-text (Superwhisper, MacWhisper, or similar). Expect mangled words, phonetic errors, "
            "run-on sentences, missing or incorrect punctuation. Both Russian and English arrive via voice.\n"
            "Additionally, the user has reduced hand coordination from past trauma — typed input also contains many keystroke typos. "
            "This is an ACCESSIBILITY CONSTRAINT, not a quality indicator.\n"
            "Interpret intent through the noise:\n"
            "- DO NOT literal-parse mangled words.\n"
            "- DO NOT ask did-you-mean questions on obvious phonetic or keystroke typos.\n"
            "- DO NOT correct or comment on the user's typos unless explicitly asked.\n"
            "- Only ask for clarification when there is genuine ambiguity at the intent level, not at the spelling level."
        ),

        # ERM status
        erm_line(),
    ]

    overlay = read_local_overlay()
    if overlay:
        blocks.append("LOCAL OVERLAY (from ~/.claude/van-workflow-local.md):\n" + overlay)

    return "\n\n".join(blocks)


def main() -> int:
    try:
        ctx = build_context()

        # Size guardrails. Hard cap matches Claude Code's internal cap; above it
        # the output would be persisted to disk and replaced with a preview.
        size = len(ctx)
        if size > MAX_HOOK_OUTPUT_CHARS:
            sys.stderr.write(
                f"van-workflow: SessionStart output is {size} chars, exceeds "
                f"MAX_HOOK_OUTPUT_CHARS={MAX_HOOK_OUTPUT_CHARS}. Claude Code will "
                f"persist it to disk instead of inlining. Trim "
                f"~/.claude/van-workflow-local.md to reduce size.\n"
            )
        elif size > SOFT_WARN_CHARS:
            sys.stderr.write(
                f"van-workflow: SessionStart output is {size} chars, approaching "
                f"the {MAX_HOOK_OUTPUT_CHARS}-char cap. Consider trimming the "
                f"local overlay.\n"
            )

        out = {
            "hookSpecificOutput": {
                "hookEventName": "SessionStart",
                "additionalContext": ctx,
            }
        }
        sys.stdout.write(json.dumps(out))
    except Exception:
        # Graceful: never fail session start because the hook misbehaved.
        sys.stdout.write(json.dumps({
            "hookSpecificOutput": {
                "hookEventName": "SessionStart",
                "additionalContext": "",
            }
        }))
    return 0


if __name__ == "__main__":
    sys.exit(main())
