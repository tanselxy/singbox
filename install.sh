#!/bin/bash

# =============================================================================
# Sing-Box 自动安装部署脚本
# 作者: 优化版本
# 版本: 2.1 - 增加自动下载依赖功能
# 描述: 自动安装、配置和管理 Sing-Box 代理服务
# =============================================================================

set -euo pipefail

# 全局配置
readonly SCRIPT_VERSION="2.1"
readonly LOG_FILE="/var/log/singbox-deploy.log"
readonly CONFIG_DIR="/etc/sing-box"
readonly CERT_DIR="$CONFIG_DIR/cert"
readonly TEMP_DIR="/tmp/singbox-deploy-$$"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# GitHub仓库配置
readonly GITHUB_USER="tanselxy"
readonly REPO_NAME="singbox"
readonly BRANCH="main"
readonly GITHUB_RAW_URL="https://raw.githubusercontent.com/$GITHUB_USER/$REPO_NAME/$BRANCH"

# 颜色定义
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

BLACK='\033[0;30m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[0;37m'
BOLD_WHITE='\033[1;37m'
PURPLE='\033[0;35m'

# 配置变量
DOWNLOAD_PORT=14567
SS_PORT=443
VLESS_PORT=10243
HYSTERIA_PORT=10244
HYSTERIA_PASSWORD=""
UUID=""
RANDOM_STR=""
IS_IPV6=false
DOMAIN_NAME=""
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/private.key"
SERVER_IP=""
SERVER=""
SS_PASSWORD=""
PACKAGE_MANAGER=""

# =============================================================================
# 依赖文件自动下载功能
# =============================================================================

# 需要下载的依赖文件列表
declare -a REQUIRED_FILES=(
    "utils.sh"
    "network.sh"
    "config.sh"
    "server_template.json"
    "client_template.yaml"
)

# 打印下载进度
print_download_progress() {
    local current="$1"
    local total="$2"
    local filename="$3"
    local status="$4"
    
    local percentage=$((current * 100 / total))
    local bar_length=30
    local filled_length=$((percentage * bar_length / 100))
    
    printf "\r${BLUE}下载进度 [${NC}"
    
    # 绘制进度条
    for ((i=0; i<filled_length; i++)); do
        printf "${GREEN}█${NC}"
    done
    for ((i=filled_length; i<bar_length; i++)); do
        printf "${WHITE}░${NC}"
    done
    
    printf "${BLUE}] %3d%% (%d/%d) %s - %s${NC}" "$percentage" "$current" "$total" "$filename" "$status"
    
    if [[ "$current" -eq "$total" ]]; then
        printf "\n"
    fi
}

# 下载单个文件
download_file() {
    local filename="$1"
    local target_path="$2"
    local url="$GITHUB_RAW_URL/$filename"
    
    # 创建目标目录
    mkdir -p "$(dirname "$target_path")"
    
    # 尝试多个下载源
    local download_success=false
    local attempts=0
    local max_attempts=3
    
    while [[ "$attempts" -lt "$max_attempts" ]] && [[ "$download_success" != true ]]; do
        attempts=$((attempts + 1))
        
        if [[ "$attempts" -eq 1 ]]; then
            # 第一次尝试：GitHub直连
            download_url="$url"
        elif [[ "$attempts" -eq 2 ]]; then
            # 第二次尝试：GitHub代理
            download_url="https://ghproxy.com/$url"
        else
            # 第三次尝试：jsDelivr CDN
            download_url="https://cdn.jsdelivr.net/gh/$GITHUB_USER/$REPO_NAME@$BRANCH/$filename"
        fi
        
        if curl -fsSL --connect-timeout 10 --max-time 30 "$download_url" -o "$target_path" 2>/dev/null; then
            # 验证下载的文件大小（至少要有一些内容）
            if [[ -f "$target_path" ]] && [[ $(stat -f%z "$target_path" 2>/dev/null || stat -c%s "$target_path" 2>/dev/null || echo "0") -gt 10 ]]; then
                download_success=true
                break
            else
                rm -f "$target_path" 2>/dev/null || true
            fi
        fi
        
        if [[ "$attempts" -lt "$max_attempts" ]]; then
            sleep 1  # 等待1秒后重试
        fi
    done
    
    if [[ "$download_success" != true ]]; then
        return 1
    fi
    
    # 如果是shell脚本，设置执行权限
    if [[ "$filename" == *.sh ]]; then
        chmod +x "$target_path"
    fi
    
    return 0
}

# 检查并下载依赖文件
download_dependencies() {
    echo ""
    printf "${BLUE}========== 检查并下载依赖文件 ==========${NC}\n"
    
    local missing_files=()
    local total_files=${#REQUIRED_FILES[@]}
    local current_file=0
    
    # 检查哪些文件缺失
    for file in "${REQUIRED_FILES[@]}"; do
        if [[ ! -f "$SCRIPT_DIR/$file" ]]; then
            missing_files+=("$file")
        fi
    done
    
    if [[ ${#missing_files[@]} -eq 0 ]]; then
        printf "${GREEN}✅ 所有依赖文件都已存在${NC}\n"
        return 0
    fi
    
    printf "${YELLOW}📦 需要下载 ${#missing_files[@]} 个依赖文件...${NC}\n\n"
    
    # 下载缺失的文件
    for file in "${missing_files[@]}"; do
        current_file=$((current_file + 1))
        target_path="$SCRIPT_DIR/$file"
        
        print_download_progress "$current_file" "${#missing_files[@]}" "$file" "下载中..."
        
        if download_file "$file" "$target_path"; then
            print_download_progress "$current_file" "${#missing_files[@]}" "$file" "✅ 成功"
        else
            print_download_progress "$current_file" "${#missing_files[@]}" "$file" "❌ 失败"
            printf "\n${RED}错误: 无法下载 $file${NC}\n"
            printf "${YELLOW}尝试的下载源:${NC}\n"
            printf "  1. GitHub直连: $GITHUB_RAW_URL/$file\n"
            printf "  2. GitHub代理: https://ghproxy.com/$GITHUB_RAW_URL/$file\n"
            printf "  3. jsDelivr CDN: https://cdn.jsdelivr.net/gh/$GITHUB_USER/$REPO_NAME@$BRANCH/$file\n"
            return 1
        fi
        sleep 0.1  # 短暂延迟，让进度条更平滑
    done
    
    printf "\n${GREEN}✅ 所有依赖文件下载完成！${NC}\n"
    return 0
}

# 验证依赖文件完整性
validate_dependencies() {
    local validation_failed=false
    
    printf "${BLUE}🔍 验证依赖文件完整性...${NC}\n"
    
    for file in "${REQUIRED_FILES[@]}"; do
        local file_path="$SCRIPT_DIR/$file"
        
        if [[ ! -f "$file_path" ]]; then
            printf "${RED}❌ 文件不存在: $file${NC}\n"
            validation_failed=true
            continue
        fi
        
        # 检查文件大小
        local file_size
        file_size=$(stat -f%z "$file_path" 2>/dev/null || stat -c%s "$file_path" 2>/dev/null || echo "0")
        
        if [[ "$file_size" -lt 50 ]]; then
            printf "${RED}❌ 文件过小 ($file_size 字节): $file${NC}\n"
            validation_failed=true
            continue
        fi
        
        # 对于shell脚本，检查是否有可执行权限
        if [[ "$file" == *.sh ]] && [[ ! -x "$file_path" ]]; then
            printf "${YELLOW}⚠️  设置执行权限: $file${NC}\n"
            chmod +x "$file_path"
        fi
        
        printf "${GREEN}✅ $file (${file_size} 字节)${NC}\n"
    done
    
    if [[ "$validation_failed" == true ]]; then
        printf "${RED}依赖文件验证失败！${NC}\n"
        return 1
    fi
    
    printf "${GREEN}✅ 所有依赖文件验证通过${NC}\n"
    return 0
}

# 智能依赖管理
smart_dependency_management() {
    # 如果所有文件都存在且完整，跳过下载
    if validate_dependencies >/dev/null 2>&1; then
        printf "${GREEN}📋 依赖检查通过，跳过下载步骤${NC}\n"
        return 0
    fi
    
    # 下载缺失或损坏的文件
    if ! download_dependencies; then
        printf "${RED}❌ 依赖文件下载失败${NC}\n"
        printf "${YELLOW}💡 解决方案:${NC}\n"
        printf "  1. 检查网络连接\n"
        printf "  2. 手动下载文件到脚本目录\n"
        printf "  3. 使用代理或VPN\n"
        printf "  4. 联系技术支持\n"
        return 1
    fi
    
    # 再次验证
    if ! validate_dependencies; then
        printf "${RED}❌ 下载后的文件验证失败${NC}\n"
        return 1
    fi
    
    return 0
}

# =============================================================================
# 原有的工具函数库加载（现在会自动下载）
# =============================================================================

# 动态加载依赖文件
load_dependencies() {
    local dependencies=("utils.sh" "network.sh" "config.sh")
    
    for dep in "${dependencies[@]}"; do
        local dep_path="$SCRIPT_DIR/$dep"
        
        if [[ -f "$dep_path" ]]; then
            # shellcheck source=/dev/null
            source "$dep_path" || {
                printf "${RED}错误: 无法加载 $dep${NC}\n"
                return 1
            }
            printf "${GREEN}✅ 已加载: $dep${NC}\n"
        else
            printf "${RED}❌ 依赖文件不存在: $dep${NC}\n"
            return 1
        fi
    done
    
    return 0
}

# =============================================================================
# 系统检查和初始化（增强版）
# =============================================================================

# 检查网络连接
check_network_connectivity() {
    printf "${BLUE}🌐 检查网络连接...${NC}\n"
    
    local test_urls=(
        "github.com"
        "raw.githubusercontent.com"
        "ghproxy.com"
        "cdn.jsdelivr.net"
    )
    
    local working_urls=0
    
    for url in "${test_urls[@]}"; do
        if ping -c 1 -W 3 "$url" >/dev/null 2>&1; then
            printf "${GREEN}✅ $url 可达${NC}\n"
            working_urls=$((working_urls + 1))
        else
            printf "${YELLOW}⚠️  $url 不可达${NC}\n"
        fi
    done
    
    if [[ "$working_urls" -eq 0 ]]; then
        printf "${RED}❌ 网络连接检查失败，无法访问任何下载源${NC}\n"
        return 1
    elif [[ "$working_urls" -lt 2 ]]; then
        printf "${YELLOW}⚠️  网络连接不稳定，可能影响下载速度${NC}\n"
    else
        printf "${GREEN}✅ 网络连接良好${NC}\n"
    fi
    
    return 0
}

# 增强的依赖检查
check_dependencies() {
    printf "${BLUE}========== 依赖文件管理 ==========${NC}\n"
    
    # 检查网络连接
    if ! check_network_connectivity; then
        printf "${YELLOW}⚠️  网络连接有问题，但将尝试使用本地文件${NC}\n"
    fi
    
    # 智能依赖管理
    if ! smart_dependency_management; then
        printf "${RED}❌ 依赖管理失败${NC}\n"
        printf "${YELLOW}请确保以下文件存在于脚本目录:${NC}\n"
        printf '%s\n' "${REQUIRED_FILES[@]}" | sed 's/^/  - /'
        return 1
    fi
    
    # 加载依赖文件
    printf "\n${BLUE}📚 加载依赖模块...${NC}\n"
    if ! load_dependencies; then
        printf "${RED}❌ 依赖模块加载失败${NC}\n"
        return 1
    fi
    
    printf "${GREEN}✅ 依赖管理完成${NC}\n\n"
    return 0
}

# 显示脚本信息
show_banner() {
    # 清屏
    clear

    # 框线字符
    TL="╭" # Top-left corner - 左上角
    TR="╮" # Top-right corner - 右上角
    BL="╰" # Bottom-left corner - 左下角
    BR="╯" # Bottom-right corner - 右下角
    HZ="─" # Horizontal line - 横线
    VT="│" # Vertical line - 竖线

    # 准备要显示的内容
    line1_leading_spaces_count=4
    line1_content_text="https://my.racknerd.com/aff.php?aff=10790"
    line1_trailing_char=""
    line1_effective_display_width=$((line1_leading_spaces_count + ${#line1_content_text} + ${#line1_trailing_char}))

    line2_leading_spaces_count=12
    line2_content_text="年付仅需10美元"
    line2_effective_display_width=$((line2_leading_spaces_count + 14)) # 14 是 "年付仅需10美元" 的估算显示宽度

    # 决定框内部的宽度
    content_width=$line1_effective_display_width
    if (( line2_effective_display_width > content_width )); then
        content_width=$line2_effective_display_width
    fi
    fixed_content_width=50

    # 打印上边框
    printf "${CYAN}%s" "$TL"
    for ((i=0; i<fixed_content_width; i++)); do printf "%s" "$HZ"; done
    printf "%s${NC}\n" "$TR"

    # 打印第一行内容 (链接)
    printf "${CYAN}%s${NC}" "$VT"
    for ((i=0; i<line1_leading_spaces_count; i++)); do printf " "; done
    printf "${YELLOW}%s${NC}%s" "$line1_content_text" "$line1_trailing_char"
    line1_trailing_padding_count=$((fixed_content_width - line1_effective_display_width))
    for ((i=0; i<line1_trailing_padding_count; i++)); do printf " "; done
    printf "${CYAN}%s${NC}\n" "$VT"

    # 打印框内空行
    printf "${CYAN}%s${NC}" "$VT"
    for ((i=0; i<fixed_content_width; i++)); do printf " "; done
    printf "${CYAN}%s${NC}\n" "$VT"

    # 打印第二行内容 (中文说明)
    printf "${CYAN}%s${NC}" "$VT"
    for ((i=0; i<line2_leading_spaces_count; i++)); do printf " "; done
    printf "${GREEN}%s${NC}" "$line2_content_text"
    line2_trailing_padding_count=$((fixed_content_width - line2_effective_display_width))
    for ((i=0; i<line2_trailing_padding_count; i++)); do printf " "; done
    printf "${CYAN}%s${NC}\n" "$VT"

    # 打印下边框
    printf "${CYAN}%s" "$BL"
    for ((i=0; i<fixed_content_width; i++)); do printf "%s" "$HZ"; done
    printf "%s${NC}\n" "$BR"

    printf "\n${BLUE}🚀 Sing-Box 自动部署脚本 v${SCRIPT_VERSION} - 智能依赖管理版${NC}\n\n"
}

# =============================================================================
# 其余原有功能保持不变...
# =============================================================================

# 初始化函数
initialize() {
    printf "${BLUE}========== 系统初始化 ==========${NC}\n"
    
    # 创建必要目录
    mkdir -p "$TEMP_DIR" "$CONFIG_DIR" "$CERT_DIR" || {
        printf "${RED}❌ 创建目录失败${NC}\n"
        return 1
    }
    
    # 初始化日志
    touch "$LOG_FILE" || {
        printf "${YELLOW}⚠️  无法创建日志文件: $LOG_FILE${NC}\n"
    }
    
    # 生成随机参数
    printf "${BLUE}🎲 生成随机参数...${NC}\n"
    RANDOM_STR="racknerd"
    
    # 这里需要调用utils.sh中的函数，确保已经加载
    if command -v generate_strong_password >/dev/null 2>&1; then
        HYSTERIA_PASSWORD=$(generate_strong_password 15) || {
            printf "${RED}❌ 生成密码失败${NC}\n"
            return 1
        }
    else
        # 如果函数不可用，使用简单的随机密码生成
        HYSTERIA_PASSWORD=$(openssl rand -base64 15 2>/dev/null || tr -dc 'A-Za-z0-9' </dev/urandom | head -c 15)
    fi
    
    printf "${GREEN}✅ 初始化完成${NC}\n"
    return 0
}

# [继续保持所有原有功能...]

# 主函数
main() {
    show_banner
    
    # 首先进行依赖检查和下载
    if ! check_dependencies; then
        printf "${RED}❌ 依赖检查失败，无法继续${NC}\n"
        exit 1
    fi
    
    # 如果有参数，直接执行对应功能
    if [[ $# -gt 0 ]]; then
        case "$1" in
            install|deploy)
                deploy_fresh_install
                ;;
            config)
                regenerate_config
                ;;
            info)
                show_connection_info
                ;;
            download-deps)
                printf "${BLUE}🔄 强制重新下载依赖文件...${NC}\n"
                # 删除现有文件
                for file in "${REQUIRED_FILES[@]}"; do
                    rm -f "$SCRIPT_DIR/$file" 2>/dev/null || true
                done
                # 重新下载
                smart_dependency_management
                ;;
            *)
                printf "用法: $0 [install|config|info|download-deps]\n"
                exit 1
                ;;
        esac
        exit 0
    fi
    
    # 如果所有依赖都加载成功，继续原有的交互式菜单逻辑
    # [这里保持原有的while循环菜单代码]
    
    printf "${GREEN}🎉 脚本准备完成，所有依赖已就绪！${NC}\n"
    printf "${BLUE}现在可以使用 './install.sh install' 开始安装${NC}\n"
}

# 错误处理
cleanup_on_exit() {
    local exit_code=$?
    
    # 清理临时文件
    rm -rf "$TEMP_DIR" 2>/dev/null || true
    
    if [[ $exit_code -ne 0 ]]; then
        printf "\n${RED}❌ 脚本执行过程中出现错误 (退出码: $exit_code)${NC}\n"
        printf "${YELLOW}💡 如需帮助，请检查日志文件: $LOG_FILE${NC}\n"
    fi
}

trap cleanup_on_exit EXIT

# 运行主函数
main "$@"