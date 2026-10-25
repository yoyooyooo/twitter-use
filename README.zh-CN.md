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

直接下载最新的静态二进制。脚本先确定 GitHub Release，核对 `SHA256SUMS` 之后再写入 `~/.local/bin/twitter`。设置 `GH_TOKEN` 或 `GITHUB_TOKEN` 时，版本号改走 releases API。

```bash
curl -fsSL https://raw.githubusercontent.com/yoyooyooo/twitter-use/main/install.sh | sh
```

固定版本用 `sh -s -- v0.1.3`。`TWITTER_INSTALL_DIR` 可以改安装目录。支持 `twitter-linux-amd64`、`twitter-linux-arm64`、`twitter-darwin-amd64`、`twitter-darwin-arm64`。

在检出目录里用 Go 1.27.2 或更新版本编译：

```bash
go install ./cmd/twitter
```

二进制名字是 `twitter`。`go install` 把它放到 `GOBIN`；`GOBIN` 为空时放到 `$(go env GOPATH)/bin`。这个目录需要在 `PATH` 里。

## 第一次成功

创建 `~/.twitter-use/accounts.json`，权限 `600`。可以有一个 `primary`，它是首页、书签和写入的默认身份，并且默认不参加公开轮换。其他账号默认进号池。`alias` 是可选短名。不要把这个文件提交进仓库。

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

`twitter --as spare search golang` 这一次只用该账号，限流也不换号。`twitter --as main home` 看该账号的推荐。两个主账号或两个相同短名会在加载时失败。只有没有 JSON 文件时，才读取旧的 `accounts.txt`。

然后读一条公开帖子：

```bash
twitter search golang -t Latest -n 1 --json
```

成功是退出码 0，并且 `"ok": true`。`-n 1` 且有结果时，`pagination.stop` 是 `max_count`。

这个文件不存在，且没有设置 `TWITTER_ACCOUNTS_FILE` 时，公开读改用一个登录，而不是号池。设置 `TWITTER_AUTH_TOKEN` 和 `TWITTER_CT0`，或者让命令在这台机器上尝试浏览器会话。没有界面的环境解不开 macOS 钥匙串。

### 主目录只读时

容器或云工作区的主目录只读时，把两个号池路径都指向仓库外可写的私有目录。将下面的 `/path/to/private/twitter-use` 换成自己的目录：

```bash
install -d -m 700 /path/to/private/twitter-use
export TWITTER_ACCOUNTS_FILE=/path/to/private/twitter-use/accounts.json
export TWITTER_ACCOUNT_STATE=/path/to/private/twitter-use/account_pool.json
touch "$TWITTER_ACCOUNTS_FILE"
chmod 600 "$TWITTER_ACCOUNTS_FILE"
```

先把账号 JSON 写入该文件，再执行命令。只设置 `TWITTER_ACCOUNTS_FILE` 不会迁移状态文件；状态文件仍默认使用 `~/.twitter-use/account_pool.json`。

### 代理

设置了 `TWITTER_PROXY` 时，所有请求都优先使用该配置，不再自动选择环境代理。显式设置为空字符串表示直连。未设置时，HTTP 请求使用 `HTTP_PROXY` / `http_proxy`，HTTPS 请求使用 `HTTPS_PROXY` / `https_proxy`，匹配 `NO_PROXY` / `no_proxy` 的目标绕过代理。没有匹配的代理配置时直连。

环境代理遵循 Go 的 `httpproxy` 规则，支持主机名、端口、IP/CIDR 和 `*` 绕过项。`NO_PROXY` 只作用于自动选择环境代理，不覆盖显式的 `TWITTER_PROXY`。重定向后的每个目标会重新判断。代理 URL 无效时返回配置错误，不会静默退回直连。

显式指定代理时，使用包含协议和主机的 URL：

```bash
export TWITTER_PROXY=http://proxy.example:8080
```

标准代理环境变量也接受 `host:port`。URL 可以包含代理认证信息，不要将其提交到仓库或写入日志。

## 命令

账号文件存在时，`pool` 为 true 的账号轮换处理这些公开读：`search`、`user`、`posts`、`tweet`、`article`、`followers`、`following`。`likes` 只有写了用户名才走号池。

这些命令固定一个账号，不轮换：`whoami`、`home`、`bookmarks`、不带用户名的 `likes`，以及 `like`、`unlike`、`bookmark`、`unbookmark`、`retweet`、`unretweet`、`follow`、`unfollow`。

`user`、`posts`、`followers`、`following`、`likes` 省略用户名时，用当前身份。写了用户名就走号池。

这些命令的账号按这个顺序选取：`--as`；同时设置的 `TWITTER_AUTH_TOKEN` 和 `TWITTER_CT0`；`primary: true` 的账号；`~/.twitter-use/identity` 里记下的标签；一个确实能返回用户名的浏览器会话。选中的浏览器标签会被记住。主账号除非显式 `pool: true`，否则不参加公开轮换。平时不要导出那两个变量，否则会盖住主账号。

列表共用 `-n` 和 `--cursor`。帖子时间线还有 `--since` 和 `--until`（`YYYY-MM-DD` 或 RFC3339）。`search` 把日期交给 X 的 `since:` / `until:`。其它时间线在本地碰到下界就停。`-n` 超过 500 会在发请求前失败。标准输出不是终端时，默认 JSON，正文不截断。

`TWITTER_ACCOUNTS_FILE` 和 `TWITTER_ACCOUNT_STATE` 只覆盖号池路径。`TWITTER_BROWSER` 指定先试哪个浏览器。`TWITTER_PROXY` 是可选代理。

## 隐私

命令会访问 X，并可能从 `abs.twimg.com` 下载公开网页脚本来计算客户端事务号。算不出来就省略这个头，请求继续。返回 404 的查询号可能从公开的 [twitter-openapi](https://github.com/fa0311/twitter-openapi) 占位文件刷新。

本地文件：

| 路径 | 内容 |
| --- | --- |
| `~/.twitter-use/accounts.json` | 账号令牌、短名和角色。权限 `600`。 |
| `~/.twitter-use/accounts.txt` | 旧号池行。只有没有 JSON 文件时才使用。 |
| `~/.twitter-use/account_pool.json` | SHA-256 标签和冷却时间。没有令牌。 |
| `~/.twitter-use/identity` | 没有主账号时，上次选中的浏览器登录标签。 |

macOS 上，浏览器路径可能通过钥匙串读取 Cookie。Linux 使用 `secret-tool`。Windows 的应用绑定 Cookie 解密没有实现，请用环境变量。令牌只发给 X。

初始化请求只向 HTTPS 的 `x.com` 或 `twitter.com` 附带 Cookie。重定向到其他主机、HTTP 或非标准端口时，会移除凭据请求头。API 错误信息在截断或显示之前，会将已知的登录凭据和 Cookie 值脱敏。

## 限制

`twitter tweet` 返回 `focal`、`ancestors` 和 `replies`，并会继续翻折叠的线程 cursor。X 如果带了回复和会话 id，每条帖子都会保留。`posts --with-replies` 包含这个账号自己的回复。没有发帖、回复、引用或删除。列表时间线没有实现。一次获取最多 500 条。一条帖子的回复可能在 X 报告的回复数还更高时停下；`pagination` 会说明这一点，而不是假装线程已经完整。TLS 使用 [tls-client](https://github.com/bogdanfinn/tls-client) 的 Chrome 133、HTTP/2。它不是与浏览器逐字节相同的伪装。

## 开发

```bash
go test ./...
```

贡献约定在 [CONTRIBUTING.md](./CONTRIBUTING.md)。漏洞报告见 [SECURITY.md](./SECURITY.md)。变更记录是 [CHANGELOG.md](./CHANGELOG.md)。
