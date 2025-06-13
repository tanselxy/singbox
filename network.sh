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
    
    # 尝试获取IPv4地址
    SERVER_IP=$(curl -4 -s https://api64.ipify.org ||curl -4 -s --max-time 10 ifconfig.me || curl -4 -s --max-time 10 ipinfo.io/ip || echo "")
    
    if [[ -n "$SERVER_IP" ]]; then
        log_info "获取到IPv4地址: $SERVER_IP"
        
        # 检查是否为Cloudflare
        local org
        org=$(curl -s --max-time 10 https://ipinfo.io/org 2>/dev/null || echo "")
        if echo "$org" | grep -qi "cloudflare"; then
            setup_cloudflare_domain
        fi
    else
        log_info "无法获取IPv4地址，尝试IPv6..."
        SERVER_IP=$(curl -6 -s --max-time 10 ifconfig.me || curl -6 -s --max-time 10 ipinfo.io/ip || echo "")
        
        if [[ -n "$SERVER_IP" ]]; then
            IS_IPV6=true
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
            break
        else
            print_error "输入的不是有效的域名格式，请重新输入"
        fi
    done
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
        print_success "证书文件和私钥文件已存在"
    else
        print_error "缺少证书文件或私钥文件："
        [[ ! -f "$CERT_FILE" ]] && echo "  - 缺少证书文件: $CERT_FILE"
        [[ ! -f "$KEY_FILE" ]] && echo "  - 缺少私钥文件: $KEY_FILE"
        error_exit "请确保证书文件存在"
    fi
}

# =============================================================================
# WARP安装和配置
# =============================================================================

# 安装WARP
install_warp() {
    log_info "安装 WARP..."
    
    # 下载wgcf
    if ! curl -H 'Cache-Control: no-cache' -o "$TEMP_DIR/wgcf" \
        "https://raw.githubusercontent.com/tanselxy/singbox/main/wgcf_2.2.15_linux_amd64"; then
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
    country_code=$(curl -s --max-time 10 https://ipapi.co/country/ 2>/dev/null || echo "US")
    
    case "$country_code" in
        TW) SERVER="www.apple.com" ;;
        NG) SERVER="unn.edu.ng" ;;
        JP) SERVER="www.tms-e.co.jp" ;;
        US) SERVER="www.thewaltdisneycompany.com" ;;
        NL) SERVER="nl.servutech.com" ;;
        DE) SERVER="www.mediamarkt.de" ;;
        HK) SERVER="www.apple.com" ;;
        *) SERVER="www.apple.com" ;;
    esac
    
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
# 代理链接生成函数
# =============================================================================

# 生成所有代理链接
generate_proxy_links() {
    log_info "生成所有代理链接..."
    
    echo ""
    print_colored "$RED" "=================== 代理链接汇总 ==================="
    echo ""
    
    # 1. Reality链接
    local reality_link="vless://${UUID}@${SERVER_IP}:${VLESS_PORT}?security=reality&flow=xtls-rprx-vision&type=tcp&sni=${SERVER}&fp=chrome&pbk=Y_-yCHC3Qi-Kz6OWpueQckAJSQuGEKffwWp8MlFgwTs&sid=0123456789abcded&encryption=none#Reality"
    echo "🔷 Reality (VLESS) 链接:"
    echo "$reality_link"
    echo ""
    
    # 2. Hysteria2链接
    local hy2_link="hysteria2://${HYSTERIA_PASSWORD}@${SERVER_IP}:${HYSTERIA_PORT}?insecure=1&alpn=h3&sni=bing.com#Hysteria2"
    echo "🚀 Hysteria2 链接:"
    echo "$hy2_link"
    echo ""
    
    # 3. Trojan链接
    local trojan_link="trojan://${HYSTERIA_PASSWORD}@${SERVER_IP}:63333?sni=bing.com&type=ws&path=%2Ftrojan&host=bing.com&allowInsecure=1&udp=true&alpn=http%2F1.1#Trojan"
    echo "🛡️ Trojan WS 链接:"
    echo "$trojan_link"
    echo ""
    
    # 4. TUIC链接
    local tuic_link="tuic://${UUID}:@${SERVER_IP}:61555?alpn=h3&allow_insecure=1&congestion_control=bbr#TUIC"
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
    local ss_link="ss://${ss_encoded}@${SERVER_IP}:59000#SS专线"
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
    local server="$SERVER_IP"
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
    is_use=$(curl -s --max-time 5 "$url" 2>/dev/null || echo "")
    
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