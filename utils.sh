#!/bin/bash

# =============================================================================
# 工具函数库 - utils.sh
# 包含日志、颜色输出、密码生成、端口检测等通用工具函数
# =============================================================================

# 注意：默认配置和错误处理已在主脚本中加载

# =============================================================================
# 增强的日志系统
# =============================================================================

# 结构化日志函数
log() {
    local level="$1"
    local level_num
    local color
    shift
    
    # 确定日志级别数值和颜色
    case "$level" in
        "ERROR") level_num=$LOG_LEVEL_ERROR; color="$RED" ;;
        "WARN")  level_num=$LOG_LEVEL_WARN;  color="$YELLOW" ;;
        "INFO")  level_num=$LOG_LEVEL_INFO;  color="$GREEN" ;;
        "DEBUG") level_num=$LOG_LEVEL_DEBUG; color="$BLUE" ;;
        *) level_num=$LOG_LEVEL_INFO; color="$NC" ;;
    esac
    
    # 检查是否需要输出此级别的日志
    if [[ $level_num -gt ${CURRENT_LOG_LEVEL:-$LOG_LEVEL_INFO} ]]; then
        return 0
    fi
    
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local message="[$timestamp] [$level] $*"
    
    # 输出到控制台（带颜色）
    echo -e "${color}${message}${NC}"
    
    # 输出到日志文件（不带颜色）
    if [[ -n "${LOG_FILE:-}" ]]; then
        echo "$message" >> "$LOG_FILE" 2>/dev/null || true
        
        # 日志轮转
        rotate_log_if_needed
    fi
}

# 日志轮转函数
rotate_log_if_needed() {
    if [[ ! -f "$LOG_FILE" ]]; then
        return 0
    fi
    
    local file_size
    file_size=$(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null || echo "0")
    local max_size=$((10 * 1024 * 1024))  # 10MB
    
    if [[ $file_size -gt $max_size ]]; then
        # 备份当前日志
        local backup_file="${LOG_FILE}.$(date +%Y%m%d_%H%M%S)"
        mv "$LOG_FILE" "$backup_file"
        
        # 压缩旧日志
        gzip "$backup_file" 2>/dev/null || true
        
        # 清理过期日志
        find "$(dirname "$LOG_FILE")" -name "$(basename "$LOG_FILE").*.gz" -mtime +7 -delete 2>/dev/null || true
        
        log_info "日志已轮转: $backup_file"
    fi
}

log_info() { log "INFO" "$@"; }
log_warn() { log "WARN" "$@"; }
log_error() { log "ERROR" "$@"; }
log_debug() { log "DEBUG" "$@"; }

# 注意：错误处理函数已移动到 error_handler.sh

# 颜色输出函数
print_colored() {
    local color="$1"
    local message="$2"
    echo -e "${color}${message}${NC}"
}

print_success() { print_colored "$GREEN" "$1"; }
print_warning() { print_colored "$YELLOW" "$1"; }
print_error() { print_colored "$RED" "$1"; }
print_info() { print_colored "$BLUE" "$1"; }

# =============================================================================
# 系统检查函数
# =============================================================================

# 检查是否为root用户
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error_exit "此脚本必须以 root 用户身份运行"
    fi
}

# 检查系统兼容性
check_system() {
    local os_id
    os_id=$(grep '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"' 2>/dev/null || echo "unknown")
    
    case "$os_id" in
        ubuntu|debian)
            log_info "检测到支持的系统: $os_id (Debian系)"
            PACKAGE_MANAGER="apt"
            ;;
        centos|rhel|rocky|almalinux|fedora)
            log_info "检测到支持的系统: $os_id (RedHat系)"
            PACKAGE_MANAGER="yum"
            # 检查是否有dnf
            if command -v dnf >/dev/null 2>&1; then
                PACKAGE_MANAGER="dnf"
            fi
            ;;
        *)
            error_exit "不支持的操作系统: $os_id。支持的系统: Ubuntu, Debian, CentOS, RHEL, Rocky Linux, AlmaLinux, Fedora"
            ;;
    esac
}

# 检查网络连接
check_network() {
    if ! ping -c 1 -W 5 8.8.8.8 >/dev/null 2>&1; then
        error_exit "网络连接检查失败"
    fi
    log_info "网络连接正常"
}

# 检查和启动systemd-resolved
check_and_start_systemd_resolved() {
    log_info "检查 systemd-resolved 服务..."
    
    if ! systemctl is-active --quiet systemd-resolved; then
        log_info "启动 systemd-resolved 服务..."
        systemctl start systemd-resolved >/dev/null 2>&1
    fi
}

# =============================================================================
# 端口和密码生成函数
# =============================================================================

# 获取可用端口
get_available_port() {
    local start_range="$1"
    local end_range="$2"
    local port
    
    # 检查 shuf 命令是否可用
    if ! command -v shuf >/dev/null 2>&1; then
        # 使用备用方法生成随机端口
        for attempt in {1..50}; do
            port=$((start_range + (RANDOM % (end_range - start_range + 1))))
            if ! lsof -i:"$port" >/dev/null 2>&1; then
                echo "$port"
                return 0
            fi
        done
    else
        # 使用 shuf 命令
        for attempt in {1..50}; do
            port=$(shuf -i "$start_range-$end_range" -n 1 2>/dev/null) || {
                port=$((start_range + (RANDOM % (end_range - start_range + 1))))
            }
            
            if ! lsof -i:"$port" >/dev/null 2>&1; then
                echo "$port"
                return 0
            fi
        done
    fi
    
    return 1
}

# 生成强密码
generate_strong_password() {
    local length="${1:-15}"
    local charset="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
    
    # 方法1: 使用 openssl
    if command -v openssl >/dev/null 2>&1; then
        local result
        result=$(openssl rand -base64 32 2>/dev/null | tr -dc "$charset" | head -c "$length" 2>/dev/null)
        if [[ -n "$result" && ${#result} -eq $length ]]; then
            echo "$result"
            return 0
        fi
    fi
    
    # 方法2: 使用 /dev/urandom
    if [[ -r /dev/urandom ]]; then
        local result
        result=$(tr -dc "$charset" < /dev/urandom 2>/dev/null | head -c "$length" 2>/dev/null)
        if [[ -n "$result" && ${#result} -eq $length ]]; then
            echo "$result"
            return 0
        fi
    fi
    
    # 方法3: 使用系统时间和PID的组合
    local timestamp=$(date +%s%N 2>/dev/null || date +%s)
    local pid=$$
    local seed="${timestamp}${pid}"
    
    # 简单的伪随机生成
    local result=""
    for i in $(seq 1 "$length"); do
        local index=$((seed % ${#charset}))
        result="${result}${charset:$index:1}"
        seed=$((seed / ${#charset} + i * 1103515245 + 12345))
    done
    
    echo "$result"
    return 0
}

# 生成Base64编码密码
generate_base64_password() {
    local length="${1:-32}"
    
    # 方法1: 使用 openssl
    if command -v openssl >/dev/null 2>&1; then
        local result
        result=$(openssl rand -base64 "$length" 2>/dev/null | tr -d '\n' 2>/dev/null)
        if [[ -n "$result" ]]; then
            echo "$result"
            return 0
        fi
    fi
    
    # 方法2: 使用 base64 命令
    if command -v base64 >/dev/null 2>&1 && [[ -r /dev/urandom ]]; then
        local result
        result=$(head -c "$length" /dev/urandom 2>/dev/null | base64 2>/dev/null | tr -d '\n' 2>/dev/null)
        if [[ -n "$result" ]]; then
            echo "$result"
            return 0
        fi
    fi
    
    # 方法3: 使用 dd 和 base64
    if command -v dd >/dev/null 2>&1 && command -v base64 >/dev/null 2>&1; then
        local result
        result=$(dd if=/dev/urandom bs=1 count="$length" 2>/dev/null | base64 2>/dev/null | tr -d '\n' 2>/dev/null)
        if [[ -n "$result" ]]; then
            echo "$result"
            return 0
        fi
    fi
    
    # 方法4: 生成备用密码
    local charset="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
    local timestamp=$(date +%s%N 2>/dev/null || date +%s)
    local result=""
    
    for i in $(seq 1 "$length"); do
        local index=$((timestamp % ${#charset}))
        result="${result}${charset:$index:1}"
        timestamp=$((timestamp / ${#charset} + i * 1103515245))
    done
    
    echo "$result"
    return 0
}

# 生成随机字符串
generate_random_string() {
    local length="${1:-6}"
    
    # 方法1: 使用 /dev/urandom 和 tr
    if command -v tr >/dev/null 2>&1; then
        local result
        result=$(tr -dc 'a-zA-Z' < /dev/urandom 2>/dev/null | head -c "$length" 2>/dev/null)
        if [[ -n "$result" && ${#result} -eq $length ]]; then
            echo "$result"
            return 0
        fi
    fi
    
    # 方法2: 使用 openssl
    if command -v openssl >/dev/null 2>&1; then
        local result
        result=$(openssl rand -hex 10 2>/dev/null | tr -dc 'a-zA-Z' | head -c "$length" 2>/dev/null)
        if [[ -n "$result" && ${#result} -ge $length ]]; then
            echo "${result:0:$length}"
            return 0
        fi
    fi
    
    # 方法3: 使用时间戳和进程ID的组合
    local timestamp=$(date +%s 2>/dev/null || echo "123456")
    local pid=$$
    local combined="${timestamp}${pid}"
    local result
    result=$(echo "$combined" | md5sum 2>/dev/null | tr -dc 'a-zA-Z' | head -c "$length" 2>/dev/null)
    if [[ -n "$result" && ${#result} -ge $length ]]; then
        echo "${result:0:$length}"
        return 0
    fi
    
    # 方法4: 简单的备用方法
    echo "backup$(date +%s | tail -c 4)"
    return 0
}

# =============================================================================
# 网络优化函数
# =============================================================================

# 启用BBR
enable_bbrOld() {
    log_info "启用BBR..."
    
    # 加载BBR模块
    modprobe tcp_bbr 2>/dev/null || true
    
    # 设置BBR
    {
        echo "net.core.default_qdisc=fq"
        echo "net.ipv4.tcp_congestion_control=bbr"
    } >> /etc/sysctl.conf
    
    sysctl -p >/dev/null 2>&1
    
    # 验证BBR状态
    if sysctl net.ipv4.tcp_congestion_control | grep -q bbr && lsmod | grep -q bbr; then
        print_success "BBR 已成功启用"
    else
        log_warn "BBR 启用可能失败，请检查系统配置"
    fi
}

enable_bbr() {
    log_info "开始启用BBR..."

    # 检查内核版本 (BBR 大致在 4.9+ 内核引入)
    # uname -r 输出类似 3.10.0-1160.el7.x86_64 或 5.14.0-70.el9.x86_64
    current_kernel_major=$(uname -r | cut -d. -f1)
    current_kernel_minor=$(uname -r | cut -d. -f2)

    if [ "$current_kernel_major" -lt 4 ] || ([ "$current_kernel_major" -eq 4 ] && [ "$current_kernel_minor" -lt 9 ]); then
        log_error "当前内核版本 $(uname -r) 过低，可能不支持BBR。请先升级内核 (建议 4.9+)。"
        return 1
    fi
    log_info "当前内核版本 $(uname -r) 符合要求。"

    log_info "尝试加载BBR模块..."
    if ! modprobe tcp_bbr; then
        log_warn "加载 tcp_bbr 模块失败。可能是内核未编译该模块或已内建。"
        # 即使 modprobe 失败也继续尝试，因为 BBR 可能已内建在内核中
    else
        log_info "tcp_bbr 模块加载成功或已加载。"
    fi

    log_info "配置sysctl参数..."
    # 确保配置不重复添加
    sysctl_conf_changed=0
    if ! grep -qFx "net.core.default_qdisc=fq" /etc/sysctl.conf; then
        echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
        sysctl_conf_changed=1
    fi
    if ! grep -qFx "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
        sysctl_conf_changed=1
    fi

    if [ "$sysctl_conf_changed" -eq 1 ]; then
        log_info "应用sysctl参数..."
        if sysctl -p; then
            log_info "sysctl参数已应用。"
        else
            log_error "sysctl -p 执行失败，请检查 /etc/sysctl.conf 配置。"
            return 1
        fi
    else
        log_info "sysctl参数已存在，无需重复配置。尝试直接应用当前内核设置（以防万一）。"
        # 即使文件没变，也执行一次确保当前内核参数是最新的（虽然通常不需要）
        if ! sysctl net.core.default_qdisc=fq >/dev/null 2>&1 || ! sysctl net.ipv4.tcp_congestion_control=bbr >/dev/null 2>&1; then
             log_warn "尝试直接设置fq或bbr到当前内核时遇到问题，但将依赖/etc/sysctl.conf的加载。"
        fi
        # 再次执行 sysctl -p 确保所有配置加载
        if sysctl -p; then
             log_info "sysctl参数已确认。"
        else
            log_error "sysctl -p 执行失败，请检查 /etc/sysctl.conf 配置。"
            return 1
        fi
    fi
    
    log_info "验证BBR状态..."
    # 检查当前拥塞控制算法
    current_congestion_control=$(sysctl -n net.ipv4.tcp_congestion_control)
    # 检查模块加载状态 (BBR 可能编译进内核，此时 lsmod 可能看不到 tcp_bbr，但拥塞控制算法是对的就行)
    module_loaded=$(lsmod | grep "^tcp_bbr\s" || echo "not_found") 

    if [[ "$current_congestion_control" == "bbr" ]]; then
        print_success "BBR 已成功启用 (net.ipv4.tcp_congestion_control = bbr)。"
        if [[ "$module_loaded" != "not_found" ]]; then
            log_info "tcp_bbr 模块已加载。"
        else
            log_info "tcp_bbr 模块未显式加载 (可能已内建于内核)。"
        fi
    else
        log_warn "BBR 启用可能失败，请检查系统配置。"
        log_warn "当前拥塞控制算法: $current_congestion_control"
        if [[ "$module_loaded" == "not_found" ]]; then
             log_warn "tcp_bbr 模块也未显式加载。"
        fi
        return 1
    fi
    return 0
}

# 优化网络参数
optimize_network() {
    log_info "优化网络参数..."
    
    cat >> /etc/sysctl.conf <<EOF
net.ipv4.tcp_fastopen=3
net.ipv4.tcp_slow_start_after_idle=0
net.ipv4.tcp_notsent_lowat=16384
EOF
    
    sysctl -p >/dev/null 2>&1
    print_success "网络优化完成"
}

# =============================================================================
# 安全配置函数
# =============================================================================

# 安装和配置fail2ban
setup_fail2ban() {
    log_info "自动安装fail2ban防止暴力登陆，安装超过60秒跳过安装.."
    
    case "$PACKAGE_MANAGER" in
        "apt")
            #timeout 300 apt-get update >/dev/null 2>&1 || log_warn "apt更新超时，继续执行"
            timeout 60 apt-get install -y fail2ban >/dev/null 2>&1 || {
                log_error "fail2ban安装失败，跳过此步骤"
                return 0
            }
            ;;
        "yum"|"dnf")
            log_info "配置 EPEL 源...预计需要1-2分钟"
            # 尝试安装EPEL，设置超时
            timeout 180 $PACKAGE_MANAGER install -y epel-release >/dev/null 2>&1 || {
                log_warn "EPEL源安装超时或失败，尝试备用方法"
                
                # 备用方法：手动添加EPEL源
                if [[ "$PACKAGE_MANAGER" == "yum" ]]; then
                    # CentOS 7
                    timeout 60 yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm >/dev/null 2>&1 || true
                else
                    # CentOS 8+
                    timeout 60 dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm >/dev/null 2>&1 || true
                fi
            }
            
            log_info "安装 fail2ban..."
            timeout 180 $PACKAGE_MANAGER install -y fail2ban >/dev/null 2>&1 || {
                log_error "fail2ban安装失败，跳过此步骤"
                return 0
            }
            ;;
    esac
    
    log_info "配置 fail2ban..."
    
    # 检测日志文件路径
    local auth_log="/var/log/auth.log"
    if [[ ! -f "$auth_log" ]]; then
        # CentOS/RHEL 使用 secure 日志
        auth_log="/var/log/secure"
    fi
    
    cat > /etc/fail2ban/jail.local <<EOF
[DEFAULT]
ignoreip = 127.0.0.1/8
bantime = -1
findtime = 86400
maxretry = 10

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = $auth_log
EOF
    
    # 启动服务，如果失败也不影响主要功能
    if systemctl enable fail2ban >/dev/null 2>&1 && systemctl start fail2ban >/dev/null 2>&1; then
        print_success "fail2ban 配置完成"
    else
        log_warn "fail2ban服务启动失败，但不影响主要功能"
    fi
}

# 修改SSH端口
change_ssh_port() {
    print_warning "是否修改SSH端口（NAT机器请选择n）? (y/n) [n]: "
    read -r modify_port
    modify_port=${modify_port:-n}
    
    if [[ "$modify_port" =~ ^[Yy]$ ]]; then
        log_info "修改SSH端口..."
        
        local new_ssh_port=40001
        
        # 安装SSH服务器
        case "$PACKAGE_MANAGER" in
            "apt")
                apt-get install -y openssh-server >/dev/null 2>&1
                ;;
            "yum"|"dnf")
                $PACKAGE_MANAGER install -y openssh-server >/dev/null 2>&1
                ;;
        esac
        
        # 备份配置文件
        cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak.$(date +%Y%m%d_%H%M%S)
        
        # 修改端口
        if grep -q "^#Port 22" /etc/ssh/sshd_config; then
            sed -i "s/^#Port 22/Port $new_ssh_port/" /etc/ssh/sshd_config
        elif grep -q "^Port" /etc/ssh/sshd_config; then
            sed -i "s/^Port.*/Port $new_ssh_port/" /etc/ssh/sshd_config
        else
            echo "Port $new_ssh_port" >> /etc/ssh/sshd_config
        fi
        
        # 重启SSH服务
        if systemctl list-units --type=service | grep -q ssh.service; then
            systemctl restart ssh
        elif systemctl list-units --type=service | grep -q sshd.service; then
            systemctl restart sshd
        fi
        
        # 配置防火墙
        if command -v ufw >/dev/null 2>&1; then
            ufw allow "$new_ssh_port/tcp" >/dev/null 2>&1 || true
        elif command -v firewall-cmd >/dev/null 2>&1; then
            # CentOS/RHEL 防火墙
            firewall-cmd --permanent --add-port="$new_ssh_port/tcp" >/dev/null 2>&1 || true
            firewall-cmd --reload >/dev/null 2>&1 || true
        fi
        
        print_warning "请使用端口 $new_ssh_port 进行SSH登录"
    fi
}

# =============================================================================
# HTTP服务器函数
# =============================================================================

# 启动安全的HTTP下载服务
start_http_server() {
    local http_dir="/root"
    local bind_ip="${HTTP_BIND_IP:-127.0.0.1}"  # 默认只绑定本地
    
    # 检查端口是否被占用
    if lsof -i:"$DOWNLOAD_PORT" >/dev/null 2>&1; then
        log_info "HTTP服务已在端口 $DOWNLOAD_PORT 运行"
        return 0
    fi
    
    log_info "启动HTTP下载服务 (绑定: $bind_ip:$DOWNLOAD_PORT)..."
    cd "$http_dir" || error_exit "无法切换到目录 $http_dir"
    
    # 启动Python HTTP服务器，只绑定本地接口
    nohup python3 -m http.server "$DOWNLOAD_PORT" --bind "$bind_ip" >/dev/null 2>&1 &
    local http_pid=$!
    
    # 等待服务启动
    sleep 2
    if kill -0 "$http_pid" 2>/dev/null; then
        print_success "HTTP服务已启动 (PID: $http_pid, 端口: $DOWNLOAD_PORT)"
        # 保存PID供后续清理
        echo "$http_pid" > "/tmp/singbox_http_$$.pid"
    else
        log_error "HTTP服务启动失败"
        return 1
    fi
}

# 提供下载链接
provide_download_link() {
    echo ""
    print_colored "$RED" "=================== 下载链接 ==================="
    echo ""
    echo "配置文件下载地址:"
    echo "http://$SERVER_IP:$DOWNLOAD_PORT/singbox_racknerd.yaml"
    echo ""
    print_colored "$RED" "=============================================="
    echo ""
}

# 清理任务
schedule_cleanup() {
    log_info "设置定时清理任务..."
    
    # 在后台启动清理任务
    (
        sleep 600  # 10分钟后清理
        
        # log_info "执行定时清理..."
        # rm -f /root/singbox_*.yaml
        
        # 关闭HTTP服务
        local pid
        pid=$(lsof -t -i:"$DOWNLOAD_PORT" 2>/dev/null || echo "")
        if [[ -n "$pid" ]]; then
            kill -9 "$pid" 2>/dev/null || true
            #log_info "HTTP服务已关闭"
        fi
        
        #log_info "清理任务完成"
    ) &
}

# =============================================================================
# 二维码生成函数
# =============================================================================

# 生成二维码
generate_qr_codes() {
    if ! command -v qrencode >/dev/null 2>&1; then
        log_info "安装二维码生成工具..."
        case "$PACKAGE_MANAGER" in
            "apt")
                apt-get update >/dev/null 2>&1
                apt-get install -y qrencode >/dev/null 2>&1
                ;;
            "yum"|"dnf")
                $PACKAGE_MANAGER install -y epel-release >/dev/null 2>&1
                $PACKAGE_MANAGER install -y qrencode >/dev/null 2>&1
                ;;
        esac
    fi
    
    log_info "生成二维码..."
    
    # 生成主要协议的二维码
    local reality_link="vless://${UUID}@${SERVER_IP}:${VLESS_PORT}?security=reality&flow=xtls-rprx-vision&type=tcp&sni=${SERVER}&fp=chrome&pbk=Y_-yCHC3Qi-Kz6OWpueQckAJSQuGEKffwWp8MlFgwTs&sid=0123456789abcded&encryption=none#Reality"
    local hy2_link="hysteria2://${HYSTERIA_PASSWORD}@${SERVER_IP}:${HYSTERIA_PORT}?insecure=1&alpn=h3&sni=bing.com#Hysteria2"
    local trojan_link="trojan://${HYSTERIA_PASSWORD}@${SERVER_IP}:63333?sni=bing.com&type=ws&path=%2Ftrojan&host=bing.com&allowInsecure=1&udp=true&alpn=http%2F1.1#Trojan"
    local tuic_link="tuic://${UUID}:@${SERVER_IP}:61555?alpn=h3&allow_insecure=1&congestion_control=bbr#TUIC"
    
    echo ""
    print_colored "$BLUE" "=============== 二维码生成 ==============="
    echo ""
    
    print_info "🔷 Reality 二维码:"
    qrencode -t ANSIUTF8 "$reality_link" 2>/dev/null || echo "二维码生成失败"
    echo ""
    
    print_info "🚀 Hysteria2 二维码:"
    qrencode -t ANSIUTF8 "$hy2_link" 2>/dev/null || echo "二维码生成失败"
    echo ""
    
    print_info "🛡️ Trojan 二维码:"
    qrencode -t ANSIUTF8 "$trojan_link" 2>/dev/null || echo "二维码生成失败"
    echo ""
    
    print_info "⚡ TUIC 二维码:"
    qrencode -t ANSIUTF8 "$tuic_link" 2>/dev/null || echo "二维码生成失败"
    echo ""
    
    # ShadowTLS v3 + SS2022 二维码
    if [[ -f /tmp/ss2022_link.tmp ]]; then
        local ss2022_link
        ss2022_link=$(cat /tmp/ss2022_link.tmp 2>/dev/null)
        if [[ -n "$ss2022_link" ]]; then
            print_info "🔐 ShadowTLS v3 + SS2022 二维码:"
            qrencode -t ANSIUTF8 "$ss2022_link" 2>/dev/null || echo "二维码生成失败"
            echo ""
        fi
        rm -f /tmp/ss2022_link.tmp
    fi
    
    # SS专线二维码
    if [[ -f /tmp/ss_link.tmp ]]; then
        local ss_link
        ss_link=$(cat /tmp/ss_link.tmp 2>/dev/null)
        if [[ -n "$ss_link" ]]; then
            print_info "📡 SS专线 二维码:"
            qrencode -t ANSIUTF8 "$ss_link" 2>/dev/null || echo "二维码生成失败"
            echo ""
        fi
        rm -f /tmp/ss_link.tmp
    fi
    
    print_colored "$BLUE" "======================================="
    echo ""
}