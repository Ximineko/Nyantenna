# Nyantenna

[![License: PolyForm Noncommercial 1.0.0](https://img.shields.io/badge/License-PolyForm--Noncommercial--1.0.0-blue.svg)](https://polyformproject.org/licenses/noncommercial/1.0.0)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)
[![Nuxt 4](https://img.shields.io/badge/Nuxt-4-00DC82?logo=nuxt.js)](web-next/package.json)

面向蜂窝模组（Quectel EC20/EC25/EG25/EM20 等）的管理平台，把模组管理、短信、eSIM 与 Web 后台整合到一个服务里。仅供个人学习与技术研究。

## 项目来源

**本项目是 [VoHive](https://github.com/iniwex5/vohive)（作者 iniwex5）的二次开发**，沿用原作者选用的
[PolyForm Noncommercial License 1.0.0](LICENSE)。按协议 *Notices* 条款，分发时须保留署名声明：

> Required Notice: Copyright iniwex5 (https://github.com/iniwex5/vohive)

该声明在 [LICENSE](LICENSE) 中原样保留，再分发时请勿移除或修改。

## 核心特性

| 模块 | 说明 |
| --- | --- |
| 多模组管理 | USB 热插拔自动发现、多设备实时状态监控 |
| 多后端支持 | `at` / `qmi` / `mbim` 三种模组后端，另有 `pcsc` 纯读卡器设备类型 |
| 短信中心 | 短信收发、会话与联系人管理、USSD 交互，消息落库可查 |
| eSIM 管理 | 直接管理 eUICC：Profile 下载（SM-DP+）、启用/停用、重命名、删除 |
| 代理引擎 | 内建 SOCKS5 / HTTP，基于 `SO_BINDTODEVICE` 按设备网卡绑定出站流量 |
| 通知推送 | Telegram、Email、PushPlus、Bark、飞书、QQ 等 |
| 多架构构建 | amd64 / arm64 / arm7 跨平台编译 |

### PC/SC 读卡器设备

可把 PC/SC 读卡器作为一类独立设备接入：卡内能力（AKA 鉴权、IMSI/ICCID/SPN 读取）为真实实现，
射频能力不存在，界面与 API 已按此收敛。**IMEI 需手工填写**——读卡器没有基带，卡上也不存 IMEI。

依赖 cgo 与 `libpcsclite`，隔离在 `pcsc` 构建标签之后，默认构建不受影响：

```bash
CGO_ENABLED=0 go build -tags "with_utls nomsgpack" ./cmd/nyantenna        # 默认
CGO_ENABLED=1 go build -tags "with_utls nomsgpack pcsc" ./cmd/nyantenna   # 启用 PC/SC
```

运行前需安装并启动 `pcscd` 与 CCID 驱动。

## 技术栈

Go 1.26+（Gin、GORM、Viper、euicc-go） · Nuxt 4 + Nuxt UI 4 · SQLite（`data/nyantenna.db`）

## 免责声明

- **用途定位**：本项目面向个人学习与技术研究，不建议用于生产环境；部署与使用风险由使用者自行承担。
- **合规使用**：使用者须自行确保符合所在地区的法律法规及电信运营商服务条款，不得用于任何违法违规用途。因违规使用产生的一切责任由使用者自行承担，与本项目作者及贡献者无关。
- **非官方项目**：与 Quectel 及任何模组/芯片厂商均无关联或授权关系。
- **无担保**：本软件按"现状"提供，不附带任何明示或暗示的担保。因使用或无法使用本软件造成的任何直接或间接损失，作者及贡献者不承担责任。

### 关于 VoWiFi 库

VoHive 依赖的 `github.com/iniwex5/vowifi-go` 模块并未公开，本项目无法获得其源码。

因此本项目使用的是**按原作者思路独立实现的重构版**：以公开的协议规范（3GPP TS 24.011 / 24.229 /
24.341 / 33.203、RFC 3261 / 5448 / 7296 等）为准绳重新实现，**非**原版 `vowifi-go` 库本身。

- 不是等价替换，行为与原版并不一一对应，功能仍在逐步补齐
- 相关缺陷请勿反馈给 VoHive 原作者——那不是原版代码的行为

## License

沿用上游 [PolyForm Noncommercial License 1.0.0](LICENSE)，**仅限非商业用途**，禁止任何形式的商业使用。

本项目作为衍生作品，无权单独授出超出该协议范围的权利；如需商业授权请联系上游原作者。
再分发时请保留 LICENSE 中的 `Required Notice`。
