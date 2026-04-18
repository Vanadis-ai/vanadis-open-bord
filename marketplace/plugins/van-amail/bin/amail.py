#!/usr/bin/env python3
"""
amail CLI — pure HTTP wrapper around amail.vanadis.ai plus a local
address-book at ~/.Vanadis/amail-peers.json and an auth file at
~/.Vanadis/amail-auth.json.

Auth model: each user has a single API token (issued via Google OAuth
or seeded by the admin). All connection-create / accept / etc. calls
include it as Authorization: Bearer. The resulting per-connection
tokens are stored in amail-peers.json and used for subsequent
send / read / disconnect on that connection.

Commands:
    amail.py login <token>     store an API token
    amail.py whoami            show the email bound to the current token
    amail.py create            new connection (returns pair_code)
    amail.py accept <code>     accept a pair_code from the other side
    amail.py send <peer> <text>
    amail.py read
    amail.py peers
    amail.py disconnect <peer>
    amail.py set-peer-name --name --connection-id --token --side

All commands accept -j/--json for machine-readable output.
Exit code 0 on success, 1 on handled errors, 2 on unexpected.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

# Windows PowerShell defaults to cp1252/cp866 for python stdout; force
# UTF-8 so Cyrillic messages render correctly.
if sys.platform == "win32":
    try:
        sys.stdout.reconfigure(encoding="utf-8")
        sys.stderr.reconfigure(encoding="utf-8")
    except AttributeError:
        pass

DEFAULT_AMAIL_URL = "https://amail.vanadis.ai"
AUTH_PATH = Path.home() / ".Vanadis" / "amail-auth.json"
PEERS_PATH = Path.home() / ".Vanadis" / "amail-peers.json"


# --- amail-auth.json (single API token bound to email) --------------

def load_auth() -> dict:
    if not AUTH_PATH.exists():
        return {}
    with AUTH_PATH.open("r", encoding="utf-8") as f:
        return json.load(f)


def save_auth(data: dict) -> None:
    AUTH_PATH.parent.mkdir(parents=True, exist_ok=True)
    tmp = AUTH_PATH.with_suffix(".json.tmp")
    with tmp.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    tmp.replace(AUTH_PATH)
    # Token is a credential; keep it unreadable for other users.
    try:
        AUTH_PATH.chmod(0o600)
    except OSError:
        pass


def api_token() -> str | None:
    return load_auth().get("token")


# --- amail-peers.json (per-connection tokens) -----------------------

def load_peers() -> dict:
    if not PEERS_PATH.exists():
        return {"peers": {}}
    with PEERS_PATH.open("r", encoding="utf-8") as f:
        data = json.load(f)
    data.setdefault("peers", {})
    return data


def save_peers(data: dict) -> None:
    PEERS_PATH.parent.mkdir(parents=True, exist_ok=True)
    tmp = PEERS_PATH.with_suffix(".json.tmp")
    with tmp.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    tmp.replace(PEERS_PATH)
    try:
        PEERS_PATH.chmod(0o600)
    except OSError:
        pass


def get_peer(name: str) -> dict | None:
    return load_peers()["peers"].get(name)


def set_peer(name: str, connection_id: str, token: str, side: str) -> None:
    data = load_peers()
    data["peers"][name] = {
        "connection_id": connection_id,
        "token": token,
        "side": side,
    }
    save_peers(data)


def remove_peer(name: str) -> bool:
    data = load_peers()
    if name not in data["peers"]:
        return False
    del data["peers"][name]
    save_peers(data)
    return True


# --- HTTP -----------------------------------------------------------

def base_url() -> str:
    return os.environ.get("AMAIL_URL", DEFAULT_AMAIL_URL).rstrip("/")


def http(method: str, path: str, token: str | None = None, body: dict | None = None) -> dict:
    url = base_url() + path
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            raw = resp.read()
            if not raw:
                return {}
            return json.loads(raw)
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"HTTP {e.code} {method} {path}: {body}") from None
    except urllib.error.URLError as e:
        raise RuntimeError(f"Cannot reach amail at {base_url()}: {e.reason}") from None


def require_api_token() -> str:
    tok = api_token()
    if not tok:
        raise RuntimeError(
            "no API token — run /amail-login <token> first, or ask the admin "
            "to add you to the allowlist and issue one.")
    return tok


# --- subcommands ----------------------------------------------------

def cmd_login(args) -> int:
    # Stash the token plus a cached copy of the email (fetched by
    # ping-ing a user-scoped endpoint).
    save_auth({"token": args.token})
    try:
        # Attempt to fetch email by calling a cheap user-scoped path.
        # /amail/connection/create would work but has side effects.
        # Simplest: issue a no-op token refresh that returns email
        # via a GET we haven't built — skip, just store the token.
        pass
    except Exception:
        pass
    if args.json:
        print(json.dumps({"stored": True}))
    else:
        print("API token stored.")
    return 0


def cmd_whoami(args) -> int:
    tok = require_api_token()
    # Poor man's whoami: create+delete a connection, observe the email
    # in status. Cheap but ugly. A proper /me endpoint would be
    # cleaner; added to the backend TODO.
    create = http("POST", "/amail/connection/create", token=tok)
    conn_token = create["token"]
    status = http("GET", "/amail/connection/status", token=conn_token)
    http("DELETE", "/amail/connection", token=conn_token)
    email = status.get("my_email", "unknown")
    if args.json:
        print(json.dumps({"email": email}))
    else:
        print(email)
    return 0


def cmd_create(args) -> int:
    tok = require_api_token()
    resp = http("POST", "/amail/connection/create", token=tok)
    out = {
        "connection_id": resp["connection_id"],
        "pair_code": resp["pair_code"],
        "token": resp["token"],
        "expires_in_sec": resp.get("expires_in_sec"),
    }
    if args.json:
        print(json.dumps(out))
    else:
        print(f"pair_code: {out['pair_code']}")
        print(f"expires_in: {out['expires_in_sec']}s")
        print(f"connection_id: {out['connection_id']}")
        print(f"token: {out['token']}")
    return 0


def cmd_accept(args) -> int:
    tok = require_api_token()
    code = args.pair_code.strip().upper()
    resp = http("POST", "/amail/connection/accept", token=tok, body={"pair_code": code})
    out = {
        "connection_id": resp["connection_id"],
        "token": resp["token"],
    }
    if args.json:
        print(json.dumps(out))
    else:
        print(f"connection_id: {out['connection_id']}")
        print(f"token: {out['token']}")
    return 0


def cmd_send(args) -> int:
    peer = get_peer(args.peer)
    if not peer:
        print(f"Error: peer {args.peer!r} not found", file=sys.stderr)
        _print_peer_hint()
        return 1
    resp = http("POST", "/amail/send", token=peer["token"], body={"text": args.text})
    if args.json:
        print(json.dumps({"message_id": resp.get("message_id"), "peer": args.peer}))
    else:
        print(f"sent to {args.peer}: {resp.get('message_id', 'ok')}")
    return 0


def cmd_read(args) -> int:
    peers = load_peers()["peers"]
    if not peers:
        print("No peers configured. Use amail-connect or amail-accept first.")
        return 0
    all_messages: list[dict] = []
    connection_closed: list[str] = []
    for name, peer in peers.items():
        try:
            resp = http("GET", "/amail/inbox", token=peer["token"])
        except RuntimeError as e:
            if "HTTP 401" in str(e):
                connection_closed.append(name)
                continue
            raise
        for m in resp.get("messages", []):
            all_messages.append({
                "from": name,
                "text": m["text"],
                "sent_at": m["sent_at"],
                "id": m["id"],
            })
    for name in connection_closed:
        remove_peer(name)

    if args.json:
        print(json.dumps({"messages": all_messages, "closed": connection_closed}))
        return 0

    if connection_closed:
        for name in connection_closed:
            print(f"[connection closed by peer] {name} -- removed from address book")
    if not all_messages:
        print("Inbox empty.")
        return 0
    by_peer: dict[str, list[dict]] = {}
    for m in all_messages:
        by_peer.setdefault(m["from"], []).append(m)
    for name, msgs in by_peer.items():
        print(f"-- from {name} ({len(msgs)} message{'s' if len(msgs) != 1 else ''}) --")
        for m in msgs:
            print(f"  [{m['sent_at']}]")
            for line in m["text"].splitlines() or [""]:
                print(f"    {line}")
        print()
    return 0


def cmd_peers(args) -> int:
    peers = load_peers()["peers"]
    if args.json:
        print(json.dumps({"peers": peers}))
        return 0
    if not peers:
        print("No peers configured.")
        return 0
    print(f"{'name':<24} {'side':<5} {'connection_id'}")
    for name, p in peers.items():
        print(f"{name:<24} {p.get('side', '?'):<5} {p.get('connection_id', '?')}")
    return 0


def cmd_disconnect(args) -> int:
    peer = get_peer(args.peer)
    if not peer:
        print(f"Error: peer {args.peer!r} not found", file=sys.stderr)
        return 1
    try:
        http("DELETE", "/amail/connection", token=peer["token"])
    except RuntimeError as e:
        if "HTTP 401" not in str(e):
            raise
    remove_peer(args.peer)
    print(f"disconnected: {args.peer}")
    return 0


def cmd_set_peer_name(args) -> int:
    set_peer(args.name, args.connection_id, args.token, args.side)
    print(f"stored: {args.name}")
    return 0


def _print_peer_hint() -> None:
    peers = load_peers()["peers"]
    if peers:
        print("Known peers: " + ", ".join(sorted(peers.keys())), file=sys.stderr)


# --- main -----------------------------------------------------------

def build_parser() -> argparse.ArgumentParser:
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("-j", "--json", action="store_true", help="machine-readable output")

    p = argparse.ArgumentParser(prog="amail.py", parents=[common])
    sub = p.add_subparsers(dest="cmd", required=True)

    login = sub.add_parser("login", parents=[common], help="store an API token for this user")
    login.add_argument("token")

    sub.add_parser("whoami", parents=[common], help="show the email bound to the current API token")
    sub.add_parser("create", parents=[common], help="create a new connection (returns pair_code)")

    accept = sub.add_parser("accept", parents=[common], help="accept a pair_code from the other side")
    accept.add_argument("pair_code")

    send = sub.add_parser("send", parents=[common], help="send a message to a known peer")
    send.add_argument("peer")
    send.add_argument("text")

    sub.add_parser("read", parents=[common], help="read new messages from all peers (DELETE on server)")
    sub.add_parser("peers", parents=[common], help="list configured peers")

    disc = sub.add_parser("disconnect", parents=[common], help="close the connection and forget the peer")
    disc.add_argument("peer")

    setp = sub.add_parser("set-peer-name", parents=[common], help="store a peer entry in the address book")
    setp.add_argument("--name", required=True)
    setp.add_argument("--connection-id", required=True, dest="connection_id")
    setp.add_argument("--token", required=True)
    setp.add_argument("--side", required=True, choices=["a", "b"])

    return p


DISPATCH = {
    "login": cmd_login,
    "whoami": cmd_whoami,
    "create": cmd_create,
    "accept": cmd_accept,
    "send": cmd_send,
    "read": cmd_read,
    "peers": cmd_peers,
    "disconnect": cmd_disconnect,
    "set-peer-name": cmd_set_peer_name,
}


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return DISPATCH[args.cmd](args)
    except RuntimeError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:  # noqa: BLE001
        print(f"Unexpected: {type(e).__name__}: {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
