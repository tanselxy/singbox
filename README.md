# Sing-Box 最新版自动部署脚本

## 📋 项目简介

这是一个模块化的 Sing-Box 自动部署脚本面板

## 🏗️ 系统架构

```
singbox-deploy/
├── install.sh              # 主安装脚本
├── utils.sh                # 工具函数库
├── network.sh              # 网络检测和配置库
├── config.sh               # 配置生成库
├── server_template.json    # 服务器配置模板
├── client_template.yaml    # 客户端配置模板
└── README.md               # 使用说明文档
```

## 🚀 快速开始

### 1. 执行安装脚本

```bash
bash <(curl -sL singbox.soups.eu.org/get)
```

## 📚 功能模块说明

### 🔧 主脚本 (install.sh)

主控制脚本，提供完整的安装和管理功能：

- **交互式菜单系统**
- **命令行接口支持**
- **依赖检查和错误处理**
- **服务生命周期管理**

#### 支持的命令

| 命令 | 说明 |
|------|------|
| `install` 或 `deploy` | 全新安装部署 |
| `config` | 重新生成配置 |
| `info` | 显示连接信息 |
| `start` | 启动服务 |
| `stop` | 停止服务 |
| `restart` | 重启服务 |
| `status` | 查看服务状态 |
| `logs` | 查看实时日志 |
| `uninstall` | 卸载服务 |

### 🛠️ 工具函数库 (utils.sh)

提供通用的工具函数：

- **日志系统** - 统一的日志记录和输出
- **颜色输出** - 美化的终端输出
- **密码生成** - 强密码和随机字符串生成
- **端口检测** - 自动获取可用端口
- **系统检查** - 兼容性和环境检查
- **网络优化** - BBR和TCP参数优化
- **安全配置** - SSH端口修改和fail2ban配置

### 🌐 网络库 (network.sh)

处理网络相关的功能：

- **IP地址检测** - 自动检测IPv4/IPv6
- **域名配置** - Cloudflare域名设置
- **WARP安装** - IPv6环境的WARP配置
- **代理链接生成** - 各种协议的连接链接
- **二维码生成** - 方便移动设备扫码

### ⚙️ 配置库 (config.sh)

负责配置文件的生成和管理：

- **模板渲染** - 基于模板生成配置文件
- **配置验证** - 语法检查和连通性测试
- **配置备份** - 自动备份和恢复功能
- **参数更新** - 动态更新配置参数
- **凭据管理** - UUID和密码重置

### 📄 配置模板

#### 客户端 (client_template.yaml)

## 🔌 支持的协议

| 协议  | 特点 | 推荐场景 |
|------|------|----------|
| **VLESS Reality** | 高性能，难检测 | 主力协议 |
| **Hysteria2**  | 高速UDP，适合视频 | 流媒体 |
| **ShadowTLS v3**  | 强伪装能力 | 严格环境 |
| **Trojan WS**  | WebSocket传输 | CDN加速 |
| **TUIC**  | 现代QUIC协议 | 低延迟 |
| **SS Direct**  | 简单稳定 | 备用线路 |
| **VLESS CDN**  | CDN友好 | 如果你的vps只有IPv6环境，仅此协议可用 |

## 📱 客户端支持

### 推荐客户端

| 平台 | 客户端 | 配置格式 |
|------|--------|----------|
| **Windows** | v2rayN, Clash Verge | 订阅链接/YAML |
| **macOS** | ClashX Pro, V2rayU | 订阅链接/YAML |
| **iOS** | Shadowrocket, Quantumult X | 单独链接 |
| **Android** | v2rayNG, Clash Meta | 订阅链接/YAML |
| **Linux** | sing-box, clash | YAML配置 |

### 配置方式

1. **扫描二维码** - 移动设备推荐
2. **复制链接** - 单个协议导入
3. **下载配置文件** - 完整配置导入

## 🔒 安全特性

### 传输安全

- **Reality协议** - 真实TLS握手，难以检测
- **ShadowTLS v3** - 强大的流量伪装
- **自签证书** - 防止证书特征检测
- **随机端口** - 避免固定端口被封锁

### 系统安全

- **SSH端口修改** - 减少暴力破解
- **fail2ban防护** - 自动封禁恶意IP
- **BBR加速** - 优化网络性能
- **自动清理** - 定时清理敏感文件

## 📊 监控和日志

### 日志文件

- **部署日志**: `/var/log/singbox-deploy.log`
- **服务日志**: `journalctl -u sing-box -f`
- **系统日志**: `/var/log/syslog`

## 📋 系统要求

### 操作系统

- **Ubuntu** 18.04+ 
- **Debian** 10+
- **CentOS** 8+ 
- **AlmaLinux** 8+
- **Rock** 

### 硬件要求

- **CPU**: 1核心+
- **内存**: 512MB+
- **存储**: 1GB+
- **网络**: 稳定的互联网连接

### 10美元vps推荐

| 机房 | 价格 | CPU | 内存 | 硬盘 | 流量 | 购买 |
|------|------|-----|------|------|------|------|
| 洛杉矶 | $10.96/年 | 1cpu | 2g内存 | 20g硬盘 | 2T流量/月 | [推荐](https://my.racknerd.com/aff.php?aff=10790&pid=912) |
| 洛杉矶 | $17.66/年 | 2cpu | 2g内存 | 30g硬盘 | 4T流量/月 | [购买](https://my.racknerd.com/aff.php?aff=10790&pid=913) |
| 洛杉矶 | $29.89/年 | 3cpu | 3.5g内存 | 60g硬盘 | 5T流量/月 | [购买](https://my.racknerd.com/aff.php?aff=10790&pid=914) |
| 洛杉矶 | $54.99/年 | 4cpu | 5g内存 | 100g硬盘 | 10T流量/月 | [购买](https://my.racknerd.com/aff.php?aff=10790&pid=915) |


## 📄 许可证

本项目采用 MIT 许可证，详情请查看 [LICENSE](LICENSE) 文件。

## 🆘 获取帮助

- **GitHub Issues**: [提交问题](https://github.com/tanselxy/singbox/issues)
- **Telegram**: [vps交流群](https://t.me/singboxy)

---

**⚠️ 重要提醒**: 请在遵守当地法律法规的前提下使用本工具。
