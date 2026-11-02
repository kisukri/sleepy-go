[日本語](README.md) | **简体中文** | [English](README.en.md)

# Sleepy Go

一个轻量的个人在线状态页：展示你当前的状态（活着 / 似了）、正在使用的设备和应用，设备通过客户端或 iOS 快捷指令实时上报。服务端用 Go 编写，单个二进制、单个 SQLite 文件即可运行。

## 功能

- **状态展示**：留白为主的简约页面，显示手动状态、正在使用的设备、应用、电量、音乐，以及访客自己的本地时间；设备显示顺序可以在后台调整；浅色 / 深色可以用按钮切换。
- **日文、中文、英文**：用 `page.language` 选择界面语言（默认日文，设为 `zh` 即为中文）。
- **实时更新**：SSE 推送，断线自动重连，手机切回页面立即刷新。
- **管理后台**：切换状态、管理设备、隐私模式；点击首页预览里的文字即可修改文案，支持随机文案与变量。
- **单文件运行**：页面、样式、脚本、图标和时区数据都嵌入二进制，不需要 Python、Node 或前端构建。
- **安全选项**：域名白名单、设备专用密钥、可关闭网址中的密钥，可选的反向代理示例（Caddy / nginx）。
- **客户端兼容**：提供 `/api/…` 接口，以及兼容原版 Sleepy 客户端的旧接口（如 `/device/set`）。

## 一键部署（Docker）

需要一台装好 Docker（含 Compose 插件）和 git 的 Linux 服务器：

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | bash
```

脚本会把仓库克隆到 `~/sleepy-go`，生成随机密钥，构建镜像并在 `9010` 端口启动，最后打印访问地址和密钥。

**可选：域名与 HTTPS。** 脚本不会安装反向代理；如果你想用自己的域名和 HTTPS，可以自行安装 Caddy 或 nginx，参考 [`deploy/examples/`](deploy/examples/) 里的配置，并让 Sleepy 只在本机监听、只接受该域名：

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | LISTEN=127.0.0.1 ALLOWED_HOSTS=status.example.com bash
```

已经安装过的服务器同样带上这两个变量执行 `bash ~/sleepy-go/deploy/install.sh` 即可，原有密钥和设置保留（`LISTEN=-` 恢复对外监听，`ALLOWED_HOSTS=-` 清除白名单）。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `INSTALL_DIR` | `~/sleepy-go` | 安装目录 |
| `PORT` | `9010` | Sleepy 的端口 |
| `LISTEN` | 所有网卡 | 设为 `127.0.0.1` 时只在本机监听（配合自己的反向代理） |
| `ALLOWED_HOSTS` | 无 | 允许的域名，逗号分隔 |
| `SECRET` | 自动生成 | 后台登录与设备上报使用的密钥 |
| `PAGE_NAME` | `Sleepy` | 页面上显示的名字 |
| `PAGE_LANGUAGE` | `ja` | 界面语言（`ja` / `zh` / `en`）；从原版迁移时默认 `zh` |
| `OLD_DIR` | `~/sleepy` | 原版 Sleepy 所在目录（见下方“从原版 Sleepy 迁移”） |
| `REMOVE_OLD` | `1` | 迁移成功后删除原版目录（会先打包备份）；设为 `0` 则保留 |

**更新**：再次运行同一个脚本即可（`git pull` + 重新构建 + 重启，会有几秒中断）：

```sh
bash ~/sleepy-go/deploy/install.sh
```

密钥保存在 `~/sleepy-go/docker-compose.yml`（权限 600，不会提交到 Git），数据在 `~/sleepy-go/data/`。

## 安全建议

1. 用自己的反向代理启用 HTTPS（示例见 `deploy/examples/`）。没有 HTTPS 时，密钥会以明文在网络上传输。
2. 在 `data/config.yaml` 设置 `main.device_secret`，客户端改用这个设备密钥，它只能上报设备；`main.secret` 只用于后台。
3. 快捷指令改为在请求头中发送 `Authorization: Bearer <设备密钥>` 后，设置 `main.allow_query_secret: false`。

详见 [配置说明 · 安全](docs/configuration.md#セキュリティ)（日文）。

## 从原版 Sleepy 迁移

如果服务器上已经用 Docker 跑着原版 [Sleepy](https://github.com/sleepy-project/sleepy)（默认在 `~/sleepy`，其他位置用 `OLD_DIR=/path` 指定），直接运行上面的一键命令。脚本会：

1. 先克隆并构建新版，旧版在此期间继续运行；
2. 沿用旧 `docker-compose.yml` 中的密钥、名字等 `sleepy_` 设置和端口；
3. 停止旧容器，把旧目录完整打包为 `~/sleepy-python-backup-<时间>.tgz`，复制 `data/`（数据库、文案、配置）；
4. 启动 Sleepy Go 并做健康检查，失败则自动恢复旧版；成功后删除旧目录和旧镜像。

客户端不用改：密钥、设备 ID 和上报地址都保持不变。后台需要重新登录一次。手动迁移见 [迁移指南](docs/migration.md)（日文）。

## 其他运行方式

### Docker Compose（手动）

```sh
git clone https://github.com/kisukri/sleepy-go.git && cd sleepy-go
mkdir -p data
export SLEEPY_UID="$(id -u)" SLEEPY_GID="$(id -g)"
export SLEEPY_MAIN_SECRET='替换为你自己的长随机密钥'
export SLEEPY_PAGE_NAME='你的名字'
docker compose -f deploy/compose.yaml up --build -d
```

### 从源码运行

需要 **Go 1.25+**（`go.mod` 指定工具链 go1.27.3，较旧的 Go 会自动下载）：

```sh
git clone https://github.com/kisukri/sleepy-go.git && cd sleepy-go
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/sleepy-go ./cmd/sleepy-go
export sleepy_main_secret='替换为你自己的长随机密钥'
export sleepy_page_name='你的名字'
./bin/sleepy-go
```

默认监听 `127.0.0.1:9010`、数据目录为当前目录下的 `data/`。对外提供服务时设置 `sleepy_main_host=0.0.0.0`，或用 `-data-dir /path/to/data` 指定数据目录。反向代理与 HTTPS 见 [部署与运行](docs/operations.md)（日文）。

## 配置

简单设置用 `sleepy_` 环境变量（如 `sleepy_main_secret`、`sleepy_page_name`）；完整配置复制示例文件：

```sh
cp configs/config.example.yaml data/config.yaml
```

优先级从低到高：默认值 → `data/.env` / 环境变量 → `data/config.yaml` → `data/config.toml` → `data/config.json`。密钥不能为空，过短会在日志中提示。字段说明见 [配置说明](docs/configuration.md)（日文）。

## 客户端与 API

客户端说明见 [`clients/`](clients/README.md)（日文）：Windows 推荐用 Go 版客户端（一个 exe，不需要 Python，支持窗口、空闲、电量和定时心跳），另有 Python 版、PowerShell 版，以及 Linux（KDE、Hyprland）脚本；iOS/iPadOS 可用快捷指令。填写服务地址、密钥、设备 ID 和显示名称即可上报。

新客户端建议用 `Authorization: Bearer <密钥>` 并发送 JSON POST 到 `/api/device/set`。接口说明见 [API 文档](docs/api.md)（日文）。

下面两种客户端的“密钥”，设置了 `main.device_secret` 就填它，否则填 `main.secret`（也就是后台登录用的密钥）。

### Windows 客户端

1. 打开本仓库 [Actions 里的 Go 工作流](https://github.com/kisukri/sleepy-go/actions/workflows/go.yml)，进入最近一次成功的运行，下载 `sleepy-client-windows`（需要登录 GitHub）。解压后，一般电脑用 `sleepy-client.exe`，ARM 版 Windows 用 `sleepy-client-arm64.exe`。
2. 把 exe 放到一个固定的文件夹，双击运行一次。同目录下会生成 `sleepy-client.toml`，程序随即退出。
3. 用记事本打开 `sleepy-client.toml`，填写这几项：

   ```toml
   server = "https://status.example.com"
   secret = "你的密钥"
   device_id = "my-pc"
   device_name = "我的电脑"
   ```

4. 再双击一次，开始上报。程序没有窗口，运行情况可以看同目录下的 `sleepy-client.log`。
5. 开机自启：按 `Win + R`，输入 `shell:startup`，把 exe 的快捷方式放进打开的文件夹。

exe 没有签名，第一次运行可能被 SmartScreen 拦下，点“更多信息”→“仍要运行”即可。其他选项见 [clients/](clients/README.md#sleepy-clientgo-版)（日文）。

### iOS 快捷指令

需要 iOS 18 以上。在“快捷指令”App 里新建一个快捷指令，按顺序添加：

1. **获取当前 App**
2. **获取电池电量**（不需要显示电量可以省略）
3. **文本**：`[`、电池电量、`%]`、当前 App 的“名称”依次拼在一起，例如 `[87%]微信`。页面会把方括号里的百分比显示为电量；写成 `[🔋87%]` 也可以，充电中可以写 `[87%⚡]`。
4. **获取 URL 内容**：
   - URL：`https://status.example.com/api/device/set`
   - 方法：`POST`
   - 头部：`Authorization`，值为 `Bearer 你的密钥`
   - 请求体：JSON

   | 键 | 类型 | 值 |
   | --- | --- | --- |
   | `id` | 文本 | `iphone`（设备的唯一 ID） |
   | `show_name` | 文本 | `iPhone`（页面上显示的名字） |
   | `using` | 布尔值 | 开 |
   | `status` | 文本 | 第 3 步的“文本” |

手动运行一次，页面上出现这台设备就说明成功了。之后在“自动化”→“App”里选中所有 App，选“已打开”和“立即运行”，再指定这个快捷指令，以后每打开一个 App 都会自动上报。

地址要用 `https://`。用 `http://` 时密钥会明文传输，反向代理跳转到 HTTPS 时还可能把 POST 变成 GET，或者丢掉请求头。

## 项目结构

```text
.
├── cmd/sleepy-go/          # 服务器入口
├── cmd/sleepy-client/      # Windows 用 Go 客户端
├── internal/sleepy/        # 配置、HTTP 接口、鉴权、SQLite、SSE、文案及测试
├── web/                    # 嵌入二进制的页面、样式、脚本和文案编辑器
├── clients/                # 其他客户端（Python、PowerShell、Shell）
├── configs/                # 配置示例
├── deploy/                 # 一键部署脚本、Dockerfile、Compose 及反向代理示例
└── docs/                   # 配置、API、部署与迁移文档（日文）
```

## 开发

```sh
go test -race ./...
go vet ./...
```

## 许可

[MIT License](LICENSE) © Kisukri。`clients/` 中的客户端脚本来自 [sleepy-project/sleepy](https://github.com/sleepy-project/sleepy)，保留原作者的 MIT 许可证（[clients/LICENSE](clients/LICENSE)）。感谢原项目作者及客户端贡献者。内置的 Newsreader 字体采用 SIL Open Font License 1.1（[web/static/fonts/OFL.txt](web/static/fonts/OFL.txt)）。
