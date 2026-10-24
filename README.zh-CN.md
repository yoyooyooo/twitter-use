<h1 align="center">twitter-use</h1>
<p align="center">在终端里读 X、做少量账号操作。公开读和用来写的那个身份是分开的。</p>
<p align="center">
  <a href="./README.md">English</a> · <a href="./README.zh-CN.md">简体中文</a>
</p>
<p align="center">
  <a href="#安装">安装</a> ·
  <a href="#第一次成功">第一次成功</a> ·
  <a href="#命令">命令</a> ·
  <a href="#隐私">隐私</a> ·
  <a href="#限制">限制</a>
</p>

Agent 翻公开帖子时，不必消耗那个用来点赞、加书签和关注的登录。公开读在号池里轮换。首页、书签、自己的点赞和写操作固定走一个身份。

一次成功的搜索是这样的。列表都会说明为什么停下。

```json
{
  "ok": true,
  "schema_version": "1",
  "data": [{ "id": "…", "text": "…" }],
  "pagination": { "stop": "max_count", "nextCursor": "…" }
}
```

`-n` 达到上限时 `stop` 是 `max_count`。`nextCursor` 即使为空也会出现。继续读时把它传给 `--cursor`。`since_reached` 表示本地时间下界到了，不是 X 没有更多帖子。

## 安装

需要 Go 1.27.2 或更新。用 `go version` 确认。

在本模块的检出目录里：

```bash
go install ./cmd/twitter
```

二进制名字是 `twitter`。`go install` 把它放到 `GOBIN`；`GOBIN` 为空时放到 `$(go env GOPATH)/bin`。这个目录需要在 `PATH` 里。

模块可以从 `github.com/yoyooyooo/twitter-use` 拉取时：

```bash
go install github.com/yoyooyooo/twitter-use/cmd/twitter@latest
```

这条命令要求仓库已经在该模块路径上。本地检出不依赖它。

没有 Go 的机器可以用 `v*` GitHub Release 上的二进制。文件名是 `twitter-linux-amd64`、`twitter-linux-arm64`、`twitter-darwin-amd64`、`twitter-darwin-arm64`，以及 `SHA256SUMS`。核对校验和之后，把对应文件放到 `PATH` 里并命名为 `twitter`。

## 第一次成功

创建 `~/.twitter-use/accounts.txt`，权限 `600`。每行一个账号，`auth_token` 和 `ct0` 用空格分开。一行也是合法号池。不要把这个文件提交进仓库。

```bash
install -d -m 700 ~/.twitter-use
install -m 600 /dev/null ~/.twitter-use/accounts.txt
```

然后读一条公开帖子：

```bash
twitter search golang -t Latest -n 1 --json
```

成功是退出码 0，并且 `"ok": true`。`-n 1` 且有结果时，`pagination.stop` 是 `max_count`。

这个文件不存在，且没有设置 `TWITTER_ACCOUNTS_FILE` 时，公开读改用一个登录，而不是号池。设置 `TWITTER_AUTH_TOKEN` 和 `TWITTER_CT0`，或者让命令在这台机器上尝试浏览器会话。没有界面的环境解不开 macOS 钥匙串。

## 命令

`~/.twitter-use/accounts.txt` 存在时，这些公开读走号池：`search`、`user`、`posts`、`tweet`、`article`、`followers`、`following`。`likes` 只有写了用户名才走号池。

这些永远走当前身份，不进号池：`whoami`、`home`、`bookmarks`、不带用户名的 `likes`，以及 `like`、`unlike`、`bookmark`、`unbookmark`、`retweet`、`unretweet`、`follow`、`unfollow`。

`user`、`posts`、`followers`、`following`、`likes` 省略用户名时，用当前身份。写了用户名就走号池。

身份按这个顺序选取：`TWITTER_AUTH_TOKEN` 和 `TWITTER_CT0`；`~/.twitter-use/identity` 里记下的标签；一个确实能返回用户名的浏览器会话。选中的浏览器标签会被记住。号池不是身份来源。

列表共用 `-n` 和 `--cursor`。帖子时间线还有 `--since` 和 `--until`（`YYYY-MM-DD` 或 RFC3339）。`search` 把日期交给 X 的 `since:` / `until:`。其它时间线在本地碰到下界就停。`-n` 超过 500 会在发请求前失败。标准输出不是终端时，默认 JSON，正文不截断。

`TWITTER_ACCOUNTS_FILE` 和 `TWITTER_ACCOUNT_STATE` 只覆盖号池路径。`TWITTER_BROWSER` 指定先试哪个浏览器。`TWITTER_PROXY` 是可选代理。

## 隐私

命令会访问 X，并可能从 `abs.twimg.com` 下载公开网页脚本来计算客户端事务号。算不出来就省略这个头，请求继续。返回 404 的查询号可能从公开的 [twitter-openapi](https://github.com/fa0311/twitter-openapi) 占位文件刷新。

本地文件：

| 路径 | 内容 |
| --- | --- |
| `~/.twitter-use/accounts.txt` | 号池令牌。权限 `600`。 |
| `~/.twitter-use/account_pool.json` | SHA-256 标签和冷却时间。没有令牌。 |
| `~/.twitter-use/identity` | 上次选中的浏览器登录标签。 |

macOS 上，浏览器路径可能通过钥匙串读取 Cookie。Linux 使用 `secret-tool`。Windows 的应用绑定 Cookie 解密没有实现，请用环境变量。令牌只发给 X。

## 限制

没有发帖、回复、引用或删除。列表时间线没有实现。一次获取最多 500 条。一条帖子的回复可能在 X 报告的回复数还更高时停下；`pagination` 会说明这一点，而不是假装线程已经完整。TLS 使用 [tls-client](https://github.com/bogdanfinn/tls-client) 的 Chrome 133、HTTP/2。它不是与浏览器逐字节相同的伪装。

## 开发

```bash
go test ./...
```

贡献约定在 [CONTRIBUTING.md](./CONTRIBUTING.md)。漏洞报告见 [SECURITY.md](./SECURITY.md)。变更记录是 [CHANGELOG.md](./CHANGELOG.md)。
