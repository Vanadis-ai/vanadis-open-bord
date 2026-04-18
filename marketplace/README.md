# Vanadis open marketplace

A small public Claude Code plugin marketplace by Vanadis.

## Install

```
/plugin marketplace add https://github.com/Vanadis-ai/vanadis-open-bord/tree/main/marketplace
```

Or locally:

```
git clone https://github.com/Vanadis-ai/vanadis-open-bord.git
/plugin marketplace add /path/to/vanadis-open-bord/marketplace
```

## Available plugins

| Plugin | Description | Version |
|--------|-------------|---------|
| van-amail | agent mail — direct Claude Code ↔ Claude Code messaging with preview + confirm | 0.1.0 |

## Relationship to the rest of the repo

This marketplace lives alongside the [`amail`](../amail) service in one repo because the plugin and the server it talks to are two halves of one product: install the plugin on your Claude Code, point it at an `amail` instance, and a message flows from one agent to another.

## License

MIT
