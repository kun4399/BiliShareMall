<div align="center">
  <a href="https://github.com/mikumifa/BiliShareMall" target="_blank">
    <img width="200" src="build/appicon.png" alt="logo">
  </a>
  <h1 id="koishi">BiliShareMall</h1>

![GitHub all releases](https://img.shields.io/github/downloads/mikumifa/BiliShareMall/total)
![GitHub release (with filter)](https://img.shields.io/github/v/release/mikumifa/BiliShareMall)
![GitHub issues](https://img.shields.io/github/issues/mikumifa/BiliShareMall)
![GitHub Repo stars](https://img.shields.io/github/stars/mikumifa/BiliShareMall)

</div>

图形化 B 站会员购魔力赏市集爬虫/搜索工具（Wails + Go + Vue3）。

## 快速安装

下载最新 release（按系统选择安装包）：
[https://github.com/mikumifa/BiliShareMall/releases](https://github.com/mikumifa/BiliShareMall/releases)

Windows 推荐使用 `installer.exe` 结尾的安装包。

## 环境要求

- Go `>= 1.23`（与当前 `go.mod` 保持一致）
- Node.js `>= 18.12`
- pnpm `>= 8.7`
- Wails CLI v2（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）

## 本地开发与编译

### 1. 安装前端依赖

```bash
pnpm --dir frontend install
```

### 2. 启动开发模式

```bash
wails dev -loglevel Info -tags fts5 -race
```

或使用：

```bash
make run
```

### 3. 运行测试与类型检查

```bash
go test ./...
go test ./internal/...
pnpm --dir frontend exec vue-tsc --noEmit
```

### 4. 一键构建校验（推荐）

```bash
./bin/build_verify.sh
```

若依赖已安装可加速：

```bash
./bin/build_verify.sh --skip-install
```

### 5. 生产构建

```bash
wails build -tags fts5
```

Windows 需要 NSIS 安装包时：

```bash
wails build -tags fts5 -nsis
```

## 构建产物路径

- macOS: `build/bin/BiliShareMall.app`
- Windows: `build/bin/BiliShareMall.exe`
- Linux: `build/bin/BiliShareMall`

## 常见问题

### 1) 搜索筛选范围无结果

当前 B 站接口会对价格/折扣等筛选进行服务端校验，建议按照接口提供的筛选范围使用。

### 2) 程序使用问题 / bug 反馈

- 使用讨论：<https://github.com/mikumifa/BiliShareMall/discussions>
- 提交 bug 或需求：<https://github.com/mikumifa/BiliShareMall/issues/new/choose>

## 项目贡献者

<!-- readme: collaborators,contributors -start -->
<table>
	<tbody>
		<tr>
            <td align="center">
                <a href="https://github.com/mikumifa">
                    <img src="https://avatars.githubusercontent.com/u/99951454?v=4" width="100;" alt="mikumifa"/>
                    <br />
                    <sub><b>mikumifa</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/XTxxxx">
                    <img src="https://avatars.githubusercontent.com/u/113696527?v=4" width="100;" alt="XTxxxx"/>
                    <br />
                    <sub><b>lttx</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/xixihaha1235">
                    <img src="https://avatars.githubusercontent.com/u/138993779?v=4" width="100;" alt="xixihaha1235"/>
                    <br />
                    <sub><b>xixihaha1235</b></sub>
                </a>
            </td>
		</tr>
	<tbody>
</table>
<!-- readme: collaborators,contributors -end -->

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=mikumifa/BiliShareMall&type=Date)](https://star-history.com/#mikumifa/BiliShareMall&Date)

## 捐赠

如果你想支持这个项目：[爱发电](https://afdian.com/a/mikumifa)
