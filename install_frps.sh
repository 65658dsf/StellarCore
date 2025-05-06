#!/bin/bash

# 检查root权限
if [ "$EUID" -ne 0 ]; then
    echo "请使用root权限运行此脚本"
    exit 1
fi

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

# 下载函数，带重试
download_with_retry() {
    local url=$1
    local output=$2
    local max_retries=3
    local retry_count=0
    local success=false

    while [ $retry_count -lt $max_retries ] && [ "$success" = false ]; do
        echo "下载尝试 $(($retry_count + 1))/$max_retries: $url"
        
        # 使用wget下载，设置超时和重试选项
        if wget --timeout=30 --tries=3 -q --show-progress "$url" -O "$output"; then
            # 检查文件是否为空或过小（可能是下载不完整）
            if [ -s "$output" ] && [ $(stat -c%s "$output") -gt 1000 ]; then
                echo "下载成功，文件大小: $(stat -c%s "$output") 字节"
                success=true
            else
                echo "下载的文件异常，文件大小过小: $(stat -c%s "$output") 字节"
                rm -f "$output"
                retry_count=$((retry_count + 1))
            fi
        else
            echo "下载失败，将重试"
            rm -f "$output"
            retry_count=$((retry_count + 1))
        fi
        
        if [ "$success" = false ] && [ $retry_count -lt $max_retries ]; then
            echo "等待 3 秒后重试..."
            sleep 3
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

# 尝试从GitHub下载
download_from_github() {
    echo "尝试从GitHub备用源下载..."
    
    # 直接使用已知的版本号和下载链接模式
    VERSION="1.0.0"
    ASSET_NAME="StellarCore_${VERSION}_linux_${FRP_ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/65658dsf/StellarCore/releases/download/VS${VERSION}/${ASSET_NAME}"
    
    echo "使用GitHub版本: $VERSION"
    echo "尝试从GitHub下载: $DOWNLOAD_URL"
    
    # 下载文件
    if download_with_retry "$DOWNLOAD_URL" "stellarcore.tar.gz"; then
        if try_extract "stellarcore.tar.gz"; then
            echo "从GitHub成功下载并解压"
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

# 尝试从GitHub直接下载二进制文件
download_binary_directly() {
    echo "尝试直接下载二进制文件..."
    
    # 直接下载二进制文件
    BINARY_URL="https://github.com/65658dsf/StellarCore/releases/download/VS1.0.0/StellarCore"
    echo "尝试下载: $BINARY_URL"
    
    if download_with_retry "$BINARY_URL" "StellarCore"; then
        chmod +x StellarCore
        echo "二进制文件下载成功"
        BINARY_PATH="StellarCore"
        return 0
    fi
    
    echo "二进制文件下载失败"
    return 1
}

# 获取并安装最新版本
download_and_install() {
    # 获取系统架构
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)    FRP_ARCH="amd64" ;;
        aarch64)   FRP_ARCH="arm64" ;;
        armv7l)    FRP_ARCH="arm" ;;
        *)          echo "不支持的架构: $ARCH"; exit 1 ;;
    esac

    # 获取最新版本
    echo "正在获取下载源信息..."
    DIRS_RESPONSE=$(curl -s "https://file.stellarfrp.top/api/fs/list?path=StellarCore")
    
    # 获取所有下载源
    DOWNLOAD_DIRS=$(echo $DIRS_RESPONSE | grep -o '"name":"[^"]*","size":0,"is_dir":true' | cut -d'"' -f4)
    
    # 尝试每个下载源
    DOWNLOAD_SUCCESSFUL=false
    
    if [ -n "$DOWNLOAD_DIRS" ]; then
        for dir in $DOWNLOAD_DIRS; do
            echo "尝试下载源: $dir"
            
            # 获取frps目录
            FRPS_RESPONSE=$(curl -s "https://file.stellarfrp.top/api/fs/list?path=StellarCore/$dir")
            FRPS_DIR=$(echo $FRPS_RESPONSE | grep -o '"name":"frps","size":0,"is_dir":true')
            
            if [ -n "$FRPS_DIR" ]; then
                echo "在源 $dir 中找到frps目录"
                
                # 获取版本目录
                VERSIONS_RESPONSE=$(curl -s "https://file.stellarfrp.top/api/fs/list?path=StellarCore/$dir/frps")
                CURRENT_VERSION=$(echo $VERSIONS_RESPONSE | grep -o '"name":"[^"]*","size":0,"is_dir":true' | head -1 | cut -d'"' -f4)
                
                if [ -n "$CURRENT_VERSION" ]; then
                    echo "在源 $dir 中找到版本: $CURRENT_VERSION"
                    
                    # 检查是否有适合当前系统的安装包
                    FILES_RESPONSE=$(curl -s "https://file.stellarfrp.top/api/fs/list?path=StellarCore/$dir/frps/$CURRENT_VERSION")
                    CURRENT_FILES=$(echo $FILES_RESPONSE | grep -o "\"name\":\"StellarCore_${CURRENT_VERSION}_linux_${FRP_ARCH}[^\"]*" | cut -d'"' -f4)
                    
                    if [ -n "$CURRENT_FILES" ]; then
                        echo "找到适合当前系统的安装包，将尝试下载"
                        
                        for file in $CURRENT_FILES; do
                            DOWNLOAD_URL="https://file.stellarfrp.top/d/StellarFrp/StellarCore/$dir/frps/$CURRENT_VERSION/$file"
                            echo "尝试下载: $DOWNLOAD_URL"
                            
                            if download_with_retry "$DOWNLOAD_URL" "stellarcore.tar.gz"; then
                                if try_extract "stellarcore.tar.gz"; then
                                    DOWNLOAD_SUCCESSFUL=true
                                    break
                                else
                                    echo "解压失败，尝试下一个文件"
                                    rm -f stellarcore.tar.gz
                                fi
                            else
                                echo "下载失败，尝试下一个文件"
                            fi
                        done
                        
                        if [ "$DOWNLOAD_SUCCESSFUL" = true ]; then
                            break
                        else
                            echo "从源 $dir 下载和解压均失败，尝试下一个源"
                        fi
                    else
                        echo "在源 $dir 中未找到适合当前系统的安装包，尝试下一个源"
                    fi
                else
                    echo "在源 $dir 中未找到版本信息，尝试下一个源"
                fi
            else
                echo "在源 $dir 中未找到frps目录，尝试下一个源"
            fi
        done
    else
        echo "无法获取下载源信息，将尝试其他下载方式"
    fi

    # 如果所有源都失败，尝试从GitHub下载
    if [ "$DOWNLOAD_SUCCESSFUL" != true ]; then
        echo "从StellarFrp所有源下载均失败，尝试从GitHub下载..."
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
    rm -f stellarcore.tar.gz
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
    if netstat -tuln | grep -q ":$port "; then
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
    
    # 重启服务
    systemctl start stellarfrps
    
    echo "StellarFrps 服务更新完成"
}

# 生成随机字符串函数
generate_random_string() {
    tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w ${1:-32} | head -n 1
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
cat >> /etc/stellarcore/frps.toml << EOF
[[httpPlugins]]
addr = "https://preview.api.stellarfrp.top"
path = "/api/v1/proxy/auth"
ops = ["Login", "NewProxy", "CloseProxy"]
EOF

    # 配置防火墙
    echo "配置防火墙..."
    if command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port=$BIND_PORT/tcp
        firewall-cmd --permanent --add-port=$DASH_PORT/tcp
        firewall-cmd --permanent --add-port=$MIN_PORT-$MAX_PORT/tcp
        firewall-cmd --reload
    elif command -v ufw &> /dev/null; then
        ufw allow $BIND_PORT/tcp
        ufw allow $DASH_PORT/tcp
        ufw allow $MIN_PORT:$MAX_PORT/tcp
        ufw reload
    else
        echo "警告：未找到支持的防火墙工具，请手动开放端口"
    fi

    # 创建系统服务
    echo "创建系统服务..."
    cat > /etc/systemd/system/stellarfrps.service << EOF
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

    systemctl status stellarfrps.service
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
