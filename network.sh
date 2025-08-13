#!/bin/bash

# =============================================================================
# 网络检测和配置函数库 - network.sh
# 包含IP检测、域名配置、WARP安装等网络相关功能
# =============================================================================

# =============================================================================
# IP检测和配置函数
# =============================================================================

# 检测IP类型并获取服务器IP
detect_ip_and_setup() {
    log_info "检测服务器IP地址..."
    
    # 使用多源检测IPv4地址
    local ipv4_result
    for url in "${IPV4_CHECK_URLS[@]}"; do
        log_debug "尝试IPv4检测: $url"
        if ipv4_result=$(safe_curl "$url" "$NETWORK_TIMEOUT" 1); then
            if [[ "$ipv4_result" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
                SERVER_IP="$ipv4_result"
                break
            fi
        fi
    done
    
    if [[ -n "$SERVER_IP" ]]; then
        log_info "获取到IPv4地址: $SERVER_IP"
        
        # 检查是否为Cloudflare
        local org
        if org=$(safe_curl "https://ipinfo.io/org" "$NETWORK_TIMEOUT" 1); then
            if echo "$org" | grep -qi "cloudflare"; then
                setup_cloudflare_domain
            fi
        fi
    else
        log_info "无法获取IPv4地址，尝试IPv6..."
        
        # 使用多源检测IPv6地址
        local ipv6_result
        for url in "${IPV6_CHECK_URLS[@]}"; do
            log_debug "尝试IPv6检测: $url"
            if ipv6_result=$(curl -6 -s --max-time "$NETWORK_TIMEOUT" "$url" 2>/dev/null); then
                if [[ "$ipv6_result" =~ ^[0-9a-fA-F:]+$ ]]; then
                    SERVER_IP="$ipv6_result"
                    IS_IPV6=true
                    break
                fi
            fi
        done
        
        if [[ -n "$SERVER_IP" ]]; then
            log_info "获取到IPv6地址: $SERVER_IP"
            setup_ipv6_domain
            install_warp
        else
            error_exit "无法获取服务器的公网IP地址"
        fi
    fi
}

# 设置Cloudflare域名
setup_cloudflare_domain() {
    while true; do
        read -p "请输入 Cloudflare 上的域名: " DOMAIN_NAME
        if [[ "$DOMAIN_NAME" =~ ^([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$ ]]; then
            CERT_FILE="/etc/ssl/cert/certCDN.pem"
            KEY_FILE="/etc/ssl/cert/privateCDN.key"
            
            # 验证域名解析并申请证书
            verify_domain_cloudflare
            break
        else
            print_error "输入的不是有效的域名格式，请重新输入"
        fi
    done
}

# 验证Cloudflare域名解析
verify_domain_cloudflare() {
    log_info "验证Cloudflare域名解析..."
    
    # 安装DNS工具
    case "$PACKAGE_MANAGER" in
        "apt")
            apt-get update >/dev/null 2>&1
            apt-get install -y dnsutils >/dev/null 2>&1
            ;;
        "yum"|"dnf")
            $PACKAGE_MANAGER install -y bind-utils >/dev/null 2>&1
            ;;
    esac
    
    local domain_ip
    domain_ip=$(dig A "$DOMAIN_NAME" +short | head -n1)
    
    print_info "本机 IP 地址: $SERVER_IP"
    print_info "域名解析 IP: $domain_ip"
    
    if [[ "$SERVER_IP" == "$domain_ip" ]]; then
        print_success "域名解析地址与本机 IP 一致"
        verify_certificates
    else
        log_warn "域名解析地址与本机 IP 不一致，这在Cloudflare代理模式下是正常的"
        print_info "继续申请证书..."
        verify_certificates
    fi
}

# 设置IPv6域名
setup_ipv6_domain() {
    while true; do
        read -p "IPv6 必须拥有域名和证书，请输入您已解析在 Cloudflare 的域名: " DOMAIN_NAME
        if [[ "$DOMAIN_NAME" =~ ^([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$ ]]; then
            verify_domain_resolution
            break
        else
            print_error "输入的不是有效的域名格式，请重新输入"
        fi
    done
}

# 验证域名解析
verify_domain_resolution() {
    log_info "验证域名解析..."
    
    # 安装DNS工具
    case "$PACKAGE_MANAGER" in
        "apt")
            apt-get update >/dev/null 2>&1
            apt-get install -y dnsutils >/dev/null 2>&1
            ;;
        "yum"|"dnf")
            $PACKAGE_MANAGER install -y bind-utils >/dev/null 2>&1
            ;;
    esac
    
    local local_ipv6 domain_ipv6
    local_ipv6=$(curl -6 -s --max-time 10 ifconfig.me || echo "")
    domain_ipv6=$(dig AAAA "$DOMAIN_NAME" +short | head -n1)
    
    print_info "本机 IPv6 地址: $local_ipv6"
    print_info "域名解析 IPv6: $domain_ipv6"
    
    if [[ "$local_ipv6" == "$domain_ipv6" ]]; then
        print_success "域名解析地址与本机 IPv6 一致"
        CERT_FILE="/etc/ssl/cert/certCDN.pem"
        KEY_FILE="/etc/ssl/cert/privateCDN.key"
        verify_certificates
    else
        error_exit "域名解析地址与本机 IPv6 不一致，请检查 Cloudflare 解析设置"
    fi
}

# 验证证书文件
verify_certificates() {
    mkdir -p "$(dirname "$CERT_FILE")"
    
    if [[ -f "$CERT_FILE" && -f "$KEY_FILE" ]]; then
        # 检查证书是否过期
        if check_certificate_expiry "$CERT_FILE"; then
            print_success "证书文件和私钥文件已存在且有效"
            return 0
        else
            log_warn "证书已过期或即将过期，将重新申请"
        fi
    fi
    
    # 如果证书不存在或已过期，自动申请证书
    print_info "开始为域名 $DOMAIN_NAME 申请 SSL 证书..."
    request_ssl_certificate
}

# =============================================================================
# SSL 证书管理功能
# =============================================================================

# 检查证书是否过期
check_certificate_expiry() {
    local cert_file="$1"
    
    if [[ ! -f "$cert_file" ]]; then
        return 1
    fi
    
    # 获取证书的过期时间（Unix时间戳）
    local cert_expiry
    cert_expiry=$(openssl x509 -in "$cert_file" -noout -dates | grep 'notAfter' | cut -d'=' -f2)
    local expiry_timestamp
    expiry_timestamp=$(date -d "$cert_expiry" +%s 2>/dev/null)
    
    if [[ -z "$expiry_timestamp" ]]; then
        log_warn "无法解析证书过期时间"
        return 1
    fi
    
    # 获取当前时间戳
    local current_timestamp
    current_timestamp=$(date +%s)
    
    # 计算剩余天数（30天缓冲期）
    local remaining_seconds=$((expiry_timestamp - current_timestamp))
    local remaining_days=$((remaining_seconds / 86400))
    
    log_info "证书剩余有效期：$remaining_days 天"
    
    if [[ $remaining_days -gt 30 ]]; then
        return 0  # 证书有效
    else
        return 1  # 证书即将过期或已过期
    fi
}

# 安装 certbot
install_certbot() {
    log_info "安装 certbot..."
    
    case "$PACKAGE_MANAGER" in
        "apt")
            apt-get update >/dev/null 2>&1
            apt-get install -y snapd >/dev/null 2>&1 || {
                # 如果snapd安装失败，使用apt安装
                apt-get install -y certbot >/dev/null 2>&1
                return $?
            }
            # 使用snap安装certbot
            snap install core >/dev/null 2>&1
            snap refresh core >/dev/null 2>&1
            snap install --classic certbot >/dev/null 2>&1
            ln -sf /snap/bin/certbot /usr/bin/certbot 2>/dev/null || true
            ;;
        "yum"|"dnf")
            # 安装EPEL源
            $PACKAGE_MANAGER install -y epel-release >/dev/null 2>&1 || true
            $PACKAGE_MANAGER install -y certbot >/dev/null 2>&1
            ;;
    esac
    
    # 验证安装
    if command -v certbot >/dev/null 2>&1; then
        log_info "certbot 安装成功"
        return 0
    else
        log_error "certbot 安装失败"
        return 1
    fi
}

# 申请 SSL 证书
request_ssl_certificate() {
    # 安装 certbot
    if ! install_certbot; then
        error_exit "安装 certbot 失败"
    fi
    
    log_info "为域名 $DOMAIN_NAME 申请 SSL 证书..."
    
    # 停止可能占用80端口的服务
    local services_to_stop=("nginx" "apache2" "httpd" "caddy")
    local stopped_services=()
    
    for service in "${services_to_stop[@]}"; do
        if systemctl is-active --quiet "$service" 2>/dev/null; then
            log_info "临时停止服务: $service"
            systemctl stop "$service"
            stopped_services+=("$service")
        fi
    done
    
    # 使用 standalone 模式申请证书
    local certbot_email="admin@${DOMAIN_NAME}"
    log_info "使用邮箱: $certbot_email"
    
    # 申请证书
    if certbot certonly \
        --standalone \
        --non-interactive \
        --agree-tos \
        --email "$certbot_email" \
        --domains "$DOMAIN_NAME" \
        --keep-until-expiring \
        --expand; then
        
        log_info "证书申请成功"
        
        # 复制证书到指定目录
        local letsencrypt_cert="/etc/letsencrypt/live/$DOMAIN_NAME/fullchain.pem"
        local letsencrypt_key="/etc/letsencrypt/live/$DOMAIN_NAME/privkey.pem"
        
        if [[ -f "$letsencrypt_cert" && -f "$letsencrypt_key" ]]; then
            cp "$letsencrypt_cert" "$CERT_FILE"
            cp "$letsencrypt_key" "$KEY_FILE"
            
            # 设置正确的权限
            chmod 644 "$CERT_FILE"
            chmod 600 "$KEY_FILE"
            
            print_success "证书已保存到:"
            print_info "  证书文件: $CERT_FILE"
            print_info "  私钥文件: $KEY_FILE"
            
            # 设置自动续期
            setup_certificate_renewal
        else
            log_error "证书文件未找到"
        fi
    else
        log_error "证书申请失败"
        
        # 恢复停止的服务
        for service in "${stopped_services[@]}"; do
            log_info "恢复服务: $service"
            systemctl start "$service"
        done
        
        error_exit "SSL证书申请失败，请检查域名解析和网络连接"
    fi
    
    # 恢复停止的服务
    for service in "${stopped_services[@]}"; do
        log_info "恢复服务: $service"
        systemctl start "$service"
    done
}

# 设置证书自动续期
setup_certificate_renewal() {
    log_info "设置证书自动续期..."
    
    # 创建续期脚本
    cat > /usr/local/bin/renew-singbox-cert.sh <<'EOF'
#!/bin/bash
# SingBox 证书自动续期脚本

DOMAIN_NAME="$1"
CERT_FILE="/etc/ssl/cert/certCDN.pem"
KEY_FILE="/etc/ssl/cert/privateCDN.key"

if [[ -z "$DOMAIN_NAME" ]]; then
    echo "错误：缺少域名参数"
    exit 1
fi

# 续期证书
if certbot renew --quiet; then
    # 复制新证书
    if [[ -f "/etc/letsencrypt/live/$DOMAIN_NAME/fullchain.pem" && -f "/etc/letsencrypt/live/$DOMAIN_NAME/privkey.pem" ]]; then
        cp "/etc/letsencrypt/live/$DOMAIN_NAME/fullchain.pem" "$CERT_FILE"
        cp "/etc/letsencrypt/live/$DOMAIN_NAME/privkey.pem" "$KEY_FILE"
        
        # 设置权限
        chmod 644 "$CERT_FILE"
        chmod 600 "$KEY_FILE"
        
        # 重启 sing-box 服务
        systemctl restart sing-box 2>/dev/null || true
        
        echo "证书续期成功并已重启 sing-box 服务"
    fi
else
    echo "证书续期失败"
    exit 1
fi
EOF
    
    chmod +x /usr/local/bin/renew-singbox-cert.sh
    
    # 添加到 crontab（每天凌晨2点检查）
    local cron_job="0 2 * * * /usr/local/bin/renew-singbox-cert.sh $DOMAIN_NAME >> /var/log/singbox-cert-renewal.log 2>&1"
    
    # 检查是否已存在相同的定时任务
    if ! crontab -l 2>/dev/null | grep -F "/usr/local/bin/renew-singbox-cert.sh" >/dev/null; then
        (crontab -l 2>/dev/null; echo "$cron_job") | crontab -
        log_info "已设置证书自动续期定时任务"
    else
        log_info "证书自动续期定时任务已存在"
    fi
}

# =============================================================================
# WARP安装和配置
# =============================================================================

# 安装WARP
install_warp() {
    log_info "安装 WARP..."
    
    # 下载wgcf使用改进的下载函数
    local wgcf_url="https://raw.githubusercontent.com/tanselxy/singbox/main/wgcf_2.2.15_linux_amd64"
    if ! download_file "$wgcf_url" "$TEMP_DIR/wgcf" "$DOWNLOAD_TIMEOUT"; then
        error_exit "下载 wgcf 失败"
    fi
    
    mv "$TEMP_DIR/wgcf" /usr/local/bin/wgcf
    chmod +x /usr/local/bin/wgcf
    
    # 注册WARP账户
    if [[ ! -f wgcf-account.toml ]]; then
        log_info "注册 WARP 账户..."
        wgcf register
    fi
    
    wgcf generate
    
    # 修改配置
    sed -i 's/^\(DNS *=.*\)/# \1/' wgcf-profile.conf
    sed -i 's/^\(AllowedIPs *= ::\/0\)/# \1/' wgcf-profile.conf
    
    # 安装WireGuard
    setup_system_sources
    
    cp wgcf-profile.conf /etc/wireguard/wgcf.conf
    
    # 启动WireGuard
    if ! ip link show wgcf >/dev/null 2>&1; then
        wg-quick up wgcf
    fi
    
    local warp_ip
    warp_ip=$(curl --interface wgcf https://api.ipify.org 2>/dev/null || echo "获取失败")
    print_success "WARP IPv4 地址: $warp_ip"
}

# 设置系统源
setup_system_sources() {
    local os_id
    os_id=$(grep '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"')
    
    case "$os_id" in
        ubuntu)
            log_info "设置 Ubuntu 源..."
            cat > /etc/apt/sources.list <<EOF
deb http://archive.ubuntu.com/ubuntu noble main restricted universe multiverse
deb http://archive.ubuntu.com/ubuntu noble-updates main restricted universe multiverse
deb http://security.ubuntu.com/ubuntu noble-security main restricted universe multiverse
EOF
            ;;
        debian)
            log_info "设置 Debian 源..."
            cat > /etc/apt/sources.list <<EOF
deb http://deb.debian.org/debian bookworm main contrib non-free
deb http://deb.debian.org/debian bookworm-updates main contrib non-free
deb http://security.debian.org/debian-security bookworm-security main contrib non-free
EOF
            ;;
        centos)
            log_info "设置 CentOS 源..."
            # CentOS Stream 源配置
            if [[ -f /etc/yum.repos.d/CentOS-Stream-BaseOS.repo ]]; then
                log_info "检测到 CentOS Stream，保持默认源配置"
            else
                log_warn "CentOS 源配置保持默认"
            fi
            ;;
        rhel|rocky|almalinux)
            log_info "设置 $os_id 源..."
            # 启用EPEL源
            if command -v dnf >/dev/null 2>&1; then
                dnf install -y epel-release >/dev/null 2>&1 || true
            else
                yum install -y epel-release >/dev/null 2>&1 || true
            fi
            ;;
        fedora)
            log_info "Fedora 源配置保持默认"
            ;;
    esac
    
    # 安装WireGuard
    case "$PACKAGE_MANAGER" in
        "apt")
            apt-get update >/dev/null 2>&1
            apt-get install -y wireguard >/dev/null 2>&1
            ;;
        "yum"|"dnf")
            $PACKAGE_MANAGER install -y wireguard-tools >/dev/null 2>&1
            ;;
    esac
}

# =============================================================================
# 域名选择函数
# =============================================================================

# 根据地区选择推荐域名
select_domain() {
    log_info "根据地区选择推荐域名..."
    
    local country_code
    if ! country_code=$(safe_curl "https://ipapi.co/country/" "$NETWORK_TIMEOUT" 1); then
        log_warn "无法检测地区，使用默认地区 US"
        country_code="US"
    fi
    
    # 使用关联数组选择域名
    SERVER="${DOMAIN_BY_COUNTRY[$country_code]:-${DOMAIN_BY_COUNTRY[DEFAULT]}}"
    
    print_info "当前地区: $country_code，推荐域名: $SERVER"
    
    read -p "是否使用推荐域名 $SERVER？(Y/n): " use_suggested
    if [[ "$use_suggested" =~ ^[Nn]$ ]]; then
        read -p "是否自定义域名？(Y/n): " input_custom
        if [[ "$input_custom" =~ ^[Yy]$ ]]; then
            read -p "请输入域名: " SERVER
        else
            SERVER="www.apple.com"
        fi
    fi
    
    log_info "使用域名: $SERVER"
}

# =============================================================================
# WARP 检测和智能 IP 选择函数
# =============================================================================

# WARP检测结果缓存
IS_WARP_CACHED=""

# 检测IPv4是否为WARP（带缓存，避免重复检测）
is_warp_ipv4() {
    # 如果已经检测过，直接返回缓存结果
    if [[ -n "$IS_WARP_CACHED" ]]; then
        [[ "$IS_WARP_CACHED" == "true" ]] && return 0 || return 1
    fi
    
    # 首次检测
    if [[ -n "$SERVER_IP" ]]; then
        local org
        if org=$(safe_curl "https://ipinfo.io/org" "$NETWORK_TIMEOUT" 1); then
            if echo "$org" | grep -qi "cloudflare"; then
                IS_WARP_CACHED="true"
                return 0  # 是WARP
            fi
        fi
    fi
    
    IS_WARP_CACHED="false"
    return 1  # 不是WARP
}

# 根据WARP状态选择最佳IP
get_optimal_ip() {
    if is_warp_ipv4; then
        # 如果有IPv6域名，优先使用域名
        if [[ "$IS_IPV6" == true && -n "$DOMAIN_NAME" ]]; then
            echo "$DOMAIN_NAME"
        # 如果有IPv6地址但没有域名，使用IPv6地址
        elif [[ "$IS_IPV6" == true && -n "$SERVER_IP" && "$SERVER_IP" =~ ^[0-9a-fA-F:]+$ ]]; then
            echo "[$SERVER_IP]"  # IPv6地址需要用方括号包围
        # 如果有其他域名，使用域名
        elif [[ -n "$DOMAIN_NAME" ]]; then
            echo "$DOMAIN_NAME"
        else
            echo "$SERVER_IP"
        fi
    else
        echo "$SERVER_IP"    # 使用原IP
    fi
}

# =============================================================================
# 代理链接生成函数
# =============================================================================

# 生成所有代理链接
generate_proxy_links() {
    log_info "生成所有代理链接..."
    
    # 获取最佳IP地址
    local optimal_ip
    optimal_ip=$(get_optimal_ip)
    
    echo ""
    print_colored "$RED" "=================== 代理链接汇总 ==================="
    echo ""
    
    # 1. Reality链接
    local reality_link="vless://${UUID}@${optimal_ip}:${VLESS_PORT}?security=reality&flow=xtls-rprx-vision&type=tcp&sni=${SERVER}&fp=chrome&pbk=Y_-yCHC3Qi-Kz6OWpueQckAJSQuGEKffwWp8MlFgwTs&sid=0123456789abcded&encryption=none#Reality"
    echo "🔷 Reality (VLESS) 链接:"
    echo "$reality_link"
    echo ""
    
    # 2. Hysteria2链接
    local hy2_link="hysteria2://${HYSTERIA_PASSWORD}@${optimal_ip}:${HYSTERIA_PORT}?insecure=1&alpn=h3&sni=bing.com#Hysteria2"
    echo "🚀 Hysteria2 链接:"
    echo "$hy2_link"
    echo ""
    
    # 3. Trojan链接
    local trojan_link="trojan://${HYSTERIA_PASSWORD}@${optimal_ip}:63333?sni=bing.com&type=ws&path=%2Ftrojan&host=bing.com&allowInsecure=1&udp=true&alpn=http%2F1.1#Trojan"
    echo "🛡️ Trojan WS 链接:"
    echo "$trojan_link"
    echo ""
    
    # 4. TUIC链接
    local tuic_link="tuic://${UUID}:@${optimal_ip}:61555?alpn=h3&allow_insecure=1&congestion_control=bbr#TUIC"
    echo "⚡ TUIC 链接:"
    echo "$tuic_link"
    echo ""
    
    # 5. ShadowTLS + SS2022链接
    if [[ -n "$SS_PASSWORD" ]]; then
        generate_ss2022_link "$SS_PASSWORD"
    else
        log_warn "SS密码未找到，跳过ShadowTLS链接生成"
    fi
    echo ""
    
    # 6. SS专线链接
    local ss_encoded
    ss_encoded=$(echo -n "aes-128-gcm:${HYSTERIA_PASSWORD}" | base64 2>/dev/null | tr -d '\n')
    local ss_link="ss://${ss_encoded}@${optimal_ip}:59000#SS专线"
    echo "📡 SS 专线链接:"
    echo "$ss_link"
    echo ""
    
    # 保存SS专线链接供二维码使用
    echo "$ss_link" > /tmp/ss_link.tmp
    
    # 7. IPv6链接（如果有域名）
    if [[ "$IS_IPV6" == true && -n "$DOMAIN_NAME" ]]; then
        generate_ipv6_link
        echo ""
    fi
    
    print_colored "$RED" "=============================================="
    echo ""
}

# 生成SS2022链接
generate_ss2022_link() {
    local ss_password="$1"
    local server=$(get_optimal_ip)
    local port="$SS_PORT"
    local cipher="2022-blake3-chacha20-poly1305"
    local plugin_host="$SERVER"
    local plugin_password="AaaY/lgWSBlSQtDmd0UpFnqR1JJ9JTHn0CLBv12KO5o="
    local plugin_version="3"
    local name="ShadowTLS-v3"

    # 创建用户信息部分并Base64编码
    local user_info="${cipher}:${ss_password}"
    local user_info_base64
    user_info_base64=$(echo -n "$user_info" | base64)

    # 创建shadow-tls JSON并Base64编码
    local shadow_tls_json="{\"address\":\"$server\",\"password\":\"$plugin_password\",\"version\":\"$plugin_version\",\"host\":\"$plugin_host\",\"port\":\"$port\"}"
    local shadow_tls_base64
    shadow_tls_base64=$(echo -n "$shadow_tls_json" | base64)

    # 构建完整的SS URL
    local url="ss://${user_info_base64}@[${server}]:${port}?shadow-tls=${shadow_tls_base64}#$(echo -n "$name" | sed 's/ /%20/g')"
    
    echo "🔐 ShadowTLS v3 + SS2022 链接:"
    echo "$url"
    
    # 返回链接供二维码使用
    echo "$url" > /tmp/ss2022_link.tmp
}

# 生成IPv6链接
generate_ipv6_link() {
    local optimization_domain="$DOMAIN_NAME"
    
    # 检查域名是否被微信屏蔽
    local url="https://cgi.urlsec.qq.com/index.php?m=url&a=validUrl&url=https://$DOMAIN_NAME"
    local is_use
    is_use=$(safe_curl "$url" 5 1 || echo "")
    
    if echo "$is_use" | grep -q '"evil_type":0' 2>/dev/null; then
        log_info "域名通过微信检测"
    else
        log_warn "域名可能被微信屏蔽，使用备用域名"
        optimization_domain="csgo.com"
    fi
    
    local ipv6_link="vless://${UUID}@${optimization_domain}:443?encryption=none&security=tls&type=ws&host=${DOMAIN_NAME}&sni=${DOMAIN_NAME}&path=%2Fvless#IPv6节点"
    
    echo "🌐 IPv6 节点链接:"
    echo "$ipv6_link"
}