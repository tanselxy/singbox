#!/bin/bash

echo "=== 修复 HTTP 服务绑定问题 ==="

# 检查当前状态
echo "1. 检查当前HTTP服务状态:"
if lsof -i :14567 >/dev/null 2>&1; then
    echo "当前HTTP服务进程:"
    lsof -i :14567
    echo ""
    
    # 获取进程ID
    HTTP_PID=$(lsof -t -i :14567)
    echo "HTTP服务PID: $HTTP_PID"
    
    # 查看完整命令
    echo "完整命令行:"
    ps -fp "$HTTP_PID" 2>/dev/null || echo "无法获取命令行"
    echo ""
    
    # 停止当前服务
    echo "2. 停止当前HTTP服务..."
    kill "$HTTP_PID" 2>/dev/null && echo "✓ 已停止旧服务" || echo "✗ 停止失败"
    sleep 2
else
    echo "当前没有HTTP服务在14567端口运行"
fi

# 启动新的正确配置的服务
echo ""
echo "3. 启动新的HTTP服务 (绑定0.0.0.0)..."

# 切换到正确目录
cd /root || { echo "❌ 无法切换到/root目录"; exit 1; }

# 启动新服务
echo "启动命令: python3 -m http.server 14567 --bind 0.0.0.0"
nohup python3 -m http.server 14567 --bind 0.0.0.0 >/dev/null 2>&1 &
NEW_PID=$!

# 等待服务启动
sleep 3

# 验证新服务
echo ""
echo "4. 验证新服务状态:"
if lsof -i :14567 >/dev/null 2>&1; then
    echo "✓ HTTP服务已启动"
    lsof -i :14567
    echo ""
    
    # 检查绑定地址
    BIND_INFO=$(lsof -i :14567 | grep python3 | awk '{print $9}')
    if [[ "$BIND_INFO" == "*:14567" ]] || [[ "$BIND_INFO" =~ "0.0.0.0:14567" ]]; then
        echo "✅ 服务正确绑定到所有接口: $BIND_INFO"
    else
        echo "⚠️  服务可能仍绑定到本地: $BIND_INFO"
    fi
    
    # 保存新PID
    echo "$NEW_PID" > /tmp/singbox_http_fixed.pid
    echo "新服务PID已保存: $NEW_PID"
else
    echo "❌ 服务启动失败"
fi

echo ""
echo "=== 修复完成 ==="
echo "现在外部应该可以通过 http://服务器IP:14567/ 访问了"