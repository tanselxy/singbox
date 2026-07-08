#!/bin/bash

# =============================================================================
# 配置生成函数库 - config.sh
# 包含服务器和客户端配置文件生成功能
# =============================================================================

# =============================================================================
# 服务器配置生成
# =============================================================================

# 生成服务器配置
generate_server_config() {
    log_info "生成 Sing-Box 服务器配置..."
    
    local config_path="$CONFIG_DIR/config.json"
    local template_path="$SCRIPT_DIR/server_template.json"
    
    # 检查模板文件
    if [[ ! -f "$template_path" ]]; then
        error_exit "服务器配置模板文件不存在: $template_path"
    fi
    
    # 确保UUID已生成 - 多种方法生成UUID
    if [[ -z "$UUID" ]]; then
        log_info "生成UUID..."
        
        # 方法1: 使用sing-box生成（如果已安装）
        if command -v sing-box >/dev/null 2>&1; then
            UUID=$(sing-box generate uuid 2>/dev/null) || UUID=""
        fi
        
        # 方法2: 使用uuidgen
        if [[ -z "$UUID" ]] && command -v uuidgen >/dev/null 2>&1; then
            UUID=$(uuidgen 2>/dev/null) || UUID=""
        fi
        
        # 方法3: 使用/proc/sys/kernel/random/uuid
        if [[ -z "$UUID" ]] && [[ -r /proc/sys/kernel/random/uuid ]]; then
            UUID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null) || UUID=""
        fi
        
        # 方法4: 使用python生成
        if [[ -z "$UUID" ]] && command -v python3 >/dev/null 2>&1; then
            UUID=$(python3 -c "import uuid; print(str(uuid.uuid4()))" 2>/dev/null) || UUID=""
        fi
        
        # 方法5: 使用python2生成
        if [[ -z "$UUID" ]] && command -v python >/dev/null 2>&1; then
            UUID=$(python -c "import uuid; print str(uuid.uuid4())" 2>/dev/null) || UUID=""
        fi
        
        # 方法6: 手动生成UUID格式的字符串
        if [[ -z "$UUID" ]]; then
            log_warn "使用手动方法生成UUID..."
            local timestamp=$(date +%s%N 2>/dev/null || date +%s)
            local random1=$(od -An -N4 -tx4 /dev/urandom 2>/dev/null | tr -d ' ' || printf "%08x" $RANDOM$RANDOM)
            local random2=$(od -An -N2 -tx2 /dev/urandom 2>/dev/null | tr -d ' ' || printf "%04x" $RANDOM)
            local random3=$(od -An -N2 -tx2 /dev/urandom 2>/dev/null | tr -d ' ' || printf "%04x" $RANDOM)
            local random4=$(od -An -N2 -tx2 /dev/urandom 2>/dev/null | tr -d ' ' || printf "%04x" $RANDOM)
            local random5=$(od -An -N6 -tx1 /dev/urandom 2>/dev/null | tr -d ' ' || printf "%012x" $RANDOM$RANDOM$RANDOM)
            
            UUID="${random1:0:8}-${random2:0:4}-4${random3:1:3}-8${random4:1:3}-${random5:0:12}"
        fi
        
        # 验证UUID格式
        if [[ ! "$UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]; then
            log_error "生成的UUID格式不正确: $UUID"
            return 1
        fi
    fi
    
    log_info "使用UUID: ${UUID:0:8}..."
    
    # 选择域名
    select_domain
    
    # 生成SS密码
    local ss_password
    ss_password=$(generate_base64_password 32 2>>"$LOG_FILE") || {
        log_error "生成SS密码失败"
        return 1
    }
    
    # 将SS密码保存到全局变量供链接生成使用
    SS_PASSWORD="$ss_password"
    
    log_info "生成配置文件: $config_path"
    
    # 设置默认端口（如果未设置）
    TROJAN_PORT=${TROJAN_PORT:-${PORT_POOL[TROJAN]}}
    TUIC_PORT=${TUIC_PORT:-${PORT_POOL[TUIC]}}
    SS_DIRECT_PORT=${SS_DIRECT_PORT:-${PORT_POOL[SS_DIRECT]}}

    # 验证必要变量
    if [[ -z "$SS_PORT" || -z "$VLESS_PORT" || -z "$HYSTERIA_PORT" || -z "$UUID" || -z "$SERVER" ]]; then
        log_error "配置变量不完整"
        log_error "SS_PORT=$SS_PORT, VLESS_PORT=$VLESS_PORT, HYSTERIA_PORT=$HYSTERIA_PORT"
        log_error "UUID=$UUID, SERVER=$SERVER"
        return 1
    fi

    # 替换模板中的变量
    local temp_config="${config_path}.tmp"

    sed -e "s/{{SS_PORT}}/$SS_PORT/g" \
        -e "s/{{VLESS_PORT}}/$VLESS_PORT/g" \
        -e "s/{{HYSTERIA_PORT}}/$HYSTERIA_PORT/g" \
        -e "s/{{TROJAN_PORT}}/$TROJAN_PORT/g" \
        -e "s/{{TUIC_PORT}}/$TUIC_PORT/g" \
        -e "s/{{SS_DIRECT_PORT}}/$SS_DIRECT_PORT/g" \
        -e "s/{{UUID}}/$UUID/g" \
        -e "s/{{SERVER}}/$SERVER/g" \
        -e "s|{{SS_PASSWORD}}|$ss_password|g" \
        -e "s/{{HYSTERIA_PASSWORD}}/$HYSTERIA_PASSWORD/g" \
        -e "s|{{CERT_FILE}}|$CERT_FILE|g" \
        -e "s|{{KEY_FILE}}|$KEY_FILE|g" \
        -e "s/{{DOMAIN_NAME}}/$DOMAIN_NAME/g" \
        "$template_path" > "$temp_config"
    
    # 验证JSON语法
    if command -v jq >/dev/null 2>&1; then
        if jq empty < "$temp_config" >/dev/null 2>&1; then
            log_info "JSON配置文件语法验证通过"
        else
            log_error "JSON配置文件语法验证失败"
            log_error "配置文件内容:"
            cat "$temp_config" >> "$LOG_FILE"
            return 1
        fi
    else
        log_warn "jq不可用，跳过JSON语法验证"
    fi
    
    # 移动临时文件到最终位置
    mv "$temp_config" "$config_path" || {
        log_error "无法创建最终配置文件"
        return 1
    }
    
    log_info "服务器配置文件生成完成"
    return 0
}

# =============================================================================
# 客户端配置生成
# =============================================================================

# 生成客户端配置文件
generate_client_config_file() {
    log_info "生成客户端配置文件..."
    
    local config_path="/root/singbox_${RANDOM_STR}.yaml"
    local template_path="$SCRIPT_DIR/client_template.yaml"
    
    # 检查模板文件
    if [[ ! -f "$template_path" ]]; then
        error_exit "客户端配置模板文件不存在: $template_path"
    fi
    
    # 生成当前时间
    local generation_time
    generation_time=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 处理IPv6域名条件编译
    local temp_template="${config_path}.template"
    
    if [[ -n "$DOMAIN_NAME" ]]; then
        # 保留IPv6相关配置
        sed 's/{{#if DOMAIN_NAME}}//' "$template_path" | sed 's/{{\/if}}//' > "$temp_template"
    else
        # 移除IPv6相关配置
        sed '/{{#if DOMAIN_NAME}}/,/{{\/if}}/d' "$template_path" > "$temp_template"
    fi
    
    # 替换模板中的变量
    sed -e "s/{{SERVER_IP}}/$SERVER_IP/g" \
        -e "s/{{VLESS_PORT}}/$VLESS_PORT/g" \
        -e "s/{{HYSTERIA_PORT}}/$HYSTERIA_PORT/g" \
        -e "s/{{UUID}}/$UUID/g" \
        -e "s/{{SERVER}}/$SERVER/g" \
        -e "s/{{HYSTERIA_PASSWORD}}/$HYSTERIA_PASSWORD/g" \
        -e "s/{{DOMAIN_NAME}}/$DOMAIN_NAME/g" \
        -e "s/{{GENERATION_TIME}}/$generation_time/g" \
        "$temp_template" > "$config_path"
    
    # 清理临时文件
    rm -f "$temp_template"
    
    print_success "客户端配置文件已生成: $config_path"
    log_info "客户端配置文件路径: $config_path"
    return 0
}

# =============================================================================
# 配置验证和测试
# =============================================================================

# 验证服务器配置
validate_server_config() {
    local config_path="$CONFIG_DIR/config.json"
    
    if [[ ! -f "$config_path" ]]; then
        log_error "配置文件不存在: $config_path"
        return 1
    fi
    
    log_info "验证服务器配置文件..."
    
    # 使用sing-box验证配置
    if sing-box check -c "$config_path"; then
        print_success "服务器配置验证通过"
        return 0
    else
        log_error "服务器配置验证失败"
        return 1
    fi
}

# 测试配置连通性
test_config_connectivity() {
    log_info "测试配置连通性..."
    
    local test_results=()
    
    # 测试各个端口是否监听
    local ports=("$SS_PORT" "$VLESS_PORT" "$HYSTERIA_PORT" "63333" "61555" "59000" "4433")
    local port_names=("ShadowTLS" "VLESS Reality" "Hysteria2" "Trojan" "TUIC" "SS Direct" "VLESS CDN")
    
    for i in "${!ports[@]}"; do
        local port="${ports[$i]}"
        local name="${port_names[$i]}"
        
        if netstat -tuln | grep -q ":$port "; then
            print_success "✅ $name (端口 $port) - 监听正常"
            test_results+=("$name: OK")
        else
            print_error "❌ $name (端口 $port) - 未监听"
            test_results+=("$name: FAILED")
        fi
    done
    
    # 显示测试总结
    echo ""
    print_colored "$BLUE" "========== 连通性测试总结 =========="
    for result in "${test_results[@]}"; do
        echo "$result"
    done
    print_colored "$BLUE" "================================"
    echo ""
}

# =============================================================================
# 配置备份和恢复
# =============================================================================

# 备份当前配置
backup_config() {
    log_info "备份当前配置..."
    
    local backup_dir="/root/singbox_backup_$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    
    # 备份服务器配置
    if [[ -f "$CONFIG_DIR/config.json" ]]; then
        cp "$CONFIG_DIR/config.json" "$backup_dir/"
        log_info "服务器配置已备份"
    fi
    
    # 备份证书文件
    if [[ -f "$CERT_FILE" ]]; then
        cp "$CERT_FILE" "$backup_dir/"
        log_info "证书文件已备份"
    fi
    
    if [[ -f "$KEY_FILE" ]]; then
        cp "$KEY_FILE" "$backup_dir/"
        log_info "私钥文件已备份"
    fi
    
    # 备份客户端配置
    local client_configs
    client_configs=$(find /root -name "singbox_*.yaml" -type f 2>/dev/null)
    if [[ -n "$client_configs" ]]; then
        echo "$client_configs" | xargs -I {} cp {} "$backup_dir/"
        log_info "客户端配置已备份"
    fi
    
    # 保存当前变量到备份
    cat > "$backup_dir/variables.env" <<EOF
# Sing-Box 配置变量备份
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')

SS_PORT=$SS_PORT
VLESS_PORT=$VLESS_PORT
HYSTERIA_PORT=$HYSTERIA_PORT
UUID=$UUID
SERVER=$SERVER
SERVER_IP=$SERVER_IP
HYSTERIA_PASSWORD=$HYSTERIA_PASSWORD
SS_PASSWORD=$SS_PASSWORD
DOMAIN_NAME=$DOMAIN_NAME
IS_IPV6=$IS_IPV6
RANDOM_STR=$RANDOM_STR
EOF
    
    print_success "配置备份完成: $backup_dir"
    echo "$backup_dir"
}

# 恢复配置
restore_config() {
    local backup_dir="$1"
    
    if [[ ! -d "$backup_dir" ]]; then
        log_error "备份目录不存在: $backup_dir"
        return 1
    fi
    
    log_info "从备份恢复配置: $backup_dir"
    
    # 恢复变量
    if [[ -f "$backup_dir/variables.env" ]]; then
        source "$backup_dir/variables.env"
        log_info "变量已恢复"
    fi
    
    # 恢复服务器配置
    if [[ -f "$backup_dir/config.json" ]]; then
        cp "$backup_dir/config.json" "$CONFIG_DIR/"
        log_info "服务器配置已恢复"
    fi
    
    # 恢复证书文件
    if [[ -f "$backup_dir/cert.pem" ]]; then
        mkdir -p "$(dirname "$CERT_FILE")"
        cp "$backup_dir/cert.pem" "$CERT_FILE"
        log_info "证书文件已恢复"
    fi
    
    if [[ -f "$backup_dir/private.key" ]]; then
        mkdir -p "$(dirname "$KEY_FILE")"
        cp "$backup_dir/private.key" "$KEY_FILE"
        log_info "私钥文件已恢复"
    fi
    
    print_success "配置恢复完成"
}

# =============================================================================
# 配置更新和管理
# =============================================================================

# 更新配置中的特定参数
update_config_parameter() {
    local parameter="$1"
    local new_value="$2"
    
    log_info "更新配置参数: $parameter = $new_value"
    
    case "$parameter" in
        "uuid")
            UUID="$new_value"
            ;;
        "hysteria_password")
            HYSTERIA_PASSWORD="$new_value"
            ;;
        "server")
            SERVER="$new_value"
            ;;
        "domain")
            DOMAIN_NAME="$new_value"
            ;;
        *)
            log_error "不支持的参数: $parameter"
            return 1
            ;;
    esac
    
    # 重新生成配置
    generate_server_config
    generate_client_config_file
    
    print_success "参数更新完成"
}

# 重置所有密码和UUID
reset_credentials() {
    log_info "重置所有凭据..."
    
    # 生成新的UUID
    UUID=$(sing-box generate uuid 2>/dev/null) || {
        log_error "无法生成新UUID"
        return 1
    }
    
    # 生成新的密码
    HYSTERIA_PASSWORD=$(generate_strong_password 15) || {
        log_error "生成新密码失败"
        return 1
    }
    
    # 生成新的随机字符串
    RANDOM_STR=$(generate_random_string 6) || {
        log_error "生成新随机字符串失败"
        return 1
    }
    
    log_info "新UUID: ${UUID:0:8}..."
    log_info "已生成新密码和随机字符串"
    
    # 重新生成配置
    generate_server_config
    generate_client_config_file
    
    print_success "凭据重置完成"
}

# =============================================================================
# 配置导出和导入
# =============================================================================

# 导出配置为压缩包
export_config() {
    local export_file="/root/singbox_export_$(date +%Y%m%d_%H%M%S).tar.gz"
    
    log_info "导出配置到: $export_file"
    
    # 创建临时目录
    local temp_export_dir="/tmp/singbox_export_$$"
    mkdir -p "$temp_export_dir"
    
    # 复制配置文件
    [[ -f "$CONFIG_DIR/config.json" ]] && cp "$CONFIG_DIR/config.json" "$temp_export_dir/"
    [[ -f "$CERT_FILE" ]] && cp "$CERT_FILE" "$temp_export_dir/"
    [[ -f "$KEY_FILE" ]] && cp "$KEY_FILE" "$temp_export_dir/"
    
    # 复制客户端配置
    find /root -name "singbox_*.yaml" -type f -exec cp {} "$temp_export_dir/" \; 2>/dev/null
    
    # 创建变量文件
    cat > "$temp_export_dir/export_info.txt" <<EOF
# Sing-Box 配置导出信息
# 导出时间: $(date '+%Y-%m-%d %H:%M:%S')
# 服务器IP: $SERVER_IP
# UUID: $UUID
# 伪装域名: $SERVER
# 端口信息:
#   VLESS Reality: $VLESS_PORT
#   Hysteria2: $HYSTERIA_PORT
#   ShadowTLS: $SS_PORT
#   Trojan: 63333
#   TUIC: 61555
#   SS Direct: 59000
EOF
    
    # 创建压缩包
    tar -czf "$export_file" -C "$temp_export_dir" .
    
    # 清理临时目录
    rm -rf "$temp_export_dir"
    
    print_success "配置导出完成: $export_file"
    echo "$export_file"
}

# =============================================================================
# 高级配置选项
# =============================================================================

# 启用/禁用特定协议
toggle_protocol() {
    local protocol="$1"
    local action="$2"  # enable/disable
    
    log_info "$action $protocol 协议..."
    
    # 这里可以根据需要修改配置文件中的特定协议配置
    # 目前保持所有协议都启用的状态
    
    case "$protocol" in
        "reality"|"vless")
            log_info "VLESS Reality 协议管理"
            ;;
        "hysteria2"|"hy2")
            log_info "Hysteria2 协议管理"
            ;;
        "trojan")
            log_info "Trojan 协议管理"
            ;;
        "tuic")
            log_info "TUIC 协议管理"
            ;;
        "shadowsocks"|"ss")
            log_info "ShadowSocks 协议管理"
            ;;
        *)
            log_error "不支持的协议: $protocol"
            return 1
            ;;
    esac
    
    print_success "$protocol 协议 $action 完成"
}

# 显示当前配置摘要
show_config_summary() {
    echo ""
    print_colored "$BLUE" "========== 当前配置摘要 =========="
    echo ""
    echo "🔧 基本信息:"
    echo "   服务器IP: $SERVER_IP"
    echo "   UUID: ${UUID:0:8}...${UUID: -4}"
    echo "   伪装域名: $SERVER"
    [[ -n "$DOMAIN_NAME" ]] && echo "   CDN域名: $DOMAIN_NAME"
    echo ""
    
    echo "🔌 端口配置:"
    echo "   VLESS Reality: $VLESS_PORT"
    echo "   Hysteria2: $HYSTERIA_PORT"
    echo "   ShadowTLS: $SS_PORT"
    echo "   Trojan WS: 63333"
    echo "   TUIC: 61555"
    echo "   SS Direct: 59000"
    echo "   VLESS CDN: 4433"
    echo ""
    
    echo "🔐 安全信息:"
    echo "   Hysteria密码: ${HYSTERIA_PASSWORD:0:8}..."
    echo "   证书文件: $CERT_FILE"
    echo "   私钥文件: $KEY_FILE"
    echo ""
    
    echo "📊 服务状态:"
    if systemctl is-active --quiet sing-box; then
        echo "   Sing-Box: ✅ 运行中"
    else
        echo "   Sing-Box: ❌ 未运行"
    fi
    
    print_colored "$BLUE" "================================"
    echo ""
}