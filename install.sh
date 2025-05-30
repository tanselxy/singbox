#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# CentOS/RHEL Sing-Box 安装函数
install_singbox_centos() {
    printf "${BLUE}🎩 为 CentOS/RHEL 系统安装 Sing-Box...${NC}\n"
    
    # 检测架构
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64) arch="arm64" ;;
        armv7l) arch="armv7" ;;
        *) 
            printf "${RED}❌ 不支持的架构: $arch${NC}\n"
            return 1
            ;;
    esac
    
    printf "${BLUE}🔍 检测到架构: $arch${NC}\n"
    
    # 获取最新版本
    printf "${BLUE}📡 获取最新版本信息...${NC}\n"
    local latest_version
    latest_version=$(curl -s https://api.github.com/repos/SagerNet/sing-box/releases/latest | grep -oP '"tag_name": "\K[^"]+' 2>/dev/null || echo "v1.8.0")
    
    printf "${GREEN}📋 最新版本: $latest_version${NC}\n"
    
    # 创建临时目录
    local temp_dir="/tmp/singbox-install"
    mkdir -p "$temp_dir"
    cd "$temp_dir" || return 1
    
    # 下载URL
    local download_url="https://github.com/SagerNet/sing-box/releases/download/$latest_version/sing-box-${latest_version#v}-linux-$arch.tar.gz"
    
    printf "${BLUE}📥 下载 Sing-Box...${NC}\n"
    
    # 尝试多个下载源
    local downloaded=false
    
    # 源1: GitHub 直接下载
    if curl -L --progress-bar "$download_url" -o "sing-box.tar.gz" 2>/dev/null; then
        downloaded=true
        printf "${GREEN}✅ GitHub 下载成功${NC}\n"
    else
        printf "${YELLOW}⚠️  GitHub 下载失败，尝试代理源...${NC}\n"
        
        # 源2: GitHub 代理
        local proxy_url="https://ghproxy.com/$download_url"
        if curl -L --progress-bar "$proxy_url" -o "sing-box.tar.gz" 2>/dev/null; then
            downloaded=true
            printf "${GREEN}✅ 代理源下载成功${NC}\n"
        else
            printf "${YELLOW}⚠️  代理源失败，尝试 jsDelivr...${NC}\n"
            
            # 源3: jsDelivr CDN
            local jsdelivr_url="https://cdn.jsdelivr.net/gh/SagerNet/sing-box@$latest_version/release/sing-box-${latest_version#v}-linux-$arch.tar.gz"
            if curl -L --progress-bar "$jsdelivr_url" -o "sing-box.tar.gz" 2>/dev/null; then
                downloaded=true
                printf "${GREEN}✅ jsDelivr 下载成功${NC}\n"
            fi
        fi
    fi
    
    if [[ "$downloaded" != true ]]; then
        printf "${RED}❌ 所有下载源都失败${NC}\n"
        printf "${YELLOW}💡 请检查网络连接或手动下载${NC}\n"
        return 1
    fi
    
    # 验证下载的文件
    if [[ ! -f "sing-box.tar.gz" ]] || [[ $(stat -c%s "sing-box.tar.gz" 2>/dev/null || echo "0") -lt 1000 ]]; then
        printf "${RED}❌ 下载的文件无效${NC}\n"
        return 1
    fi
    
    printf "${BLUE}📦 解压安装包...${NC}\n"
    if ! tar -xzf sing-box.tar.gz; then
        printf "${RED}❌ 解压失败${NC}\n"
        return 1
    fi
    
    # 查找解压后的目录
    local extracted_dir
    extracted_dir=$(find . -name "sing-box-*" -type d | head -1)
    
    if [[ -z "$extracted_dir" ]] || [[ ! -d "$extracted_dir" ]]; then
        printf "${RED}❌ 无法找到解压后的目录${NC}\n"
        return 1
    fi
    
    # 验证二进制文件
    if [[ ! -f "$extracted_dir/sing-box" ]]; then
        printf "${RED}❌ 二进制文件不存在${NC}\n"
        return 1
    fi
    
    # 安装二进制文件
    printf "${BLUE}📥 安装二进制文件...${NC}\n"
    cp "$extracted_dir/sing-box" /usr/local/bin/
    chmod +x /usr/local/bin/sing-box
    
    # 创建符号链接
    ln -sf /usr/local/bin/sing-box /usr/bin/sing-box
    
    # 验证安装
    if ! command -v sing-box >/dev/null 2>&1; then
        printf "${RED}❌ 安装后仍无法找到 sing-box 命令${NC}\n"
        return 1
    fi
    
    # 测试版本
    local version_info
    version_info=$(sing-box version 2>/dev/null || echo "版本获取失败")
    printf "${GREEN}✅ 安装成功！版本: $version_info${NC}\n"
    
    # 创建系统用户
    printf "${BLUE}👤 创建系统用户...${NC}\n"
    if ! id sing-box >/dev/null 2>&1; then
        useradd -r -s /sbin/nologin sing-box 2>/dev/null || {
            printf "${YELLOW}⚠️  创建用户失败，将使用root运行${NC}\n"
        }
    fi
    
    # 创建必要目录
    printf "${BLUE}📁 创建配置目录...${NC}\n"
    mkdir -p /etc/sing-box
    mkdir -p /var/log/sing-box
    
    # 创建systemd服务文件
    printf "${BLUE}🛠️  创建系统服务...${NC}\n"
    cat > /etc/systemd/system/sing-box.service << 'EOF'
[Unit]
Description=sing-box service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target

[Service]
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SYS_PTRACE CAP_DAC_READ_SEARCH
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SYS_PTRACE CAP_DAC_READ_SEARCH
ExecStart=/usr/local/bin/sing-box run -c /etc/sing-box/config.json
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=10s
LimitNOFILE=infinity
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF
    
    # 重新加载systemd
    systemctl daemon-reload
    
    printf "${GREEN}🎉 Sing-Box 安装完成！${NC}\n"
    
    # 清理临时文件
    cd / && rm -rf "$temp_dir" 2>/dev/null || true
    
    return 0
}

# 主函数
main() {
    printf "${BLUE}=== CentOS Sing-Box 安装器 ===${NC}\n\n"
    
    # 检查root权限
    if [[ $EUID -ne 0 ]]; then
        printf "${RED}❌ 需要root权限运行此脚本${NC}\n"
        printf "${YELLOW}请使用: sudo $0${NC}\n"
        exit 1
    fi
    
    # 检查系统
    if ! command -v yum >/dev/null 2>&1 && ! command -v dnf >/dev/null 2>&1; then
        printf "${RED}❌ 此脚本仅适用于 CentOS/RHEL/Fedora 系统${NC}\n"
        exit 1
    fi
    
    # 安装必要工具
    printf "${BLUE}📦 安装必要工具...${NC}\n"
    if command -v yum >/dev/null 2>&1; then
        yum install -y curl wget tar gzip >/dev/null 2>&1
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y curl wget tar gzip >/dev/null 2>&1
    fi
    
    # 开始安装
    if install_singbox_centos; then
        printf "\n${GREEN}✅ 安装完成！${NC}\n"
        printf "${BLUE}💡 下一步：${NC}\n"
        printf "  1. 创建配置文件: /etc/sing-box/config.json\n"
        printf "  2. 启动服务: systemctl start sing-box\n"
        printf "  3. 设置开机自启: systemctl enable sing-box\n"
        printf "  4. 查看状态: systemctl status sing-box\n"
    else
        printf "\n${RED}❌ 安装失败${NC}\n"
        exit 1
    fi
}

main "$@"