# Sing-Box 最新版自动部署脚本系统

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

### 1. 下载和准备

```bash
# 执行后会自动下载所需脚本并安装singbox
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

#### 服务器模板 (server_template.json)

包含完整的服务器端配置，支持多种协议：

- **ShadowTLS v3** + SS2022
- **VLESS Reality** 
- **Hysteria2**
- **Trojan WebSocket**
- **TUIC**
- **ShadowSocks 专线**
- **VLESS WebSocket CDN**

#### 客户端模板 (client_template.yaml)

Clash风格的客户端配置：

- **代理组配置** - 手动选择、自动测速、故障转移
- **分流规则** - 中国大陆直连，其他走代理
- **DNS配置** - 优化的DNS设置

## 🔌 支持的协议

| 协议 | 端口范围 | 特点 | 推荐场景 |
|------|----------|------|----------|
| **VLESS Reality** | 20000-20010 | 高性能，难检测 | 主力协议 |
| **Hysteria2** | 50000-50010 | 高速UDP，适合视频 | 流媒体 |
| **ShadowTLS v3** | 31000-31010 | 强伪装能力 | 严格环境 |
| **Trojan WS** | 63333 | WebSocket传输 | CDN加速 |
| **TUIC** | 61555 | 现代QUIC协议 | 低延迟 |
| **SS Direct** | 59000 | 简单稳定 | 备用线路 |
| **VLESS CDN** | 4433 | CDN友好 | IPv6环境 |

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

## 🛠️ 高级用法

### 配置管理

```bash
# 备份当前配置
sudo ./install.sh backup

# 重置所有密码
sudo ./install.sh reset-credentials

# 更新特定参数
sudo ./install.sh update-param uuid new-uuid-value

# 导出配置包
sudo ./install.sh export
```

### 协议管理

```bash
# 禁用特定协议
sudo ./install.sh toggle-protocol hysteria2 disable

# 启用特定协议
sudo ./install.sh toggle-protocol reality enable

# 查看配置摘要
sudo ./install.sh summary
```

### 连通性测试

```bash
# 测试所有端口
sudo ./install.sh test-connectivity

# 验证配置文件
sudo ./install.sh validate-config
```

## 📊 监控和日志

### 日志文件

- **部署日志**: `/var/log/singbox-deploy.log`
- **服务日志**: `journalctl -u sing-box -f`
- **系统日志**: `/var/log/syslog`

### 监控命令

```bash
# 查看服务状态
systemctl status sing-box

# 查看端口监听
netstat -tuln | grep -E "(443|10243|10244)"

# 查看连接数
ss -tuln | grep sing-box

# 查看资源使用
top -p $(pidof sing-box)
```

## 🚨 故障排除

### 常见问题

#### 1. 服务启动失败

```bash
# 检查配置文件语法
sing-box check -c /etc/sing-box/config.json

# 查看详细错误
journalctl -u sing-box --no-pager -l
```

#### 2. 端口被占用

```bash
# 查看端口占用
lsof -i:端口号

# 释放端口
sudo fuser -k 端口号/tcp
```

#### 3. 证书问题

```bash
# 重新生成自签证书
openssl ecparam -genkey -name prime256v1 -out /etc/sing-box/cert/private.key
openssl req -new -x509 -days 36500 -key /etc/sing-box/cert/private.key -out /etc/sing-box/cert/cert.pem -subj "/CN=bing.com"
```

#### 4. 网络连接问题

```bash
# 测试外网连接
curl -4 ifconfig.me
curl -6 ifconfig.me

# 检查DNS解析
nslookup google.com

# 测试端口连通性
telnet 服务器IP 端口
```

### 紧急恢复

```bash
# 停止所有服务
sudo systemctl stop sing-box

# 恢复默认配置
sudo ./install.sh config

# 重启服务
sudo systemctl start sing-box
```

## 📋 系统要求

### 操作系统

- **Ubuntu** 18.04+ 
- **Debian** 10+
- **CentOS** 8+ (实验性支持)

### 硬件要求

- **CPU**: 1核心+
- **内存**: 512MB+
- **存储**: 1GB+
- **网络**: 稳定的互联网连接

### 网络环境

- **IPv4**: 必需
- **IPv6**: 可选
- **域名**: 可选（IPv6环境推荐）
- **证书**: 自动生成或手动提供

## 🤝 贡献指南

### 开发环境

```bash
# 克隆项目
git clone https://github.com/your-repo/singbox-deploy.git
cd singbox-deploy

# 设置开发环境
chmod +x *.sh
```

### 代码规范

- 使用 `bash` Shell
- 遵循 Google Shell Style Guide
- 添加详细的注释
- 包含错误处理

### 提交流程

1. Fork 项目
2. 创建功能分支
3. 编写代码和测试
4. 提交 Pull Request

## 📄 许可证

本项目采用 MIT 许可证，详情请查看 [LICENSE](LICENSE) 文件。

## 🆘 获取帮助

- **GitHub Issues**: [提交问题](https://github.com/your-repo/singbox-deploy/issues)
- **讨论区**: [参与讨论](https://github.com/your-repo/singbox-deploy/discussions)
- **Telegram**: [@SingBoxSupport](https://t.me/SingBoxSupport)

---

**⚠️ 重要提醒**: 请在遵守当地法律法规的前提下使用本工具。