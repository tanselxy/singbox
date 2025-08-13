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

# 注意：未使用的颜色变量已清理



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

# 加载默认配置
source "$SCRIPT_DIR/defaults.conf" 2>/dev/null || {
    echo "错误: 无法加载默认配置文件 defaults.conf"
    exit 1
}

# 加载错误处理模块
source "$SCRIPT_DIR/error_handler.sh" 2>/dev/null || {
    echo "错误: 无法加载错误处理模块 error_handler.sh"
    exit 1
}

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

# 初始化错误处理
init_error_handler

# =============================================================================
# 系统检查和初始化
# =============================================================================

# 检查并创建缺失的文件
check_dependencies() {
    local missing_files=()
    
    # 检查基本文件
    [[ ! -f "$SCRIPT_DIR/utils.sh" ]] && missing_files+=("utils.sh")
    [[ ! -f "$SCRIPT_DIR/network.sh" ]] && missing_files+=("network.sh") 
    [[ ! -f "$SCRIPT_DIR/config.sh" ]] && missing_files+=("config.sh")
    [[ ! -f "$SCRIPT_DIR/server_template.json" ]] && missing_files+=("server_template.json")
    [[ ! -f "$SCRIPT_DIR/client_template.yaml" ]] && missing_files+=("client_template.yaml")
    
    # 检查新增文件，如果不存在则自动创建
    if [[ ! -f "$SCRIPT_DIR/defaults.conf" ]]; then
        echo "信息: 创建缺失的 defaults.conf 文件..."
        create_defaults_conf
    fi
    
    if [[ ! -f "$SCRIPT_DIR/error_handler.sh" ]]; then
        echo "信息: 创建缺失的 error_handler.sh 文件..."
        create_error_handler
    fi
    
    if [[ ${#missing_files[@]} -gt 0 ]]; then
        print_error "缺少以下核心依赖文件:"
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


# 简化的横幅显示函数
show_banner() {
    clear
    
    echo ""
    print_colored "$CYAN" "╭──────────────────────────────────────────────────╮"
    print_colored "$CYAN" "│ ${YELLOW}https://my.racknerd.com/aff.php?aff=10790&pid=912${CYAN}│"
    print_colored "$CYAN" "│                                                  │"
    print_colored "$CYAN" "│            ${GREEN}年付仅需10美元${CYAN}                      │"
    print_colored "$CYAN" "╰──────────────────────────────────────────────────╯"
    echo ""
    
    print_colored "$BLUE" "========== Sing-Box 自动部署脚本 v$SCRIPT_VERSION =========="
    print_colored "$GREEN" "作者: 优化版本 | 功能: 一键部署多协议代理服务"
    print_colored "$BLUE" "================================================"
    echo ""
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
    
    # 获取可用端口（使用配置文件中的范围）
    log_info "分配可用端口..."
    VLESS_PORT=$(get_available_port "$DEFAULT_VLESS_PORT_MIN" "$DEFAULT_VLESS_PORT_MAX" 2>>"$LOG_FILE") || {
        log_error "获取VLESS端口失败"
        return 1
    }
    
    SS_PORT=$(get_available_port "$DEFAULT_SS_PORT_MIN" "$DEFAULT_SS_PORT_MAX" 2>>"$LOG_FILE") || {
        log_error "获取SS端口失败"  
        return 1
    }
    
    HYSTERIA_PORT=$(get_available_port "$DEFAULT_HYSTERIA_PORT_MIN" "$DEFAULT_HYSTERIA_PORT_MAX" 2>>"$LOG_FILE") || {
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

# 安装自签证书（增强安全性）
install_self_signed_cert() {
    log_info "生成自签证书..."
    
    mkdir -p "$CERT_DIR"
    
    local private_key="$CERT_DIR/private.key"
    local cert_file="$CERT_DIR/cert.pem"
    local cn="${CERT_CN:-bing.com}"
    
    # 生成私钥
    if ! openssl ecparam -genkey -name prime256v1 -out "$private_key" 2>/dev/null; then
        error_exit "生成私钥失败"
    fi
    
    # 生成证书
    if ! openssl req -new -x509 -days "$CERT_VALIDITY_DAYS" -key "$private_key" -out "$cert_file" \
        -subj "/CN=$cn" 2>/dev/null; then
        error_exit "生成证书失败"
    fi
    
    # 设置安全的文件权限
    chmod "$DEFAULT_KEY_PERMS" "$private_key"
    chmod "$DEFAULT_CERT_PERMS" "$cert_file"
    chown root:root "$private_key" "$cert_file"
    
    # 验证证书
    if openssl x509 -in "$cert_file" -text -noout >/dev/null 2>&1; then
        print_success "自签证书生成完成并验证通过"
        log_info "证书有效期: $CERT_VALIDITY_DAYS 天"
    else
        error_exit "证书验证失败"
    fi
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
    
    # 设置安全的文件权限
    chmod 600 "$CONFIG_DIR/config.json"
    chown root:root "$CONFIG_DIR/config.json"
    
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
    #setup_fail2ban || log_warn "fail2ban配置失败，但不影响主要功能"
    
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

# =============================================================================
# 向后兼容性：自动创建缺失的新增文件
# =============================================================================

# 创建 defaults.conf 文件
create_defaults_conf() {
    cat > "$SCRIPT_DIR/defaults.conf" <<'EOF'
#!/bin/bash

# =============================================================================
# Sing-Box 默认配置文件
# 统一管理硬编码值和默认配置
# =============================================================================

# 端口配置
readonly DEFAULT_DOWNLOAD_PORT=14567
readonly DEFAULT_SS_PORT=443
readonly DEFAULT_VLESS_PORT_MIN=20000
readonly DEFAULT_VLESS_PORT_MAX=20010
readonly DEFAULT_HYSTERIA_PORT_MIN=50000
readonly DEFAULT_HYSTERIA_PORT_MAX=50010
readonly DEFAULT_SS_PORT_MIN=31000
readonly DEFAULT_SS_PORT_MAX=31010

# 固定端口
readonly TROJAN_PORT=63333
readonly TUIC_PORT=61555
readonly SS_DIRECT_PORT=59000
readonly VLESS_CDN_PORT=4433

# 网络配置
readonly NETWORK_TIMEOUT=10
readonly DOWNLOAD_TIMEOUT=300
readonly MAX_RETRIES=3
readonly RETRY_DELAY=2

# 密码和安全配置
readonly DEFAULT_PASSWORD_LENGTH=15
readonly UUID_REGEX="^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

# 文件路径
readonly LOG_ROTATE_SIZE="10M"
readonly LOG_MAX_FILES=5
readonly TEMP_FILE_TTL=600  # 10分钟

# 域名配置
declare -A DOMAIN_BY_COUNTRY=(
    ["TW"]="www.apple.com"
    ["NG"]="unn.edu.ng" 
    ["JP"]="www.tms-e.co.jp"
    ["US"]="www.thewaltdisneycompany.com"
    ["NL"]="nl.servutech.com"
    ["DE"]="www.mediamarkt.de"
    ["HK"]="www.apple.com"
    ["DEFAULT"]="www.apple.com"
)

# 下载源配置
readonly GITHUB_RELEASES_API="https://api.github.com/repos/SagerNet/sing-box/releases/latest"
readonly GITHUB_PROXY="https://ghproxy.com/"
readonly FALLBACK_VERSION="v1.8.0"

# 系统源配置
readonly UBUNTU_SOURCES="http://archive.ubuntu.com/ubuntu"
readonly DEBIAN_SOURCES="http://deb.debian.org/debian"
readonly SECURITY_SOURCES="http://security.debian.org/debian-security"

# 网络检测
readonly IPV4_CHECK_URLS=(
    "https://api64.ipify.org"
    "https://ifconfig.me"
    "https://ipinfo.io/ip"
)

readonly IPV6_CHECK_URLS=(
    "https://ifconfig.me"
    "https://ipinfo.io/ip"
)

# HTTP服务器安全配置
readonly HTTP_BIND_IP="127.0.0.1"  # 只绑定本地，增强安全性
readonly HTTP_MAX_CONNECTIONS=10

# 证书配置
readonly CERT_VALIDITY_DAYS=36500
readonly CERT_CN="bing.com"
readonly DEFAULT_CERT_PERMS=600
readonly DEFAULT_KEY_PERMS=600

# Reality配置
readonly REALITY_PUBLIC_KEY="Y_-yCHC3Qi-Kz6OWpueQckAJSQuGEKffwWp8MlFgwTs"
readonly REALITY_SHORT_ID="0123456789abcded"

# 日志级别
readonly LOG_LEVEL_ERROR=1
readonly LOG_LEVEL_WARN=2
readonly LOG_LEVEL_INFO=3
readonly LOG_LEVEL_DEBUG=4

# 当前日志级别 (可通过环境变量 SINGBOX_LOG_LEVEL 覆盖)
CURRENT_LOG_LEVEL=${SINGBOX_LOG_LEVEL:-$LOG_LEVEL_INFO}
EOF
    echo "✓ defaults.conf 文件已创建"
}

# 创建 error_handler.sh 文件
create_error_handler() {
    cat > "$SCRIPT_DIR/error_handler.sh" <<'EOF'
#!/bin/bash

# =============================================================================
# 错误处理和重试机制模块 (简化版本)
# 提供基本的错误处理、重试逻辑和网络请求功能
# =============================================================================

# =============================================================================
# 错误处理函数
# =============================================================================

# 统一的错误退出函数
error_exit() {
    local message="$1"
    local exit_code="${2:-1}"
    
    echo "[ERROR] $message" >&2
    
    # 执行清理操作
    cleanup_on_error
    
    exit "$exit_code"
}

# 错误时的清理函数
cleanup_on_error() {
    echo "[INFO] 执行错误清理操作..."
    
    # 清理临时文件
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf "$TEMP_DIR"
    fi
    
    # 停止可能启动的HTTP服务
    local http_pid
    http_pid=$(lsof -t -i:"${DOWNLOAD_PORT:-14567}" 2>/dev/null || echo "")
    if [[ -n "$http_pid" ]]; then
        kill -TERM "$http_pid" 2>/dev/null || true
    fi
}

# 检查函数返回值并处理错误
check_result() {
    local result=$?
    local message="$1"
    local exit_on_error="${2:-true}"
    
    if [[ $result -ne 0 ]]; then
        if [[ "$exit_on_error" == "true" ]]; then
            error_exit "$message (退出码: $result)" "$result"
        else
            echo "[ERROR] $message (退出码: $result)" >&2
            return $result
        fi
    fi
    
    return 0
}

# =============================================================================
# 重试机制函数
# =============================================================================

# 带重试的命令执行
retry_command() {
    local max_attempts="${1:-3}"
    local delay="${2:-2}"
    shift 2
    local command=("$@")
    
    local attempt=1
    local result
    
    while [[ $attempt -le $max_attempts ]]; do
        if "${command[@]}"; then
            return 0
        fi
        
        result=$?
        
        if [[ $attempt -lt $max_attempts ]]; then
            echo "[WARN] 命令执行失败 (尝试 $attempt/$max_attempts)，${delay}秒后重试..." >&2
            sleep "$delay"
        fi
        
        ((attempt++))
    done
    
    return $result
}

# =============================================================================
# 网络请求函数
# =============================================================================

# 安全的网络请求函数
safe_curl() {
    local url="$1"
    local timeout="${2:-10}"
    local max_retries="${3:-3}"
    
    local curl_args=(
        --max-time "$timeout"
        --connect-timeout "$timeout"
        --retry 0
        --fail
        --silent
        --show-error
        --location
        --user-agent "Sing-Box-Installer/2.0"
    )
    
    curl_args+=("$url")
    
    # 使用重试机制
    retry_command "$max_retries" 2 curl "${curl_args[@]}"
}

# 下载文件函数
download_file() {
    local url="$1"
    local output_file="$2"
    local timeout="${3:-300}"
    local max_retries="${4:-3}"
    
    echo "[INFO] 下载文件: $url -> $output_file"
    
    # 创建输出目录
    mkdir -p "$(dirname "$output_file")"
    
    # 下载文件
    if safe_curl "$url" "$timeout" "$max_retries" > "$output_file"; then
        local downloaded_size
        downloaded_size=$(stat -f%z "$output_file" 2>/dev/null || stat -c%s "$output_file" 2>/dev/null || echo "0")
        
        if [[ "$downloaded_size" -lt 100 ]]; then
            echo "[ERROR] 下载的文件过小 (${downloaded_size} 字节)，可能下载失败" >&2
            rm -f "$output_file"
            return 1
        fi
        
        echo "[INFO] 下载完成: $output_file (大小: $downloaded_size 字节)"
        return 0
    else
        echo "[ERROR] 下载失败: $url" >&2
        rm -f "$output_file" 2>/dev/null || true
        return 1
    fi
}

# =============================================================================
# 网络连接检测
# =============================================================================

# 检测网络连通性
check_network_connectivity() {
    # IPv4测试地址
    local ipv4_urls=(
        "8.8.8.8"
        "1.1.1.1"
        "114.114.114.114"
    )
    
    # IPv6测试地址
    local ipv6_urls=(
        "2001:4860:4860::8888"  # Google DNS IPv6
        "2606:4700:4700::1111"  # Cloudflare DNS IPv6
        "2400:3200::1"          # 阿里DNS IPv6
    )
    
    # 先测试IPv4连接
    for url in "${ipv4_urls[@]}"; do
        if ping -c 1 -W 5 "$url" >/dev/null 2>&1; then
            return 0
        fi
    done
    
    # 如果IPv4失败，测试IPv6连接
    for url in "${ipv6_urls[@]}"; do
        if ping6 -c 1 -W 5 "$url" >/dev/null 2>&1; then
            return 0
        fi
    done
    
    return 1
}

# =============================================================================
# 初始化函数
# =============================================================================

# 初始化错误处理模块
init_error_handler() {
    # 设置错误陷阱
    set -euo pipefail
    
    # 设置EXIT陷阱
    trap cleanup_on_exit EXIT
    
    # 设置错误陷阱
    trap 'error_exit "脚本在第 $LINENO 行出错" $?' ERR
}

# 清理退出函数
cleanup_on_exit() {
    local exit_code=$?
    
    if [[ $exit_code -ne 0 ]]; then
        cleanup_on_error
    fi
}
EOF
    chmod +x "$SCRIPT_DIR/error_handler.sh"
    echo "✓ error_handler.sh 文件已创建"
}

# 注意：错误处理已移至 error_handler.sh 模块

# 运行主函数
main "$@"