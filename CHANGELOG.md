# Changelog

## 0.1.0

First public release. A `v*` tag publishes static binaries for Linux and macOS on amd64 and arm64.

Public reads: `search`, `user`, `posts`, `tweet`, `article`, `followers`, `following`, and `likes` when a screen name is given. These use `~/.twitter-use/accounts.txt` when that file exists.

Identity commands: `whoami`, `home`, `bookmarks`, `likes` without a screen name, and the reversible writes `like`, `unlike`, `bookmark`, `unbookmark`, `retweet`, `unretweet`, `follow`, `unfollow`.

Posting, replies, quotes, deletes, and list timelines are not part of this surface. `-n` above 500 fails. List results include `pagination.stop` and `pagination.nextCursor`. Non-terminal stdout defaults to JSON.
