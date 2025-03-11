#!/bin/bash
# 定义颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color (重置颜色)
# 下载服务端口默认为8080
PORT=14567
#ss端口
ssPort=443
#vless端口
vlessPort=10243
#hysteria端口
hysteriaPort=10244
#强密码
hysteriaPassword=xyz0123456789!A
#uuid
uuid=0
#文件名
RANDOM_STR=$(cat /dev/urandom | tr -dc 'a-zA-Z' | fold -w 6 | head -n 1)

get_available_port() {
    local start_range=$1  # 起始端口范围
    local end_range=$2    # 结束端口范围提交
    
    # 在指定范围内随机选择可用端口
    while true; do
        PORTs=$(shuf -i "$start_range-$end_range" -n 1)
        if ! lsof -i:"$PORTs" >/dev/null 2>&1; then
            echo "$PORTs"
            return
        fi
    done
}

generate_strong_password() {
  local password
  local char_set="ABCDEF123456GHIJKLMN!+&*OPQRS12345TU!+&*VWX456YZabcd!+&*efghijk789lmnopq!+&*rstuvwxyz0123456789!+&*"
  local password_length=15

  # 使用/dev/urandom直接生成指定长度的随机密码
  password=$(head -c $password_length /dev/urandom | base64 | tr -dc "$char_set" | head -c $password_length)

  echo "$password"
}



# 更新系统软件安装Sing-Box
install_singbox() {
  echo "更新升级系统软件包..."
  sudo apt update && sudo apt upgrade -y
  echo "修改系统时区为上海..."
  sudo timedatectl set-timezone Asia/Shanghai
  echo "正在安装 Sing-Box..."
  bash <(curl -fsSL https://sing-box.app/deb-install.sh)
  
  install_self_signed_cert
}

# 安装 warp
install_warp() {
  #检查dns解析服务是否启动
  check_and_start_systemd_resolved
  echo "正在安装 warp，端口一定要选默认40000，选13"
  bash <(curl -fsSL https://gitlab.com/fscarmen/warp_unlock/-/raw/main/unlock.sh)
}

#安装自签证书
install_self_signed_cert() {
  echo "正在安装bing自签证书..."
  local cert_dir="/etc/sing-box/cert"
  local private_key="$cert_dir/private.key"
  local cert_file="$cert_dir/cert.pem"
  local cn="bing.com"
  mkdir -p "$cert_dir"
  openssl ecparam -genkey -name prime256v1 -out "$private_key"
  openssl req -new -x509 -days 36500 -key "$private_key" -out "$cert_file" -subj "/CN=$cn"
}

# 安装 fail2ban
install_fail2ban() {
  echo "正在安装 fail2ban..."
  sudo apt-get update
  sudo apt-get install -y fail2ban
  echo "fail2ban 安装完成。"
}

# 添加一个配置 fail2ban 的函数:
configure_fail2ban() {
  echo "正在配置 fail2ban..."
  sudo tee /etc/fail2ban/jail.local >/dev/null <<EOF
[DEFAULT]
ignoreip = 127.0.0.1/8
bantime = -1
findtime = 86400
maxretry = 10

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
EOF
  echo "fail2ban 配置完成。"
}

#创建服务
start_fail2ban() {
  echo "正在启动 fail2ban 服务..."
  sudo systemctl enable fail2ban
  sudo systemctl start fail2ban
  echo "fail2ban 服务已启动。"
}

change_ssh_port() {

  # 提示用户是否安装 fail2ban
  read -p "是否修改端口? (y/n) [n]: " INSTALL_FAIL2BAN
  INSTALL_FAIL2BAN=${INSTALL_FAIL2BAN:-n}  # 默认选择是不修改端口
  if [[ "$INSTALL_FAIL2BAN" =~ ^[Yy]$ ]]; then
    echo "正在修改 SSH 端口..."
    
    # 先安装 SSH 服务器
    apt install -y openssh-server
    
    # 获取一个固定端口 
    NEW_SSH_PORT=40001
    
    # 备份原配置文件
    cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak
    
    # 修改端口配置
    sed -i "s/#Port 22/Port $NEW_SSH_PORT/" /etc/ssh/sshd_config
    
    # 如果没有Port行,则添加
    if ! grep -q "^Port" /etc/ssh/sshd_config; then
        echo "Port $NEW_SSH_PORT" >> /etc/ssh/sshd_config
    fi
    
    # 重启 SSH 服务 - 根据不同系统使用对应服务名
    if systemctl list-units --type=service | grep -q ssh.service; then
        systemctl restart ssh
    elif systemctl list-units --type=service | grep -q sshd.service; then
        systemctl restart sshd
    fi
    
    # 如果使用 UFW 防火墙,需要添加新端口规则
    if command -v ufw >/dev/null 2>&1; then
        ufw allow $NEW_SSH_PORT/tcp
    fi
    
    echo "SSH 端口已修改为: $NEW_SSH_PORT"
    echo "请使用新端口进行 SSH 连接"
  else
    echo "跳过 SSH 端口修改。"
  fi
}



#执行faild2ban
excute_fail2ban() {

    install_fail2ban
    configure_fail2ban
    start_fail2ban

}



#
check_and_start_systemd_resolved() {
    echo "正在检查 systemd-resolved 服务状态..."
    if ! systemctl is-active --quiet systemd-resolved; then
        echo "systemd-resolved 服务未运行,正在启动..."
        systemctl start systemd-resolved
        echo "systemd-resolved 服务已启动"
    else
        echo "systemd-resolved 服务已经在运行"
    fi
}

# 检查并启动长期运行的 HTTP 服务
start_http_server() {
  # HTTP 服务监听的目录
  HTTP_DIR="/root"


  # 检查端口是否被占用
  PID=$(lsof -t -i :$PORT)
  if [[ -z "$PID" ]]; then
    echo "正在启动长期运行的 HTTP 服务..."
    cd "$HTTP_DIR"
    nohup python3 -m http.server $PORT --bind 0.0.0.0 > /dev/null 2>&1 &
    echo "HTTP 服务已启动，监听端口 $PORT"
  else
    echo "HTTP 服务已运行，监听端口 $PORT (进程 $PID)"
  fi
}

# 清理任务

cleanup_task() {
  

  # 定义清除任务
  sleep 600  # 等待 30 分钟 (30 * 60 秒)

  # 删除 YAML 文件
  echo "清理文件：/root/singbox_*.yaml"
  rm -f /root/singbox_*.yaml

  # 关闭服务端口
  echo "关闭 HTTP 服务 (端口 $PORT)..."
  PID=$(lsof -t -i :$PORT)
  if [[ -n "$PID" ]]; then
    kill -9 $PID
    echo "端口 $PORT 服务已关闭。"
  else
    echo "端口 $PORT 没有运行服务，无需关闭。"
  fi
}

enable_bbr() {
    
    
    # 加载必要模块
    modprobe tcp_bbr
    
    # 设置 BBR
    echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
    sysctl -p

    # 验证 BBR 状态
    if [[ $(sysctl net.ipv4.tcp_congestion_control | grep bbr) && $(lsmod | grep bbr) ]]; then
        echo "BBR 已成功启用"
    else 
        echo "BBR 启用失败，请检查系统配置"
    fi
}

optimize_network() {
    cat >> /etc/sysctl.conf <<EOF
net.ipv4.tcp_fastopen=3
net.ipv4.tcp_slow_start_after_idle=0
net.ipv4.tcp_notsent_lowat=16384
EOF
    sysctl -p
}



checkDomin() {
    # 获取当前 IP 地址所在国家代码
    COUNTRY_CODE=$(curl -s https://ipapi.co/country/)
    echo "当前 IP 地址所在国家代码: $COUNTRY_CODE"

    case $COUNTRY_CODE in
      JP)
        SERVER="www.tms-e.co.jp"
        ;;
      US)
        SERVER="www.thewaltdisneycompany.com"
        ;;
      NL)
        SERVER="www.technologieradar.nl"
        ;;
      DE)
        SERVER="www.mediamarkt.de"
        ;;
      HK)
        SERVER="www.hkbu.edu.hk"
        ;;
      *)
        SERVER="www.apple.com"
        ;;
    esac

    # 询问用户是否使用匹配到的域名
    read -p "根据您的地区，建议使用的域名是 $SERVER，是否使用该域名？(Y/n) " USE_SUGGESTED_SERVER
    if [[ "$USE_SUGGESTED_SERVER" =~ ^[Nn]$ ]]; then
      read -p "是否想自己输入域名？(Y/n) " INPUT_SERVER
      if [[ "$INPUT_SERVER" =~ ^[Yy]$ ]]; then
        read -p "请输入您要使用的域名: " SERVER
      else
        SERVER="www.apple.com"
      fi
    fi
}


# 提供下载链接
provide_download_link() {
  SERVER_IP=$(curl -4 -s ifconfig.me)
  echo "文件已生成并可通过以下链接下载："
  # 修改下载链接显示
  echo -e "\033[31m==================Sinbox链接：==========================\033[0m"
  echo ""
  echo ""
  echo "http://$SERVER_IP:$PORT/singbox_${RANDOM_STR}.yaml"
}

# 配置服务端 config.json
configure_singbox() {
  CONFIG_PATH="/etc/sing-box/config.json"
  uuid=$(sing-box generate uuid)
  #获取域名
  checkDomin
  # 询问是否启用 ChatGPT 分流
  # read -p "是否启用 ChatGPT 分流? (y/n) [n]: " ENABLE_CHATGPT
  ENABLE_CHATGPT="n"  # 如果用户直接回车，默认值为 n

  # 准备路由规则
  if [[ "${ENABLE_CHATGPT}" == "y" ]]; then
    install_warp
    CHATGPT_RULES='{
        "rule_set": ["geosite-chatgpt"],
        "outbound": "socks-netflix"
      },
      {
        "domain": ["nodeseek.com"],
        "outbound": "socks-netflix"
      },
       {
        "geosite": "cn",
        "geoip": "cn",
        "outbound": "direct"
      },
      {
        "geosite": "category-ads-all",
        "outbound": "block"
      }'
    CHATGPT_OUTBOUND=',{
      "type": "socks",
      "tag": "socks-netflix",
      "server": "127.0.0.1",
      "server_port": 40000
    }'
  else
    CHATGPT_RULES='{
        "geosite": "cn",
        "geoip": "cn",
        "outbound": "direct"
      },
      {
        "geosite": "category-ads-all",
        "outbound": "block"
      }'
    CHATGPT_OUTBOUND=''
  fi

  # 生成完整 config.json 文件
  cat > $CONFIG_PATH <<EOF
{
  "log": {
    "disabled": false,
    "level": "info",
    "timestamp": true
  },
  "dns": {
    "servers": [
      {
        "tag": "local",
        "address": "https://1.1.1.1/dns-query",
        "detour": "direct"
      },
      {
        "tag": "block",
        "address": "rcode://success"
      }
    ],
    "rules": [
      {
        "rule_set": ["cn"],
        "server": "local"
      },
      {
        "rule_set": ["category-ads-all"],
        "server": "block",
        "disable_cache": true
      }
    ]
  },
  "inbounds": [
    {
      "type": "shadowtls",
      "tag": "st-in",
      "listen": "::",
      "listen_port": $ssPort,
      "version": 3,
      "users": [
        {
          "name": "username",
          "password": "AaaY/lgWSBlSQtDmd0UpFnqR1JJ9JTHn0CLBv12KO5o="
        }
      ],
      "handshake": {
        "server": "$SERVER",
        "server_port": 443
      },
      "strict_mode": true,
      "detour": "ss-in"
    },
    {
      "type": "shadowsocks",
      "tag": "ss-in",
      "listen": "127.0.0.1",
      "network": "tcp",
      "method": "2022-blake3-chacha20-poly1305",
      "password": "$ssPassword"
    },
    {
  "type": "tuic",
  "tag": "tuic-in",
  "listen": "::",
  "listen_port": 61555,
  "users": [
    {
      "uuid": "$uuid"
    }
  ],
  "congestion_control": "bbr",
  "tls": {
    "enabled": true,
    "server_name": "bing.com",
    "alpn": ["h3"],
    "certificate_path": "/etc/sing-box/cert/cert.pem",
    "key_path": "/etc/sing-box/cert/private.key"
      }
    },
    {
      "type": "vless",
      "tag": "vless-in",
      "listen": "::",
      "listen_port": $vlessPort,
      "users": [
        {
          "uuid": "$uuid",
          "flow": "xtls-rprx-vision"
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "$SERVER",
        "reality": {
          "enabled": true,
          "handshake": {
            "server": "$SERVER",
            "server_port": 443
          },
          "private_key": "QNJo_UznAk69XQeWNKtY-RdsfzJE-s5uAFso5tARWkA",
          "short_id": [
            "0123456789abcded"
          ]
        }
      }
    },
    {
      "type": "trojan",
      "tag": "trojan-in",
      "listen": "::",
      "listen_port": 63333,
      "users": [
        {
          "password": "$hysteriaPassword"
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "bing.com",
        "certificate_path": "/etc/sing-box/cert/cert.pem",
        "key_path": "/etc/sing-box/cert/private.key"
      },
      "transport": {
        "type": "ws",
        "path": "/trojan"
      }
    },
    {
      "sniff": true,
      "sniff_override_destination": true,
      "type": "hysteria2",
      "tag": "hy2-in",
      "listen": "::",
      "listen_port": $hysteriaPort,
      "users": [
          {
              "password": "$hysteriaPassword"
          }
      ],
      "tls": {
          "enabled": true,
          "alpn": [
              "h3"
          ],
          "certificate_path": "/etc/sing-box/cert/cert.pem",
          "key_path": "/etc/sing-box/cert/private.key"
      }
    }
  ],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    },
    {
      "type": "block",
      "tag": "block"
    }${CHATGPT_OUTBOUND}
  ],
  "route": {
    "rules": [
      {
        "protocol": ["http", "https"],
        "domain": ["chat.openai.com"],
        "outbound": "warp"
      },
      {
        "rule_set": ["private", "cn"],
        "outbound": "direct"
      },
      {
        "rule_set": ["category-ads-all"],
        "outbound": "block"
      }
    ],
    "rule_set": [
      {
        "tag": "cn",
        "type": "remote",
        "format": "binary",
        "url": "https://cdn.jsdelivr.net/gh/SagerNet/sing-geoip@rule-set/geoip-cn.srs",
        "download_detour": "direct"
      },
      {
        "tag": "private",
        "type": "remote",
        "format": "binary",
        "url": "https://cdn.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set/geosite-private.srs",
        "download_detour": "direct"
      },
      {
        "tag": "category-ads-all",
        "type": "remote",
        "format": "binary",
        "url": "https://cdn.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set/geosite-category-ads.srs",
        "download_detour": "direct"
      }
    ]
  }
}
EOF
  echo "服务端配置文件已保存到 $CONFIG_PATH"
}

# 启用并启动 Sing-Box 服务
enable_and_start_service() {
  echo "启用并启动 Sing-Box 服务..."
  sudo systemctl enable sing-box
  sudo systemctl start sing-box
  sudo systemctl restart sing-box
  echo "Sing-Box 服务已启用并启动。"
}



generate_v2ray_link() {
  # 使用之前已经获取的值
  V2RAY_UUID=$uuid
  V2RAY_IP="$SERVER_IP"
  V2RAY_HOST="$SERVER"
  V2RAY_PBK="Y_-yCHC3Qi-Kz6OWpueQckAJSQuGEKffwWp8MlFgwTs"
  V2RAY_SID="0123456789abcded"
  V2RAY_PORT="$vlessPort"

  # 生成 V2Ray 链接
  V2RAY_LINK="vless://${V2RAY_UUID}@${V2RAY_IP}:${V2RAY_PORT}?security=reality&flow=xtls-rprx-vision&type=tcp&sni=${V2RAY_HOST}&fp=chrome&pbk=${V2RAY_PBK}&sid=${V2RAY_SID}&encryption=none&headerType=none#reality"
  echo ""
  echo ""
  echo -e "\033[31m==================V2Ray 链接：==========================\033[0m"
  echo ""
  echo ""
  echo "$V2RAY_LINK"
  echo ""
  echo ""
}

generate_hy2_link() {
  # 使用之前已经获取的值
  hy_domain="bing.com"
  hy_password="$hysteriaPassword"
  hy_PORT="$hysteriaPort"
  hy_IP="$SERVER_IP"

  # 生成 by 链接
  hy_LINK="hysteria2://${hy_password}@${hy_IP}:${hy_PORT}?insecure=1&alpn=h3&sni=${hy_domain}#Hysteria2"
  echo ""
  echo ""
  echo -e "\033[31m==================hy2 链接：==========================\033[0m"
  echo ""
  echo ""
  echo "$hy_LINK"
  echo ""
  echo ""
}

generate_trojan_link() {
  # 使用之前已经获取的值
  trojan_domain="bing.com"
  trojan_password="$hysteriaPassword"
  trojan_PORT=63333
  trojan_IP="$SERVER_IP"

  # 生成 by 链接
  trojan_LINK="trojan://${trojan_password}@${trojan_IP}:${trojan_PORT}?sni=bing.com&type=ws&path=%2Ftrojan&host=bing.com&allowInsecure=1&udp=true&alpn=http%2F1.1"
  echo ""
  echo ""
  echo -e "\033[31m==================trojan 链接：==========================\033[0m"
  echo ""
  echo ""
  echo "$trojan_LINK"
  echo ""
  echo ""
}

generate_tuic_link() {
  # 使用之前已经获取的值
  tuic_UUID=$uuid
  tuic_IP="$SERVER_IP"
  tuic_HOST="$SERVER"

  # 生成 V2Ray 链接
  tuic_LINK="tuic://${tuic_UUID}:@${tuic_IP}:61555?alpn=h3&allow_insecure=1&congestion_control=bbr#tuic"
  echo ""
  echo ""
  echo -e "\033[31m==================tuic 链接：==========================\033[0m"
  echo ""
  echo ""
  echo "$tuic_LINK"
  echo ""
  echo ""
 
}

generate_ss2022_link() {
  # 使用之前已经获取的值
  ss2022_UUID=$uuid
  ss2022_IP="$SERVER_IP"
  ss2022_HOST="$SERVER"

  # 生成 V2Ray 链接
  convert_to_sslink
  ss2022_LINK=$(cat /tmp/ss_url.txt)
  echo ""
  echo ""
  echo -e "\033[31m==================ss2022 链接：==========================\033[0m"
  echo ""
  echo ""
  echo "$ss2022_LINK"
  echo ""
  echo ""
  echo -e "\033[31m========================================================\033[0m"
}

generate_base64() {
  local length="$1"
  
  # 默认长度为 32 字节 (会生成约 44 字符的 Base64 字符串)
  if [ -z "$length" ]; then
    length=32
  fi
  
  # 方法 1: 使用 /dev/urandom 和 base64
  openssl rand -base64 "$length"
  
  # 方法 2: 如果没有 openssl，可以使用这个替代方案
  # head -c "$length" /dev/urandom | base64
}

convert_to_sslink(){
  SERVER=$SERVER_IP
  PORT=$ssPort
  CIPHER="2022-blake3-chacha20-poly1305"
  PASSWORD=$ssPassword
  PLUGIN_HOST=$ss2022_HOST
  PLUGIN_PASSWORD="AaaY/lgWSBlSQtDmd0UpFnqR1JJ9JTHn0CLBv12KO5o="
  PLUGIN_VERSION="3"
  NAME="ShadowTLS-v3"

  # 创建用户信息部分并Base64编码
  USER_INFO="${CIPHER}:${PASSWORD}"
  USER_INFO_BASE64=$(echo -n "$USER_INFO" | base64)

  # 创建shadow-tls JSON并Base64编码
  SHADOW_TLS_JSON="{\"address\":\"$SERVER\",\"password\":\"$PLUGIN_PASSWORD\",\"version\":\"$PLUGIN_VERSION\",\"host\":\"$PLUGIN_HOST\",\"port\":\"$PORT\"}"
  SHADOW_TLS_BASE64=$(echo -n "$SHADOW_TLS_JSON" | base64)

  # 构建完整的SS URL
  URL="ss://${USER_INFO_BASE64}@${SERVER}:${PORT}?shadow-tls=${SHADOW_TLS_BASE64}#$(echo -n "$NAME" | sed 's/ /%20/g')"

  echo "$URL" > /tmp/ss_url.txt
}

cleanup_port() {
  PORT=8080
  PID=$(sudo lsof -t -i :$PORT) # 获取占用端口的进程 ID
  if [[ -n "$PID" ]]; then
    echo "端口 $PORT 已被进程 $PID 占用，正在终止..."
    sudo kill -9 $PID
    echo "端口 $PORT 已释放。"
  fi
}

generate_qr_code() {
  #echo "正在生成二维码..."

  # 检查是否安装 qrencode
  if ! command -v qrencode >/dev/null 2>&1; then
    echo "未安装 qrencode，正在安装..."
    sudo apt-get update && sudo apt-get install -y qrencode
  fi

  # 在终端显示二维码
  echo "reality二维码已生成，请扫描以下二维码："
  qrencode -t ANSIUTF8 "$V2RAY_LINK"

  echo -e "\033[31m============================================\033[0m"
  echo ""
  echo ""

  echo "hy二维码已生成，请扫描以下二维码："
  qrencode -t ANSIUTF8 "$hy_LINK"

  echo -e "\033[31m============================================\033[0m"
  echo ""
  echo ""

  echo "trojan二维码已生成，请扫描以下二维码："
  qrencode -t ANSIUTF8 "$trojan_LINK"

  echo -e "\033[31m============================================\033[0m"
  echo ""
  echo ""

  echo "tuic二维码已生成，请扫描以下二维码："
  qrencode -t ANSIUTF8 "$tuic_LINK"

  echo -e "\033[31m============================================\033[0m"
  echo ""
  echo ""

  echo "ss2022二维码已生成，请扫描以下二维码："
  qrencode -t ANSIUTF8 "$ss2022_LINK"

  echo "二维码生成完成！"
}



# 生成客户端配置文件 singbox.yaml
generate_client_config() {
  CONFIG_PATH="/root/singbox_${RANDOM_STR}.yaml"

    # 获取当前机器的公网 IP
  SERVER_IP=$(curl -4 -s ifconfig.me || curl -4 -s ipinfo.io/ip)
  if [[ -z "$SERVER_IP" ]]; then
      echo "无法获取 IPv4 地址，尝试获取 IPv6 地址..."
      SERVER_IP=$(curl -6 -s ifconfig.me || curl -6 -s ipinfo.io/ip || curl -6 -s api64.ipify.org)
      if [[ -z "$SERVER_IP" ]]; then
          echo "无法获取服务器的公网 IPv6 地址，请检查网络连接。"
          exit 1
          #echo "无法获取服务器的公网 IP 地址，请检查网络连接。"
          #exit 1
      fi
  fi

  # 使用之前输入的 SERVER 值
  HOST="$SERVER"
  SERVERNAME="$SERVER"

  # 生成客户端配置文件
  cat > $CONFIG_PATH <<EOF
# 客户端配置文件
# echo "当前时间是：$(date '+%Y-%m-%d %H:%M:%S')"
# port: 7890 # HTTP(S) 代理服务器端口
# socks-port: 7891 # SOCKS5 代理端口
mixed-port: 10801 # HTTP(S) 和 SOCKS 代理混合端口
# redir-port: 7892 # 透明代理端口，用于 Linux 和 MacOS
# Transparent proxy server port for Linux (TProxy TCP and TProxy UDP)
# tproxy-port: 7893
allow-lan: true # 允许局域网连接
bind-address: "*" # 绑定 IP 地址，仅作用于 allow-lan 为 true，'*'表示所有地址
# find-process-mode has 3 values:always, strict, off
# - always, 开启，强制匹配所有进程
# - strict, 默认，由 clash 判断是否开启
# - off, 不匹配进程，推荐在路由器上使用此模式
find-process-mode: strict
mode: rule
#自定义 geodata url
geox-url:
  geoip: "https://cdn.jsdelivr.net/gh/Loyalsoldier/v2ray-rules-dat@release/geoip.dat"
  geosite: "https://cdn.jsdelivr.net/gh/Loyalsoldier/v2ray-rules-dat@release/geosite.dat"
  mmdb: "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb"
log-level: debug # 日志等级 silent/error/warning/info/debug
ipv6: true # 开启 IPv6 总开关，关闭阻断所有 IPv6 链接和屏蔽 DNS 请求 AAAA 记录
external-controller: 0.0.0.0:9093 # RESTful API 监听地址
secret: "123456" # RESTful API的密码 (可选)
# tcp-concurrent: true # TCP 并发连接所有 IP, 将使用最快握手的 TCP
#external-ui: /path/to/ui/folder # 配置 WEB UI 目录，使用 http://{{external-controller}}/ui 访问
# interface-name: en0 # 设置出口网卡
# 全局 TLS 指纹，优先低于 proxy 内的 client-fingerprint
# 可选： "chrome","firefox","safari","ios","random","none" options.
# Utls is currently support TLS transport in TCP/grpc/WS/HTTP for VLESS/Vmess and trojan.
global-client-fingerprint: chrome
# routing-mark:6666 # 配置 fwmark 仅用于 Linux
# 实验性选择
# experimental:
# 类似于 /etc/hosts, 仅支持配置单个 IP
# hosts:
  # '*.clash.dev': 127.0.0.1
  # '.dev': 127.0.0.1
  # 'alpha.clash.dev': '::1'
  # test.com: [1.1.1.1, 2.2.2.2]
  # clash.lan: clash # clash 为特别字段，将加入本地所有网卡的地址
  # baidu.com: google.com # 只允许配置一个别名
profile: # 存储 select 选择记录
  store-selected: true
  # 持久化 fake-ip
  store-fake-ip: true
# 嗅探域名
sniffer:
  enable: true
  sniffing:
    - tls
    - http
  # 强制对此域名进行嗅探
dns:
  enable: true #开启Clash内置DNS服务器，默认为false
  prefer-h3: true # 开启 DoH 支持 HTTP/3，将并发尝试
  listen: 0.0.0.0:53 # 开启 DNS 服务器监听
  ipv6: true # false 将返回 AAAA 的空结果
  # ipv6-timeout: 300 # 单位：ms，内部双栈并发时，向上游查询 AAAA 时，等待 AAAA 的时间，默认 100ms
  # 解析nameserver和fallback的DNS服务器
  # 填入纯IP的DNS服务器
  default-nameserver:
    - 114.114.114.114
    - 223.5.5.5
  enhanced-mode: fake-ip # 模式fake-ip
  fake-ip-range: 198.18.0.1/16 # fake-ip 池设置
  # use-hosts: true # 查询 hosts
  # 配置不使用fake-ip的域名
  fake-ip-filter:
    - "*.lan"
    - "*.localdomain"
    - "*.example"
    - "*.invalid"
    - "*.localhost"
    - "*.test"
    - "*.local"
    - "*.home.arpa"
    - time.*.com
    - time.*.gov
    - time.*.edu.cn
    - time.*.apple.com
    - time1.*.com
    - time2.*.com
    - time3.*.com
    - time4.*.com
    - time5.*.com
    - time6.*.com
    - time7.*.com
    - ntp.*.com
    - ntp1.*.com
    - ntp2.*.com
    - ntp3.*.com
    - ntp4.*.com
    - ntp5.*.com
    - ntp6.*.com
    - ntp7.*.com
    - "*.time.edu.cn"
    - "*.ntp.org.cn"
    - "+.pool.ntp.org"
    - music.163.com
    - "*.music.163.com"
    - "*.126.net"
    - musicapi.taihe.com
    - music.taihe.com
    - songsearch.kugou.com
    - trackercdn.kugou.com
    - "*.kuwo.cn"
    - api-jooxtt.sanook.com
    - api.joox.com
    - joox.com
    - y.qq.com
    - "*.y.qq.com"
    - streamoc.music.tc.qq.com
    - mobileoc.music.tc.qq.com
    - isure.stream.qqmusic.qq.com
    - dl.stream.qqmusic.qq.com
    - aqqmusic.tc.qq.com
    - amobile.music.tc.qq.com
    - "*.xiami.com"
    - "*.music.migu.cn"
    - music.migu.cn
    - "*.msftconnecttest.com"
    - "*.msftncsi.com"
    - msftconnecttest.com
    - msftncsi.com
    - localhost.ptlogin2.qq.com
    - localhost.sec.qq.com
    - "+.srv.nintendo.net"
    - "+.stun.playstation.net"
    - xbox.*.microsoft.com
    - xnotify.xboxlive.com
    - "+.battlenet.com.cn"
    - "+.wotgame.cn"
    - "+.wggames.cn"
    - "+.wowsgame.cn"
    - "+.jd.com"
    - "+.wargaming.net"
    - proxy.golang.org
    - stun.*.*
    - stun.*.*.*
    - "+.stun.*.*"
    - "+.stun.*.*.*"
    - "+.stun.*.*.*.*"
    - heartbeat.belkin.com
    - "*.linksys.com"
    - "*.linksyssmartwifi.com"
    - "*.router.asus.com"
    - mesu.apple.com
    - swscan.apple.com
    - swquery.apple.com
    - swdownload.apple.com
    - swcdn.apple.com
    - swdist.apple.com
    - lens.l.google.com
    - stun.l.google.com
    - "+.nflxvideo.net"
    - "*.square-enix.com"
    - "*.finalfantasyxiv.com"
    - "*.ffxiv.com"
    - '*.mcdn.bilivideo.cn'
  # DNS主要域名配置
  # 支持 UDP，TCP，DoT，DoH，DoQ
  # 这部分为主要 DNS 配置，影响所有直连，确保使用对大陆解析精准的 DNS
  nameserver:
    - 114.114.114.114 # default value
    - 223.5.5.5
    - 119.29.29.29
    - https://doh.360.cn/dns-query
    - https://doh.pub/dns-query # DNS over HTTPS
    - https://dns.alidns.com/dns-query # 强制 HTTP/3，与 perfer-h3 无关，强制开启 DoH 的 HTTP/3 支持，若不支持将无法使用
  # 当配置 fallback 时，会查询 nameserver 中返回的 IP 是否为 CN，非必要配置
  # 当不是 CN，则使用 fallback 中的 DNS 查询结果
  # 确保配置 fallback 时能够正常查询
  fallback:
    - 219.141.136.10
    - 8.8.8.8
    - 1.1.1.1
    - https://cloudflare-dns.com/dns-query
    - https://dns.google/dns-query
  # 配置 fallback 使用条件
  fallback-filter:
    geoip: false # 配置是否使用 geoip
    geoip-code: CN # 当 nameserver 域名的 IP 查询 geoip 库为 CN 时，不使用 fallback 中的 DNS 查询结果
  # 如果不匹配 ipcidr 则使用 nameservers 中的结果
    ipcidr:
      - 240.0.0.0/4
    domain:
      - "+.google.com"
      - "+.facebook.com"
      - "+.youtube.com"
      - "+.githubusercontent.com"
      - "+.googlevideo.com"
proxies:
- name: ShadowTLS-v3
  type: ss
  server: $SERVER_IP
  port: $ssPort
  cipher: 2022-blake3-chacha20-poly1305
  password: $ssPassword
  plugin: shadow-tls
  client-fingerprint: chrome
  plugin-opts:
    host: "$SERVER"
    password: "AaaY/lgWSBlSQtDmd0UpFnqR1JJ9JTHn0CLBv12KO5o="
    version: 3
- name: reality
  type: vless
  server: $SERVER_IP
  port: $vlessPort
  uuid: $uuid
  network: tcp
  udp: true
  tls: true
  flow: xtls-rprx-vision
  servername: $SERVER
  client-fingerprint: chrome
  reality-opts:
    public-key: Y_-yCHC3Qi-Kz6OWpueQckAJSQuGEKffwWp8MlFgwTs
    short-id: 0123456789abcded
- name: Hysteria2
  type: hysteria2
  server: $SERVER_IP
  port: $hysteriaPort
  #  up和down均不写或为0则使用BBR流控
  # up: "30 Mbps" # 若不写单位，默认为 Mbps
  # down: "200 Mbps" # 若不写单位，默认为 Mbps
  password: $hysteriaPassword
  sni: bing.com
  skip-cert-verify: true
  alpn:
    - h3
- name: Trojan
  type: trojan
  server: $SERVER_IP
  port: 63333
  password: $hysteriaPassword
  udp: true
  sni: bing.com
  alpn:
    - http/1.1
  skip-cert-verify: true
  network: ws
  ws-opts:
    path: /trojan
    headers:
      Host: bing.com
- name: Tuic
  type: tuic
  server: 84.54.3.161
  port: 61555
  uuid: $uuid
  #password: $hysteriaPassword
  congestion-controller: bbr
  udp: true
  sni: bing.com
  alpn:
    - h3
  reduce-rtt: true
  skip-cert-verify: true # 如果使用自签证书，请改为 true
proxy-groups:
- name: PROXY
  type: select
  proxies:
    - reality
    - Hysteria2
    - Trojan
    - Tuic
rule-providers:
  reject:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/reject.txt"
    path: ./ruleset/reject.yaml
    interval: 86400
  icloud:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/icloud.txt"
    path: ./ruleset/icloud.yaml
    interval: 86400
  apple:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/apple.txt"
    path: ./ruleset/apple.yaml
    interval: 86400
  proxy:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/proxy.txt"
    path: ./ruleset/proxy.yaml
    interval: 86400
  direct:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/direct.txt"
    path: ./ruleset/direct.yaml
    interval: 86400
  private:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/private.txt"
    path: ./ruleset/private.yaml
    interval: 86400
  gfw:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/gfw.txt"
    path: ./ruleset/gfw.yaml
    interval: 86400
  greatfire:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/greatfire.txt"
    path: ./ruleset/greatfire.yaml
    interval: 86400
  tld-not-cn:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/tld-not-cn.txt"
    path: ./ruleset/tld-not-cn.yaml
    interval: 86400
  telegramcidr:
    type: http
    behavior: ipcidr
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/telegramcidr.txt"
    path: ./ruleset/telegramcidr.yaml
    interval: 86400
  cncidr:
    type: http
    behavior: ipcidr
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/cncidr.txt"
    path: ./ruleset/cncidr.yaml
    interval: 86400
  lancidr:
    type: http
    behavior: ipcidr
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/lancidr.txt"
    path: ./ruleset/lancidr.yaml
    interval: 86400
  applications:
    type: http
    behavior: classical
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/applications.txt"
    path: ./ruleset/applications.yaml
    interval: 86400
rules:
  - RULE-SET,applications,DIRECT
  - DOMAIN,clash.razord.top,DIRECT
  - DOMAIN,yacd.haishan.me,DIRECT
  - DOMAIN-SUFFIX,stream1.misakaf.org:443,PROXY
  - DOMAIN-SUFFIX,stream2.misakaf.org:443,PROXY
  - DOMAIN-SUFFIX,stream3.misakaf.org:443,PROXY
  - DOMAIN-SUFFIX,stream4.misakaf.org:443,PROXY
  - DOMAIN-SUFFIX,services.googleapis.cn,DIRECT
  - DOMAIN-SUFFIX,xn--ngstr-lra8j.com,DIRECT
  - RULE-SET,private,DIRECT
  - RULE-SET,reject,REJECT
  - RULE-SET,icloud,DIRECT
  - RULE-SET,apple,DIRECT
  - RULE-SET,proxy,PROXY
  - RULE-SET,direct,DIRECT
  - RULE-SET,lancidr,DIRECT
  - RULE-SET,cncidr,DIRECT
  - RULE-SET,telegramcidr,PROXY
  - GEOIP,LAN,DIRECT
  - GEOIP,CN,DIRECT
  - MATCH,PROXY
EOF
  echo "客户端配置文件已生成并保存到 $CONFIG_PATH"
}

# 主函数
main() {
  vlessPort=$(get_available_port 20000 30000)
  ssPort=$(get_available_port 31000 40000)
  hysteriaPort=$(get_available_port 50000 60000)
  hysteriaPassword=$(generate_strong_password)
  ssPassword=$(generate_base64 32)

 

  excute_fail2ban
  install_singbox
  configure_singbox
  
  generate_client_config
  start_http_server 
  provide_download_link
  generate_v2ray_link
  generate_hy2_link
  generate_trojan_link
  generate_tuic_link
  generate_ss2022_link
  generate_qr_code
  enable_and_start_service
  enable_bbr
  #优化网络
  optimize_network
  # 启动清除任务a
  cleanup_task &
  #修改ssh端口为40001
  change_ssh_port
  #serve_download
  echo -e "${YELLOW}所有配置完成，10分钟后清除所有对外配置文件！${NC}"
}

main