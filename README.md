<div align="center">
  <a href="https://github.com/kun4399/BiliShareMall" target="_blank">
    <img width="200" src="build/appicon.png" alt="logo">
  </a>
  <h1 id="koishi">BiliShareMall</h1>

![GitHub all releases](https://img.shields.io/github/downloads/kun4399/BiliShareMall/total)
![GitHub release (with filter)](https://img.shields.io/github/v/release/kun4399/BiliShareMall)
![GitHub issues](https://img.shields.io/github/issues/kun4399/BiliShareMall)
![GitHub Repo stars](https://img.shields.io/github/stars/kun4399/BiliShareMall)

</div>

图形化 B 站会员购魔力赏市集爬虫/搜索工具，现已同时支持桌面端与网页端（Wails + Go + Vue3）。

## 快速安装

下载最新 release（按系统选择安装包）：
[https://github.com/kun4399/BiliShareMall/releases](https://github.com/kun4399/BiliShareMall/releases)

Windows 推荐使用 `installer.exe` 结尾的安装包。

## 运行形态

- 桌面端：继续通过 Wails 构建本地桌面 app。
- 网页端：通过 Go HTTP 服务提供同一套前端页面与 API，默认监听 `3754`，适合远程服务器部署。

## 环境要求

- Go `>= 1.23`（与当前 `go.mod` 保持一致）
- Node.js `>= 18.12`
- pnpm `>= 8.7`
- Wails CLI v2（仅桌面端开发/构建需要，`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）

## 本地开发与编译

### 1. 安装前端依赖

```bash
pnpm --dir frontend install
```

### 2. 启动桌面端开发模式

```bash
wails dev -loglevel Info -tags fts5 -race
```

或使用：

```bash
make run
```

### 3. 启动网页端

生产方式运行网页端：

```bash
make run-web
```

本地联调模式（后端 `:3754` + Vite 前端开发服务器）：

```bash
make dev-web
```

网页端启动后，默认访问：

```text
http://127.0.0.1:3754
```

### 4. 运行测试与类型检查

```bash
go test ./...
go test ./internal/...
pnpm --dir frontend exec vue-tsc --noEmit
pnpm --dir frontend exec tsx --test src/gateway/runtime.test.ts
```

### 5. 一键构建校验（推荐）

```bash
./bin/build_verify.sh
```

若依赖已安装可加速：

```bash
./bin/build_verify.sh --skip-install
```

### 6. 生产构建

桌面端：

```bash
wails build -tags fts5
```

Windows 需要 NSIS 安装包时：

```bash
wails build -tags fts5 -nsis
```

网页端：

```bash
make build-web
```

## 构建产物路径

- macOS: `build/bin/BiliShareMall.app`
- Windows: `build/bin/BiliShareMall.exe`
- Linux 桌面端: `build/bin/BiliShareMall`
- 网页端服务: `build/bin/BiliShareMallWeb`

## 服务端环境变量

- `BSM_HTTP_ADDR`：网页端监听地址，默认 `:3754`
- `BSM_DATA_DIR`：数据目录覆盖路径
- `BSM_BASE_PATH`：资源根目录覆盖路径，调试或自定义部署目录时可用
- `BSM_WEB_ROOT`：网页静态资源目录覆盖路径，默认自动查找 `frontend/dist`

## 常见问题

### 1) 搜索筛选范围无结果

当前 B 站接口会对价格/折扣等筛选进行服务端校验，建议按照接口提供的筛选范围使用。

### 2) 程序使用问题 / bug 反馈

- 使用讨论：<https://github.com/kun4399/BiliShareMall/discussions>
- 提交 bug 或需求：<https://github.com/kun4399/BiliShareMall/issues/new/choose>

### 3) Linux / 远程服务器部署说明

- 当前网页端已支持 Linux 运行。
- Web API 与前端静态资源由同一个 Go 进程提供。
- 浏览器端会像桌面端一样保存 B 站 cookie，并通过 `X-Bili-Cookie` 请求头传给后端需要登录态的接口。

## 维护信息

- 仓库维护者：<https://github.com/kun4399>
