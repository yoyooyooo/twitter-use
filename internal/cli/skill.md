# twitter

Read and write X with the `twitter` binary. Do not scrape the website, and do not invent a browser login, for the commands below.

Run `twitter skill` again after upgrading. This text is the guide for the binary you just executed.

## For You is already one account

`twitter home` is the For You timeline of the current identity. `twitter home -t following` is the following timeline. Bookmarks, writes, `whoami`, and `posts`, `likes`, `user`, `followers`, and `following` with no screen name use that same identity. They do not enter the public pool and they do not switch accounts when X rate-limits them.

You do not pass `--as primary` for this. `primary: true` in `~/.twitter-use/accounts.json` is already the default identity.

Pass `--as <alias|label>` only to pin a different account for this one command, or to override cookies that are hiding `primary`. `--as` does not rotate and does not edit the account file. The 12-hex label and the optional alias are the only names. There is no numeric index.

The identity is the first match:

1. `--as`
2. `TWITTER_AUTH_TOKEN` and `TWITTER_CT0` both set. Either one alone does nothing. Together they hide `primary`. Unset both for normal use.
3. The account with `primary: true`
4. The label in `~/.twitter-use/identity`, then a browser session that returns a screen name

`twitter whoami` prints the screen name. It does not say which rule above won. A stderr line that starts with `Using primary account` is a log line, not a result field.

## Public reads use the pool

`search`, `tweet`, `article`, and `user`, `posts`, `followers`, `following`, and `likes` with a screen name rotate only accounts marked `pool: true`. The primary account stays out of that rotation unless its own `pool` field is true. One pooled account is a valid pool. Do not change accounts yourself to dodge a limit. The pool switches only after HTTP 429, JSON code 88, or an auth failure, then retries the same page.

## Pages

`-n` is the maximum number of items. The default is 50. A value above 500 fails before a request. `--cursor` resumes from `pagination.nextCursor`. Lists always include `pagination.stop` and `pagination.nextCursor`. `since_reached` means the local `--since` bound, not that X ran out of posts.

`--since` and `--until` take `YYYY-MM-DD` or RFC3339. `search` sends dates to X. Other timelines apply the lower bound locally.

Stdout that is not a terminal is JSON. On a terminal, add `--json`. Do not parse the human table.

`twitter tweet <id>` returns `focal`, `ancestors`, and `replies`. Ancestors run from the root to the parent. Missing posts are not invented. Each post keeps `inReplyToStatusId`, `inReplyToUserId`, `conversationId`, `quotedStatusId`, and `selfThreadId` when X sent them. `--ranking` is `Relevance` or `Recency`. Folded replies are followed through a `ShowMoreThreads` cursor. `pagination` can still stop while X reports more replies. Do not compute a coverage ratio from that count.

`posts --with-replies` includes replies written by that account. Without the flag, `posts` omits them.

`search -t` is `Top`, `Latest`, `Photos`, or `Videos`. It also takes `--from`, `--to`, `--lang`, `--has`, `--exclude`, `--min-likes`, and `--min-retweets`.

## Do not

There is no command to post, reply, quote, or delete. Do not print `auth_token`, `ct0`, or the raw account file. Do not call the pool with extra requests to prove that it switches. Do not add a time slice to get past a short search page; that did not return more unique posts.
