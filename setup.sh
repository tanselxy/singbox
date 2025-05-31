#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 显示信息的函数
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    info "检查系统依赖..."
    
    if ! command -v wget &> /dev/null; then
        error "wget 未安装，正在安装..."
        if command -v apt &> /dev/null; then
            apt update && apt install -y wget
        elif command -v yum &> /dev/null; then
            yum install -y wget
        else
            error "无法自动安装wget，请手动安装"
            exit 1
        fi
    fi
}

# 下载文件函数
download_files() {
    info "创建安装目录..."
    mkdir -p "SingboxInstall"
    cd SingboxInstall
    
    info "开始下载文件..."
    
    # 文件列表
    files=(
        "install.sh"
        "client_template.yaml"
        "config.sh"
        "network.sh"
        "server_template.json"
        "utils.sh"
        "fix_centos.sh"
    )
    
    # 下载每个文件
    for file in "${files[@]}"; do
        info "下载 $file..."
        if wget -q --show-progress -O "$file" "https://raw.githubusercontent.com/tanselxy/singbox/main/$file"; then
            info "✓ $file 下载成功"
        else
            warning "✗ $file 下载失败，跳过..."
        fi
    done
    
    # 给sh文件添加执行权限
    info "设置执行权限..."
    chmod +x *.sh
    
    info "所有文件下载完成！"
    ls -la
}

# 执行安装
run_install() {
    info "开始执行安装..."
    if [ -f "install.sh" ]; then
        ./install.sh
    else
        error "install.sh 不存在，无法继续安装"
        exit 1
    fi
}

# 主函数
main() {

    check_dependencies
    download_files
    

    run_install
 
}

# 执行主函数
main "$@"