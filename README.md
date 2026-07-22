# UART 短信转发器（多模块版）

基于 [dushixiang/uart_sms_forwarder](https://github.com/dushixiang/uart_sms_forwarder) 扩展的 Air780 系列短信与来电管理平台。本版本增加多模块统一管理、按模块收发短信、SIM 卡资料、通话记录、定时流量保活和可配置的无应答呼叫转移。

> 当前平台发布版：`v1.3.0-multi.4`
> Lua 插件版本：Air780EPV `1.4.0`；Air780EHV / 通用脚本 `1.3.0`

## 已验证环境

验证日期：2026-07-22

| 项目 | 已验证配置 | 结果 |
| --- | --- | --- |
| 上位机 | Linux x86_64、Docker、SQLite | 通过 |
| 模块 1 | Air780EPV、定制 V1002 PDU 固件、Lua 插件 `1.4.0` | 在线、状态读取、短信 PDU 上报及流量任务通过 |
| 模块 2 | Air780EHV、`LuatOS-SoC_V2036_Air780EHV_116.soc`、实机 Lua 插件 `1.2.2` | 在线、状态读取、来电及 PDP 重建后 50 KiB 流量任务通过；仓库插件仍为 `1.3.0` |
| 多模块 | 两个 USB 串口同时运行 | `2/2` 在线 |
| EHV 来电 | 真实来电、挂断事件及 5 秒持续时长 | 数据库、API、桌面和手机页面均通过 |
| 平台 `v1.3.0-multi.4` | 双模块状态、SIM 卡资料、PDU 解码、通知标识、来电记录、无应答转移及流量超时自动恢复 | 通过 |
| Lua 插件 | EPV `1.4.0`、EHV / 通用脚本 `1.3.0` | 通过；真实运营商转移仍需按号码单独验证 |
| 前端 | 桌面端及 390 px 手机端 | 通过，无横向溢出 |
| 自动测试 | `go test ./...`、`npm run build`、本次改动定向 ESLint | 通过 |

## v1.3.0-multi.4 更新

- LuatOS HTTP 负状态码会显示明确原因，例如 `-8` 显示为“连接或读取超时”，不再只显示难以判断的数字。
- 流量任务仅在 `HTTP -8` 且上行、下行、响应体均为 `0 B` 时自动开关一次飞行模式，等待蜂窝数据连接恢复后重试一次；其他 HTTP 错误不会重置模块，也不会无限重试。
- 真实 Air780EHV 在长期驻网后复现 PDP 数据承载失效；手动重建连接后一次请求成功，平台记录 `HTTP 200`、响应体 `51,200 B`、模块计数合计 `54,736 B`。自动恢复逻辑已用回归测试覆盖，模块无需重刷固件。

## v1.3.0-multi.3 更新

- 定时流量保活由约 5 KiB 调整为固定 50 KiB，旧流量任务会在平台启动时自动迁移。
- giffgaff 中国漫游实测 50 KiB 请求可进入账单，单次约 `£0.01`；运营商可能按不同粒度计费，平台不保证固定费用。
- 页面、数据库记录、配置示例和实际静态文件统一使用 50 KiB，日常任务不再依赖低于计费显示阈值的小流量。

## v1.3.0-multi.2 更新

- 增加按模块保存的 SIM 卡别名和本机手机号，并在短信、来电、发送失败通知及 Webhook/邮件变量中统一携带。
- 增加 Air780EPV 原始 PDU 上报和服务端 GSM7/UCS2 解码，支持长短信分片重组，并保留原文、PDU、DCS 和失败原因用于排查乱码。
- 增加 Air780EPV 专用的定制 V1002 固件、可复现源码补丁、SHA256 校验值和一次刷写说明；Air780EHV 不需要刷写该固件。
- 修正多模块页面的默认选择、SIM 资料刷新和模块卡片布局，保持现有数据库和单模块配置兼容。

真实模块先以 4 KiB 基线文件验证链路，再以 50 KiB 文件验证可见计费：

| 模块 | HTTP | 上行 | 下行 | 合计 | 连接状态 |
| --- | ---: | ---: | ---: | ---: | --- |
| Air780EPV | 200 | 460 B | 4,751 B | 5,211 B | 已关闭 |
| Air780EHV | 200 | 479 B | 4,631 B | 5,110 B | 已关闭 |
| Air780EPV（50 KiB 受控测试） | 200 | 1,460 B | 53,256 B | 54,716 B | 已关闭 |
| Air780EHV（PDP 重建后 50 KiB） | 200 | 1,480 B | 53,256 B | 54,736 B | 已关闭 |

## 功能

- 同一平台管理多个 Air780 模块，每个模块拥有独立 ID、名称和串口。
- 可在串口控制页为每个模块手动填写 SIM 卡别名和本机手机号。
- 支持 `/dev/serial/by-id/` 或自定义 udev 别名，避免重启后串口编号变化。
- 短信记录保存模块归属，可按模块筛选。
- 支持按指定模块接收、发送短信和执行串口命令。
- Air780EPV 支持服务端 PDU 解码和长短信分片重组；解码失败时保留原始数据并在页面显示原因。
- 支持定时短信任务和定时流量保活任务。
- 流量任务固定约 50 KiB，并记录 HTTP 状态、上行、下行、总流量、连接状态及错误原因。
- 流量历史独立保存在 SQLite 中，删除定时任务不会删除历史记录。
- 支持来电通知，以及钉钉、企业微信、飞书、自定义 Webhook、邮件和 Telegram 通知。
- 短信、来电和发送失败通知固定携带 `SIM1/SIM2`、SIM 卡别名和本机号码；未手填号码时自动使用模块上报值。
- 模块上报的来电会保存号码、模块、来电时间、结束时间和持续时长，可查看全部模块或单独筛选。
- 支持按模块配置无应答呼叫转移，包括开关、目标号码和 5/10/15/20/25/30 秒延时。
- 保留原项目的单模块配置兼容模式。

## Lua 插件

仓库内已包含本次真实设备验证使用的插件：

- `firmware/Air780EPV/main.lua`：Air780EPV 专用标识版本。
- `firmware/Air780EHV/main.lua`：Air780EHV 专用标识版本。
- `main.lua`：Air780EPV / Air780EHV 通用入口版本。

Air780EPV 专用脚本为 `1.4.0`，Air780EHV 和通用脚本仍为 `1.3.0`。请先选择与硬件型号完全匹配的 LuatOS 底层固件，再用 LuaTools 写入对应 `main.lua`。不要把一个型号的 `.soc` 底层固件写入其他型号。

平台升级、网页修改或数据库迁移不要求重新刷写插件。只有插件版本变化时才需要重新写入模块。

从 `1.2.2` 升级时，Air780EHV 写入对应目录的 `main.lua` 后应显示插件版本 `1.3.0`；Air780EPV 按下一段的专用说明一次写入底层固件和 `main.lua` 后应显示 `1.4.0`。串口控制页同时应显示“插件支持：支持”。

Air780EPV 的 GSM7/PDU 修复需要平台先升级，然后在 LuaTools 的同一次下载中选择 `firmware/Air780EPV/LuatOS-SoC_V1002_EC718PV_SMS-PDU-20260721.soc` 和该目录的 `main.lua`。Air780EHV 不需要刷这个修复。具体校验值见 `firmware/Air780EPV/README.md`。

## 无应答呼叫转移

在“串口控制”页选择模块后，可配置转移号码、无应答延时并启用或关闭。平台和插件都执行白名单校验，不提供任意 AT/MMI 输入：

- 启用仅生成 GSM 无应答转移格式 `**61*号码**延时#`。
- 关闭仅生成 `##61#`，用于关闭并清除无应答转移。
- 号码只允许可选的 `+` 和 3 至 20 位数字。
- 延时只允许 5、10、15、20、25、30 秒。

模块只能确认指令是否提交给基带，当前 LuatOS API 无法读取运营商侧最终状态。页面显示“已提交”不等于运营商一定已开通，操作后必须用另一号码实际拨打验证。是否支持、漫游场景限制和资费均由 SIM 卡运营商决定。

## SIM 卡资料与推送

在“串口控制”页选择 `SIM1` 或 `SIM2`，填写别名和本机手机号后保存。资料保存在 SQLite 的 `properties` 表中，平台升级或容器重建不会丢失，也不需要重新刷写模块插件。

文本通知会显示 SIM 卡槽位、别名和本机号码。自定义 Webhook 与邮件主题还可以使用以下变量：

- `{{sim_label}}`：例如 `SIM1（英国主卡）`。
- `{{sim_alias}}` / `{{module_alias}}`：手动填写的别名。
- `{{sim_number}}` / `{{phone_number}}`：手动填写或模块上报的本机号码。
- `{{module_id}}` / `{{module_name}}`：模块 ID 与配置名称。

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

建议在自己控制的 HTTP 站点放置一个 50 KiB 静态文件：

```bash
mkdir -p /var/www/html/uart-traffic
dd if=/dev/urandom of=/var/www/html/uart-traffic/payload.bin bs=1024 count=50
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

- Docker Hub：`s121934/uart_sms_forwarder_multi:1.3.0-multi.4`
- GHCR：`ghcr.io/xxs-dev/uart_sms_forwarder_multi:1.3.0-multi.4`

## 数据与升级

- SQLite 数据库默认位于 `./data/app.db`。
- 日志默认位于 `./logs/`。
- 升级前请备份 `config.yaml` 和整个 `data/` 目录。
- `traffic_records` 保存独立流量历史，不依赖定时任务是否仍然存在。
- `call_records` 保存所有模块的来电与结束时间；`call_forwarding_configs` 保存各模块上次提交的转移配置和错误原因。
- `properties` 中的 `module_identities` 保存每个模块的 SIM 卡别名和本机手机号。
- 不要把真实的 `config.yaml`、数据库、日志、手机号、短信内容或 API 密钥提交到 GitHub。

## 免责声明

本项目仅用于合法、自有设备的短信转发、设备管理和 SIM 卡保活。使用者必须遵守所在地法律法规、运营商协议及第三方服务条款。

定时短信、流量保活和呼叫转移会产生真实的短信费、流量费、通话费或漫游费用，实际计费以运营商为准。项目显示的字节数来自模块计数器，只用于运行状态参考，不构成账单依据。运营商可能限制 ICMP、HTTP、漫游流量、短信发送频率、呼叫转移或长期未使用的 SIM 卡，本项目不保证任何卡永久保号或始终可用。

刷写不匹配的底层固件、串口配置错误、供电不足、USB 重枚举、数据库损坏、配置泄露或公网暴露均可能导致设备不可用、数据丢失或产生额外费用。请在操作前备份数据并确认硬件型号。使用者对自己的设备、账号、密钥、短信内容、网络端点和费用承担全部责任。

本项目按现状提供，不承诺适用于任何特定用途，也不对直接或间接损失、数据丢失、资费、停机或设备损坏承担责任。

## 上游项目

原项目：[dushixiang/uart_sms_forwarder](https://github.com/dushixiang/uart_sms_forwarder)

本仓库保留原项目结构并在其基础上增加多模块与流量保活能力。上游项目说明见：[Air780E 短信转发器](https://blog.typesafe.cn/posts/air780e-giffgaff/)。
