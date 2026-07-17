# UART 短信转发器（多模块版）

基于 [dushixiang/uart_sms_forwarder](https://github.com/dushixiang/uart_sms_forwarder) 扩展的 Air780 系列短信管理平台。本版本重点增加多模块统一管理、按模块收发短信、定时流量保活，以及独立持久化的流量历史。

> 当前已验证发布版：`v1.2.2-multi.2`
> Lua 插件版本：`1.2.2`

## 已验证环境

验证日期：2026-07-17

| 项目 | 已验证配置 | 结果 |
| --- | --- | --- |
| 上位机 | Linux x86_64、Docker、SQLite | 通过 |
| 模块 1 | Air780EPV、Lua 插件 `1.2.2` | 在线、状态读取及流量任务通过 |
| 模块 2 | Air780EHV、`LuatOS-SoC_V2036_Air780EHV_116.soc`、Lua 插件 `1.2.2` | 在线、状态读取及流量任务通过 |
| 多模块 | 两个 USB 串口同时运行 | `2/2` 在线 |
| EHV 来电 | 真实来电、挂断事件及 5 秒持续时长 | 数据库、API、桌面和手机页面均通过 |
| 前端 | 桌面端及 390 px 手机端 | 通过，无横向溢出 |
| 自动测试 | `go test ./...`、`npm run build` | 通过 |

真实模块使用自建的 4 KB HTTP 静态文件验证：

| 模块 | HTTP | 上行 | 下行 | 合计 | 连接状态 |
| --- | ---: | ---: | ---: | ---: | --- |
| Air780EPV | 200 | 460 B | 4,751 B | 5,211 B | 已关闭 |
| Air780EHV | 200 | 479 B | 4,631 B | 5,110 B | 已关闭 |

## 功能

- 同一平台管理多个 Air780 模块，每个模块拥有独立 ID、名称和串口。
- 支持 `/dev/serial/by-id/` 或自定义 udev 别名，避免重启后串口编号变化。
- 短信记录保存模块归属，可按模块筛选。
- 支持按指定模块接收、发送短信和执行串口命令。
- 支持定时短信任务和定时流量保活任务。
- 流量任务固定约 5 KB，并记录 HTTP 状态、上行、下行、总流量、连接状态及错误原因。
- 流量历史独立保存在 SQLite 中，删除定时任务不会删除历史记录。
- 支持来电通知，以及钉钉、企业微信、飞书、自定义 Webhook、邮件和 Telegram 通知。
- Air780EHV 来电自动保存号码、模块、来电时间、结束时间和持续时长。
- 保留原项目的单模块配置兼容模式。

## Lua 插件

仓库内已包含本次真实设备验证使用的插件：

- `firmware/Air780EPV/main.lua`：Air780EPV 专用标识版本。
- `firmware/Air780EHV/main.lua`：Air780EHV 专用标识版本。
- `main.lua`：Air780EPV / Air780EHV 通用入口版本。

三个脚本的功能版本均为 `1.2.2`。请先选择与硬件型号完全匹配的 LuatOS 底层固件，再用 LuaTools 写入对应 `main.lua`。不要把 Air780EHV 的 `.soc` 底层固件写入其他型号。

平台升级、网页修改或数据库迁移不要求重新刷写插件。只有插件版本变化时才需要重新写入模块。

## 快速部署

### 1. 固定串口

先在 Linux 主机查看稳定设备路径：

```bash
ls -l /dev/serial/by-id/
```

如果两个模块没有唯一的 `by-id`，可以按物理 USB 端口建立 udev 别名，例如 `/dev/ttySMS1` 和 `/dev/ttySMS2`。

### 2. 准备配置

```bash
mkdir -p /opt/uart_sms_forwarder/{data,logs}
cd /opt/uart_sms_forwarder
curl -L https://raw.githubusercontent.com/xxs-dev/uart_sms_forwarder_multi/main/config.example.yaml -o config.yaml
curl -L https://raw.githubusercontent.com/xxs-dev/uart_sms_forwarder_multi/main/docker-compose.yml -o docker-compose.yml
```

编辑 `config.yaml`，至少修改 JWT 密钥、登录密码、模块串口和流量地址：

```yaml
App:
  JWT:
    Secret: "请替换为随机密钥"
    ExpiresHours: 168
  Modules:
    - ID: "sim1"
      Name: "短信模块 1"
      Port: "/dev/ttySMS1"
    - ID: "sim2"
      Name: "短信模块 2"
      Port: "/dev/ttySMS2"
  Scheduler:
    TrafficEndpoint: "http://your-server.example/uart-traffic/payload.bin"
```

`Users` 中的密码必须使用 bcrypt 哈希。示例密码仅用于首次测试，不应直接用于公网环境。

### 3. 准备流量保活文件

建议在自己控制的 HTTP 站点放置一个不可压缩的 4 KB 静态文件：

```bash
mkdir -p /var/www/html/uart-traffic
dd if=/dev/urandom of=/var/www/html/uart-traffic/payload.bin bs=4096 count=1
```

使用 HTTP 可以减少 TLS 握手流量。该地址必须能被 SIM 卡的移动网络直接访问，并应配置合理的访问限制，避免被公开滥用。

### 4. 启动 Docker

按照主机实际路径修改 `docker-compose.yml` 中的 `devices`，并保证容器内路径与 `config.yaml` 一致，然后运行：

```bash
docker compose up -d
docker compose logs -f --tail=100
```

默认访问地址为 `http://服务器IP:8080/`。

## 从源码构建

构建环境为 Go `1.25`、Node.js `24`：

```bash
cd web
npm ci
npm run build
cd ..
go test ./...
go build -o uart_sms_forwarder ./cmd/serv
```

推送 `v*` 标签后，GitHub Actions 会生成多平台 Release 包，并构建发布到 GitHub Container Registry 的 amd64/arm64 镜像。

当前验证镜像：

- Docker Hub：`s121934/uart_sms_forwarder_multi:1.2.2-multi.2`
- GHCR：`ghcr.io/xxs-dev/uart_sms_forwarder_multi:1.2.2-multi.2`

## 数据与升级

- SQLite 数据库默认位于 `./data/app.db`。
- 日志默认位于 `./logs/`。
- 升级前请备份 `config.yaml` 和整个 `data/` 目录。
- `traffic_records` 保存独立流量历史，不依赖定时任务是否仍然存在。
- 不要把真实的 `config.yaml`、数据库、日志、手机号、短信内容或 API 密钥提交到 GitHub。

## 免责声明

本项目仅用于合法、自有设备的短信转发、设备管理和 SIM 卡保活。使用者必须遵守所在地法律法规、运营商协议及第三方服务条款。

定时短信和流量保活会产生真实的短信费、流量费或漫游费用，实际计费以运营商为准。项目显示的字节数来自模块计数器，只用于运行状态参考，不构成账单依据。运营商可能限制 ICMP、HTTP、漫游流量、短信发送频率或长期未使用的 SIM 卡，本项目不保证任何卡永久保号或始终可用。

刷写不匹配的底层固件、串口配置错误、供电不足、USB 重枚举、数据库损坏、配置泄露或公网暴露均可能导致设备不可用、数据丢失或产生额外费用。请在操作前备份数据并确认硬件型号。使用者对自己的设备、账号、密钥、短信内容、网络端点和费用承担全部责任。

本项目按现状提供，不承诺适用于任何特定用途，也不对直接或间接损失、数据丢失、资费、停机或设备损坏承担责任。

## 上游项目

原项目：[dushixiang/uart_sms_forwarder](https://github.com/dushixiang/uart_sms_forwarder)

本仓库保留原项目结构并在其基础上增加多模块与流量保活能力。上游项目说明见：[Air780E 短信转发器](https://blog.typesafe.cn/posts/air780e-giffgaff/)。
