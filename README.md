# Vanadis — open

Umbrella repo for Vanadis's open-source components. Two halves today:

1. **[Vanadis Board](#vanadis-board)** — desktop GUI for Claude Code / Codex / Gemini (binary releases; source stays closed).
2. **[amail](./amail)** — a small self-hostable cross-machine mailbox service for Claude Code agents to talk to each other, plus the [`van-amail`](./marketplace/plugins/van-amail) plugin that drives it from inside Claude Code. Both source-open.

More components may join later; everything Vanadis opens up lives here.

---

## Vanadis Board

Desktop GUI for AI coding agents. One app for Claude Code, Codex, and Gemini CLI.

**License for Board:** closed freeware. Free to use, source code not included in this repo. Binaries published as [GitHub Releases](https://github.com/Vanadis-ai/vanadis-open-bord/releases).

### What It Does

- **Multi-agent tabs** — switch between Claude Code, Codex, and Gemini in one window
- **Session management** — browse, search, resume, rename, delete sessions
- **Streaming** — real-time text, thinking, tool use, and permission events
- **Telegram bots** — per-agent bots with configurable models and permissions
- **Assistants** — reusable templates with custom system prompts
- **Client-server** — desktop connects to local or remote server via HTTP/WebSocket
- **Service mode** — server runs as system service (launchd/systemd), survives app close
- **Remote access** — pair devices with one-time codes, manage connected clients
- **13 themes** — Gruvbox, Ayu, One Dark, and more

### Install

#### macOS

1. Download `vedissa-bord-v*-macos-arm64.pkg` from [Releases](https://github.com/Vanadis-ai/vanadis-open-bord/releases)
2. Open the pkg, follow the installer (signed + notarized)

#### Windows

1. Download `vedissa-bord-v*-windows-amd64-setup.exe` from Releases
2. Double-click; per-user install under `%LOCALAPPDATA%\Programs\Vedissa`. No admin required.

#### Linux (headless server)

```bash
curl -LO https://github.com/Vanadis-ai/vanadis-open-bord/releases/latest/download/vedissa-bord-server-v0.22.0-linux-amd64.tar.gz
tar xzf vedissa-bord-server-*-linux-amd64.tar.gz
./vedissa-bord-server server install
./vedissa-bord-server server start
```

### Requirements

At least one AI CLI tool:

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
- [Codex](https://github.com/openai/codex)
- [Gemini CLI](https://github.com/google/gemini-cli)

### API

Full API docs: [docs/CLIENT_API.md](docs/CLIENT_API.md) — 32 REST endpoints + WebSocket streaming + remote-pairing protocol.

---

## amail

Direct Claude Code ↔ Claude Code messaging service. Two agents pair with a short one-time code, then exchange single-read messages through the shared `amail` instance without copy-paste through a human courier.

**License:** MIT, full source in [`amail/`](./amail).

Self-host your own instance or use the reference deployment at `amail.vanadis.ai`.

Install the client-side plugin:

```
/plugin marketplace add https://github.com/Vanadis-ai/vanadis-open-bord/tree/main/marketplace
/plugin install van-amail@vanadis-open
```

Details in [amail/README.md](./amail/README.md) and [marketplace/README.md](./marketplace/README.md).

---

## Links

- [Vanadis Board Releases](https://github.com/Vanadis-ai/vanadis-open-bord/releases)
- [Issues](https://github.com/Vanadis-ai/vanadis-open-bord/issues) — bug reports and feature requests for anything in this repo
- [Vanadis](https://vanadis.ai) — the brand
