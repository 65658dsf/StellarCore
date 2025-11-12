#!/bin/bash

# 检查root权限
if [ "$EUID" -ne 0 ]; then
    echo "请使用root权限运行此脚本"
    exit 1
fi

# 生成随机字符串函数
generate_random_string() {
    tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w ${1:-32} | head -n 1
}

# 判断是否已安装
check_installed() {
    if [ -f "/usr/local/bin/stellarcore" ] && [ -f "/etc/systemd/system/stellarfrps.service" ]; then
        return 0
    else
        return 1
    fi
}

# 卸载函数
uninstall_stellarfrps() {
    echo "开始卸载 StellarFrps 服务..."
    # 停止并禁用服务
    systemctl stop stellarfrps 2>/dev/null
    systemctl disable stellarfrps 2>/dev/null

    # 删除文件
    rm -f /usr/local/bin/stellarcore
    rm -f /etc/systemd/system/stellarfrps.service
    rm -rf /etc/stellarcore

    # 重新加载systemd
    systemctl daemon-reload

    echo "StellarFrps 服务卸载完成"
}

# 下载函数，带重试 (使用 curl 和 wget 双重保障)
download_with_retry() {
    local url=$1
    local output=$2
    local max_retries=3
    local retry_count=0
    local success=false

    while [ $retry_count -lt $max_retries ] && [ "$success" = false ]; do
        echo "下载尝试 $(($retry_count + 1))/$max_retries: $url"

        # 优先尝试使用 curl
        if command -v curl &> /dev/null; then
            echo "使用 curl 下载..."
            # -f: 失败时返回错误码 (如 404)
            # -s: 静默模式
            # -L: 跟随重定向
            # -o: 指定输出文件
            # --retry: 重试次数
            # --retry-delay: 重试间隔秒数
            if curl -f -s -L --retry 2 --retry-delay 3 "$url" -o "$output"; then
                if [ -s "$output" ] && [ $(stat -c%s "$output" 2>/dev/null || wc -c < "$output") -gt 1000 ]; then
                    echo "curl 下载成功，文件大小: $(stat -c%s "$output" 2>/dev/null || wc -c < "$output") 字节"
                    success=true
                else
                    echo "curl 下载的文件异常，文件大小过小: $(stat -c%s "$output" 2>/dev/null || wc -c < "$output") 字节"
                    rm -f "$output"
                fi
            else
                echo "curl 下载失败"
                rm -f "$output"
            fi
        else
            echo "curl 未找到，尝试使用 wget..."
            # 如果 curl 不可用，尝试使用 wget
            # -q: 静默模式
            # -O: 指定输出文件
            # --timeout: 设置超时秒数
            # --tries: 设置重试次数 (这里设为1，由脚本逻辑控制重试)
            if wget -q --timeout=30 --tries=1 "$url" -O "$output"; then
                if [ -s "$output" ] && [ $(stat -c%s "$output" 2>/dev/null || wc -c < "$output") -gt 1000 ]; then
                    echo "wget 下载成功，文件大小: $(stat -c%s "$output" 2>/dev/null || wc -c < "$output") 字节"
                    success=true
                else
                    echo "wget 下载的文件异常，文件大小过小: $(stat -c%s "$output" 2>/dev/null || wc -c < "$output") 字节"
                    rm -f "$output"
                fi
            else
                echo "wget 下载失败"
                rm -f "$output"
            fi
        fi

        if [ "$success" = false ]; then
            retry_count=$((retry_count + 1))
            if [ $retry_count -lt $max_retries ]; then
                echo "等待 3 秒后重试..."
                sleep 3
            fi
        fi
    done

    if [ "$success" = true ]; then
        return 0
    else
        echo "达到最大重试次数，下载失败"
        return 1
    fi
}


# 尝试解压函数
try_extract() {
    local archive=$1
    echo "正在解压 $archive..."

    # 先尝试gzip格式
    if tar zxf "$archive" 2>/dev/null; then
        echo "gzip格式解压成功"
        return 0
    fi

    echo "gzip解压失败，尝试其他格式..."

    # 尝试xz格式
    if tar Jxf "$archive" 2>/dev/null; then
        echo "xz格式解压成功"
        return 0
    fi

    # 尝试bzip2格式
    if tar jxf "$archive" 2>/dev/null; then
        echo "bzip2格式解压成功"
        return 0
    fi

    # 尝试直接解压（不指定压缩算法）
    if tar xf "$archive" 2>/dev/null; then
        echo "通用格式解压成功"
        return 0
    fi

    # 如果是zip格式
    if command -v unzip &> /dev/null && unzip -q "$archive" 2>/dev/null; then
        echo "zip格式解压成功"
        return 0
    fi

    echo "所有解压方式均失败，可能是文件损坏"
    return 1
}

# 尝试从GitHub下载 (使用 GitHub API 获取最新 Release)
download_from_github() {
    echo "尝试从GitHub备用源下载..."

    GITHUB_REPO="65658dsf/StellarCore"
    API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"

    # 获取最新的 Release 信息 (使用 sed 解析 JSON)
    echo "正在获取 GitHub 最新 Release 信息..."
    RELEASE_INFO=$(curl -s "$API_URL")

    if [ -z "$RELEASE_INFO" ] || echo "$RELEASE_INFO" | grep -q '"message":"Not Found"'; then
        echo "无法获取 GitHub Release 信息或仓库不存在"
        return 1
    fi

    # 提取 tag_name (版本号)
    VERSION=$(echo "$RELEASE_INFO" | sed -n 's/.*"tag_name":"\([^"]*\)".*/\1/p' | sed 's/v//') # 去掉可能的 'v' 前缀
    if [ -z "$VERSION" ]; then
        echo "无法从 Release 信息中提取版本号"
        return 1
    fi
    echo "检测到 GitHub 最新版本: $VERSION"

    # 根据系统架构确定资产名称
    ASSET_NAME="StellarCore_${VERSION}_linux_${FRP_ARCH}.tar.gz"

    # 尝试找到匹配的资产下载 URL
    DOWNLOAD_URL=$(echo "$RELEASE_INFO" | sed -n "s/.*\"browser_download_url\":\"[^\"]*${ASSET_NAME//\./\\.}\".*/\0/p" | sed -n 's/.*"browser_download_url":"\([^"]*\)".*/\1/p')

    if [ -z "$DOWNLOAD_URL" ]; then
        echo "在 Release $VERSION 中未找到匹配的资产: $ASSET_NAME"
        # 尝试找第一个 .tar.gz 文件
        DOWNLOAD_URL=$(echo "$RELEASE_INFO" | sed -n 's/.*"browser_download_url":"\([^"]*\.tar\.gz\)".*/\1/p' | head -1)
        if [ -n "$DOWNLOAD_URL" ]; then
             echo "找到替代的 .tar.gz 资产: $DOWNLOAD_URL"
        else
            echo "在 Release $VERSION 中未找到任何 .tar.gz 资产"
            return 1
        fi
    fi

    echo "尝试从GitHub下载: $DOWNLOAD_URL"

    # 下载文件
    if download_with_retry "$DOWNLOAD_URL" "stellarcore.tar.gz"; then
        if try_extract "stellarcore.tar.gz"; then
            echo "从GitHub成功下载并解压"
            rm -f stellarcore.tar.gz # 清理
            return 0
        else
            echo "从GitHub下载的文件解压失败"
            rm -f stellarcore.tar.gz
            return 1
        fi
    fi

    echo "从GitHub下载失败"
    return 1
}

# 尝试从GitHub直接下载二进制文件 (使用 GitHub API)
download_binary_directly() {
    echo "尝试直接下载二进制文件..."

    GITHUB_REPO="65658dsf/StellarCore"
    API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"

    # 获取最新的 Release 信息
    echo "正在获取 GitHub 最新 Release 信息 (用于二进制下载)..."
    RELEASE_INFO=$(curl -s "$API_URL")

    if [ -z "$RELEASE_INFO" ] || echo "$RELEASE_INFO" | grep -q '"message":"Not Found"'; then
        echo "无法获取 GitHub Release 信息或仓库不存在 (用于二进制下载)"
        return 1
    fi

    # 提取 tag_name (版本号)
    VERSION=$(echo "$RELEASE_INFO" | sed -n 's/.*"tag_name":"\([^"]*\)".*/\1/p' | sed 's/v//')
    if [ -z "$VERSION" ]; then
        echo "无法从 Release 信息中提取版本号 (用于二进制下载)"
        return 1
    fi
    echo "检测到 GitHub 最新版本 (用于二进制下载): $VERSION"

    # 直接下载二进制文件 (假设文件名就是 StellarCore)
    BINARY_NAME="StellarCore" # 或者根据需要调整
    BINARY_URL=$(echo "$RELEASE_INFO" | sed -n "s/.*\"browser_download_url\":\"[^\"]*\/${BINARY_NAME//\./\\.}\".*/\0/p" | sed -n 's/.*"browser_download_url":"\([^"]*\)".*/\1/p' | head -1)

    if [ -z "$BINARY_URL" ]; then
        echo "在 Release $VERSION 中未找到二进制文件: $BINARY_NAME"
        return 1
    fi

    echo "尝试下载二进制文件: $BINARY_URL"

    if download_with_retry "$BINARY_URL" "StellarCore"; then
        chmod +x StellarCore
        echo "二进制文件下载成功"
        BINARY_PATH="StellarCore"
        return 0
    fi

    echo "二进制文件下载失败"
    return 1
}


# 获取并安装最新版本 (使用新的 API)
download_and_install() {
    # 获取系统架构
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)    FRP_ARCH="amd64" ;;
        aarch64)   FRP_ARCH="arm64" ;;
        armv7l)    FRP_ARCH="arm32v7" ;; # 或 arm, 根据实际包名调整
        i386|i686) FRP_ARCH="386" ;;
        *)         echo "警告：未明确支持的架构: $ARCH，尝试使用 amd64"; FRP_ARCH="amd64" ;; # 默认或退出
    esac

    echo "检测到系统架构: $ARCH, 对应 Frp 架构标识: $FRP_ARCH"

    BASE_API_URL="https://resources.stellarfrp.top/api/fs/list"
    BASE_DOWNLOAD_URL="https://resources.stellarfrp.top/d/StellarCore"
    ROOT_PATH="StellarCore"

    echo "正在获取主目录信息..."
    ROOT_RESPONSE=$(curl -s "${BASE_API_URL}?path=${ROOT_PATH}")
    if [ $? -ne 0 ]; then
        echo "无法获取主目录信息，将尝试其他下载方式"
        DOWNLOAD_SUCCESSFUL=false
    else
        # 获取所有下载源 (目录名) - 使用 sed 和 grep 提取
        # 假设 JSON 格式类似 {"code":200, "message":"success", "data":{"content":[{"name":"源1","size":0,"is_dir":true,"..."}, ...]}}
        # 提取 "name":"..." 且 "is_dir":true 的项
        DOWNLOAD_DIRS=$(echo "$ROOT_RESPONSE" | sed -n 's/.*"name":"\([^"]*\)","size":[0-9]*,"is_dir":true,.*/\1/p')

        DOWNLOAD_SUCCESSFUL=false

        if [ -n "$DOWNLOAD_DIRS" ]; then
            for dir in $DOWNLOAD_DIRS; do
                echo "尝试下载源: $dir"

                # 获取 frps 目录
                FRPS_RESPONSE=$(curl -s "${BASE_API_URL}?path=${ROOT_PATH}/${dir}")
                # 提取名为 "frps" 且是目录的项
                FRPS_DIR=$(echo "$FRPS_RESPONSE" | sed -n 's/.*"name":"frps","size":[0-9]*,"is_dir":true,.*/frps/p' | head -1)

                if [ -n "$FRPS_DIR" ]; then
                    echo "在源 $dir 中找到 frps 目录"

                    # 获取版本目录 (假设第一个是最新或可用的)
                    VERSIONS_RESPONSE=$(curl -s "${BASE_API_URL}?path=${ROOT_PATH}/${dir}/frps")
                    # 提取所有目录名作为版本
                    CURRENT_VERSION=$(echo "$VERSIONS_RESPONSE" | sed -n 's/.*"name":"\([^"]*\)","size":[0-9]*,"is_dir":true,.*/\1/p' | head -1)

                    if [ -n "$CURRENT_VERSION" ]; then
                        echo "在源 $dir 中找到版本: $CURRENT_VERSION"

                        # 检查是否有适合当前系统的安装包
                        FILES_RESPONSE=$(curl -s "${BASE_API_URL}?path=${ROOT_PATH}/${dir}/frps/${CURRENT_VERSION}")
                        # 匹配 linux 和对应架构的文件 - 使用 sed 提取
                        TARGET_FILE=$(echo "$FILES_RESPONSE" | sed -n "s/.*\"name\":\"\(StellarCore_.*_linux_${FRP_ARCH}\.tar\.gz\)\".*/\1/p" | head -1)
                        # 如果 tar.gz 没找到，尝试 zip
                        if [ -z "$TARGET_FILE" ]; then
                            TARGET_FILE=$(echo "$FILES_RESPONSE" | sed -n "s/.*\"name\":\"\(StellarCore_.*_linux_${FRP_ARCH}\.zip\)\".*/\1/p" | head -1)
                        fi

                        if [ -n "$TARGET_FILE" ]; then
                            echo "找到适合当前系统的安装包: $TARGET_FILE"
                            DOWNLOAD_URL="${BASE_DOWNLOAD_URL}/${dir}/frps/${CURRENT_VERSION}/${TARGET_FILE}"
                            echo "尝试下载: $DOWNLOAD_URL"

                            ARCHIVE_NAME="stellarcore.$(echo "$TARGET_FILE" | sed -n 's/.*\.\(tar\.gz\|zip\)$/\1/p')"
                            if download_with_retry "$DOWNLOAD_URL" "$ARCHIVE_NAME"; then
                                if try_extract "$ARCHIVE_NAME"; then
                                    DOWNLOAD_SUCCESSFUL=true
                                    rm -f "$ARCHIVE_NAME" # 清理下载的压缩包
                                    break # 成功后跳出循环
                                else
                                    echo "解压失败，尝试下一个源"
                                    rm -f "$ARCHIVE_NAME"
                                fi
                            else
                                echo "下载失败，尝试下一个源"
                            fi
                        else
                            echo "在源 $dir 版本 $CURRENT_VERSION 中未找到适合 $FRP_ARCH 的 linux 安装包，尝试下一个源"
                        fi
                    else
                        echo "在源 $dir 中未找到版本信息，尝试下一个源"
                    fi
                else
                    echo "在源 $dir 中未找到 frps 目录，尝试下一个源"
                fi
            done
        else
             echo "主目录中未找到任何下载源，将尝试其他下载方式"
        fi
    fi

    # 如果所有源都失败，尝试从GitHub下载
    if [ "$DOWNLOAD_SUCCESSFUL" != true ]; then
        echo "从 StellarFrp 所有源下载均失败，尝试从 GitHub 下载..."
        if download_from_github; then
            DOWNLOAD_SUCCESSFUL=true
        fi
    fi

    # 如果所有压缩包下载都失败，尝试直接下载二进制文件
    if [ "$DOWNLOAD_SUCCESSFUL" != true ]; then
        echo "所有压缩包下载方式均失败，尝试直接下载二进制文件..."
        if download_binary_directly; then
            DOWNLOAD_SUCCESSFUL=true
        fi
    fi

    if [ "$DOWNLOAD_SUCCESSFUL" != true ]; then
        echo "所有下载方式均失败，无法继续安装"
        exit 1
    fi

    # 查找二进制文件
    if [ -z "$BINARY_PATH" ]; then
        if [ -f "StellarCore" ]; then
            BINARY_PATH="StellarCore"
        elif [ -f "frps" ]; then
            BINARY_PATH="frps"
        else
            echo "错误：未找到可执行文件"
            exit 1
        fi
    fi

    # 安装StellarCore
    echo "安装StellarCore到/usr/local/bin"
    cp "$BINARY_PATH" /usr/local/bin/stellarcore && chmod +x /usr/local/bin/stellarcore

    # 清理临时文件
    rm -f "$BINARY_PATH"
}


# 验证数字输入函数
validate_number() {
    local input=$1
    local name=$2
    if ! [[ "$input" =~ ^[0-9]+$ ]]; then
        echo "$name 必须为数字"
        return 1
    fi
    return 0
}

# 验证端口号范围函数
validate_port() {
    local port=$1
    local name=$2
    if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
        echo "$name 必须在 1-65535 之间"
        return 1
    fi
    return 0
}

# 验证端口范围函数
validate_port_range() {
    local min=$1
    local max=$2
    if [ "$min" -gt "$max" ]; then
        echo "最小端口不能大于最大端口"
        return 1
    fi
    return 0
}

# 检查端口是否被占用
check_port() {
    local port=$1
    local service=$2
    # 更健壮的检查，避免部分匹配
    if ss -tuln | grep -E ":$port\s" > /dev/null 2>&1; then
        echo "错误: 端口 $port ($service) 已被占用，请选择其他端口"
        return 1
    fi
    return 0
}


# 更新函数
update_stellarfrps() {
    echo "开始更新 StellarFrps 服务..."
    # 备份配置文件
    if [ -f "/etc/stellarcore/frps.toml" ]; then
        cp /etc/stellarcore/frps.toml /etc/stellarcore/frps.toml.bak
        echo "已备份配置文件到 /etc/stellarcore/frps.toml.bak"
    fi

    # 停止服务
    systemctl stop stellarfrps

    # 下载并安装新版本
    download_and_install

    # 询问用户是否需要修改配置文件
    read -p "是否需要修改配置文件? (y/n): " MODIFY_CONFIG

    if [[ "$MODIFY_CONFIG" == "y" || "$MODIFY_CONFIG" == "Y" ]]; then
        echo "开始修改配置文件..."

        # 获取用户输入并立即验证
        while true; do
            read -p "请输入服务绑定端口: " BIND_PORT
            if validate_number "$BIND_PORT" "服务绑定端口" && validate_port "$BIND_PORT" "服务绑定端口" && check_port "$BIND_PORT" "服务绑定端口"; then
                break
            fi
        done

        while true; do
            read -p "请输入开放端口范围最小值: " MIN_PORT
            if validate_number "$MIN_PORT" "最小开放端口" && validate_port "$MIN_PORT" "最小开放端口"; then
                break
            fi
        done

        while true; do
            read -p "请输入开放端口范围最大值: " MAX_PORT
            if validate_number "$MAX_PORT" "最大开放端口" && validate_port "$MAX_PORT" "最大开放端口" && validate_port_range "$MIN_PORT" "$MAX_PORT"; then
                break
            fi
        done

        while true; do
            read -p "请输入面板端口: " DASH_PORT
            if validate_number "$DASH_PORT" "面板端口" && validate_port "$DASH_PORT" "面板端口" && check_port "$DASH_PORT" "面板端口"; then
                break
            fi
        done

        read -p "请输入面板用户 (留空随机生成): " DASH_USER
        read -p "请输入面板密码 (留空随机生成): " DASH_PWD

        # 如果用户名为空，生成随机用户名
        if [ -z "$DASH_USER" ]; then
            DASH_USER="user_$(generate_random_string 8)"
            echo "已生成随机用户名: $DASH_USER"
        fi

        # 如果密码为空，生成随机密码
        if [ -z "$DASH_PWD" ]; then
            DASH_PWD="$(generate_random_string 16)"
            echo "已生成随机密码: $DASH_PWD"
        fi

        # 询问用户是否开启 HTTP/HTTPS 端口
        while true; do
            read -p "是否开启 HTTP 端口 (80)? (y/n): " ENABLE_HTTP
            if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
                if check_port 80 "HTTP端口"; then
                    break
                fi
            elif [[ "$ENABLE_HTTP" == "n" || "$ENABLE_HTTP" == "N" ]]; then
                break
            else
                echo "请输入 y 或 n"
            fi
        done

        while true; do
            read -p "是否开启 HTTPS 端口 (443)? (y/n): " ENABLE_HTTPS
            if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
               if check_port 443 "HTTPS端口"; then
                   break
               fi
            elif [[ "$ENABLE_HTTPS" == "n" || "$ENABLE_HTTPS" == "N" ]]; then
                break
            else
                echo "请输入 y 或 n"
            fi
        done

        # 生成配置文件
        echo "生成配置文件..."
        # 使用 tee 和 here document 更安全地写入文件
        cat > /etc/stellarcore/frps.toml << EOF
bindPort = $BIND_PORT
allowPorts = [
  { start = $MIN_PORT, end = $MAX_PORT },
]
webServer.addr = "0.0.0.0"
webServer.port = $DASH_PORT
webServer.user = "$DASH_USER"
webServer.password = "$DASH_PWD"
EOF

        # 根据用户选择，添加 HTTP/HTTPS 端口
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
            echo "vhostHTTPPort = 80" >> /etc/stellarcore/frps.toml
        fi

        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
            echo "vhostHTTPSPort = 443" >> /etc/stellarcore/frps.toml
        fi

        # 继续添加剩余的配置
        cat >> /etc/stellarcore/frps.toml << 'EOF' # 使用 'EOF' 防止变量替换
[[httpPlugins]]
addr = "https://api.stellarfrp.top"
path = "/api/v1/proxy/auth"
ops = ["Login", "NewProxy", "CloseProxy"]
EOF

        echo "配置文件已更新"

        # 配置防火墙
        echo "配置防火墙..."
        if command -v firewall-cmd &> /dev/null; then
            firewall-cmd --permanent --add-port=$BIND_PORT/tcp
            firewall-cmd --permanent --add-port=$DASH_PORT/tcp
            firewall-cmd --permanent --add-port=$MIN_PORT-$MAX_PORT/tcp
            if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
                 firewall-cmd --permanent --add-port=80/tcp
            fi
            if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
                 firewall-cmd --permanent --add-port=443/tcp
            fi
            firewall-cmd --reload
        elif command -v ufw &> /dev/null; then
            ufw allow $BIND_PORT/tcp
            ufw allow $DASH_PORT/tcp
            ufw allow $MIN_PORT:$MAX_PORT/tcp
            if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
                 ufw allow 80/tcp
            fi
            if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
                 ufw allow 443/tcp
            fi
            ufw reload
        else
            echo "警告：未找到支持的防火墙工具，请手动开放端口"
        fi

        # 显示配置信息
        PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s https://ifconfig.me || curl -s https://api.ip.sb/ip)
        echo "
=== 重要配置信息(请将以下信息发送到管理员) ===
服务绑定端口: $BIND_PORT
开放端口范围: $MIN_PORT-$MAX_PORT
面板访问地址: http://${PUBLIC_IP}:$DASH_PORT
面板用户名: $DASH_USER
面板密码: $DASH_PWD
==================
"
    else
        echo "保留原有配置文件"
    fi

    # 重启服务
    systemctl start stellarfrps

    echo "StellarFrps 服务更新完成"
}


# 安装函数
install_stellarfrps() {
    echo "开始安装 StellarFrps 服务..."

    # 下载并安装
    download_and_install

    # 创建配置目录
    mkdir -p /etc/stellarcore

    # 获取用户输入并立即验证
    while true; do
        read -p "请输入服务绑定端口: " BIND_PORT
        if validate_number "$BIND_PORT" "服务绑定端口" && validate_port "$BIND_PORT" "服务绑定端口" && check_port "$BIND_PORT" "服务绑定端口"; then
            break
        fi
    done

    while true; do
        read -p "请输入开放端口范围最小值: " MIN_PORT
        if validate_number "$MIN_PORT" "最小开放端口" && validate_port "$MIN_PORT" "最小开放端口"; then
            break
        fi
    done

    while true; do
        read -p "请输入开放端口范围最大值: " MAX_PORT
        if validate_number "$MAX_PORT" "最大开放端口" && validate_port "$MAX_PORT" "最大开放端口" && validate_port_range "$MIN_PORT" "$MAX_PORT"; then
            break
        fi
    done

    while true; do
        read -p "请输入面板端口: " DASH_PORT
        if validate_number "$DASH_PORT" "面板端口" && validate_port "$DASH_PORT" "面板端口" && check_port "$DASH_PORT" "面板端口"; then
            break
        fi
    done

    read -p "请输入面板用户 (留空随机生成): " DASH_USER
    read -p "请输入面板密码 (留空随机生成): " DASH_PWD

    # 如果用户名为空，生成随机用户名
    if [ -z "$DASH_USER" ]; then
        DASH_USER="user_$(generate_random_string 8)"
        echo "已生成随机用户名: $DASH_USER"
    fi

    # 如果密码为空，生成随机密码
    if [ -z "$DASH_PWD" ]; then
        DASH_PWD="$(generate_random_string 16)"
        echo "已生成随机密码: $DASH_PWD"
    fi

    # 询问用户是否开启 HTTP/HTTPS 端口
    while true; do
        read -p "是否开启 HTTP 端口 (80)? (y/n): " ENABLE_HTTP
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
            if check_port 80 "HTTP端口"; then
                break
            fi
        elif [[ "$ENABLE_HTTP" == "n" || "$ENABLE_HTTP" == "N" ]]; then
            break
        else
            echo "请输入 y 或 n"
        fi
    done

    while true; do
        read -p "是否开启 HTTPS 端口 (443)? (y/n): " ENABLE_HTTPS
        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
            if check_port 443 "HTTPS端口"; then
                break
            fi
        elif [[ "$ENABLE_HTTPS" == "n" || "$ENABLE_HTTPS" == "N" ]]; then
            break
        else
            echo "请输入 y 或 n"
        fi
    done

    # 生成配置文件
    echo "生成配置文件..."
    cat > /etc/stellarcore/frps.toml << EOF
bindPort = $BIND_PORT
allowPorts = [
  { start = $MIN_PORT, end = $MAX_PORT },
]
webServer.addr = "0.0.0.0"
webServer.port = $DASH_PORT
webServer.user = "$DASH_USER"
webServer.password = "$DASH_PWD"
EOF

    # 根据用户选择，添加 HTTP/HTTPS 端口
    if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
        echo "vhostHTTPPort = 80" >> /etc/stellarcore/frps.toml
    fi

    if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
        echo "vhostHTTPSPort = 443" >> /etc/stellarcore/frps.toml
    fi

    # 继续添加剩余的配置
    cat >> /etc/stellarcore/frps.toml << 'EOF' # 使用 'EOF' 防止变量替换
[[httpPlugins]]
addr = "https://api.stellarfrp.top"
path = "/api/v1/proxy/auth"
ops = ["Login", "NewProxy", "CloseProxy"]
EOF

    # 配置防火墙
    echo "配置防火墙..."
    if command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port=$BIND_PORT/tcp
        firewall-cmd --permanent --add-port=$DASH_PORT/tcp
        firewall-cmd --permanent --add-port=$MIN_PORT-$MAX_PORT/tcp
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
             firewall-cmd --permanent --add-port=80/tcp
        fi
        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
             firewall-cmd --permanent --add-port=443/tcp
        fi
        firewall-cmd --reload
    elif command -v ufw &> /dev/null; then
        ufw allow $BIND_PORT/tcp
        ufw allow $DASH_PORT/tcp
        ufw allow $MIN_PORT:$MAX_PORT/tcp
        if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
             ufw allow 80/tcp
        fi
        if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
             ufw allow 443/tcp
        fi
        ufw reload
    else
        echo "警告：未找到支持的防火墙工具，请手动开放端口"
    fi

    # 创建系统服务
    echo "创建系统服务..."
    cat > /etc/systemd/system/stellarfrps.service << 'EOF' # 使用 'EOF' 防止变量替换
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

    # 设置服务权限
    chmod 644 /etc/systemd/system/stellarfrps.service

    # 重新加载systemd并启动服务
    systemctl daemon-reload
    systemctl enable stellarfrps.service
    systemctl start stellarfrps.service

    # 显示配置信息
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s https://ifconfig.me || curl -s https://api.ip.sb/ip)
    echo "
=== 重要配置信息(请将以下信息发送到管理员) ===
服务绑定端口: $BIND_PORT
开放端口范围: $MIN_PORT-$MAX_PORT
面板访问地址: http://${PUBLIC_IP}:$DASH_PORT
面板用户名: $DASH_USER
面板密码: $DASH_PWD
==================
"

    systemctl status stellarfrps.service --no-pager -l
}


# 显示菜单
echo "欢迎使用 StellarFrps 安装脚本"
echo "-------------"

if check_installed; then
    echo "检测到 StellarFrps 已安装"
    echo "请选择操作："
    echo "1. 更新 StellarFrps"
    echo "2. 卸载 StellarFrps"
    echo "3. 退出脚本"
    read -p "请输入选项 [1-3]: " OPTION

    case $OPTION in
        1)
            update_stellarfrps
            ;;
        2)
            uninstall_stellarfrps
            ;;
        3)
            echo "已取消操作，脚本退出"
            exit 0
            ;;
        *)
            echo "无效选项，脚本退出"
            exit 1
            ;;
    esac
else
    echo "未检测到 StellarFrps 安装"
    echo "请选择操作："
    echo "1. 安装 StellarFrps"
    echo "2. 退出脚本"
    read -p "请输入选项 [1-2]: " OPTION

    case $OPTION in
        1)
            install_stellarfrps
            ;;
        2)
            echo "已取消操作，脚本退出"
            exit 0
            ;;
        *)
            echo "无效选项，脚本退出"
            exit 1
            ;;
    esac
fi

# 脚本结束
exit 0



