#!/bin/bash

# =============================================================================
# 错误处理和重试机制模块
# 提供统一的错误处理、重试逻辑和网络请求功能
# =============================================================================

# 加载默认配置
source "$(dirname "${BASH_SOURCE[0]}")/defaults.conf"

# =============================================================================
# 错误处理函数
# =============================================================================

# 简单的调试日志函数（避免依赖utils.sh）
log_debug() {
    # 只在debug模式下输出，默认不输出
    [[ "${DEBUG:-}" == "1" ]] && echo "[DEBUG] $*" >&2 || true
}

# 统一的错误退出函数
error_exit() {
    local message="$1"
    local exit_code="${2:-1}"
    local line_number="${3:-${LINENO}}"
    local function_name="${4:-${FUNCNAME[1]}}"
    
    log_error "[$function_name:$line_number] $message"
    
    # 执行清理操作
    cleanup_on_error
    
    exit "$exit_code"
}

# 错误时的清理函数
cleanup_on_error() {
    log_info "执行错误清理操作..."
    
    # 清理临时文件
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf "$TEMP_DIR"
        log_debug "清理临时目录: $TEMP_DIR"
    fi
    
    # 停止可能启动的HTTP服务
    local http_pid
    http_pid=$(lsof -t -i:"${DOWNLOAD_PORT:-$DEFAULT_DOWNLOAD_PORT}" 2>/dev/null || echo "")
    if [[ -n "$http_pid" ]]; then
        kill -TERM "$http_pid" 2>/dev/null || true
        log_debug "停止HTTP服务进程: $http_pid"
    fi
    
    # 清理过期的配置文件
    find /root -name "singbox_*.yaml" -mmin +$((TEMP_FILE_TTL/60)) -delete 2>/dev/null || true
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
            log_error "$message (退出码: $result)"
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
    local max_attempts="${1:-$MAX_RETRIES}"
    local delay="${2:-$RETRY_DELAY}"
    shift 2
    local command=("$@")
    
    local attempt=1
    local result
    
    while [[ $attempt -le $max_attempts ]]; do
        log_debug "执行命令 (尝试 $attempt/$max_attempts): ${command[*]}"
        
        if "${command[@]}"; then
            return 0
        fi
        
        result=$?
        
        if [[ $attempt -lt $max_attempts ]]; then
            log_warn "命令执行失败 (尝试 $attempt/$max_attempts)，${delay}秒后重试..."
            sleep "$delay"
        else
            log_error "命令执行失败，已达最大重试次数 ($max_attempts)"
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
    local timeout="${2:-${NETWORK_TIMEOUT:-10}}"
    local max_retries="${3:-${MAX_RETRIES:-3}}"
    local output_file="${4:-}"
    
    # 基本curl参数
    local curl_args=(
        --max-time "$timeout"
        --connect-timeout "$timeout"
        --retry 0  # 我们自己处理重试
        --fail
        --silent
        --show-error
        --location
    )
    
    # 添加User-Agent
    curl_args+=(--user-agent "Sing-Box-Installer/2.0")
    
    # 如果指定了输出文件
    if [[ -n "$output_file" ]]; then
        curl_args+=(--output "$output_file")
    fi
    
    curl_args+=("$url")
    
    # 使用重试机制
    retry_command "$max_retries" "$RETRY_DELAY" curl "${curl_args[@]}"
}

# 下载文件函数
download_file() {
    local url="$1"
    local output_file="$2"
    local timeout="${3:-$DOWNLOAD_TIMEOUT}"
    local max_retries="${4:-$MAX_RETRIES}"
    
    log_info "下载文件: $url -> $output_file"
    
    # 创建输出目录
    mkdir -p "$(dirname "$output_file")"
    
    # 检查现有文件
    if [[ -f "$output_file" ]]; then
        log_debug "文件已存在，检查大小..."
        local file_size
        file_size=$(stat -f%z "$output_file" 2>/dev/null || stat -c%s "$output_file" 2>/dev/null || echo "0")
        if [[ "$file_size" -gt 1000 ]]; then
            log_info "使用现有文件: $output_file (大小: $file_size 字节)"
            return 0
        else
            log_warn "现有文件过小，重新下载"
            rm -f "$output_file"
        fi
    fi
    
    # 下载文件
    if safe_curl "$url" "$timeout" "$max_retries" "$output_file"; then
        # 验证下载的文件
        local downloaded_size
        downloaded_size=$(stat -f%z "$output_file" 2>/dev/null || stat -c%s "$output_file" 2>/dev/null || echo "0")
        
        if [[ "$downloaded_size" -lt 1000 ]]; then
            log_error "下载的文件过小 (${downloaded_size} 字节)，可能下载失败"
            rm -f "$output_file"
            return 1
        fi
        
        log_info "下载完成: $output_file (大小: $downloaded_size 字节)"
        return 0
    else
        log_error "下载失败: $url"
        rm -f "$output_file" 2>/dev/null || true
        return 1
    fi
}

# 多源下载函数
multi_source_download() {
    local filename="$1"
    local output_file="$2"
    shift 2
    local urls=("$@")
    
    log_info "尝试从多个源下载: $filename"
    
    for i in "${!urls[@]}"; do
        local url="${urls[$i]}"
        local source_name="源$((i+1))"
        
        log_info "尝试 $source_name: $url"
        
        if download_file "$url" "$output_file"; then
            log_info "$source_name 下载成功"
            return 0
        else
            log_warn "$source_name 下载失败"
        fi
    done
    
    log_error "所有下载源均失败: $filename"
    return 1
}

# =============================================================================
# 网络连接检测
# =============================================================================

# 检测网络连通性
check_network_connectivity() {
    log_info "检测网络连通性..."
    
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
    log_debug "测试IPv4连接..."
    for url in "${ipv4_urls[@]}"; do
        if ping -c 1 -W 5 "$url" >/dev/null 2>&1; then
            log_info "网络连接正常 (IPv4测试地址: $url)"
            return 0
        fi
    done
    
    # 如果IPv4失败，测试IPv6连接
    log_debug "IPv4连接失败，尝试IPv6连接..."
    for url in "${ipv6_urls[@]}"; do
        if ping6 -c 1 -W 5 "$url" >/dev/null 2>&1; then
            log_info "网络连接正常 (IPv6测试地址: $url)"
            return 0
        fi
    done
    
    log_error "网络连接检查失败 (IPv4和IPv6都不可达)"
    return 1
}

# 检测DNS解析
check_dns_resolution() {
    local domain="${1:-google.com}"
    
    log_debug "检测DNS解析: $domain"
    
    if nslookup "$domain" >/dev/null 2>&1 || dig "$domain" >/dev/null 2>&1; then
        log_debug "DNS解析正常"
        return 0
    else
        log_error "DNS解析失败"
        return 1
    fi
}

# =============================================================================
# 依赖检查函数
# =============================================================================

# 检查必需命令是否存在
check_required_commands() {
    local missing_commands=()
    local required_commands=(
        "curl"
        "systemctl"
        "openssl"
        "tar"
        "grep"
        "awk"
        "sed"
    )
    
    log_info "检查必需命令..."
    
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing_commands+=("$cmd")
            log_warn "缺少命令: $cmd"
        fi
    done
    
    if [[ ${#missing_commands[@]} -gt 0 ]]; then
        log_error "缺少必需命令: ${missing_commands[*]}"
        log_error "请安装缺少的命令后重新运行脚本"
        return 1
    fi
    
    log_info "所有必需命令已安装"
    return 0
}

# 检查可选命令并提供替代方案
check_optional_commands() {
    local optional_commands=(
        "jq:JSON处理"
        "qrencode:二维码生成"
        "lsof:端口检查"
        "shuf:随机数生成"
    )
    
    log_info "检查可选命令..."
    
    for cmd_desc in "${optional_commands[@]}"; do
        local cmd="${cmd_desc%%:*}"
        local desc="${cmd_desc#*:}"
        
        if command -v "$cmd" >/dev/null 2>&1; then
            log_debug "可选命令可用: $cmd ($desc)"
        else
            log_debug "可选命令不可用: $cmd ($desc) - 将使用替代方案"
        fi
    done
}

# =============================================================================
# 初始化函数
# =============================================================================

# 初始化错误处理模块
init_error_handler() {
    # 设置错误陷阱 (不使用 -u 选项避免未绑定变量问题)
    set -eo pipefail
    
    # 设置EXIT陷阱
    trap cleanup_on_exit EXIT
    
    # 设置错误陷阱
    trap 'error_exit "脚本在第 $LINENO 行出错" $? $LINENO "${FUNCNAME[0]}"' ERR
    
    echo "[INFO] 错误处理模块已初始化"
}

# 清理退出函数
cleanup_on_exit() {
    local exit_code=$?
    
    if [[ $exit_code -eq 0 ]]; then
        log_debug "脚本正常退出"
    else
        log_warn "脚本异常退出 (退出码: $exit_code)"
        cleanup_on_error
    fi
}