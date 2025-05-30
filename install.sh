#!/bin/bash

# =============================================================================
# Sing-Box 自动安装部署脚本 - 修复版
# =============================================================================

# 第一步：立即定义所有颜色变量，避免unbound variable错误
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
CYAN='\033[0;36m'
WHITE='\033[0;37m'
MAGENTA='\033[0;35m'
BLACK='\033[0;30m'
BOLD_WHITE='\033[1;37m'
PURPLE='\033[0;35m'

# 第二步：设置bash选项
set -euo pipefail

# 第三步：显示启动信息
printf "${BLUE}🚀 Sing-Box 安装脚本启动中...${NC}\n"

# 第四步：检测执行方式
IS_PIPED_EXECUTION=false
if [[ ! -f "${BASH_SOURCE[0]:-}" ]] || [[ "${0}" == "bash" ]]; then
    IS_PIPED_EXECUTION=true
    printf "${YELLOW}💡 检测到管道执行模式${NC}\n"
fi

# 第五步：设置目录
if [[ "$IS_PIPED_EXECUTION" == true ]]; then
    SCRIPT_DIR="/tmp/singbox-install-$$"
    mkdir -p "$SCRIPT_DIR"
    printf "${BLUE}📁 工作目录: $SCRIPT_DIR${NC}\n"
else
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
    printf "${GREEN}📁 本地模式，脚本目录: $SCRIPT_DIR${NC}\n"
fi

# 配置信息
SCRIPT_VERSION="2.1"
LOG_FILE="/var/log/singbox-deploy.log"
CONFIG_DIR="/etc/sing-box"
CERT_DIR="$CONFIG_DIR/cert"
TEMP_DIR="/tmp/singbox-deploy-$$"

# GitHub仓库配置
GITHUB_USER="tanselxy"
REPO_NAME="singbox"
BRANCH="main"
BASE_URL="https://raw.githubusercontent.com/$GITHUB_USER/$REPO_NAME/$BRANCH"

# 需要下载的文件
REQUIRED_FILES=("utils.sh" "network.sh" "config.sh" "server_template.json" "client_template.yaml")

# 下载函数
download_file() {
    local file="$1"
    local target="$2"
    local url="$BASE_URL/$file"
    
    printf "${BLUE}📥 下载 $file...${NC}\n"
    
    # 尝试多个下载源
    if curl -fsSL --connect-timeout 10 "$url" -o "$target" 2>/dev/null; then
        printf "${GREEN}✅ $file 下载成功${NC}\n"
        return 0
    elif curl -fsSL --connect-timeout 10 "https://ghproxy.com/$url" -o "$target" 2>/dev/null; then
        printf "${GREEN}✅ $file 下载成功(代理)${NC}\n"
        return 0
    elif curl -fsSL --connect-timeout 10 "https://cdn.jsdelivr.net/gh/$GITHUB_USER/$REPO_NAME@$BRANCH/$file" -o "$target" 2>/dev/null; then
        printf "${GREEN}✅ $file 下载成功(CDN)${NC}\n"
        return 0
    else
        printf "${RED}❌ $file 下载失败${NC}\n"
        return 1
    fi
}

# 下载所有依赖
download_dependencies() {
    printf "${YELLOW}📦 开始下载依赖文件...${NC}\n\n"
    
    local success=0
    local total=${#REQUIRED_FILES[@]}
    
    for file in "${REQUIRED_FILES[@]}"; do
        local target="$SCRIPT_DIR/$file"
        
        if download_file "$file" "$target"; then
            # 给shell脚本执行权限
            if [[ "$file" == *.sh ]]; then
                chmod +x "$target"
            fi
            success=$((success + 1))
        fi
    done
    
    printf "\n${BLUE}📊 下载结果: $success/$total${NC}\n"
    
    if [[ $success -eq $total ]]; then
        printf "${GREEN}🎉 所有依赖文件下载完成！${NC}\n"
        return 0
    else
        printf "${RED}❌ 部分文件下载失败${NC}\n"
        return 1
    fi
}

# 简化的安装函数
deploy_fresh_install() {
    printf "${GREEN}🚀 开始 Sing-Box 全新安装...${NC}\n\n"
    
    # 检查root权限
    if [[ $EUID -ne 0 ]]; then
        printf "${RED}❌ 此脚本需要root权限运行${NC}\n"
        printf "${YELLOW}请使用: sudo $0${NC}\n"
        return 1
    fi
    
    printf "${BLUE}✅ Root权限检查通过${NC}\n"
    
    # 检测系统
    if command -v apt-get >/dev/null 2>&1; then
        PACKAGE_MANAGER="apt"
        printf "${GREEN}🐧 检测到 Ubuntu/Debian 系统${NC}\n"
    elif command -v yum >/dev/null 2>&1; then
        PACKAGE_MANAGER="yum"
        printf "${GREEN}🎩 检测到 CentOS/RHEL 系统${NC}\n"
    elif command -v dnf >/dev/null 2>&1; then
        PACKAGE_MANAGER="dnf"
        printf "${GREEN}🎩 检测到 Fedora 系统${NC}\n"
    else
        printf "${RED}❌ 不支持的系统${NC}\n"
        return 1
    fi
    
    # 更新系统
    printf "${BLUE}📦 更新系统软件包...${NC}\n"
    export DEBIAN_FRONTEND=noninteractive
    
    case "$PACKAGE_MANAGER" in
        "apt")
            apt-get update >/dev/null 2>&1
            apt-get install -y curl wget unzip >/dev/null 2>&1
            ;;
        "yum"|"dnf")
            $PACKAGE_MANAGER update -y >/dev/null 2>&1
            $PACKAGE_MANAGER install -y curl wget unzip >/dev/null 2>&1
            ;;
    esac
    
    printf "${GREEN}✅ 系统更新完成${NC}\n"
    
    # 安装sing-box
    printf "${BLUE}📥 安装 Sing-Box...${NC}\n"
    
    case "$PACKAGE_MANAGER" in
        "apt")
            if curl -fsSL https://sing-box.app/deb-install.sh | bash; then
                printf "${GREEN}✅ Sing-Box 安装成功${NC}\n"
            else
                printf "${RED}❌ Sing-Box 安装失败${NC}\n"
                return 1
            fi
            ;;
        *)
            printf "${YELLOW}⚠️  暂不支持自动安装，请手动安装 Sing-Box${NC}\n"
            ;;
    esac
    
    printf "${GREEN}🎉 安装完成！${NC}\n"
    printf "${BLUE}💡 接下来请配置 Sing-Box 服务${NC}\n"
}

# 主函数
main() {
    printf "${CYAN}"
    printf "╭─────────────────────────────────────────────╮\n"
    printf "│          Sing-Box 自动部署脚本 v%s        │\n" "$SCRIPT_VERSION"
    printf "│     https://github.com/tanselxy/singbox     │\n"
    printf "╰─────────────────────────────────────────────╯\n"
    printf "${NC}\n"
    
    # 下载依赖文件
    if ! download_dependencies; then
        printf "${RED}❌ 依赖下载失败，无法继续${NC}\n"
        return 1
    fi
    
    printf "\n${GREEN}🎉 脚本准备完成，所有依赖已就绪！${NC}\n\n"
    
    # 参数处理
    if [[ $# -gt 0 ]]; then
        case "$1" in
            install|deploy)
                printf "${BLUE}🚀 执行安装命令...${NC}\n"
                deploy_fresh_install
                ;;
            *)
                printf "${RED}❌ 未知参数: $1${NC}\n"
                printf "${YELLOW}用法: $0 [install|deploy]${NC}\n"
                return 1
                ;;
        esac
        return 0
    fi
    
    # 无参数时的处理
    if [[ "$IS_PIPED_EXECUTION" == true ]]; then
        printf "${YELLOW}❓ 是否立即开始安装 Sing-Box？${NC}\n"
        printf "${BLUE}输入 y 开始安装，5秒后自动开始 [Y/n]: ${NC}"
        
        local choice=""
        if read -t 5 -r choice 2>/dev/null || true; then
            choice=${choice:-y}
        else
            choice="y"
            printf "\n${YELLOW}⏰ 超时，自动开始安装${NC}\n"
        fi
        
        if [[ "$choice" =~ ^[Yy]$ ]] || [[ -z "$choice" ]]; then
            printf "\n${GREEN}🚀 开始自动安装...${NC}\n\n"
            deploy_fresh_install
        else
            printf "\n${BLUE}❌ 用户取消安装${NC}\n"
            printf "${YELLOW}💡 如需安装，请运行:${NC}\n"
            printf "${WHITE}cd $SCRIPT_DIR && ./install.sh install${NC}\n"
        fi
    else
        printf "${BLUE}💡 运行 './install.sh install' 开始安装${NC}\n"
        printf "${YELLOW}📖 用法: $0 [install|deploy]${NC}\n"
    fi
}

# 清理函数
cleanup() {
    if [[ "$IS_PIPED_EXECUTION" == true ]] && [[ -d "$SCRIPT_DIR" ]]; then
        printf "\n${YELLOW}🧹 清理临时文件...${NC}\n"
        # 保留文件供用户查看
        printf "${BLUE}💾 临时文件保存在: $SCRIPT_DIR${NC}\n"
    fi
}

trap cleanup EXIT

# 执行主函数
main "$@"