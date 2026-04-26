#!/usr/bin/env bash

set -u

readonly BIN_PATH="/usr/local/bin/stellarcore"
readonly CONFIG_DIR="/etc/stellarcore"
readonly CONFIG_PATH="${CONFIG_DIR}/frps.toml"
readonly SERVICE_PATH="/etc/systemd/system/stellarfrps.service"
readonly SERVICE_UNIT="stellarfrps.service"
readonly SERVICE_NAME="stellarfrps"
readonly GITHUB_REPO="65658dsf/StellarCore"
readonly GITHUB_API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
readonly BASE_API_URL="https://resources.stellarfrp.top/api/fs/list"
readonly BASE_DOWNLOAD_URL="https://resources.stellarfrp.top/d/StellarCore"
readonly ROOT_PATH="StellarCore"

TMP_ROOT=""
JQ_AVAILABLE=false
FRP_ARCH=""
PREPARED_BINARY=""
PORT_CHECK_ALLOW_CURRENT_CONFIG=false

log() {
    echo "$*"
}

warn() {
    echo "警告：$*" >&2
}

error() {
    echo "错误：$*" >&2
}

die() {
    error "$*"
    exit 1
}

cleanup() {
    if [ -n "${TMP_ROOT:-}" ] && [ -d "$TMP_ROOT" ]; then
        rm -rf "$TMP_ROOT"
    fi
}

trap cleanup EXIT INT TERM

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    die "请使用root权限运行此脚本"
fi

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/stellarcore.XXXXXX")" || die "无法创建临时目录"
mkdir -p "${TMP_ROOT}/downloads" "${TMP_ROOT}/extracts" || die "无法初始化临时目录"

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

generate_random_string() {
    local length="${1:-32}"
    tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c "$length"
    echo
}

ensure_jq() {
    if command_exists jq; then
        JQ_AVAILABLE=true
        return 0
    fi

    warn "未检测到 jq，正在尝试通过系统包管理器安装"

    if command_exists apt-get; then
        DEBIAN_FRONTEND=noninteractive apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y jq
    elif command_exists dnf; then
        dnf install -y jq
    elif command_exists yum; then
        yum install -y jq
    elif command_exists zypper; then
        zypper --non-interactive install jq
    elif command_exists apk; then
        apk add --no-cache jq
    elif command_exists pacman; then
        pacman -Sy --noconfirm jq
    else
        warn "未找到支持的包管理器，JSON 解析将使用 sed 兼容模式"
    fi

    if command_exists jq; then
        JQ_AVAILABLE=true
        log "jq 安装成功"
    else
        JQ_AVAILABLE=false
        warn "jq 不可用，已降级到 sed 兼容解析；若接口返回复杂 JSON，解析可能不如 jq 稳定"
    fi
}

file_size() {
    local file="$1"
    stat -c%s "$file" 2>/dev/null || wc -c < "$file" | tr -d ' '
}

download_with_retry() {
    local url="$1"
    local output="$2"
    local min_bytes="${3:-1024}"
    local max_retries=3
    local retry_count=0
    local size=0

    while [ "$retry_count" -lt "$max_retries" ]; do
        retry_count=$((retry_count + 1))
        rm -f "$output"
        log "下载尝试 ${retry_count}/${max_retries}: $url"

        if command_exists curl; then
            if curl -f -sS -L --connect-timeout 15 --max-time 300 --retry 2 --retry-delay 3 --retry-connrefused -o "$output" "$url"; then
                size="$(file_size "$output")"
                if [ -s "$output" ] && [ "$size" -ge "$min_bytes" ]; then
                    log "curl 下载成功，文件大小: ${size} 字节"
                    return 0
                fi
                warn "curl 下载的文件异常，文件大小: ${size} 字节"
                rm -f "$output"
            else
                warn "curl 下载失败"
                rm -f "$output"
            fi
        fi

        if command_exists wget; then
            if wget -q --timeout=30 --tries=3 --retry-connrefused -O "$output" "$url"; then
                size="$(file_size "$output")"
                if [ -s "$output" ] && [ "$size" -ge "$min_bytes" ]; then
                    log "wget 下载成功，文件大小: ${size} 字节"
                    return 0
                fi
                warn "wget 下载的文件异常，文件大小: ${size} 字节"
                rm -f "$output"
            else
                warn "wget 下载失败"
                rm -f "$output"
            fi
        fi

        if ! command_exists curl && ! command_exists wget; then
            error "未找到 curl 或 wget，无法下载"
            return 1
        fi

        if [ "$retry_count" -lt "$max_retries" ]; then
            log "等待 3 秒后重试..."
            sleep 3
        fi
    done

    error "达到最大重试次数，下载失败"
    return 1
}

fetch_api_list() {
    local api_path="$1"
    local output="$2"

    rm -f "$output"
    if command_exists curl; then
        if curl -f -sS -L --connect-timeout 15 --max-time 60 --retry 2 --retry-delay 2 -G --data-urlencode "path=${api_path}" -o "$output" "$BASE_API_URL" && [ -s "$output" ]; then
            return 0
        fi
        rm -f "$output"
    fi

    download_with_retry "${BASE_API_URL}?path=${api_path}" "$output" 1
}

split_json_objects() {
    local file="$1"

    tr '\n' ' ' < "$file" | sed -E 's/}[[:space:]]*,[[:space:]]*{/}\
{/g'
}

json_dir_names() {
    local file="$1"

    if [ "$JQ_AVAILABLE" = true ]; then
        jq -r '.data.content[]? | select(.is_dir == true) | .name // empty' "$file" 2>/dev/null
    else
        split_json_objects "$file" | sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*"is_dir"[[:space:]]*:[[:space:]]*true.*/\1/p'
    fi
}

json_file_names() {
    local file="$1"

    if [ "$JQ_AVAILABLE" = true ]; then
        jq -r '.data.content[]? | select(.is_dir != true) | .name // empty' "$file" 2>/dev/null
    else
        split_json_objects "$file" | sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
    fi
}

release_tag_name() {
    local file="$1"

    if [ "$JQ_AVAILABLE" = true ]; then
        jq -r '.tag_name // empty' "$file" 2>/dev/null
    else
        tr '\n' ' ' < "$file" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
    fi
}

release_asset_url_by_name() {
    local file="$1"
    local asset_name="$2"

    if [ "$JQ_AVAILABLE" = true ]; then
        jq -r --arg name "$asset_name" '.assets[]? | select(.name == $name) | .browser_download_url // empty' "$file" 2>/dev/null | head -n 1
    else
        split_json_objects "$file" | awk -v asset="$asset_name" 'index($0, "\"name\":\"" asset "\"") || index($0, "\"name\": \"" asset "\"")' | sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
    fi
}

release_first_asset_url_matching() {
    local file="$1"
    local pattern="$2"

    if [ "$JQ_AVAILABLE" = true ]; then
        jq -r --arg pattern "$pattern" '.assets[]? | select((.browser_download_url // "") | test($pattern)) | .browser_download_url // empty' "$file" 2>/dev/null | head -n 1
    else
        split_json_objects "$file" | sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | grep -E "$pattern" | head -n 1
    fi
}

select_target_file() {
    local file="$1"
    local arch="$2"
    local target=""

    target="$(json_file_names "$file" | grep -E "^StellarCore_.*_linux_${arch}\.tar\.gz$" | head -n 1 || true)"
    if [ -z "$target" ]; then
        target="$(json_file_names "$file" | grep -E "^StellarCore_.*_linux_${arch}\.zip$" | head -n 1 || true)"
    fi

    printf '%s\n' "$target"
}

check_archive_entries() {
    local list_file="$1"
    local entry=""

    while IFS= read -r entry; do
        [ -z "$entry" ] && continue
        case "$entry" in
            /*|../*|*/../*|*/..|..|*\\*)
                error "压缩包包含不安全路径: $entry"
                return 1
                ;;
        esac
    done < "$list_file"

    return 0
}

list_tar_archive() {
    local archive="$1"
    local list_file="$2"
    local mode=""

    for mode in plain gzip xz bzip2; do
        case "$mode" in
            plain)
                tar -tf "$archive" > "$list_file" 2>/dev/null && printf '%s\n' "$mode" && return 0
                ;;
            gzip)
                tar -ztf "$archive" > "$list_file" 2>/dev/null && printf '%s\n' "$mode" && return 0
                ;;
            xz)
                tar -Jtf "$archive" > "$list_file" 2>/dev/null && printf '%s\n' "$mode" && return 0
                ;;
            bzip2)
                tar -jtf "$archive" > "$list_file" 2>/dev/null && printf '%s\n' "$mode" && return 0
                ;;
        esac
    done

    return 1
}

extract_tar_archive() {
    local archive="$1"
    local target_dir="$2"
    local mode="$3"

    case "$mode" in
        plain) tar -xf "$archive" -C "$target_dir" ;;
        gzip) tar -zxf "$archive" -C "$target_dir" ;;
        xz) tar -Jxf "$archive" -C "$target_dir" ;;
        bzip2) tar -jxf "$archive" -C "$target_dir" ;;
        *) return 1 ;;
    esac
}

safe_extract_archive() {
    local archive="$1"
    local target_dir="$2"
    local list_file="${TMP_ROOT}/archive-list-$(generate_random_string 8).txt"
    local tar_mode=""

    mkdir -p "$target_dir" || return 1
    log "正在检查压缩包条目..."

    if tar_mode="$(list_tar_archive "$archive" "$list_file")"; then
        check_archive_entries "$list_file" || return 1
        log "正在解压 tar 压缩包..."
        extract_tar_archive "$archive" "$target_dir" "$tar_mode"
        return $?
    fi

    if command_exists unzip && unzip -Z1 "$archive" > "$list_file" 2>/dev/null; then
        check_archive_entries "$list_file" || return 1
        log "正在解压 zip 压缩包..."
        unzip -q "$archive" -d "$target_dir"
        return $?
    fi

    error "所有解压方式均失败，可能是文件损坏或缺少 unzip"
    return 1
}

find_expected_binary() {
    local search_dir="$1"
    local candidate=""

    candidate="$(find "$search_dir" -type f \( -name StellarCore -o -name frps \) -print | head -n 1)"
    if [ -z "$candidate" ]; then
        return 1
    fi

    chmod +x "$candidate"
    printf '%s\n' "$candidate"
}

download_archive_and_prepare_binary() {
    local url="$1"
    local archive_name="$2"
    local archive_path="${TMP_ROOT}/downloads/${archive_name}"
    local extract_dir="${TMP_ROOT}/extracts/$(generate_random_string 8)"
    local candidate=""

    if ! download_with_retry "$url" "$archive_path" 1024; then
        return 1
    fi

    if ! safe_extract_archive "$archive_path" "$extract_dir"; then
        return 1
    fi

    candidate="$(find_expected_binary "$extract_dir")" || {
        error "压缩包中未找到预期二进制文件 StellarCore 或 frps"
        return 1
    }

    PREPARED_BINARY="$candidate"
    return 0
}

download_direct_binary() {
    local url="$1"
    local name="$2"
    local output="${TMP_ROOT}/downloads/${name}"

    if ! download_with_retry "$url" "$output" 1024; then
        return 1
    fi

    chmod +x "$output"
    PREPARED_BINARY="$output"
    return 0
}

detect_arch() {
    local arch
    arch="$(uname -m)"

    case "$arch" in
        x86_64|amd64) FRP_ARCH="amd64" ;;
        aarch64|arm64) FRP_ARCH="arm64" ;;
        armv7l|armv7*) FRP_ARCH="arm32v7" ;;
        i386|i686) FRP_ARCH="386" ;;
        *)
            warn "未明确支持的架构: $arch，尝试使用 amd64"
            FRP_ARCH="amd64"
            ;;
    esac

    log "检测到系统架构: $arch, 对应 Frp 架构标识: $FRP_ARCH"
}

download_from_primary_source() {
    local root_json="${TMP_ROOT}/root.json"
    local frps_json="${TMP_ROOT}/frps.json"
    local versions_json="${TMP_ROOT}/versions.json"
    local files_json="${TMP_ROOT}/files.json"
    local dir=""
    local current_version=""
    local target_file=""
    local download_url=""

    log "正在获取主目录信息..."
    if ! fetch_api_list "$ROOT_PATH" "$root_json"; then
        warn "无法获取主目录信息"
        return 1
    fi

    while IFS= read -r dir; do
        [ -z "$dir" ] && continue
        log "尝试下载源: $dir"

        if ! fetch_api_list "${ROOT_PATH}/${dir}" "$frps_json"; then
            warn "无法获取源 ${dir} 的目录信息，尝试下一个源"
            continue
        fi

        if ! json_dir_names "$frps_json" | grep -Fxq "frps"; then
            warn "在源 ${dir} 中未找到 frps 目录，尝试下一个源"
            continue
        fi

        if ! fetch_api_list "${ROOT_PATH}/${dir}/frps" "$versions_json"; then
            warn "无法获取源 ${dir} 的版本信息，尝试下一个源"
            continue
        fi

        current_version="$(json_dir_names "$versions_json" | head -n 1)"
        if [ -z "$current_version" ]; then
            warn "在源 ${dir} 中未找到版本信息，尝试下一个源"
            continue
        fi
        log "在源 ${dir} 中找到版本: $current_version"

        if ! fetch_api_list "${ROOT_PATH}/${dir}/frps/${current_version}" "$files_json"; then
            warn "无法获取源 ${dir} 版本 ${current_version} 的文件信息，尝试下一个源"
            continue
        fi

        target_file="$(select_target_file "$files_json" "$FRP_ARCH")"
        if [ -z "$target_file" ]; then
            warn "在源 ${dir} 版本 ${current_version} 中未找到适合 ${FRP_ARCH} 的 linux 安装包，尝试下一个源"
            continue
        fi

        log "找到适合当前系统的安装包: $target_file"
        download_url="${BASE_DOWNLOAD_URL}/${dir}/frps/${current_version}/${target_file}"
        log "尝试下载: $download_url"

        if download_archive_and_prepare_binary "$download_url" "$target_file"; then
            return 0
        fi

        warn "下载或解压失败，尝试下一个源"
    done < <(json_dir_names "$root_json")

    warn "主目录中未找到可用下载源"
    return 1
}

download_from_github() {
    local release_json="${TMP_ROOT}/github-release.json"
    local version=""
    local asset_name=""
    local download_url=""

    log "尝试从 GitHub 备用源下载..."
    if ! download_with_retry "$GITHUB_API_URL" "$release_json" 1; then
        return 1
    fi

    version="$(release_tag_name "$release_json" | sed 's/^v//')"
    if [ -z "$version" ]; then
        error "无法从 GitHub Release 信息中提取版本号"
        return 1
    fi
    log "检测到 GitHub 最新版本: $version"

    asset_name="StellarCore_${version}_linux_${FRP_ARCH}.tar.gz"
    download_url="$(release_asset_url_by_name "$release_json" "$asset_name")"

    if [ -z "$download_url" ]; then
        asset_name="StellarCore_${version}_linux_${FRP_ARCH}.zip"
        download_url="$(release_asset_url_by_name "$release_json" "$asset_name")"
    fi

    if [ -z "$download_url" ]; then
        warn "未找到精确匹配资产，尝试使用第一个 .tar.gz 资产"
        download_url="$(release_first_asset_url_matching "$release_json" '\.tar\.gz$')"
        asset_name="${download_url##*/}"
    fi

    if [ -z "$download_url" ]; then
        error "在 GitHub Release ${version} 中未找到可用资产"
        return 1
    fi

    log "尝试从 GitHub 下载: $download_url"
    download_archive_and_prepare_binary "$download_url" "$asset_name"
}

download_binary_directly() {
    local release_json="${TMP_ROOT}/github-release-direct.json"
    local download_url=""
    local binary_name=""

    log "尝试直接下载二进制文件..."
    if ! download_with_retry "$GITHUB_API_URL" "$release_json" 1; then
        return 1
    fi

    download_url="$(release_asset_url_by_name "$release_json" "StellarCore")"
    binary_name="StellarCore"
    if [ -z "$download_url" ]; then
        download_url="$(release_asset_url_by_name "$release_json" "frps")"
        binary_name="frps"
    fi

    if [ -z "$download_url" ]; then
        error "在 GitHub Release 中未找到二进制文件 StellarCore 或 frps"
        return 1
    fi

    log "尝试下载二进制文件: $download_url"
    download_direct_binary "$download_url" "$binary_name"
}

download_and_prepare() {
    PREPARED_BINARY=""
    detect_arch

    if download_from_primary_source; then
        return 0
    fi

    warn "从 StellarFrp 所有源下载均失败，尝试从 GitHub 下载"
    if download_from_github; then
        return 0
    fi

    warn "所有压缩包下载方式均失败，尝试直接下载二进制文件"
    if download_binary_directly; then
        return 0
    fi

    error "所有下载方式均失败，无法继续安装"
    return 1
}

install_binary_atomic() {
    local source_binary="$1"
    local temp_binary="${BIN_PATH}.tmp.$$"

    mkdir -p "$(dirname "$BIN_PATH")" || return 1
    cp "$source_binary" "$temp_binary" || {
        rm -f "$temp_binary"
        return 1
    }
    chmod 755 "$temp_binary" || {
        rm -f "$temp_binary"
        return 1
    }
    mv -f "$temp_binary" "$BIN_PATH"
}

ensure_config_dir() {
    mkdir -p "$CONFIG_DIR" || return 1
    chmod 700 "$CONFIG_DIR" || return 1
}

install_config_file() {
    local source_config="$1"
    local temp_config="${CONFIG_PATH}.tmp.$$"

    ensure_config_dir || return 1
    cp "$source_config" "$temp_config" || {
        rm -f "$temp_config"
        return 1
    }
    chmod 600 "$temp_config" || {
        rm -f "$temp_config"
        return 1
    }
    mv -f "$temp_config" "$CONFIG_PATH"
}

validate_number() {
    local input="$1"
    local name="$2"
    if ! [[ "$input" =~ ^[0-9]+$ ]]; then
        error "$name 必须为数字"
        return 1
    fi
    return 0
}

validate_port() {
    local port="$1"
    local name="$2"
    if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
        error "$name 必须在 1-65535 之间"
        return 1
    fi
    return 0
}

validate_port_range() {
    local min="$1"
    local max="$2"
    if [ "$min" -gt "$max" ]; then
        error "最小端口不能大于最大端口"
        return 1
    fi
    return 0
}

current_config_ports() {
    if [ ! -f "$CONFIG_PATH" ]; then
        return 0
    fi

    sed -n 's/^[[:space:]]*\(bindPort\|webServer\.port\|vhostHTTPPort\|vhostHTTPSPort\)[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\2/p' "$CONFIG_PATH"
}

is_current_config_port() {
    local port="$1"

    [ "$PORT_CHECK_ALLOW_CURRENT_CONFIG" = true ] || return 1
    current_config_ports | grep -Fxq "$port"
}

port_in_use() {
    local port="$1"

    if command_exists ss; then
        ss -tuln 2>/dev/null | grep -E "[.:]${port}[[:space:]]" >/dev/null 2>&1
        return $?
    fi

    if command_exists netstat; then
        netstat -tuln 2>/dev/null | grep -E "[.:]${port}[[:space:]]" >/dev/null 2>&1
        return $?
    fi

    return 2
}

check_port() {
    local port="$1"
    local service="$2"

    port_in_use "$port"
    local status=$?

    if [ "$status" -eq 0 ]; then
        if is_current_config_port "$port"; then
            warn "端口 ${port} (${service}) 当前已被现有 StellarFrps 配置使用，更新时允许复用"
            return 0
        fi
        error "端口 ${port} (${service}) 已被占用，请选择其他端口"
        return 1
    fi

    if [ "$status" -eq 2 ]; then
        warn "未找到 ss 或 netstat，无法自动检查端口 ${port} (${service}) 是否被占用"
    fi

    return 0
}

prompt_runtime_config() {
    while true; do
        read -r -p "请输入服务绑定端口: " BIND_PORT
        if validate_number "$BIND_PORT" "服务绑定端口" && validate_port "$BIND_PORT" "服务绑定端口" && check_port "$BIND_PORT" "服务绑定端口"; then
            break
        fi
    done

    while true; do
        read -r -p "请输入开放端口范围最小值: " MIN_PORT
        if validate_number "$MIN_PORT" "最小开放端口" && validate_port "$MIN_PORT" "最小开放端口"; then
            break
        fi
    done

    while true; do
        read -r -p "请输入开放端口范围最大值: " MAX_PORT
        if validate_number "$MAX_PORT" "最大开放端口" && validate_port "$MAX_PORT" "最大开放端口" && validate_port_range "$MIN_PORT" "$MAX_PORT"; then
            break
        fi
    done

    while true; do
        read -r -p "请输入面板端口: " DASH_PORT
        if ! validate_number "$DASH_PORT" "面板端口" || ! validate_port "$DASH_PORT" "面板端口"; then
            continue
        fi
        if [ "$DASH_PORT" = "$BIND_PORT" ]; then
            error "面板端口不能与服务绑定端口相同"
            continue
        fi
        if check_port "$DASH_PORT" "面板端口"; then
            break
        fi
    done

    read -r -p "请输入面板用户 (留空随机生成): " DASH_USER
    read -r -p "请输入面板密码 (留空随机生成): " DASH_PWD

    if [ -z "$DASH_USER" ]; then
        DASH_USER="user_$(generate_random_string 8)"
        log "已生成随机用户名: $DASH_USER"
    fi

    if [ -z "$DASH_PWD" ]; then
        DASH_PWD="$(generate_random_string 16)"
        log "已生成随机密码: $DASH_PWD"
    fi

    while true; do
        read -r -p "是否开启 HTTP 端口 (80)? (y/n): " ENABLE_HTTP
        case "$ENABLE_HTTP" in
            y|Y)
                check_port 80 "HTTP端口" && break
                ;;
            n|N)
                break
                ;;
            *)
                error "请输入 y 或 n"
                ;;
        esac
    done

    while true; do
        read -r -p "是否开启 HTTPS 端口 (443)? (y/n): " ENABLE_HTTPS
        case "$ENABLE_HTTPS" in
            y|Y)
                check_port 443 "HTTPS端口" && break
                ;;
            n|N)
                break
                ;;
            *)
                error "请输入 y 或 n"
                ;;
        esac
    done
}

write_config_file() {
    local target="$1"

    {
        printf 'bindPort = %s\n' "$BIND_PORT"
        printf 'allowPorts = [\n'
        printf '  { start = %s, end = %s },\n' "$MIN_PORT" "$MAX_PORT"
        printf ']\n'
        printf 'webServer.addr = "0.0.0.0"\n'
        printf 'webServer.port = %s\n' "$DASH_PORT"
        printf 'webServer.user = "%s"\n' "$DASH_USER"
        printf 'webServer.password = "%s"\n' "$DASH_PWD"
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
            printf 'vhostHTTPPort = 80\n'
        fi
        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
            printf 'vhostHTTPSPort = 443\n'
        fi
        printf '[[httpPlugins]]\n'
        printf 'addr = "https://api.stellarfrp.top"\n'
        printf 'path = "/api/v1/proxy/auth"\n'
        printf 'ops = ["Login", "NewProxy", "CloseProxy"]\n'
    } > "$target" || return 1

    chmod 600 "$target"
}

configure_firewall() {
    log "配置防火墙..."

    if command_exists firewall-cmd; then
        firewall-cmd --permanent --add-port="${BIND_PORT}/tcp"
        firewall-cmd --permanent --add-port="${DASH_PORT}/tcp"
        firewall-cmd --permanent --add-port="${MIN_PORT}-${MAX_PORT}/tcp"
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
            firewall-cmd --permanent --add-port=80/tcp
        fi
        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
            firewall-cmd --permanent --add-port=443/tcp
        fi
        firewall-cmd --reload
    elif command_exists ufw; then
        ufw allow "${BIND_PORT}/tcp"
        ufw allow "${DASH_PORT}/tcp"
        ufw allow "${MIN_PORT}:${MAX_PORT}/tcp"
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
            ufw allow 80/tcp
        fi
        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
            ufw allow 443/tcp
        fi
        ufw reload
    else
        warn "未找到支持的防火墙工具，请手动开放端口"
    fi
}

get_public_ip() {
    local ip=""

    if command_exists curl; then
        ip="$(curl -fsS --connect-timeout 5 --max-time 10 https://api.ipify.org 2>/dev/null || true)"
        [ -n "$ip" ] || ip="$(curl -fsS --connect-timeout 5 --max-time 10 https://ifconfig.me 2>/dev/null || true)"
        [ -n "$ip" ] || ip="$(curl -fsS --connect-timeout 5 --max-time 10 https://api.ip.sb/ip 2>/dev/null || true)"
    elif command_exists wget; then
        ip="$(wget -qO- --timeout=10 https://api.ipify.org 2>/dev/null || true)"
        [ -n "$ip" ] || ip="$(wget -qO- --timeout=10 https://ifconfig.me 2>/dev/null || true)"
        [ -n "$ip" ] || ip="$(wget -qO- --timeout=10 https://api.ip.sb/ip 2>/dev/null || true)"
    fi

    printf '%s\n' "${ip:-服务器公网IP}"
}

show_config_summary() {
    local public_ip
    public_ip="$(get_public_ip)"

    cat << EOF

=== 重要配置信息(请将以下信息发送到管理员) ===
服务绑定端口: $BIND_PORT
开放端口范围: $MIN_PORT-$MAX_PORT
面板访问地址: http://${public_ip}:$DASH_PORT
面板用户名: $DASH_USER
面板密码: $DASH_PWD
==================

EOF
}

write_service_file() {
    local temp_service="${SERVICE_PATH}.tmp.$$"

    cat > "$temp_service" << 'EOF'
[Unit]
Description=StellarFrp Server Service
After=network.target

[Service]
Type=simple
User=root
Restart=on-failure
RestartSec=5s
ExecStart=/usr/local/bin/stellarcore -c /etc/stellarcore/frps.toml

[Install]
WantedBy=multi-user.target
EOF

    chmod 644 "$temp_service" || {
        rm -f "$temp_service"
        return 1
    }
    mv -f "$temp_service" "$SERVICE_PATH"
}

check_installed() {
    [ -f "$BIN_PATH" ] && [ -f "$SERVICE_PATH" ]
}

start_service() {
    systemctl start "$SERVICE_UNIT" && systemctl is-active --quiet "$SERVICE_UNIT"
}

uninstall_stellarfrps() {
    log "开始卸载 StellarFrps 服务..."
    systemctl stop "$SERVICE_UNIT" 2>/dev/null || true
    systemctl disable "$SERVICE_UNIT" 2>/dev/null || true

    rm -f "$BIN_PATH"
    rm -f "$SERVICE_PATH"
    rm -rf "$CONFIG_DIR"

    systemctl daemon-reload
    log "StellarFrps 服务卸载完成"
}

install_stellarfrps() {
    local staged_config="${TMP_ROOT}/frps.toml.install"

    log "开始安装 StellarFrps 服务..."

    download_and_prepare || die "下载或准备安装文件失败"
    install_binary_atomic "$PREPARED_BINARY" || die "安装 StellarCore 到 ${BIN_PATH} 失败"

    prompt_runtime_config
    write_config_file "$staged_config" || die "生成配置文件失败"
    install_config_file "$staged_config" || die "安装配置文件失败"
    configure_firewall

    log "创建系统服务..."
    write_service_file || die "创建系统服务失败"

    systemctl daemon-reload || die "重新加载 systemd 失败"
    systemctl enable "$SERVICE_UNIT" || die "启用服务失败"
    start_service || die "启动服务失败，请手动检查 systemctl status ${SERVICE_UNIT}"

    show_config_summary
    systemctl status "$SERVICE_UNIT" --no-pager -l
}

restore_backup_binary() {
    local backup_binary="$1"

    if [ -n "$backup_binary" ] && [ -f "$backup_binary" ]; then
        install_binary_atomic "$backup_binary" || warn "恢复旧二进制失败，请手动检查 ${BIN_PATH}"
    fi
}

restore_backup_config() {
    local backup_config="$1"

    if [ -n "$backup_config" ] && [ -f "$backup_config" ]; then
        install_config_file "$backup_config" || warn "恢复旧配置失败，请手动检查 ${CONFIG_PATH}"
    fi
}

rollback_and_restart_old_service() {
    local backup_binary="$1"
    local backup_config="$2"

    warn "新版本启动失败，正在恢复旧二进制并尝试重新启动原服务"
    restore_backup_binary "$backup_binary"
    restore_backup_config "$backup_config"
    systemctl daemon-reload
    if start_service; then
        log "旧服务已恢复启动"
    else
        error "旧服务恢复启动失败，请手动检查 systemctl status ${SERVICE_UNIT}"
    fi
}

update_stellarfrps() {
    local backup_binary="${TMP_ROOT}/stellarcore.backup"
    local backup_config="${TMP_ROOT}/frps.toml.backup"
    local staged_config=""
    local modify_config=""

    log "开始更新 StellarFrps 服务..."

    if [ -f "$BIN_PATH" ]; then
        cp -p "$BIN_PATH" "$backup_binary" || die "备份旧二进制失败"
    else
        backup_binary=""
    fi

    if [ -f "$CONFIG_PATH" ]; then
        cp -p "$CONFIG_PATH" "$backup_config" || die "备份配置文件失败"
        cp -p "$CONFIG_PATH" "${CONFIG_PATH}.bak"
        chmod 600 "${CONFIG_PATH}.bak"
        log "已备份配置文件到 ${CONFIG_PATH}.bak"
    else
        backup_config=""
    fi

    download_and_prepare || {
        error "下载或准备新版本失败，原服务未停止"
        return 1
    }

    read -r -p "是否需要修改配置文件? (y/n): " modify_config
    if [[ "$modify_config" == "y" || "$modify_config" == "Y" ]]; then
        log "开始修改配置文件..."
        PORT_CHECK_ALLOW_CURRENT_CONFIG=true
        prompt_runtime_config
        PORT_CHECK_ALLOW_CURRENT_CONFIG=false

        staged_config="${TMP_ROOT}/frps.toml.update"
        write_config_file "$staged_config" || {
            error "生成新配置失败，原服务未停止"
            return 1
        }
    else
        log "保留原有配置文件"
    fi

    log "停止服务并替换二进制..."
    systemctl stop "$SERVICE_UNIT" || warn "停止服务命令返回异常，仍继续更新"

    if ! install_binary_atomic "$PREPARED_BINARY"; then
        rollback_and_restart_old_service "$backup_binary" "$backup_config"
        return 1
    fi

    if [ -n "$staged_config" ]; then
        if ! install_config_file "$staged_config"; then
            rollback_and_restart_old_service "$backup_binary" "$backup_config"
            return 1
        fi
    fi

    systemctl daemon-reload
    if ! start_service; then
        rollback_and_restart_old_service "$backup_binary" "$backup_config"
        return 1
    fi

    if [ -n "$staged_config" ]; then
        configure_firewall
        show_config_summary
    fi

    log "StellarFrps 服务更新完成"
}

ensure_jq

log "欢迎使用 StellarFrps 安装脚本"
log "-------------"

if check_installed; then
    log "检测到 StellarFrps 已安装"
    log "请选择操作："
    log "1. 更新 StellarFrps"
    log "2. 卸载 StellarFrps"
    log "3. 退出脚本"
    read -r -p "请输入选项 [1-3]: " OPTION

    case "$OPTION" in
        1)
            update_stellarfrps
            ;;
        2)
            uninstall_stellarfrps
            ;;
        3)
            log "已取消操作，脚本退出"
            exit 0
            ;;
        *)
            error "无效选项，脚本退出"
            exit 1
            ;;
    esac
else
    log "未检测到 StellarFrps 安装"
    log "请选择操作："
    log "1. 安装 StellarFrps"
    log "2. 退出脚本"
    read -r -p "请输入选项 [1-2]: " OPTION

    case "$OPTION" in
        1)
            install_stellarfrps
            ;;
        2)
            log "已取消操作，脚本退出"
            exit 0
            ;;
        *)
            error "无效选项，脚本退出"
            exit 1
            ;;
    esac
fi

exit 0
