# Changelog

## 0.1.1

Public reads honor `TWITTER_PROXY` and the standard proxy environment. An empty `TWITTER_PROXY` forces a direct connection. Invalid proxy URLs fail instead of falling through. Bootstrap requests drop credential headers when a redirect leaves HTTPS `x.com` or `twitter.com`. API errors redact known login and cookie values.

## 0.1.0

First public release. A `v*` tag publishes static binaries for Linux and macOS on amd64 and arm64.

Public reads: `search`, `user`, `posts`, `tweet`, `article`, `followers`, `following`, and `likes` when a screen name is given. These use `~/.twitter-use/accounts.txt` when that file exists.

Identity commands: `whoami`, `home`, `bookmarks`, `likes` without a screen name, and the reversible writes `like`, `unlike`, `bookmark`, `unbookmark`, `retweet`, `unretweet`, `follow`, `unfollow`.

Posting, replies, quotes, deletes, and list timelines are not part of this surface. `-n` above 500 fails. List results include `pagination.stop` and `pagination.nextCursor`. Non-terminal stdout defaults to JSON.
