#!/bin/bash

echo "=== 快速修复HTTP服务启动问题 ==="

# 设置变量
DOWNLOAD_PORT=14567
HTTP_DIR="/root"

# 1. 停止所有可能占用端口的进程
echo "1. 清理端口占用..."
if lsof -i :$DOWNLOAD_PORT >/dev/null 2>&1; then
    echo "停止占用端口的进程..."
    kill $(lsof -t -i :$DOWNLOAD_PORT) 2>/dev/null || true
    sleep 3
    echo "端口清理完成"
else
    echo "端口 $DOWNLOAD_PORT 未被占用"
fi

# 2. 切换到正确目录
echo ""
echo "2. 切换到HTTP服务目录..."
cd "$HTTP_DIR" || { echo "❌ 无法切换到 $HTTP_DIR"; exit 1; }
echo "当前目录: $(pwd)"

# 3. 检查Python版本和http.server模块
echo ""
echo "3. 检查Python环境..."
python3 --version 2>/dev/null || echo "python3 不可用"
python --version 2>/dev/null || echo "python 不可用"

# 测试http.server模块
echo "测试python3 http.server模块..."
python3 -c "import http.server" 2>/dev/null && echo "✓ python3 http.server 可用" || echo "✗ python3 http.server 不可用"

# 4. 尝试多种启动方法
echo ""
echo "4. 尝试启动HTTP服务..."

# 方法1: python3 with --bind 0.0.0.0
echo "尝试方法1: python3 -m http.server $DOWNLOAD_PORT --bind 0.0.0.0"
nohup python3 -m http.server $DOWNLOAD_PORT --bind 0.0.0.0 >/tmp/http_server.log 2>&1 &
HTTP_PID=$!
sleep 3

if kill -0 $HTTP_PID 2>/dev/null && lsof -i :$DOWNLOAD_PORT >/dev/null 2>&1; then
    echo "✅ 方法1成功! PID: $HTTP_PID"
    BIND_INFO=$(lsof -i :$DOWNLOAD_PORT | grep python | awk '{print $9}')
    echo "绑定地址: $BIND_INFO"
else
    echo "❌ 方法1失败，尝试方法2..."
    kill $HTTP_PID 2>/dev/null || true
    
    # 方法2: python3 without --bind
    echo "尝试方法2: python3 -m http.server $DOWNLOAD_PORT"
    nohup python3 -m http.server $DOWNLOAD_PORT >/tmp/http_server.log 2>&1 &
    HTTP_PID=$!
    sleep 3
    
    if kill -0 $HTTP_PID 2>/dev/null && lsof -i :$DOWNLOAD_PORT >/dev/null 2>&1; then
        echo "✅ 方法2成功! PID: $HTTP_PID"
        BIND_INFO=$(lsof -i :$DOWNLOAD_PORT | grep python | awk '{print $9}')
        echo "绑定地址: $BIND_INFO"
    else
        echo "❌ 方法2失败，尝试方法3..."
        kill $HTTP_PID 2>/dev/null || true
        
        # 方法3: python (not python3)
        if command -v python >/dev/null 2>&1; then
            echo "尝试方法3: python -m http.server $DOWNLOAD_PORT"
            nohup python -m http.server $DOWNLOAD_PORT >/tmp/http_server.log 2>&1 &
            HTTP_PID=$!
            sleep 3
            
            if kill -0 $HTTP_PID 2>/dev/null && lsof -i :$DOWNLOAD_PORT >/dev/null 2>&1; then
                echo "✅ 方法3成功! PID: $HTTP_PID"
                BIND_INFO=$(lsof -i :$DOWNLOAD_PORT | grep python | awk '{print $9}')
                echo "绑定地址: $BIND_INFO"
            else
                echo "❌ 所有方法都失败了"
                kill $HTTP_PID 2>/dev/null || true
                HTTP_PID=""
            fi
        else
            echo "❌ python 命令不可用"
            HTTP_PID=""
        fi
    fi
fi

# 5. 最终验证
echo ""
echo "5. 最终验证..."
if [[ -n "$HTTP_PID" ]] && lsof -i :$DOWNLOAD_PORT >/dev/null 2>&1; then
    echo "🎉 HTTP服务启动成功!"
    echo "服务详情:"
    lsof -i :$DOWNLOAD_PORT
    echo ""
    echo "访问地址: http://$(curl -s ifconfig.me):$DOWNLOAD_PORT/"
    echo "日志文件: /tmp/http_server.log"
    
    # 保存PID
    echo $HTTP_PID > /tmp/singbox_http_fixed.pid
    echo "PID已保存到 /tmp/singbox_http_fixed.pid"
else
    echo "💥 HTTP服务启动失败"
    echo "错误日志:"
    cat /tmp/http_server.log 2>/dev/null || echo "无日志文件"
fi

echo ""
echo "=== 修复脚本完成 ==="