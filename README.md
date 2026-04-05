# Vanadis Board (Open)

Desktop GUI for AI coding agents. One app for Claude Code, Codex, and Gemini CLI.

## What It Does

- **Multi-agent tabs** -- switch between Claude Code, Codex, and Gemini in one window
- **Session management** -- browse, search, resume, rename, delete sessions
- **Streaming** -- real-time text, thinking, tool use, and permission events
- **Telegram bots** -- per-agent bots with configurable models and permissions
- **Assistants** -- reusable templates with custom system prompts
- **Client-server** -- desktop connects to local or remote server via HTTP/WebSocket
- **Service mode** -- server runs as system service (launchd/systemd), survives app close
- **Remote access** -- pair devices with one-time codes, manage connected clients
- **13 themes** -- Gruvbox, Ayu, One Dark, and more

## Install

### macOS (Desktop)

1. Download `vanadis-bord-v*.macos-arm64.zip` from [Releases](https://github.com/Vanadis-ai/vanadis-open-bord/releases)
2. Unzip and move to Applications
3. Open Vanadis Board

### Linux (Headless Server)

```bash
# Download
curl -LO https://github.com/Vanadis-ai/vanadis-open-bord/releases/latest/download/vanadis-bord-server-v0.13.0-linux-amd64.tar.gz
tar xzf vanadis-bord-server-v0.13.0-linux-amd64.tar.gz

# Run
./vanadis-bord-server-linux-amd64 --port 18420

# Or install as service
./vanadis-bord-server-linux-amd64 server install
./vanadis-bord-server-linux-amd64 server start
```

## Requirements

At least one AI CLI tool installed:

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (Anthropic)
- [Codex](https://github.com/openai/codex) (OpenAI)
- [Gemini CLI](https://github.com/google/gemini-cli) (Google)

## Service Mode

The server can run independently as a system service:

1. Open the app, go to **Settings > Server**
2. Click **Start service**
3. Close the desktop -- server keeps running
4. Telegram bots stay online, remote clients stay connected
5. Reopen the desktop -- it reconnects to the running service

### CLI

```bash
vanadis-bord-server server install   # register as system service
vanadis-bord-server server start     # start
vanadis-bord-server server status    # check
vanadis-bord-server server stop      # stop
vanadis-bord-server server uninstall # remove
```

## Remote Access

Connect from another device (mobile app, another desktop):

1. On the server: **Settings > Server > Generate Code**
2. A 6-character pairing code appears (valid 5 minutes)
3. On the client: enter server URL + pairing code
4. Client receives a persistent bearer token
5. Manage connected clients in **Settings > Server > Paired Clients**

## API

Full API documentation: [docs/CLIENT_API.md](docs/CLIENT_API.md)

- REST API: 32 endpoints for agents, sessions, settings, templates, telegram, filesystem
- WebSocket: real-time streaming with auto-reconnect
- Auth: localhost bypass for local, bearer tokens for remote

## License

Closed freeware. Free to use, source code not included.

## Links

- [Releases](https://github.com/Vanadis-ai/vanadis-open-bord/releases)
- [Issues](https://github.com/Vanadis-ai/vanadis-open-bord/issues) -- bug reports and feature requests
- [API Docs](docs/CLIENT_API.md)
