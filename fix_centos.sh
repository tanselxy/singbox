#!/bin/bash

# =============================================================================
# CentOS 快速修复脚本
# 解决常见的安装卡顿问题
# =============================================================================

set -e

echo "🔧 CentOS 环境快速修复工具"
echo "================================"

# 检查是否为root
if [[ $EUID -ne 0 ]]; then
    echo "❌ 请使用 root 权限运行此脚本"
    exit 1
fi

# 检查系统
OS_ID=$(grep '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"' 2>/dev/null || echo "unknown")
echo "📋 检测到系统: $OS_ID"

if [[ ! "$OS_ID" =~ ^(centos|rhel|rocky|almalinux|fedora)$ ]]; then
    echo "❌ 此脚本仅适用于 CentOS/RHEL 系列系统"
    exit 1
fi

# 确定包管理器
if command -v dnf >/dev/null 2>&1; then
    PKG_MGR="dnf"
else
    PKG_MGR="yum"
fi

echo "📦 使用包管理器: $PKG_MGR"

# 清理包管理器缓存
echo "🧹 清理包管理器缓存..."
$PKG_MGR clean all >/dev/null 2>&1 || true

# 更新系统
echo "🔄 更新系统包列表..."
timeout 300 $PKG_MGR update -y >/dev/null 2>&1 || {
    echo "⚠️ 系统更新超时，但不影响后续安装"
}

# 安装基础工具
echo "🛠️ 安装基础工具..."
timeout 180 $PKG_MGR install -y curl wget tar gzip >/dev/null 2>&1 || {
    echo "⚠️ 基础工具安装失败，可能影响后续步骤"
}

# 配置EPEL源
echo "📂 配置 EPEL 源..."
if ! rpm -q epel-release >/dev/null 2>&1; then
    # 方法1：通过包管理器安装
    timeout 120 $PKG_MGR install -y epel-release >/dev/null 2>&1 || {
        echo "⚠️ EPEL源安装超时，尝试备用方法..."
        
        # 方法2：直接下载RPM包
        if [[ "$OS_ID" == "centos" ]]; then
            if grep -q "release 7" /etc/centos-release 2>/dev/null; then
                EPEL_URL="https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm"
            else
                EPEL_URL="https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm"
            fi
            
            timeout 60 rpm -ivh "$EPEL_URL" >/dev/null 2>&1 || echo "⚠️ EPEL备用安装也失败"
        fi
    }
else
    echo "✅ EPEL源已存在"
fi

# 测试网络连接
echo "🌐 测试网络连接..."
if ping -c 1 -W 5 8.8.8.8 >/dev/null 2>&1; then
    echo "✅ 网络连接正常"
else
    echo "❌ 网络连接异常，可能影响下载"
fi

# 禁用SELinux（临时）
echo "🔒 临时禁用 SELinux..."
setenforce 0 2>/dev/null || true

# 停止可能冲突的服务
echo "⏹️ 停止可能冲突的服务..."
systemctl stop firewalld 2>/dev/null || true

# 优化DNS
echo "🔍 优化 DNS 配置..."
cat > /etc/resolv.conf <<EOF
nameserver 8.8.8.8
nameserver 1.1.1.1
nameserver 114.114.114.114
EOF

# 设置时区
echo "🕐 设置时区..."
timedatectl set-timezone Asia/Shanghai 2>/dev/null || true

# 安装必要的依赖
echo "📋 安装必要依赖..."
DEPS="openssl curl lsof netstat-nat bind-utils"
for dep in $DEPS; do
    timeout 60 $PKG_MGR install -y "$dep" >/dev/null 2>&1 || echo "⚠️ $dep 安装失败"
done

echo ""
echo "🎉 CentOS 环境修复完成！"
echo "================================"
echo "现在可以重新运行 Sing-Box 安装脚本："
echo "sudo ./install.sh install"
echo ""
echo "如果仍有问题，可以跳过 fail2ban："
echo "- 编辑 install.sh"
echo "- 注释掉 setup_fail2ban 这一行"