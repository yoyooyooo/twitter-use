# Changelog

## Unreleased

`twitter skill` prints the agent guide for this binary. `SKILL.md` tells an agent to run that command, and carries the same guide when the installed binary is older.

## 0.1.3

`install.sh` downloads a GitHub Release binary, checks `SHA256SUMS`, and writes `~/.local/bin/twitter`. Pass a tag to pin one release.

`twitter tweet` keeps reply links that the GraphQL payload already contained: `inReplyToStatusId`, `inReplyToUserId`, `conversationId`, `quotedStatusId`, and `selfThreadId`. Its JSON data is `focal`, `ancestors`, and `replies` instead of one flat list. An unknown post id is `not_found`. Tweet detail also follows a `ShowMoreThreads` cursor after the bottom cursor ends. `posts --with-replies` uses `UserTweetsAndReplies`.

## 0.1.2

`~/.twitter-use/accounts.json` replaces the text pool file. Each account has `auth_token`, `ct0`, an optional `alias`, at most one `primary`, and `pool`. A primary defaults to staying out of public rotation. Other accounts default to the pool. Legacy `accounts.txt` still loads when the JSON file is absent, and every line stays pool-only.

`twitter --as <alias|label>` uses that account for one command and does not rotate or edit the file. Identity commands use `TWITTER_AUTH_TOKEN` and `TWITTER_CT0` when both are set, otherwise the primary account, otherwise a browser login. Public reads still rotate only through pooled accounts.

## 0.1.1

Public reads honor `TWITTER_PROXY` and the standard proxy environment. An empty `TWITTER_PROXY` forces a direct connection. Invalid proxy URLs fail instead of falling through. Bootstrap requests drop credential headers when a redirect leaves HTTPS `x.com` or `twitter.com`. API errors redact known login and cookie values.

## 0.1.0

First public release. A `v*` tag publishes static binaries for Linux and macOS on amd64 and arm64.

Public reads: `search`, `user`, `posts`, `tweet`, `article`, `followers`, `following`, and `likes` when a screen name is given. These use `~/.twitter-use/accounts.txt` when that file exists.

Identity commands: `whoami`, `home`, `bookmarks`, `likes` without a screen name, and the reversible writes `like`, `unlike`, `bookmark`, `unbookmark`, `retweet`, `unretweet`, `follow`, `unfollow`.

Posting, replies, quotes, deletes, and list timelines are not part of this surface. `-n` above 500 fails. List results include `pagination.stop` and `pagination.nextCursor`. Non-terminal stdout defaults to JSON.
