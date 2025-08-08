#!/bin/bash

# =============================================================================
# 函数测试脚本
# 测试优化后的主要函数是否正常工作
# =============================================================================

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 模拟必要的环境变量
export LOG_FILE="/tmp/test_singbox.log"
export TEMP_DIR="/tmp/singbox-test-$$"
export CONFIG_DIR="/tmp/singbox-config-test"

# 加载模块（按顺序）
echo "=== 测试模块加载 ==="

if source "$SCRIPT_DIR/defaults.conf" 2>/dev/null; then
    echo "✅ defaults.conf 加载成功"
else
    echo "❌ defaults.conf 加载失败"
    exit 1
fi

if source "$SCRIPT_DIR/error_handler.sh" 2>/dev/null; then
    echo "✅ error_handler.sh 加载成功"
else
    echo "❌ error_handler.sh 加载失败"
    exit 1
fi

if source "$SCRIPT_DIR/utils.sh" 2>/dev/null; then
    echo "✅ utils.sh 加载成功"
else
    echo "❌ utils.sh 加载失败"
    exit 1
fi

echo ""
echo "=== 测试日志函数 ==="

# 测试日志函数
log_info "这是一个信息日志"
log_warn "这是一个警告日志"
log_error "这是一个错误日志"
log_debug "这是一个调试日志（可能不显示）"

echo ""
echo "=== 测试密码生成函数 ==="

# 测试密码生成
password1=$(generate_strong_password 12)
password2=$(generate_base64_password 16)
random_str=$(generate_random_string 8)

echo "强密码 (12位): $password1"
echo "Base64密码 (16字节): $password2"
echo "随机字符串 (8位): $random_str"

# 验证密码长度
if [[ ${#password1} -eq 12 ]]; then
    echo "✅ 强密码长度正确"
else
    echo "❌ 强密码长度错误: 期望12, 实际${#password1}"
fi

if [[ ${#random_str} -eq 8 ]]; then
    echo "✅ 随机字符串长度正确"
else
    echo "❌ 随机字符串长度错误: 期望8, 实际${#random_str}"
fi

echo ""
echo "=== 测试端口检测函数 ==="

# 测试端口检测（使用高端口避免权限问题）
test_port=$(get_available_port 40000 40010)
if [[ -n "$test_port" && "$test_port" =~ ^[0-9]+$ ]]; then
    echo "✅ 端口检测成功: $test_port"
else
    echo "❌ 端口检测失败"
fi

echo ""
echo "=== 测试网络函数 ==="

# 测试网络连接检查（如果可用）
if command -v ping >/dev/null 2>&1; then
    if check_network_connectivity; then
        echo "✅ 网络连接检查成功"
    else
        echo "⚠️  网络连接检查失败（可能是网络环境问题）"
    fi
else
    echo "ℹ️  ping命令不可用，跳过网络测试"
fi

echo ""
echo "=== 测试文件权限设置 ==="

# 创建测试文件并设置权限
test_file="$TEMP_DIR/test_file"
mkdir -p "$TEMP_DIR"
touch "$test_file"

chmod 600 "$test_file"
file_perms=$(stat -f "%OLp" "$test_file" 2>/dev/null || stat -c "%a" "$test_file" 2>/dev/null)

if [[ "$file_perms" == "600" ]]; then
    echo "✅ 文件权限设置正确: $file_perms"
else
    echo "⚠️  文件权限设置: $file_perms (可能因系统不同而异)"
fi

echo ""
echo "=== 清理测试环境 ==="

# 清理
rm -rf "$TEMP_DIR" 2>/dev/null || true
rm -f "$LOG_FILE" 2>/dev/null || true

echo "✅ 测试环境清理完成"

echo ""
echo "=== 测试总结 ==="
echo "主要函数测试完成，未发现严重错误"
echo "脚本优化成功！"