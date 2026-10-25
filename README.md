<h1 align="center">twitter-use</h1>
<p align="center">Read and act on X from the terminal, with public reads separated from the account you write as.</p>
<p align="center">
  <a href="./README.md">English</a> · <a href="./README.zh-CN.md">简体中文</a>
</p>
<p align="center">
  <a href="#install">Install</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#commands">Commands</a> ·
  <a href="#privacy">Privacy</a> ·
  <a href="#limits">Limits</a>
</p>

An agent can page through public posts without spending the login that likes, bookmarks, and follows. Public reads rotate through a pool. Home, bookmarks, your own likes, and writes stay on one identity.

A successful search looks like this. The list always includes why it stopped.

```json
{
  "ok": true,
  "schema_version": "1",
  "data": [{ "id": "…", "text": "…" }],
  "pagination": { "stop": "max_count", "nextCursor": "…" }
}
```

`stop` is `max_count` when `-n` is reached. `nextCursor` is present even when it is empty. Pass it back as `--cursor` to continue. A stop of `since_reached` means a local time bound ended the page, not that X ran out of posts.

## Install

Download the latest static binary. The script resolves the GitHub Release, checks `SHA256SUMS`, then writes `~/.local/bin/twitter`. Set `GH_TOKEN` or `GITHUB_TOKEN` to resolve the tag through the releases API.

```bash
curl -fsSL https://raw.githubusercontent.com/yoyooyooo/twitter-use/main/install.sh | sh
```

Pin a release with `sh -s -- v0.1.3`. `TWITTER_INSTALL_DIR` changes the destination. Supported files are `twitter-linux-amd64`, `twitter-linux-arm64`, `twitter-darwin-amd64`, and `twitter-darwin-arm64`.

From a checkout, with Go 1.27.2 or newer:

```bash
go install ./cmd/twitter
```

`go install` names the binary `twitter` and puts it in `GOBIN`, or in `$(go env GOPATH)/bin` when `GOBIN` is empty. That directory has to be on `PATH`.

## Quick start

Create `~/.twitter-use/accounts.json` with mode `600`. One account may be `primary`; it is the default for home, bookmarks, and writes, and it stays out of public rotation unless `pool` is true. Other accounts default to the pool. `alias` is an optional short name. Do not commit this file.

```bash
install -d -m 700 ~/.twitter-use
install -m 600 /dev/null ~/.twitter-use/accounts.json
```

```json
{
  "version": 1,
  "accounts": [
    {"alias": "main", "primary": true, "auth_token": "...", "ct0": "..."},
    {"alias": "spare", "auth_token": "...", "ct0": "..."}
  ]
}
```

`twitter --as spare search golang` uses that account for one command and does not switch on a rate limit. `twitter --as main home` reads that account's recommendations. Two primaries, or two identical aliases, fail at load. A legacy `accounts.txt` is used only when `accounts.json` is absent.

Then read one public post:

```bash
twitter search golang -t Latest -n 1 --json
```

Success is exit code 0 and `"ok": true`. With `-n 1` and a result, `pagination.stop` is `max_count`.

If that file is missing and `TWITTER_ACCOUNTS_FILE` is unset, a public read uses one login instead of a pool. Set `TWITTER_AUTH_TOKEN` and `TWITTER_CT0`, or let the command try a browser session on this machine. A headless environment cannot unlock a macOS keychain.

### Read-only home directories

For a container or cloud workspace with a read-only home directory, set both pool paths to a writable private directory outside the checkout. Replace `/path/to/private/twitter-use` below with your directory:

```bash
install -d -m 700 /path/to/private/twitter-use
export TWITTER_ACCOUNTS_FILE=/path/to/private/twitter-use/accounts.json
export TWITTER_ACCOUNT_STATE=/path/to/private/twitter-use/account_pool.json
touch "$TWITTER_ACCOUNTS_FILE"
chmod 600 "$TWITTER_ACCOUNTS_FILE"
```

Put the JSON accounts into that file before running a command. Setting `TWITTER_ACCOUNTS_FILE` alone does not move the state file; its default remains `~/.twitter-use/account_pool.json`.

### Proxies

If `TWITTER_PROXY` is set, it overrides environment proxy discovery for every request. An explicitly empty value selects direct connections. Otherwise, HTTP requests use `HTTP_PROXY` / `http_proxy`, HTTPS requests use `HTTPS_PROXY` / `https_proxy`, and `NO_PROXY` / `no_proxy` can bypass the proxy for matching destinations. Without a matching proxy setting, connections are direct.

Environment discovery uses Go's `httpproxy` rules, including hostname, port, IP/CIDR, and `*` bypass entries. `NO_PROXY` applies only to environment discovery, not an explicit `TWITTER_PROXY`. Redirect destinations are evaluated separately. Invalid proxy URLs fail with a configuration error rather than silently falling back to a direct connection.

To explicitly select a proxy, use a URL with a scheme and host:

```bash
export TWITTER_PROXY=http://proxy.example:8080
```

Standard proxy environment values also accept `host:port`. Proxy credentials may be included in the URL; do not commit or log them.

## Agents

Run `twitter skill` before the first command. It prints the account and pagination rules for this binary. For You is `twitter home`; the primary account is already selected, and `--as` is only an override.

## Commands

Public reads rotate through accounts with `pool: true` when an account file exists: `search`, `user`, `posts`, `tweet`, `article`, `followers`, `following`. `likes` uses the pool only when you pass a screen name.

These use one selected account and do not rotate: `whoami`, `home`, `bookmarks`, `likes` with no screen name, `like`, `unlike`, `bookmark`, `unbookmark`, `retweet`, `unretweet`, `follow`, `unfollow`.

Omit the screen name on `user`, `posts`, `followers`, `following`, and `likes` to mean the current identity. A name on those commands uses the pool.

The account for those commands is the first of: `--as`; `TWITTER_AUTH_TOKEN` and `TWITTER_CT0` together; the account with `primary: true`; the label stored in `~/.twitter-use/identity`; a browser session that actually returns a screen name. The chosen browser label is remembered. A primary account is not used for public rotation unless its `pool` field is true. Leave the two variables unset for normal use, or they hide the primary account.

Shared list flags are `-n` and `--cursor`. Post timelines also take `--since` and `--until` (`YYYY-MM-DD` or RFC3339). `search` sends those dates to X as `since:` and `until:`. Other timelines stop locally at the lower bound. `-n` above 500 fails before any request. When stdout is not a terminal, the default output is JSON and text is not shortened.

`TWITTER_ACCOUNTS_FILE` and `TWITTER_ACCOUNT_STATE` override the pool paths. `TWITTER_BROWSER` chooses which browser to try first. `TWITTER_PROXY` is an optional proxy.

## Privacy

The command talks to X, and it may download public web scripts from `abs.twimg.com` to build a client transaction id. If that id cannot be built, the request still goes out without the header. Query ids that return 404 can be refreshed from the public [twitter-openapi](https://github.com/fa0311/twitter-openapi) placeholder.

Local files:

| Path | What it holds |
| --- | --- |
| `~/.twitter-use/accounts.json` | Account tokens, aliases, and roles. Mode `600`. |
| `~/.twitter-use/accounts.txt` | Legacy pool lines, used only when the JSON file is absent. |
| `~/.twitter-use/account_pool.json` | SHA-256 labels and cooldowns. Not the tokens. |
| `~/.twitter-use/identity` | Label of the browser login last chosen, when no primary is set. |

On macOS the browser path can read cookies through the Keychain. On Linux it uses `secret-tool`. Windows app-bound cookie decryption is not implemented; use the environment variables there. Tokens are sent only to X.

Bootstrap requests attach cookies only to HTTPS `x.com` or `twitter.com`. Redirects outside these hosts, to HTTP, or to a nonstandard port drop credential headers. API error messages redact known login and cookie values before truncation or display.

## Limits

`twitter tweet` returns `focal`, `ancestors`, and `replies`, and follows folded thread cursors. Each post includes reply and conversation ids when X sent them. `posts --with-replies` includes that account's replies. There is no command to post, reply, quote, or delete. List timelines are not implemented. A fetch stops at 500 items. Replies on a tweet can stop while X still reports a higher reply count; `pagination` says so instead of pretending the thread is complete. The TLS profile is Chrome 133 over HTTP/2 through [tls-client](https://github.com/bogdanfinn/tls-client). It is not a byte-for-byte browser impersonation.

## Development

```bash
go test ./...
```

Contribution expectations are in [CONTRIBUTING.md](./CONTRIBUTING.md). Report a vulnerability through [SECURITY.md](./SECURITY.md). The change history is [CHANGELOG.md](./CHANGELOG.md).
