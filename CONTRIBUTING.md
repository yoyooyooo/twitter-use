# Contributing

The public facts live in [README.md](./README.md) and [README.zh-CN.md](./README.zh-CN.md). Change a command, default, or limit in both files in the same change. Do not leave one language behind.

## Setup

Go 1.27.2 or newer.

```bash
go test ./...
gofmt -w .
```

`go test ./...` is the check that must pass. It does not call X and does not read `~/.twitter-use`.

## Secrets

Do not commit `accounts.txt`, cookies, `auth_token`, or `ct0`. The pool file belongs in `~/.twitter-use/` with mode `600`. The state file may contain only SHA-256 labels and timestamps.

A pull request should say what a caller can observe, and include a test when the change is a command name, a pagination stop, or an account-selection rule.
