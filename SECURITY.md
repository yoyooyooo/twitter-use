# Security

## Report a vulnerability

Use [GitHub private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) on `github.com/yoyooyooo/twitter-use`. Do not open a public issue for a token leak, a cookie-handling bug, or a way to make the pool use the identity login.

There is no separate security email. A report should wait for a maintainer response rather than assume a fix window.

## Scope

Supported code is the default branch. Account tokens in `~/.twitter-use/accounts.txt` and browser cookies are in scope when the tool stores, logs, or sends them somewhere other than X. The state file is supposed to keep SHA-256 labels only.

The public web client bearer compiled into the binary is X's published web token. It is not a user credential. Finding that string is not a vulnerability.
