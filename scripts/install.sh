#!/usr/bin/env bash
# =============================================================================
# singbox-panel bootstrap installer
#
# 用法:
#   bash <(curl -sL singbox.soups.eu.org/get)
#
# 作用: 探测系统架构 -> 下载对应 singbox-panel 二进制 -> 安装到 /usr/local/bin
#       -> 运行 `singbox-panel install` 完成部署。
# 除下载器本身外，所有部署逻辑都在 Go 二进制里。
# =============================================================================
set -euo pipefail

REPO="tanselxy/singbox"
BIN_NAME="singbox-panel"
INSTALL_PATH="/usr/local/bin/${BIN_NAME}"
# 允许通过环境变量覆盖版本，默认取最新 release。
VERSION="${SINGBOX_PANEL_VERSION:-latest}"

err() { echo "错误: $*" >&2; exit 1; }
info() { echo ">>> $*"; }

[[ "$(id -u)" -eq 0 ]] || err "请以 root 运行（sudo bash ...）"

detect_arch() {
	local m
	m="$(uname -m)"
	case "$m" in
		x86_64|amd64)   echo "amd64" ;;
		aarch64|arm64)  echo "arm64" ;;
		*) err "不支持的架构: $m（目前提供 amd64 / arm64）" ;;
	esac
}

detect_os() {
	local os
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	[[ "$os" == "linux" ]] || err "仅支持 Linux，当前: $os"
	echo "$os"
}

resolve_url() {
	local os="$1" arch="$2"
	local asset="${BIN_NAME}_${os}_${arch}"
	if [[ "$VERSION" == "latest" ]]; then
		echo "https://github.com/${REPO}/releases/latest/download/${asset}"
	else
		echo "https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
	fi
}

main() {
	local os arch url tmp
	os="$(detect_os)"
	arch="$(detect_arch)"
	url="$(resolve_url "$os" "$arch")"

	info "下载 ${BIN_NAME} (${os}/${arch})"
	tmp="$(mktemp)"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$tmp" || err "下载失败: $url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$tmp" "$url" || err "下载失败: $url"
	else
		err "需要 curl 或 wget"
	fi

	[[ -s "$tmp" ]] || err "下载的文件为空"
	install -m 0755 "$tmp" "$INSTALL_PATH"
	rm -f "$tmp"
	info "已安装到 ${INSTALL_PATH}"

	info "开始部署"
	exec "$INSTALL_PATH" install "$@"
}

main "$@"
