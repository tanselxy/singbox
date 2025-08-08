#!/bin/bash

echo "=== HTTP 服务绑定问题诊断 ==="

# 1. 检查当前运行的脚本版本
echo "1. 检查当前目录和文件:"
pwd
ls -la | grep -E "(install\.sh|utils\.sh|defaults\.conf)"

echo ""
echo "2. 检查 defaults.conf 内容:"
if [[ -f "defaults.conf" ]]; then
    grep -n "HTTP_BIND_IP" defaults.conf || echo "defaults.conf 中没有找到 HTTP_BIND_IP"
else
    echo "defaults.conf 文件不存在"
fi

echo ""
echo "3. 检查 utils.sh 中的 start_http_server 函数:"
if [[ -f "utils.sh" ]]; then
    grep -A 10 -B 2 "start_http_server()" utils.sh
    echo ""
    echo "HTTP_BIND_IP 使用情况:"
    grep -n "HTTP_BIND_IP" utils.sh || echo "utils.sh 中没有使用 HTTP_BIND_IP"
else
    echo "utils.sh 文件不存在"
fi

echo ""
echo "4. 检查环境变量和配置:"
echo "HTTP_BIND_IP=${HTTP_BIND_IP:-未设置}"
echo "DOWNLOAD_PORT=${DOWNLOAD_PORT:-未设置}"

echo ""
echo "5. 检查当前HTTP服务进程:"
if lsof -i :14567 >/dev/null 2>&1; then
    echo "当前 HTTP 服务详情:"
    lsof -i :14567
    echo ""
    
    # 获取进程的完整命令行
    HTTP_PID=$(lsof -t -i :14567)
    echo "进程完整命令行:"
    cat /proc/$HTTP_PID/cmdline | tr '\0' ' ' 2>/dev/null || ps -fp $HTTP_PID
    echo ""
else
    echo "没有HTTP服务在14567端口运行"
fi

echo ""
echo "6. 模拟 start_http_server 函数调用:"
# 加载配置
if [[ -f "defaults.conf" ]]; then
    source defaults.conf
    echo "defaults.conf 已加载"
else
    echo "defaults.conf 不存在，使用默认值"
fi

# 模拟函数中的变量赋值
local_bind_ip="${HTTP_BIND_IP:-0.0.0.0}"
echo "模拟的 bind_ip 值: $local_bind_ip"

echo ""
echo "7. 测试 Python HTTP 服务器命令:"
echo "将要执行的命令: python3 -m http.server 14567 --bind $local_bind_ip"

echo ""
echo "=== 诊断完成 ==="