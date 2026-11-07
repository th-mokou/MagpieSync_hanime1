# MagpieSync Hanime1 插件

为 [MagpieSync](https://github.com/kamisangk/MagpieSync) 提供 Hanime1 站点支持的编译期插件。

本仓库存放插件目录中的源码和资源，依赖 MagpieSync 的内部接口，需要放入主项目中编译，不能作为独立应用运行。

## 功能

- 使用 Cookie 登录，支持配置浏览器 User-Agent 和代理。
- 只读同步喜欢的影片、稍后再看和播放列表。
- 解析站点官方视频下载链接。
- 提供独立的站点图标、清晰度选项和界面元数据。
- 通过主项目的驱动接口识别登录失效和站点验证错误。

## 安装与构建

将本仓库中的文件（不包括 `.git` 目录）复制到兼容版本 MagpieSync 的 `internal/sites/plugins/hanime1/` 目录，然后在主项目根目录执行：

```sh
go generate ./internal/sites/all
go test ./internal/sites/plugins/hanime1/... -count=1
go test ./internal/sites -count=1
go build ./cmd/magpiesync
```

生成的插件注册文件属于主项目，不在本插件仓库中维护。

使用构建标签 `no_hanime1` 可以排除该插件；使用 `explicit_sites,site_hanime1` 可以在白名单构建中选择该插件。例如：

```sh
go build -tags "explicit_sites,site_hanime1" ./cmd/magpiesync
```

## 登录说明

在 MagpieSync 中选择 Hanime1，粘贴已登录浏览器的完整 Cookie，并填写与该浏览器一致的 User-Agent。需要代理时，请使用与获取 Cookie 时相同的出口 IP。

本插件不会自动完成浏览器验证。Cookie 过期或遇到站点验证时，请在浏览器中完成验证后重新复制 Cookie；需要时同时提供有效的 `cf_clearance`。

## 测试与兼容性

插件引用主项目的 `internal` 包，因此不单独提供 `go.mod`，测试需要在 MagpieSync 内运行。

当前源码已在 MagpieSync 提交 `2410cb3` 的环境中通过插件测试和静态检查：

```sh
go test ./internal/sites/plugins/hanime1/... -count=1
go vet ./internal/sites/plugins/hanime1/...
```

## 许可证

采用 Apache License 2.0，详见 [LICENSE](LICENSE)。
