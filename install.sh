#!/bin/bash

# =============================================================================
# Sing-Box 自动安装部署脚本
# 作者: 优化版本
# 版本: 2.0
# 描述: 自动安装、配置和管理 Sing-Box 代理服务
# =============================================================================

set -euo pipefail

# 全局配置
readonly SCRIPT_VERSION="2.0"
readonly LOG_FILE="/var/log/singbox-deploy.log"
readonly CONFIG_DIR="/etc/sing-box"
readonly CERT_DIR="$CONFIG_DIR/cert"
readonly TEMP_DIR="/tmp/singbox-deploy-$$"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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
# 工具函数库
# =============================================================================

source "$SCRIPT_DIR/utils.sh" 2>/dev/null || {
    echo "错误: 无法加载工具函数库 utils.sh"
    exit 1
}

source "$SCRIPT_DIR/network.sh" 2>/dev/null || {
    echo "错误: 无法加载网络函数库 network.sh"
    exit 1
}

source "$SCRIPT_DIR/config.sh" 2>/dev/null || {
    echo "错误: 无法加载配置函数库 config.sh"
    exit 1
}

# =============================================================================
# 系统检查和初始化
# =============================================================================

# 检查依赖文件
check_dependencies() {
    local missing_files=()
    
    [[ ! -f "$SCRIPT_DIR/utils.sh" ]] && missing_files+=("utils.sh")
    [[ ! -f "$SCRIPT_DIR/network.sh" ]] && missing_files+=("network.sh") 
    [[ ! -f "$SCRIPT_DIR/config.sh" ]] && missing_files+=("config.sh")
    [[ ! -f "$SCRIPT_DIR/server_template.json" ]] && missing_files+=("server_template.json")
    [[ ! -f "$SCRIPT_DIR/client_template.yaml" ]] && missing_files+=("client_template.yaml")
    
    if [[ ${#missing_files[@]} -gt 0 ]]; then
        print_error "缺少以下依赖文件:"
        printf ' - %s\n' "${missing_files[@]}"
        error_exit "请确保所有文件都在同一目录下"
    fi
}

# 显示脚本信息
# Define Colors



# 定义 ANSI 颜色代码
# 如果您的脚本中已经有这些颜色变量 (如 $BLUE, $GREEN 等)
# 并且有 print_colored 函数，您可以调整下面的代码以使用它们。
# 这里为了独立性和精确控制单个字符颜色，我将直接使用 echo -e。


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
    # 第一行：链接
    # 原始格式："    ${YELLOW}https://my.racknerd.com/aff.php?aff=10790${NC}║"
    # 前导空格数: 4
    # URL+║ 文本: "https://my.racknerd.com/aff.php?aff=10790║" (长度 44)
    # 整行有效内容显示长度 (包括前导空格和结尾的║): 4 + 44 = 48
    line1_leading_spaces_count=4
    line1_content_text="https://my.racknerd.com/aff.php?aff=10790&pid=912" # 不包括║，方便添加颜色
    line1_trailing_char=""
    line1_effective_display_width=$((line1_leading_spaces_count + ${#line1_content_text} + ${#line1_trailing_char}))


    # 第二行：中文说明
    # 原始格式："            ${GREEN}年付仅需10美元${NC}"
    # 前导空格数: 12
    # 中文+数字文本: "年付仅需10美元"
    #   假设中文每个字占2个显示单元，数字/字母占1个。
    #   年(2)付(2)仅(2)需(2)1(1)0(1)美(2)元(2) = 14 个显示单元宽度
    # 整行有效内容显示长度 (包括前导空格): 12 + 14 = 26
    line2_leading_spaces_count=12
    line2_content_text="年付仅需10美元"
    line2_effective_display_width=$((line2_leading_spaces_count + 14)) # 14 是 "年付仅需10美元" 的估算显示宽度

    # 决定框内部的宽度，以最长的那一行内容为基准，再加一点点缀空间
    content_width=$line1_effective_display_width
    if (( line2_effective_display_width > content_width )); then
        content_width=$line2_effective_display_width
    fi
    # 可以稍微增加一点宽度，让内容不至于太挤
    # content_width=$((content_width + 2)) # 例如，左右各增加1个空格的内边距
    # 或者，我们可以设定一个固定的期望宽度，比如50或52，然后计算两端填充
    # 这里我们使用一个固定的内部宽度，以使得两行文本右侧对齐（通过填充空格）
    fixed_content_width=50 # 您可以调整这个值

    # 打印上边框
    printf "${CYAN}%s" "$TL"
    for ((i=0; i<fixed_content_width; i++)); do printf "%s" "$HZ"; done
    printf "%s${NC}\n" "$TR"

    # 打印第一行内容 (链接)
    printf "${CYAN}%s${NC}" "$VT"
    # 打印前导空格
    for ((i=0; i<line1_leading_spaces_count; i++)); do printf " "; done
    # 打印带颜色的链接和结尾字符
    printf "${YELLOW}%s${NC}%s" "$line1_content_text" "$line1_trailing_char"
    # 计算并打印末尾填充空格
    line1_trailing_padding_count=$((fixed_content_width - line1_effective_display_width))
    for ((i=0; i<line1_trailing_padding_count; i++)); do printf " "; done
    printf "${CYAN}%s${NC}\n" "$VT"

    # 打印框内空行（用于分隔）
    printf "${CYAN}%s${NC}" "$VT"
    for ((i=0; i<fixed_content_width; i++)); do printf " "; done
    printf "${CYAN}%s${NC}\n" "$VT"

    # 打印第二行内容 (中文说明)
    printf "${CYAN}%s${NC}" "$VT"
    # 打印前导空格
    for ((i=0; i<line2_leading_spaces_count; i++)); do printf " "; done
    # 打印带颜色的中文说明
    printf "${GREEN}%s${NC}" "$line2_content_text"
    # 计算并打印末尾填充空格
    line2_trailing_padding_count=$((fixed_content_width - line2_effective_display_width))
    for ((i=0; i<line2_trailing_padding_count; i++)); do printf " "; done
    printf "${CYAN}%s${NC}\n" "$VT"

    # 打印下边框
    printf "${CYAN}%s" "$BL"
    for ((i=0; i<fixed_content_width; i++)); do printf "%s" "$HZ"; done
    printf "%s${NC}\n" "$BR"

    # 您原来版本中信息块之后的空行，如果需要，可以在调用此函数后再用 echo 添加
    

}



# 初始化函数
initialize() {
    log_info "========== Sing-Box 部署开始 =========="
    
    # 创建必要目录
    mkdir -p "$TEMP_DIR" "$CONFIG_DIR" "$CERT_DIR" || {
        log_error "创建目录失败"
        return 1
    }
    
    # 初始化日志
    touch "$LOG_FILE" || {
        echo "无法创建日志文件: $LOG_FILE"
        return 1
    }
    
    # 生成随机参数
    log_info "生成随机参数..."
    RANDOM_STR="racknerd" || {
        log_error "生成随机字符串失败"
        return 1
    }
    
    HYSTERIA_PASSWORD=$(generate_strong_password 15) || {
        log_error "生成密码失败"
        return 1
    }
    
    # 获取可用端口
    log_info "分配可用端口..."
    VLESS_PORT=$(get_available_port 20000 20010 2>>"$LOG_FILE") || {
        log_error "获取VLESS端口失败"
        return 1
    }
    
    SS_PORT=$(get_available_port 31000 31010 2>>"$LOG_FILE") || {
        log_error "获取SS端口失败"  
        return 1
    }
    
    HYSTERIA_PORT=$(get_available_port 50000 50010 2>>"$LOG_FILE") || {
        log_error "获取Hysteria端口失败"
        return 1
    }
    
    log_info "初始化完成"
    log_info "VLESS端口: $VLESS_PORT"
    log_info "SS端口: $SS_PORT"
    log_info "Hysteria端口: $HYSTERIA_PORT"
    
    return 0
}

# =============================================================================
# 软件安装函数
# =============================================================================

# 更新系统并安装Sing-Box
install_singbox() {
    #log_info "更新系统软件包,请耐心等待1-2分钟..."
    export DEBIAN_FRONTEND=noninteractive
    
    # case "$PACKAGE_MANAGER" in
    #     "apt")
    #         apt-get update >/dev/null 2>&1
    #         ;;
    #     "yum"|"dnf")
    #         $PACKAGE_MANAGER update -y >/dev/null 2>&1
    #         ;;
    # esac
    
    log_info "设置系统时区为上海..."
    timedatectl set-timezone Asia/Shanghai 2>/dev/null || true
    
    log_info "安装 Sing-Box..."
    
    # 根据系统选择安装方式
    case "$PACKAGE_MANAGER" in
        "apt")
            if ! curl -fsSL https://sing-box.app/deb-install.sh | bash; then
                error_exit "Sing-Box 安装失败"
            fi
            ;;
        "yum"|"dnf")
            # CentOS/RHEL 手动安装
            install_singbox_manual
            ;;
    esac
    
    install_self_signed_cert
}

# 手动安装Sing-Box（用于CentOS/RHEL）
install_singbox_manual() {
    log_info "手动安装 Sing-Box..."
    
    # 检测系统架构
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64) arch="arm64" ;;
        armv7l) arch="armv7" ;;
        *) error_exit "不支持的架构: $arch" ;;
    esac
    
    # 获取最新版本
    log_info "获取最新版本信息..."
    local latest_version
    latest_version=$(curl -s https://api.github.com/repos/SagerNet/sing-box/releases/latest | grep -oP '"tag_name": "\K[^"]+' 2>/dev/null || echo "v1.8.0")
    
    log_info "下载 Sing-Box $latest_version ($arch)..."
    
    # 确保在临时目录
    cd "$TEMP_DIR" || error_exit "无法切换到临时目录"
    
    # 下载URL
    local download_url="https://github.com/SagerNet/sing-box/releases/download/$latest_version/sing-box-${latest_version#v}-linux-$arch.tar.gz"
    
    # 尝试多个下载源
    local downloaded=false
    
    # 源1: GitHub 直接下载
    if curl -L -o "sing-box.tar.gz" "$download_url" 2>/dev/null; then
        downloaded=true
        log_info "从 GitHub 下载成功"
    else
        log_warn "GitHub 下载失败，尝试镜像源..."
        
        # 源2: GitHub 代理
        local proxy_url="https://ghproxy.com/https://github.com/SagerNet/sing-box/releases/download/$latest_version/sing-box-${latest_version#v}-linux-$arch.tar.gz"
        if curl -L -o "sing-box.tar.gz" "$proxy_url" 2>/dev/null; then
            downloaded=true
            log_info "从代理源下载成功"
        else
            log_warn "代理源下载失败，尝试备用版本..."
            
            # 源3: 固定版本下载
            local backup_url="https://github.com/SagerNet/sing-box/releases/download/v1.8.0/sing-box-1.8.0-linux-$arch.tar.gz"
            if curl -L -o "sing-box.tar.gz" "$backup_url" 2>/dev/null; then
                downloaded=true
                log_info "从备用源下载成功"
            fi
        fi
    fi
    
    if [[ "$downloaded" != true ]]; then
        error_exit "所有下载源都失败，无法下载 Sing-Box"
    fi
    
    # 验证下载的文件
    if [[ ! -f "sing-box.tar.gz" ]] || [[ $(stat -f%z "sing-box.tar.gz" 2>/dev/null || stat -c%s "sing-box.tar.gz" 2>/dev/null || echo "0") -lt 1000 ]]; then
        error_exit "下载的文件无效或过小"
    fi
    
    log_info "解压安装包..."
    if ! tar -xzf sing-box.tar.gz; then
        error_exit "解压失败，可能文件损坏"
    fi
    
    # 查找解压后的目录
    local extracted_dir
    extracted_dir=$(find . -name "sing-box-*" -type d | head -1)
    
    if [[ -z "$extracted_dir" ]] || [[ ! -d "$extracted_dir" ]]; then
        error_exit "无法找到解压后的目录"
    fi
    
    # 验证二进制文件
    if [[ ! -f "$extracted_dir/sing-box" ]]; then
        error_exit "二进制文件不存在: $extracted_dir/sing-box"
    fi
    
    # 安装二进制文件
    log_info "安装二进制文件..."
    cp "$extracted_dir/sing-box" /usr/local/bin/
    chmod +x /usr/local/bin/sing-box
    
    # 创建符号链接到 /usr/bin
    ln -sf /usr/local/bin/sing-box /usr/bin/sing-box
    
    # 验证安装
    if ! command -v sing-box >/dev/null 2>&1; then
        error_exit "Sing-Box 安装后仍无法找到命令"
    fi
    
    # 测试版本
    local version_info
    version_info=$(sing-box version 2>/dev/null || echo "版本获取失败")
    log_info "安装的版本: $version_info"
    
    # # 创建系统用户
    # log_info "创建系统用户..."
    # if ! id sing-box >/dev/null 2>&1; then
    #     useradd -r -s /sbin/nologin sing-box 2>/dev/null || {
    #         log_warn "创建用户失败，使用root运行"
    #     }
    # fi
    
    # 创建systemd服务文件
    log_info "创建系统服务..."
    cat > /etc/systemd/system/sing-box.service <<EOF
[Unit]
Description=sing-box service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target

[Service]
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SYS_PTRACE CAP_DAC_READ_SEARCH
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SYS_PTRACE CAP_DAC_READ_SEARCH
ExecStart=/usr/local/bin/sing-box run -c /etc/sing-box/config.json
ExecReload=/bin/kill -HUP \$MAINPID
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
    
    print_success "Sing-Box 手动安装完成"
    
    # 清理下载文件
    cd / && rm -rf "$TEMP_DIR/sing-box"* 2>/dev/null || true
}

# 安装自签证书
install_self_signed_cert() {
    log_info "生成自签证书..."
    
    mkdir -p "$CERT_DIR"
    
    local private_key="$CERT_DIR/private.key"
    local cert_file="$CERT_DIR/cert.pem"
    local cn="bing.com"
    
    openssl ecparam -genkey -name prime256v1 -out "$private_key"
    openssl req -new -x509 -days 36500 -key "$private_key" -out "$cert_file" \
        -subj "/CN=$cn" 2>/dev/null
    
    print_success "自签证书生成完成"
}

# =============================================================================
# 服务管理函数
# =============================================================================

# 启动和管理服务
enable_and_start_service() {
    log_info "启动 Sing-Box 服务..."
    
    # 验证 sing-box 命令是否可用
    if ! command -v sing-box >/dev/null 2>&1; then
        error_exit "sing-box 命令未找到，安装可能失败"
    fi
    
    # 验证配置文件存在
    if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
        error_exit "配置文件不存在: $CONFIG_DIR/config.json"
    fi
    
    # 检查配置文件语法
    log_info "验证配置文件语法..."
    if ! sing-box check -c "$CONFIG_DIR/config.json"; then
        log_error "配置文件语法错误，显示配置内容进行调试："
        echo "=== 配置文件内容 ==="
        cat "$CONFIG_DIR/config.json"
        echo "=================="
        error_exit "Sing-Box 配置文件语法错误"
    fi
    
    log_info "配置文件语法验证通过"
    
    # 设置文件权限
    chmod 644 "$CONFIG_DIR/config.json"
    
    # 启用并启动服务
    log_info "启用服务..."
    systemctl enable sing-box >/dev/null 2>&1
    
    log_info "启动服务..."
    systemctl restart sing-box
    
    # 等待服务启动
    sleep 3
    
    # 检查服务状态
    if systemctl is-active --quiet sing-box; then
        print_success "Sing-Box 服务启动成功"
        
        # 显示服务状态
        log_info "服务状态："
        
        systemctl status sing-box --no-pager -l | head -20
    else
        log_error "Sing-Box 服务启动失败"
        log_error "服务状态："
        systemctl status sing-box --no-pager -l
        log_error "服务日志："
        journalctl -u sing-box --no-pager -l | tail -20
        error_exit "Sing-Box 服务启动失败"
    fi
}

# 停止服务
stop_service() {
    log_info "停止 Sing-Box 服务..."
    systemctl stop sing-box >/dev/null 2>&1 || true
    systemctl disable sing-box >/dev/null 2>&1 || true
    print_success "服务已停止"
}

# 重启服务
restart_service() {
    log_info "重启 Sing-Box 服务..."
    systemctl restart sing-box
    
    if systemctl is-active --quiet sing-box; then
        print_success "服务重启成功"
    else
        error_exit "服务重启失败"
    fi
}

# 查看服务状态
show_service_status() {
    echo ""
    print_colored "$BLUE" "========== 服务状态 =========="
    
    if systemctl is-active --quiet sing-box; then
        print_success "✅ Sing-Box 服务运行中"
    else
        print_error "❌ Sing-Box 服务未运行"
    fi
    
    echo ""
    echo "详细状态:"
    systemctl status sing-box --no-pager -l
    echo ""
}

# =============================================================================
# 菜单系统
# =============================================================================

# 显示主菜单
show_main_menu() {
    echo ""
    print_colored "$BLUE" "========== Sing-Box 管理面板 =========="
    echo "1. 全新安装部署"
    echo "2. 重新生成配置"
    echo "3. 显示连接信息"
    echo "4. 重启服务"
    echo "5. 停止服务"
    echo "6. 查看服务状态"
    echo "7. 查看实时日志"
    echo "8. 卸载服务"
    echo "0. 退出"
    print_colored "$BLUE" "====================================="
}

# 处理菜单选择
handle_menu_choice() {
    local choice="$1"
    
    case "$choice" in
        1)
            deploy_fresh_install
            ;;
        2)
            regenerate_config
            ;;
        3)
            show_connection_info
            ;;
        4)
            restart_service
            ;;
        5)
            stop_service
            ;;
        6)
            show_service_status
            ;;
        7)
            show_realtime_logs
            ;;
        8)
            uninstall_service
            ;;
        0)
            echo "退出程序"
            exit 0
            ;;
        *)
            print_error "无效选择，请重新输入"
            ;;
    esac
}

# =============================================================================
# 部署和管理功能
# =============================================================================

# 全新安装部署
deploy_fresh_install() {
    log_info "开始全新安装部署..."
    
    # 系统检查
    check_root || error_exit "Root权限检查失败"
    check_system || error_exit "系统兼容性检查失败"
    check_network || error_exit "网络连接检查失败"
    
    # 初始化
    if ! initialize; then
        error_exit "初始化失败"
    fi
    
    # 网络和安全配置
    #change_ssh_port || log_warn "SSH端口配置可能失败"
    detect_ip_and_setup || error_exit "IP地址检测和配置失败"
    setup_fail2ban || log_warn "fail2ban配置失败，但不影响主要功能"
    
    # 软件安装和配置
    install_singbox || error_exit "Sing-Box安装失败"
    generate_server_config || error_exit "服务器配置生成失败"
    generate_client_config_file || error_exit "客户端配置生成失败"
    
    # 启动服务
    start_http_server || log_warn "HTTP服务启动可能失败"
    enable_and_start_service || error_exit "Sing-Box服务启动失败"
    
    # 网络优化
    enable_bbr || log_warn "BBR启用可能失败"
    optimize_network || log_warn "网络优化可能失败"
    
    # 显示结果
    show_deployment_results
    
    print_success "========== 部署完成 =========="
}

# 重新生成配置
regenerate_config() {
    log_info "重新生成配置文件..."
    
    if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
        print_error "未找到现有配置，请先进行全新安装"
        return 1
    fi
    
    # 重新生成配置
    generate_server_config || error_exit "服务器配置生成失败"
    generate_client_config_file || error_exit "客户端配置生成失败"
    
    # 重启服务
    restart_service
    
    print_success "配置重新生成完成"
}

# 显示连接信息
show_connection_info() {
    if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
        print_error "未找到配置文件，请先进行安装部署"
        return 1
    fi
    
    # 从现有配置中读取信息或重新检测
    detect_ip_and_setup >/dev/null 2>&1 || true
    
    generate_proxy_links
    generate_qr_codes
    provide_download_link
    start_http_server
    schedule_cleanup
}

# 显示实时日志
show_realtime_logs() {
    print_info "显示实时日志 (按 Ctrl+C 退出):"
    echo ""
    
    if [[ -f "$LOG_FILE" ]]; then
        tail -f "$LOG_FILE" /var/log/sing-box.log 2>/dev/null || \
        journalctl -u sing-box -f
    else
        journalctl -u sing-box -f
    fi
}

# 卸载服务
uninstall_service() {
    read -p "确定要卸载 Sing-Box 服务吗？(y/N): " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        log_info "卸载 Sing-Box 服务..."
        
        # 停止服务
        stop_service
        
        # 删除配置文件
        rm -rf "$CONFIG_DIR" 2>/dev/null || true
        rm -f "$LOG_FILE" 2>/dev/null || true
        
        # 卸载软件包
        apt-get remove --purge -y sing-box >/dev/null 2>&1 || true
        
        print_success "卸载完成"
    else
        print_info "取消卸载"
    fi
}

# 显示部署结果
show_deployment_results() {
    provide_download_link
    generate_proxy_links  
    generate_qr_codes
    
    echo ""
    print_warning "配置文件将在10分钟后自动清理"
    print_info "日志文件位置: $LOG_FILE"
    echo ""
    
    # 设置清理任务
    schedule_cleanup
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    show_banner
    
    # 检查依赖
    check_dependencies
    
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
            start)
                enable_and_start_service
                ;;
            stop)
                stop_service
                ;;
            restart)
                restart_service
                ;;
            status)
                show_service_status
                ;;
            logs)
                show_realtime_logs
                ;;
            uninstall)
                uninstall_service
                ;;
            *)
                echo "用法: $0 [install|config|info|start|stop|restart|status|logs|uninstall]"
                exit 1
                ;;
        esac
        exit 0
    fi
    
    # 交互式菜单
    while true; do
        show_main_menu
        read -p "请选择操作 [0-8]: " choice
        handle_menu_choice "$choice"
        
        echo ""
        read -p "按回车键继续..." -r
    done
}

# 错误处理
trap cleanup_on_exit EXIT

# 运行主函数
main "$@"