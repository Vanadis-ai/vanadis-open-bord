# Vanadis — open

> **AI Craftery Bord (formerly Vanadis Board / Vedissa Bord) has moved.**
> New home: **[Vanadis-ai/aicraftery](https://github.com/Vanadis-ai/aicraftery)**.
> All current releases, install scripts, and updates live there.

This repo is now the home for Vanadis's open-source side projects:

1. **[amail](#amail)** — a small self-hostable cross-machine mailbox service for Claude Code agents to talk to each other, plus the [`van-amail`](./marketplace/plugins/van-amail) plugin that drives it from inside Claude Code.
2. **[marketplace](#marketplace)** — Claude Code plugins published as a marketplace.

Both source-open. The desktop client (Bord) is closed-source freeware and lives in the [`aicraftery`](https://github.com/Vanadis-ai/aicraftery) repo as binary releases.

---

## AI Craftery Bord (moved)

If you came here for the desktop GUI for Claude Code / Codex / Gemini:

→ [**Vanadis-ai/aicraftery**](https://github.com/Vanadis-ai/aicraftery) — releases, install scripts, updates.

Quick install:

**macOS**
```
curl -fsSL https://releases.aicraftery.com/install-mac.sh | bash
```

**Linux**
```
curl -fsSL https://releases.aicraftery.com/install-linux.sh | bash
```

Old `releases.vedissa.com` URLs and old `vedissa-bord-*` artifacts on this repo's Releases tab are kept for archival reference, but new versions ship only at `aicraftery.com`.

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

## marketplace

Vanadis's Claude Code plugins, registered as a marketplace. Add it to your Claude Code:

```
/plugin marketplace add https://github.com/Vanadis-ai/vanadis-open-bord/tree/main/marketplace
```

Browse the catalog in [marketplace/README.md](./marketplace/README.md).

---

## Links

- [AI Craftery (new home for Bord)](https://github.com/Vanadis-ai/aicraftery)
- [aicraftery.com](https://aicraftery.com) — product site
- [Vanadis](https://vanadis.ai) — the brand
- [Issues](https://github.com/Vanadis-ai/vanadis-open-bord/issues) — bug reports and feature requests for `amail` and the marketplace
