# Vanadis Board -- Client API Reference

Complete API documentation for building clients (mobile, web, CLI) that connect to the Vanadis Board server. The server is a Go HTTP/WebSocket application that provides a unified interface to AI coding agents (Claude Code, Codex, Gemini CLI).

**Default port:** 18420

---

## Table of Contents

1. [Connection Flow](#1-connection-flow)
2. [Authentication](#2-authentication)
3. [REST API Reference](#3-rest-api-reference)
4. [WebSocket Protocol](#4-websocket-protocol)
5. [Streaming Flow](#5-streaming-flow)
6. [Permission Handling](#6-permission-handling)
7. [Session Management](#7-session-management)
8. [Error Handling](#8-error-handling)
9. [Reconnection Strategy](#9-reconnection-strategy)
10. [Data Types Reference](#10-data-types-reference)

---

## 1. Connection Flow

### Overview

A client connects to the server in three phases:

1. **Discover** -- Find the server (local or remote).
2. **Authenticate** -- Obtain a bearer token (remote clients) or connect directly (localhost).
3. **Connect WebSocket** -- Establish a persistent WS connection for real-time streaming.

### Step-by-step

```
1. GET /api/ping
   -> {"status": "pong"}
   Server is reachable.

2. (Remote only) POST /api/auth/pair
   Exchange a pairing code for a bearer token.

3. Connect WebSocket: ws://<host>:18420/ws[?token=<bearer_token>]
   On open, send a "hello" message with client metadata.

4. GET /api/agents
   Discover available agents (claude, codex, gemini, etc.).

5. GET /api/settings
   Load user preferences (theme, font size, open tabs, etc.).

6. Ready to send messages and receive streaming events.
```

### Server Discovery

For local usage, the server runs on `localhost:18420`. For remote clients (mobile apps connecting over the network), the server IP/hostname must be provided by the user or discovered via mDNS/Bonjour (not built into the server -- the client must implement this if desired).

---

## 2. Authentication

### Localhost Bypass

Requests originating from `127.0.0.1` or `::1` (loopback) skip authentication entirely. No token is required for localhost connections.

### Remote Authentication -- Pairing Flow

Remote clients must obtain a bearer token through a pairing code exchange:

#### Step 1: Generate Pairing Code (from localhost)

A pairing code must be generated from a trusted client (the desktop app or a localhost request).

```
POST /api/auth/pairing-code
(no body required)

Response:
{
  "code": "A3F1B2"
}
```

The code is 6 uppercase hex characters, valid for 5 minutes.

#### Step 2: Exchange Code for Token (from remote client)

The remote client sends the pairing code along with a display name:

```
POST /api/auth/pair
Content-Type: application/json

{
  "code": "A3F1B2",
  "name": "iPhone 15 Pro"
}

Success Response (200):
{
  "token": "a1b2c3d4e5f6...64_hex_chars"
}

Failure Response (403):
"invalid code"
```

The token is a 64-character hex string. Store it securely (Keychain on iOS, EncryptedSharedPreferences on Android).

#### Step 3: Use Token

Include the token in all subsequent requests:

**HTTP requests:**
```
Authorization: Bearer <token>
```

**WebSocket connections:**
```
ws://<host>:18420/ws?token=<token>
```

The token can also be passed as a query parameter `?token=<token>` on HTTP requests (for environments where custom headers are inconvenient).

### Managing Paired Clients

#### List Paired Clients

```
GET /api/auth/clients

Response:
[
  {
    "id": "a1b2c3d4e5f6g7h8",
    "name": "iPhone 15 Pro",
    "created": "2026-04-05T10:00:00Z",
    "last_seen": "2026-04-05T12:30:00Z"
  }
]
```

Note: `token_hash` is never returned in list responses.

#### Revoke a Client

```
DELETE /api/auth/clients/{id}

Response:
{"ok": true}
```

After revocation, the client's token is immediately invalid.

---

## 3. REST API Reference

All endpoints return JSON. All request bodies are JSON with `Content-Type: application/json`.

CORS is fully open (`Access-Control-Allow-Origin: *`).

### 3.1 Health

#### Ping

```
GET /api/ping

Response:
{"status": "pong"}
```

#### Hostname

```
GET /api/hostname

Response:
{"hostname": "macbook-pro.local"}
```

Returns the OS hostname of the server machine. Useful for building client display names.

### 3.2 Agents

#### List All Agents

```
GET /api/agents

Response:
[
  {
    "id": "claude",
    "display_name": "Claude Code",
    "session_noun": "conversation",
    "features": {
      "tool_panel": true,
      "permissions": true,
      "thinking": true,
      "sessions": true,
      "resume": true
    },
    "settings": [
      {
        "key": "permMode",
        "label": "Permission Mode",
        "type": "select",
        "options": [
          {"value": "default", "label": "Ask Every Time"},
          {"value": "bypassPermissions", "label": "Accept All"}
        ],
        "default": "default"
      }
    ]
  }
]
```

The `features` object tells the client which UI elements to render for each agent:

| Feature | Meaning |
|---------|---------|
| `tool_panel` | Agent emits `tool_use`/`tool_result` events -- show tool execution panel |
| `permissions` | Agent emits `permission` events -- show permission approval cards |
| `thinking` | Agent emits `thinking` events -- show thinking/reasoning blocks |
| `sessions` | Agent supports session listing and history |
| `resume` | Agent supports resuming previous sessions |

The `settings` array defines agent-specific configuration fields that should be rendered in the settings UI. Field types: `"select"`, `"text"`, `"toggle"`.

#### Get Models for Agent

```
GET /api/agents/{id}/models

Response:
[
  {"alias": "sonnet", "full_id": "claude-sonnet-4-20250514"},
  {"alias": "opus", "full_id": "claude-opus-4-20250514"},
  {"alias": "haiku", "full_id": "claude-haiku-4-20250514"}
]
```

Returns available models for the specified agent. Returns `[]` if the agent does not support model selection or is not available.

#### Get Usage for Agent

```
GET /api/agents/{id}/usage

Response:
{
  "limits": [
    {
      "label": "Daily",
      "usedPercent": 45.2,
      "used": "450K",
      "total": "1M",
      "resetsIn": "8h 30m"
    }
  ],
  "models": [
    {"model": "claude-sonnet-4-20250514", "tokens": 125000}
  ]
}
```

Returns `null` if the agent does not support usage reporting.

### 3.3 Sessions

#### List Sessions

```
GET /api/agents/{id}/sessions

Response:
[
  {
    "id": "abc123-def4-5678-90ab-cdef12345678",
    "title": "Refactor auth module",
    "cwd": "/Users/dev/myproject",
    "model": "claude-sonnet-4-20250514",
    "last_modified": 1712300000
  }
]
```

Returns up to 50 most recent sessions, ordered by modification time (newest first). `last_modified` is a Unix timestamp in seconds.

#### Load Session Messages

```
GET /api/agents/{id}/sessions/{sid}/messages

Response:
{
  "messages": [
    {
      "role": "user",
      "blocks": [
        {"type": "text", "text": "Fix the login bug"}
      ]
    },
    {
      "role": "assistant",
      "blocks": [
        {"type": "text", "text": "I'll look into the auth module..."},
        {
          "type": "tool_use",
          "id": "tu_123",
          "name": "Read",
          "input": {"file_path": "/src/auth.ts"}
        },
        {
          "type": "tool_result",
          "tool_use_id": "tu_123",
          "content": "import { hash } from...",
          "is_error": false
        }
      ]
    }
  ],
  "tokens": 15000
}
```

Block types within messages:

| Type | Fields |
|------|--------|
| `text` | `text` |
| `tool_use` | `id`, `name`, `input` |
| `tool_result` | `tool_use_id`, `content`, `is_error` |

#### Rename Session

```
PUT /api/agents/{id}/sessions/{sid}
Content-Type: application/json

{"title": "New Session Title"}

Response:
{"ok": true}
```

#### Delete Session

```
DELETE /api/agents/{id}/sessions/{sid}

Response:
{"ok": true}
```

#### Batch Delete Sessions

```
POST /api/agents/{id}/sessions/delete-batch
Content-Type: application/json

{"ids": ["session-id-1", "session-id-2", "session-id-3"]}

Response:
{"deleted": 3}
```

#### Create Bot Session

Creates a new persistent session for bot use (Telegram integration).

```
POST /api/agents/{id}/sessions/bot
Content-Type: application/json

{"cwd": "/Users/dev/myproject"}

Response:
{"id": "new-session-uuid"}
```

Returns `{"id": ""}` if `cwd` is empty or the agent doesn't support sessions.

### 3.4 Settings

#### Load Settings

```
GET /api/settings

Response:
{
  "theme": "dark",
  "fontSize": 14,
  "model": "",
  "permMode": "bypassPermissions",
  "openTabs": [
    {"id": "tab-uuid", "title": "Tab 1", "cwd": "/Users/dev", "model": ""}
  ],
  "activeTab": 0,
  "allowedDirs": ["/Users/dev/projects"],
  "sidebarWidth": 280,
  "toolPanelWidth": 400,
  "language": "en",
  "templates": [],
  "userName": "Pavlo",
  "assistantName": "Vanadis",
  "userDescription": "",
  "globalSystemPrompt": "You are a helpful AI assistant...",
  "sessionName": "Vanadis",
  "settingsVersion": 2,
  "agents": [
    {
      "id": "claude",
      "enabled": true,
      "defaults": {"model": "sonnet", "permMode": "bypassPermissions"},
      "mainBot": null,
      "mainSessionId": "",
      "openTabs": [],
      "activeTab": 0,
      "sidebarWidth": 0
    }
  ],
  "activeAgent": "claude",
  "agentTabOrder": ["claude"]
}
```

See [Settings](#settings-1) in the Data Types section for the complete schema.

#### Save Settings

```
PUT /api/settings
Content-Type: application/json

{ ...full settings object... }

Response:
{"ok": true}
```

Saving settings triggers a `settings_changed` event broadcast to all connected WebSocket clients.

#### Add Allowed Directory

```
POST /api/settings/allowed-dir
Content-Type: application/json

{"dir": "/Users/dev/new-project"}

Response:
{"ok": true}
```

Adds a directory to the allowed directories list (used for agent sandboxing). Idempotent -- adding a directory that already exists is a no-op.

#### Get System Prompt Prefix

```
GET /api/settings/system-prompt-prefix

Response:
{"prefix": "You are a helpful AI assistant. The user's name is Pavlo."}
```

Returns the combined system prompt prefix built from `globalSystemPrompt`, `userName`, and `userDescription`. Used when agents need the server-side system prompt context.

### 3.5 Filesystem

These endpoints provide server-side filesystem browsing for directory selection (working directory picker).

#### Home Directory

```
GET /api/fs/home

Response:
{"path": "/Users/dev"}
```

#### Default Directory

```
GET /api/fs/default-dir

Response:
{"path": "/Users/dev"}
```

Currently returns the same value as home directory.

#### List Directory

```
POST /api/fs/list
Content-Type: application/json

{"path": "/Users/dev"}

Response:
[
  {"name": "projects", "path": "/Users/dev/projects", "isDir": true},
  {"name": "Documents", "path": "/Users/dev/Documents", "isDir": true}
]
```

Returns only directories (not files). Hidden directories (names starting with `.`) are excluded. If `path` is empty, lists the home directory.

#### Create Directory

```
POST /api/fs/create-dir
Content-Type: application/json

{"parent": "/Users/dev/projects", "name": "new-project"}

Response:
{"path": "/Users/dev/projects/new-project"}
```

Returns `{"path": ""}` if the name is empty, contains `/`, or contains `..`.

#### Check Directory Exists

```
POST /api/fs/exists
Content-Type: application/json

{"path": "/Users/dev/projects/myapp"}

Response:
{"exists": true}
```

### 3.6 Templates

Templates are reusable session presets (pre-configured system prompt, model, working directory, etc.).

#### List Templates

```
GET /api/templates

Response:
[
  {
    "id": "tpl-1712300000000",
    "name": "Code Review",
    "model": "sonnet",
    "permMode": "default",
    "systemPrompt": "You are a code reviewer...",
    "cwd": "/Users/dev/project",
    "persist": true,
    "autoClear": false,
    "isBot": false,
    "sessionId": "",
    "agentId": "claude",
    "agentSettings": {"model": "sonnet", "permMode": "default"},
    "bot": null
  }
]
```

#### Create or Update Template

```
POST /api/templates
Content-Type: application/json

{
  "id": "",
  "name": "Code Review",
  "systemPrompt": "You are a code reviewer...",
  "cwd": "/Users/dev/project",
  "persist": true,
  "agentId": "claude",
  "agentSettings": {"model": "sonnet"}
}

Response:
[...updated templates array...]
```

If `id` is empty, the server generates one (`tpl-<unix_nano>`). If `id` matches an existing template, it is updated. The response is the full templates array after the operation.

#### Delete Template

```
DELETE /api/templates/{id}

Response:
[...updated templates array...]
```

### 3.7 Telegram Bots

#### Start Bot

```
POST /api/telegram/bots/{id}/start
Content-Type: application/json

{"agentID": "claude"}

Response:
{"error": ""}
```

An empty string `""` for `error` means success. Non-empty string is the error message.

#### Stop Bot

```
POST /api/telegram/bots/{id}/stop

Response:
{"ok": true}
```

#### Get Bot Status

```
GET /api/telegram/bots/{id}/status

Response:
{
  "running": true,
  "username": "my_ai_bot",
  "error": ""
}
```

#### Get All Bot Statuses

```
GET /api/telegram/bots/status

Response:
{
  "bot-id-1": {"running": true, "username": "bot1", "error": ""},
  "bot-id-2": {"running": false, "username": "", "error": "token invalid"}
}
```

### 3.8 Server Management

#### Server Status

```
GET /api/server/status

Response:
{
  "mode": "embedded",
  "port": 18420,
  "uptime": "2h30m15s",
  "clients": [
    {
      "name": "MacBook Pro (Desktop, macOS)",
      "type": "desktop",
      "platform": "macOS",
      "version": "0.13.0"
    }
  ],
  "serviceStatus": "not_installed"
}
```

| Field | Values |
|-------|--------|
| `mode` | `"embedded"` (running inside desktop app) or `"service"` (running as system service) |
| `serviceStatus` | `"running"`, `"stopped"`, `"not_installed"`, `"error"` |

#### Install and Start as System Service

```
POST /api/service/start

Response:
{"ok": true}
or
{"ok": false, "error": "permission denied"}
```

Installs the server binary, registers it as a system service (launchd on macOS, systemd on Linux), stops the embedded server, and starts the service. If the service fails to start, the embedded server restarts automatically.

#### Stop and Uninstall Service

```
POST /api/service/stop

Response:
{"ok": true}
```

---

## 4. WebSocket Protocol

### Connecting

```
WebSocket URL: ws://<host>:18420/ws
With auth:     ws://<host>:18420/ws?token=<bearer_token>
```

Authentication for WebSocket follows the same rules as HTTP -- localhost connections are unauthenticated, remote connections require the token as a query parameter.

### Hello Message (Client -> Server)

Immediately after the WebSocket connection opens, the client must send a `hello` message identifying itself:

```json
{
  "type": "hello",
  "name": "iPhone 15 Pro (Mobile, iOS)",
  "clientType": "mobile",
  "platform": "iOS",
  "version": "1.0.0"
}
```

| Field | Values | Description |
|-------|--------|-------------|
| `type` | `"hello"` | Required, always `"hello"` |
| `name` | string | Human-readable client name (shown in server status) |
| `clientType` | `"desktop"`, `"mobile"`, `"web"`, `"cli"` | Client category |
| `platform` | `"macOS"`, `"Linux"`, `"Windows"`, `"iOS"`, `"Android"` | Operating system |
| `version` | string | Client app version |

The `name` field is displayed in the server status panel and should be descriptive. Suggested format: `"<device_or_hostname> (<TypeLabel>, <platform>)"`.

### Client -> Server Message Types

All messages are JSON objects with a `type` field.

#### send_message

Send a user message to an agent. This starts a streaming response.

```json
{
  "type": "send_message",
  "agentID": "claude",
  "tabID": "tab-uuid-here",
  "prompt": "Fix the login bug in auth.ts",
  "cwd": "/Users/dev/myproject",
  "model": "sonnet",
  "systemPrompt": "",
  "permMode": "default"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `agentID` | Yes | Agent to use (from `GET /api/agents`) |
| `tabID` | Yes | Tab/conversation identifier (UUID v4 recommended) |
| `prompt` | Yes | User message text |
| `cwd` | Yes | Working directory for the agent process |
| `model` | No | Model override (empty = use agent default) |
| `systemPrompt` | No | System prompt override |
| `permMode` | No | Permission mode override (empty = use agent/settings default) |

The `tabID` acts as the conversation identifier. When sending a message to a `tabID` that matches an existing session (UUID format, 36 chars with dashes), the server attempts to resume that session. For new conversations, generate a fresh UUID.

#### respond_permission

Respond to a permission request from the agent.

```json
{
  "type": "respond_permission",
  "agentID": "claude",
  "tabID": "tab-uuid-here",
  "permID": "req_abc123",
  "action": "allow"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `agentID` | Yes | Agent that requested permission |
| `tabID` | Yes | Tab where the request occurred |
| `permID` | Yes | Permission request ID (from `stream:permission` event) |
| `action` | Yes | One of: `"allow"`, `"allow-all"`, `"bypass"`, `"deny"` |

Actions:

| Action | Behavior |
|--------|----------|
| `"allow"` | Allow this specific tool call |
| `"allow-all"` | Allow this tool call (semantically same as allow at the protocol level) |
| `"bypass"` | Allow this tool call (semantically same as allow at the protocol level) |
| `"deny"` | Deny this specific tool call |

At the protocol level, `allow`, `allow-all`, and `bypass` all resolve to `allow=true`. The distinction is for UI semantics only.

#### stop_query

Cancel the currently running query on a tab.

```json
{
  "type": "stop_query",
  "agentID": "claude",
  "tabID": "tab-uuid-here"
}
```

This sends a cancellation signal to the running agent process. The server will emit a `stream:complete` event when the process exits.

#### close_tab

Close a tab and stop any running query.

```json
{
  "type": "close_tab",
  "agentID": "claude",
  "tabID": "tab-uuid-here"
}
```

Functionally identical to `stop_query` -- stops the running process for the given tab.

### Server -> Client Event Types

All events are JSON objects broadcast to all connected WebSocket clients.

#### stream:init

Emitted when an agent session starts. Contains the real session ID.

```json
{
  "type": "stream:init",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "sessionID": "abc123-real-session-id",
  "slashCommands": ["/compact", "/model", "/context"]
}
```

The `sessionID` may differ from `tabID` if the agent created a new session. Store this ID for session resume.

`slashCommands` (optional) lists agent-supported slash commands.

#### stream:text

Streaming text content from the assistant.

```json
{
  "type": "stream:text",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "text": "I'll look into the auth module..."
}
```

Multiple `stream:text` events are emitted during a response. Append the `text` field to the current assistant message.

#### stream:thinking

Streaming thinking/reasoning content (when agent supports extended thinking).

```json
{
  "type": "stream:thinking",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "text": "Let me analyze the error..."
}
```

Render thinking blocks in a collapsible/dimmed section, separate from the main response text.

#### stream:tool_use

Agent is invoking a tool.

```json
{
  "type": "stream:tool_use",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "id": "tool_use_abc123",
  "name": "Read",
  "input": {
    "file_path": "/src/auth.ts",
    "limit": 100
  }
}
```

The `input` field is an arbitrary object -- its structure depends on the tool name. Display the tool name and a summary of the input in the tool panel.

#### stream:tool_result

Result returned from a tool invocation.

```json
{
  "type": "stream:tool_result",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "toolUseID": "tool_use_abc123",
  "content": "import { hashPassword } from './crypto';\n...",
  "isError": false
}
```

Match `toolUseID` to the corresponding `stream:tool_use` event's `id` to associate results with their invocations.

#### stream:permission

Agent is requesting permission to use a tool. The agent process is BLOCKED until a response is sent.

```json
{
  "type": "stream:permission",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "id": "req_abc123",
  "toolName": "Write",
  "toolInput": {
    "file_path": "/src/auth.ts",
    "content": "..."
  }
}
```

The client MUST respond with a `respond_permission` message (see [Permission Handling](#6-permission-handling)). If no response is sent, the agent process hangs indefinitely.

#### stream:permission_denied

A tool call was denied during the session (emitted as part of the result).

```json
{
  "type": "stream:permission_denied",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "toolName": "Bash",
  "toolUseID": "tool_use_xyz",
  "toolInput": {"command": "rm -rf /"}
}
```

#### stream:result

Final result metadata for the completed turn.

```json
{
  "type": "stream:result",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "sessionID": "abc123-real-session-id",
  "totalCostUSD": 0.0342,
  "tokens": 15000,
  "numTurns": 3,
  "resultText": "I've fixed the auth bug...",
  "hasDenials": false
}
```

#### stream:complete

Marks the end of a streaming response. Always emitted as the final event for any message flow.

```json
{
  "type": "stream:complete",
  "tabID": "tab-uuid-here"
}
```

This event signals that the agent has finished processing and no more events will arrive for this `tabID` until the next `send_message`.

#### stream:error

An error occurred during processing.

```json
{
  "type": "stream:error",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "message": "unknown agent: invalid-agent"
}
```

Errors are always followed by a `stream:complete` event.

#### stream:sandbox_blocked

A file access was blocked by the sandbox.

```json
{
  "type": "stream:sandbox_blocked",
  "tabID": "tab-uuid-here",
  "agentID": "claude",
  "path": "/etc/passwd"
}
```

#### settings_changed

Broadcast when settings are saved by any client.

```json
{
  "type": "settings_changed"
}
```

Clients should reload settings (`GET /api/settings`) when receiving this event to stay in sync.

---

## 5. Streaming Flow

### Complete Message Lifecycle

When a user sends a message, the following event sequence occurs:

```
Client                          Server                          Agent
  |                               |                               |
  |--- send_message ------------->|                               |
  |                               |--- start agent process ------>|
  |                               |                               |
  |<-- stream:init ---------------|<-- init event ----------------|
  |                               |                               |
  |<-- stream:thinking -----------|<-- thinking event ------------|
  |<-- stream:thinking -----------|<-- thinking event ------------|
  |                               |                               |
  |<-- stream:text ---------------|<-- text event ----------------|
  |<-- stream:text ---------------|<-- text event ----------------|
  |                               |                               |
  |<-- stream:tool_use -----------|<-- tool_use event ------------|
  |<-- stream:tool_result --------|<-- tool_result event ---------|
  |                               |                               |
  |<-- stream:permission ---------|<-- permission event ----------|
  |                               |     (agent BLOCKED)           |
  |--- respond_permission ------->|--- permission response ------>|
  |                               |     (agent RESUMES)           |
  |                               |                               |
  |<-- stream:text ---------------|<-- text event ----------------|
  |                               |                               |
  |<-- stream:result -------------|<-- result event --------------|
  |<-- stream:complete -----------|                               |
```

### Event Ordering Rules

1. `stream:init` is always the first event for a new message.
2. `stream:thinking` events may precede or be interleaved with `stream:text`.
3. `stream:tool_use` is always followed (eventually) by a matching `stream:tool_result`.
4. `stream:permission` blocks the agent until `respond_permission` is received.
5. `stream:result` is emitted once, containing final cost/token metadata.
6. `stream:complete` is always the last event. It is always emitted, even on errors.
7. Multiple tool use/result pairs may occur in sequence within a single turn.
8. The agent may perform multiple turns (text -> tools -> text -> tools -> ...) before the final result.

### Resuming a Session

To resume a previous session, send a `send_message` with the `tabID` set to the session's UUID. The server checks if a session file exists for that UUID and resumes it. The `stream:init` event will contain the confirmed `sessionID`.

### Local Commands

Certain commands are handled locally by the server without invoking the agent:

| Command | Effect |
|---------|--------|
| `/cost` | Returns session cost, tokens, and turns as a `stream:text` event |
| `/help` | Returns list of available commands |
| `/context` | Returns context (token) usage |

Other slash commands (`/compact`, `/model`) are passed through to the agent.

---

## 6. Permission Handling

### When Permissions Occur

When the agent attempts to use a tool that requires user approval (file writes, bash commands, etc.), it emits a `stream:permission` event and blocks until a response is received.

### Rendering Permission Cards

When a `stream:permission` event arrives, render a permission card showing:

1. **Tool name** -- `toolName` (e.g., `"Write"`, `"Bash"`, `"Edit"`)
2. **Tool input** -- `toolInput` object, formatted based on tool type:
   - **Bash**: Show `command` field
   - **Write**: Show `file_path` and `content` (possibly truncated)
   - **Edit**: Show `file_path`, `old_string`, `new_string`
   - **Read**: Show `file_path`
3. **Action buttons**: Allow / Deny

### Responding to Permissions

Send a `respond_permission` message with the `permID` from the event:

```json
{
  "type": "respond_permission",
  "agentID": "claude",
  "tabID": "tab-uuid",
  "permID": "req_abc123",
  "action": "allow"
}
```

### Timeout Behavior

There is no built-in timeout for permission requests. The agent process will wait indefinitely. The client should implement its own timeout UX if desired (e.g., auto-deny after 5 minutes of inactivity).

### Permission Modes

The `permMode` field in `send_message` controls the agent's permission behavior:

| Mode | Behavior |
|------|----------|
| `"default"` | Agent asks for permission on each tool use |
| `"bypassPermissions"` | Agent auto-accepts all tool calls (no permission events emitted) |

The effective permission mode is resolved in this order:
1. `permMode` in `send_message` (if non-empty)
2. Agent-specific default from settings (`agents[].defaults.permMode`)
3. Global default from settings

---

## 7. Session Management

### Session Lifecycle

1. **New session**: Send `send_message` with a fresh UUID as `tabID`. The agent creates a new session.
2. **Init event**: `stream:init` returns the real `sessionID` (may differ from `tabID`).
3. **Active session**: The session persists on the server as long as the agent process is running.
4. **Session stored**: After `stream:complete`, the session is saved to disk by the agent.
5. **Resume**: Send `send_message` with the stored `sessionID` as `tabID` to continue.

### Listing and Loading History

```
1. GET /api/agents/{id}/sessions       -> list of SessionInfo
2. GET /api/agents/{id}/sessions/{sid}/messages -> MessagesResult with full history
```

### Session Operations

- **Rename**: `PUT /api/agents/{id}/sessions/{sid}` with `{"title": "new name"}`
- **Delete**: `DELETE /api/agents/{id}/sessions/{sid}`
- **Batch delete**: `POST /api/agents/{id}/sessions/delete-batch` with `{"ids": [...]}`

### Tab-Session Mapping

The client maintains a mapping between UI tabs and session IDs:

1. When creating a new tab, generate a UUID v4 as the `tabID`.
2. When `stream:init` arrives with a different `sessionID`, update the mapping.
3. When resuming, use the stored `sessionID` as the `tabID` in `send_message`.
4. The server tracks active sessions per `tabID` and per `sessionID` for the same underlying process.

---

## 8. Error Handling

### HTTP Error Responses

| Status | Meaning |
|--------|---------|
| 400 | Bad request (malformed JSON body) |
| 401 | Unauthorized (no token provided, remote connection) |
| 403 | Forbidden (invalid token or pairing code) |
| 500 | Internal server error |

Error responses are plain text, not JSON:

```
HTTP/1.1 400 Bad Request

bad request
```

### WebSocket Errors

Agent errors during streaming are delivered as `stream:error` events, followed by `stream:complete`:

```json
{"type": "stream:error", "tabID": "tab-uuid", "agentID": "claude", "message": "unknown agent: foo"}
{"type": "stream:complete", "tabID": "tab-uuid"}
```

### Handling Agent Unavailability

If an agent is not installed or not available, `GET /api/agents` still lists it (based on registration), but operations will fail with errors. Check agent availability using the `CheckAvailability` method exposed through the agent descriptor, or handle errors gracefully when `send_message` fails.

---

## 9. Reconnection Strategy

### WebSocket Reconnection

Implement exponential backoff reconnection:

```
Initial delay:  1 second
Backoff factor: 2x
Maximum delay:  30 seconds
```

Algorithm:
1. On disconnect, wait `delay` milliseconds.
2. Attempt reconnection.
3. On success, reset `delay` to 1 second and send `hello` message.
4. On failure, set `delay = min(delay * 2, 30000)` and go to step 1.

### State Recovery After Reconnect

After reconnecting:

1. Send `hello` message to re-identify the client.
2. Reload settings: `GET /api/settings`.
3. Reload agent list: `GET /api/agents`.
4. Reload session list if the sessions panel is visible.
5. Note: any in-flight streaming for a previous `send_message` is lost. The `stream:complete` event may have been missed. The client should treat active tabs as "idle" after reconnect.

### Reference Implementation

From the JS WebSocket client:

```javascript
_scheduleReconnect() {
  setTimeout(() => {
    if (!this._shouldReconnect) return;
    this._reconnectDelay = Math.min(
      this._reconnectDelay * 2,
      this._maxReconnectDelay  // 30000
    );
    this._doConnect();
  }, this._reconnectDelay);
}
```

---

## 10. Data Types Reference

All types represented as JSON schemas derived from the Go source code.

### AgentDescriptor

```json
{
  "id": "string",
  "display_name": "string",
  "session_noun": "string",
  "features": {
    "tool_panel": "boolean",
    "permissions": "boolean",
    "thinking": "boolean",
    "sessions": "boolean",
    "resume": "boolean"
  },
  "settings": [
    {
      "key": "string",
      "label": "string",
      "type": "string (select | text | toggle)",
      "options": [
        {"value": "string", "label": "string"}
      ],
      "default": "string"
    }
  ]
}
```

### SessionInfo

```json
{
  "id": "string (UUID)",
  "title": "string",
  "cwd": "string (absolute path)",
  "model": "string",
  "last_modified": "number (Unix timestamp, seconds)"
}
```

### MessagesResult

```json
{
  "messages": [
    {
      "role": "string (user | assistant)",
      "blocks": [
        {
          "type": "string (text | tool_use | tool_result)",
          "text": "string (when type=text)",
          "id": "string (when type=tool_use)",
          "name": "string (when type=tool_use)",
          "input": "object (when type=tool_use)",
          "content": "string (when type=tool_result)",
          "is_error": "boolean (when type=tool_result)",
          "tool_use_id": "string (when type=tool_result)"
        }
      ]
    }
  ],
  "tokens": "number"
}
```

### ModelInfo

```json
{
  "alias": "string (short name, e.g. 'sonnet')",
  "full_id": "string (full model ID, e.g. 'claude-sonnet-4-20250514')"
}
```

### UsageInfo

```json
{
  "limits": [
    {
      "label": "string",
      "usedPercent": "number (0-100)",
      "used": "string (human-readable)",
      "total": "string (human-readable)",
      "resetsIn": "string (duration)"
    }
  ],
  "models": [
    {
      "model": "string",
      "tokens": "number"
    }
  ]
}
```

### Settings

```json
{
  "theme": "string (dark | light)",
  "fontSize": "number",
  "model": "string",
  "permMode": "string",
  "openTabs": ["TabInfo"],
  "activeTab": "number (index)",
  "winX": "number",
  "winY": "number",
  "winW": "number",
  "winH": "number",
  "allowedDirs": ["string"],
  "sidebarWidth": "number",
  "toolPanelWidth": "number",
  "language": "string",
  "templates": ["Template"],
  "userName": "string",
  "assistantName": "string",
  "userDescription": "string",
  "globalSystemPrompt": "string",
  "sessionName": "string",
  "mainSessionId": "string",
  "settingsVersion": "number (current: 2)",
  "agents": ["AgentConfig"],
  "activeAgent": "string (agent ID)",
  "agentTabOrder": ["string (agent IDs)"]
}
```

### AgentConfig

```json
{
  "id": "string",
  "enabled": "boolean",
  "defaults": {
    "model": "string",
    "permMode": "string"
  },
  "mainBot": "TelegramBot | null",
  "mainSessionId": "string",
  "openTabs": ["TabInfo"],
  "activeTab": "number",
  "sidebarWidth": "number"
}
```

### TabInfo

```json
{
  "id": "string (UUID)",
  "title": "string",
  "cwd": "string",
  "model": "string"
}
```

### Template

```json
{
  "id": "string (auto-generated: tpl-<unix_nano>)",
  "name": "string",
  "model": "string (deprecated, use agentSettings)",
  "permMode": "string (deprecated, use agentSettings)",
  "systemPrompt": "string",
  "cwd": "string",
  "persist": "boolean",
  "autoClear": "boolean",
  "isBot": "boolean",
  "sessionId": "string",
  "agentId": "string",
  "agentSettings": {
    "model": "string",
    "permMode": "string"
  },
  "bot": "TelegramBot | null"
}
```

### TelegramBot

```json
{
  "id": "string",
  "assistantId": "string",
  "name": "string",
  "token": "string",
  "allowedUsers": ["number (Telegram user IDs)"],
  "cwd": "string",
  "model": "string",
  "permMode": "string",
  "systemPrompt": "string",
  "autoStart": "boolean"
}
```

### BotStatus

```json
{
  "running": "boolean",
  "username": "string (Telegram bot username)",
  "error": "string (empty on success)"
}
```

### ClientInfo

```json
{
  "name": "string",
  "type": "string (desktop | mobile | web | cli)",
  "platform": "string (macOS | Linux | Windows | iOS | Android)",
  "version": "string"
}
```

### PairedClient

```json
{
  "id": "string (16-char hex)",
  "name": "string",
  "created": "string (RFC3339 UTC)",
  "last_seen": "string (RFC3339 UTC)"
}
```

### DirEntry

```json
{
  "name": "string",
  "path": "string (absolute path)",
  "isDir": "boolean (always true in list responses)"
}
```

### PermissionDenial

```json
{
  "tool_name": "string",
  "tool_use_id": "string",
  "tool_input": "object"
}
```

---

## Appendix A: Complete Endpoint Table

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/ping` | Health check |
| `GET` | `/api/hostname` | Server hostname |
| `GET` | `/api/agents` | List available agents |
| `GET` | `/api/agents/{id}/models` | List models for agent |
| `GET` | `/api/agents/{id}/usage` | Get usage/rate limit info |
| `GET` | `/api/agents/{id}/sessions` | List sessions (max 50) |
| `GET` | `/api/agents/{id}/sessions/{sid}/messages` | Load session messages |
| `PUT` | `/api/agents/{id}/sessions/{sid}` | Rename session |
| `DELETE` | `/api/agents/{id}/sessions/{sid}` | Delete session |
| `POST` | `/api/agents/{id}/sessions/delete-batch` | Batch delete sessions |
| `POST` | `/api/agents/{id}/sessions/bot` | Create bot session |
| `GET` | `/api/settings` | Load settings |
| `PUT` | `/api/settings` | Save settings |
| `POST` | `/api/settings/allowed-dir` | Add allowed directory |
| `GET` | `/api/settings/system-prompt-prefix` | Get system prompt prefix |
| `GET` | `/api/fs/home` | Home directory path |
| `GET` | `/api/fs/default-dir` | Default directory path |
| `POST` | `/api/fs/list` | List directory contents |
| `POST` | `/api/fs/create-dir` | Create directory |
| `POST` | `/api/fs/exists` | Check directory existence |
| `GET` | `/api/templates` | List templates |
| `POST` | `/api/templates` | Create/update template |
| `DELETE` | `/api/templates/{id}` | Delete template |
| `POST` | `/api/telegram/bots/{id}/start` | Start Telegram bot |
| `POST` | `/api/telegram/bots/{id}/stop` | Stop Telegram bot |
| `GET` | `/api/telegram/bots/{id}/status` | Get bot status |
| `GET` | `/api/telegram/bots/status` | Get all bot statuses |
| `POST` | `/api/auth/pair` | Exchange pairing code for token |
| `GET` | `/api/auth/clients` | List paired clients |
| `DELETE` | `/api/auth/clients/{id}` | Revoke client |
| `POST` | `/api/auth/pairing-code` | Generate pairing code |
| `GET` | `/api/server/status` | Server status and connected clients |
| `POST` | `/api/service/start` | Install and start as system service |
| `POST` | `/api/service/stop` | Stop and uninstall service |
| `GET` | `/ws` | WebSocket endpoint |

## Appendix B: WebSocket Message Type Summary

### Client -> Server

| Type | Description |
|------|-------------|
| `hello` | Client identification after connect |
| `send_message` | Send user message to agent |
| `respond_permission` | Allow or deny a tool permission |
| `stop_query` | Cancel running query |
| `close_tab` | Close tab (stops query) |

### Server -> Client

| Type | Description |
|------|-------------|
| `stream:init` | Session started, contains sessionID |
| `stream:text` | Assistant text content (streaming) |
| `stream:thinking` | Assistant thinking/reasoning (streaming) |
| `stream:tool_use` | Tool invocation started |
| `stream:tool_result` | Tool invocation result |
| `stream:permission` | Permission request (BLOCKS agent) |
| `stream:permission_denied` | A tool call was denied |
| `stream:result` | Final result with cost/token metadata |
| `stream:complete` | End of response (always final) |
| `stream:error` | Error during processing |
| `stream:sandbox_blocked` | File access blocked by sandbox |
| `settings_changed` | Settings were updated by another client |
