# Hanime1 插件

Hanime1 是 MagpieSync 的站点插件，用于读取 Hanime1 账户中的影片列表并将影片加入 MagpieSync 下载队列。

## 功能

- 使用 Hanime1 Cookie 验证账户并同步影片。
- 同步喜欢的影片、稍后再看和自定义播放列表。
- 解析影片页面的可用直链，支持 1080P、720P、480P 和 240P。
- 图片请求使用 Hanime1 页面作为 Referer。

## 登录

插件使用 Cookie 登录。请在已登录 Hanime1 的浏览器中复制完整 Cookie，并在 MagpieSync 添加账户时粘贴。至少需要 `hanime1_session`；遇到 Cloudflare 验证时，可能还需要同一网络出口下取得的 `cf_clearance`。

登录时还需提供与生成 Cookie 的浏览器一致的 User-Agent。Cookie 和 User-Agent 会随账户凭据保存；不要将真实 Cookie、验证 Cookie 或账户凭据提交到代码仓库或公开日志。Cookie 失效后，请从浏览器重新复制并更新账户信息。

建议在网络环境需要时为 MagpieSync 配置可访问 Hanime1 的代理。Cloudflare 验证 Cookie 通常与获取时的出口 IP 相关。

## 在 MagpieSync 中启用

本目录是 MagpieSync 的编译期插件，不是独立运行的程序。将插件放入主项目的 `internal/sites/plugins/hanime1` 后，在 MagpieSync 仓库根目录执行：

```sh
go generate ./internal/sites/all
go test ./internal/sites/plugins/hanime1/...
```

默认构建会包含该插件。使用站点白名单构建时，可执行：

```sh
make build SITES="hanime1"
```

## 项目结构

- `register.go`、`login.go`、`source.go`：向 MagpieSync 注册站点能力、处理登录并创建同步与下载解析器。
- `core/`：Hanime1 客户端、页面解析、列表读取和下载链接解析实现。
- `assets/`：插件使用的站点图标。

## 开发测试

在 MagpieSync 主项目根目录运行插件测试：

```sh
go test ./internal/sites/plugins/hanime1/...
```

插件通过 `!no_hanime1 && (!explicit_sites || site_hanime1)` 构建约束控制是否编译。主项目的站点清单由 `go generate ./internal/sites/all` 生成，请勿手动编辑生成文件。
