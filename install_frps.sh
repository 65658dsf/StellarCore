#!/bin/bash

# 检查root权限
if [ "$EUID" -ne 0 ]; then
    echo "请使用root权限运行此脚本"
    exit 1
fi

# --- Function Definitions ---
# 生成随机字符串函数
generate_random_string() {
    tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w ${1:-32} | head -n 1
}

# 验证数字函数 (后面会用到)
validate_number() {
    [[ "$1" =~ ^[0-9]+$ ]] || { echo "$2 必须为数字"; exit 1; }
}

# --- Check and Uninstall Existing ---
if systemctl is-active --quiet stellarfrps; then
    echo "检测到已安装stellarfrps服务，正在卸载..."
    systemctl stop stellarfrps
    systemctl disable stellarfrps
    rm -f /usr/local/bin/frps /etc/systemd/system/stellarfrps.service
    rm -rf /etc/frp
    systemctl daemon-reload
    echo "已完成旧版本卸载，开始安装新版本..."
fi

# --- System Info and FRP Version ---
# 获取系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)   FRP_ARCH="amd64" ;;
    aarch64)  FRP_ARCH="arm64" ;;
    armv7l)   FRP_ARCH="arm" ;;
    *)        echo "不支持的架构: $ARCH"; exit 1 ;;
esac

# 获取最新版本
echo "正在获取 FRP 最新版本号..."
LATEST_VERSION=$(curl -s -m 10 https://api.github.com/repos/fatedier/frp/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
if [ -z "$LATEST_VERSION" ]; then
    echo "无法获取 FRP 最新版本号，请检查网络或稍后再试。"
    exit 1
fi
echo "获取到最新版本: v$LATEST_VERSION"

# --- Conditional Download ---
# 获取公网 IP 地址
echo "正在获取公网 IP 地址..."
PUBLIC_IP=$(curl -s -m 10 https://api.ipify.org || curl -s -m 10 https://ifconfig.me || curl -s -m 10 https://ip.sb/ip || hostname -I | awk '{print $1}')

COUNTRY_CODE=""
if [ -z "$PUBLIC_IP" ]; then
    echo "警告：无法获取公网 IP 地址。将默认使用 GitHub 直接下载。"
else
    echo "获取到公网 IP: $PUBLIC_IP"
    echo "正在检测 IP 地理位置 (使用 ipinfo.io)..."
    # Use ipinfo.io to get country code, timeout after 10s
    COUNTRY_CODE=$(curl -s -m 10 "https://ipinfo.io/${PUBLIC_IP}/country")
    # Trim potential whitespace/newline
    COUNTRY_CODE=$(echo "$COUNTRY_CODE" | tr -d '[:space:]')
    if [ -z "$COUNTRY_CODE" ]; then
         echo "警告：无法检测 IP 地理位置。将默认使用 GitHub 直接下载。"
    else
         echo "检测到国家代码: $COUNTRY_CODE"
    fi
fi

# 设置下载基础 URL
if [ "$COUNTRY_CODE" == "CN" ]; then
    echo "检测到中国大陆 IP，将使用镜像下载。"
    BASE_DOWNLOAD_URL="https://gh.llkk.cc/https://github.com"
else
    if [ -n "$COUNTRY_CODE" ]; then # Only print this if we got a country code
        echo "IP 位于 $COUNTRY_CODE，将使用 GitHub 直接下载。"
    fi
    BASE_DOWNLOAD_URL="https://github.com"
fi

# 下载并解压 FRP
FRP_FILENAME="frp_${LATEST_VERSION}_linux_${FRP_ARCH}.tar.gz"
DOWNLOAD_URL="${BASE_DOWNLOAD_URL}/fatedier/frp/releases/download/v${LATEST_VERSION}/${FRP_FILENAME}"

echo "准备下载: $DOWNLOAD_URL"
wget -q --show-progress --timeout=60 --tries=3 "$DOWNLOAD_URL" -O frp.tar.gz
if [ $? -ne 0 ]; then
    echo "下载失败。请检查网络连接或尝试更换下载源 (编辑脚本中的 BASE_DOWNLOAD_URL)。"
    # 尝试备用源 (GitHub) 如果之前用了镜像
    if [ "$COUNTRY_CODE" == "CN" ]; then
        echo "尝试直接从 GitHub 下载..."
        BASE_DOWNLOAD_URL="https://github.com"
        DOWNLOAD_URL="${BASE_DOWNLOAD_URL}/fatedier/frp/releases/download/v${LATEST_VERSION}/${FRP_FILENAME}"
        wget -q --show-progress --timeout=60 --tries=3 "$DOWNLOAD_URL" -O frp.tar.gz || { echo "备用源下载失败"; exit 1; }
    else
         exit 1
    fi
fi

echo "解压文件..."
tar zxf frp.tar.gz || { echo "解压失败"; exit 1; }
cd frp_${LATEST_VERSION}_linux_${FRP_ARCH} || { echo "进入解压目录失败"; exit 1; }

# --- Installation ---
echo "安装 frps 到 /usr/local/bin"
cp frps /usr/local/bin/ && chmod +x /usr/local/bin/frps

# 创建配置目录
mkdir -p /etc/frp

# --- Configuration Input ---
# 询问是否自定义配置
read -p "是否使用自定义配置? (否则将使用默认配置: frps端口=1000, 开放端口=10000-50000, 面板端口/用户/密码=随机) (y/n, 默认 n): " USE_CUSTOM

if [[ "$USE_CUSTOM" == "y" || "$USE_CUSTOM" == "Y" ]]; then
    echo "--- 开始自定义配置 ---"
    read -p "请输入服务绑定端口 (bindPort): " BIND_PORT
    read -p "请输入允许客户端连接的端口范围最小值 (allowPorts.start): " MIN_PORT
    read -p "请输入允许客户端连接的端口范围最大值 (allowPorts.end): " MAX_PORT
    read -p "请输入面板端口 (webServer.port): " DASH_PORT
    read -p "请输入面板用户 (webServer.user, 留空随机生成): " DASH_USER
    read -p "请输入面板密码 (webServer.password, 留空随机生成): " DASH_PWD

    # 随机生成面板用户名/密码 (如果留空)
    if [ -z "$DASH_USER" ]; then DASH_USER="user_$(generate_random_string 8)"; echo "已生成随机用户名: $DASH_USER"; fi
    if [ -z "$DASH_PWD" ]; then DASH_PWD="$(generate_random_string 16)"; echo "已生成随机密码: $DASH_PWD (请务必记录)"; fi

    # 验证输入
    validate_number "$BIND_PORT" "服务绑定端口"
    validate_number "$MIN_PORT" "最小开放端口"
    validate_number "$MAX_PORT" "最大开放端口"
    validate_number "$DASH_PORT" "面板端口"
    [ "$MIN_PORT" -le "$MAX_PORT" ] || { echo "错误：最小端口不能大于最大端口"; exit 1; }
    echo "--- 自定义配置完成 ---"
else
    echo "--- 使用默认配置 ---"
    BIND_PORT=1000
    MIN_PORT=10000
    MAX_PORT=50000
    DASH_PORT=$(( RANDOM % 55536 + 10000 )) # 随机端口 10000-65535
    DASH_USER="user_$(generate_random_string 8)"
    DASH_PWD="$(generate_random_string 16)"

    echo "默认服务绑定端口: $BIND_PORT"
    echo "默认开放端口范围: $MIN_PORT-$MAX_PORT"
    echo "随机生成面板端口: $DASH_PORT"
    echo "随机生成面板用户名: $DASH_USER"
    echo "随机生成面板密码: $DASH_PWD (请务必记录)"
    echo "--- 默认配置完成 ---"
fi

# 通用配置输入
read -p "请输入节点类型 (用于 httpPlugins 路径, 例如 free/VIP): " NODE_TYPE
read -p "请输入节点名称 (用于日志/标识, 例如 HK-1): " NODE_NAME
read -p "是否开启 HTTP 虚拟主机端口 (vhostHTTPPort = 80)? (y/n, 默认 n): " ENABLE_HTTP
read -p "是否开启 HTTPS 虚拟主机端口 (vhostHTTPSPort = 443)? (y/n, 默认 n): " ENABLE_HTTPS

# --- Generate Config File ---
echo "生成配置文件 /etc/frp/frps.toml..."
cat > /etc/frp/frps.toml << EOF
bindPort = $BIND_PORT
allowPorts = [
  { start = $MIN_PORT, end = $MAX_PORT },
]
webServer.addr = "0.0.0.0"
webServer.port = $DASH_PORT
webServer.user = "$DASH_USER"
webServer.password = "$DASH_PWD"
transport.maxPoolCount = 50 # Example: Add pool count setting
log.to = "/var/log/frps.log" # Example: Log to file
log.level = "info"
log.maxDays = 3

# [[httpPlugins]] # 保留示例，但默认注释掉
# name = "user-manager"
# addr = "http://127.0.0.1:8000"
# path = "/handler"
# ops = ["Login"]

# 根据您的原始脚本添加的插件配置
[[httpPlugins]]
addr = "https://api.stellarfrp.top"
path = "/${NODE_TYPE:-default_type}" # 使用默认值防止为空
ops = ["Login"]
[[httpPlugins]]
addr = "https://api.stellarfrp.top"
path = "/check_proxy"
ops = ["NewProxy"]
EOF

# 根据用户选择，添加 HTTP/HTTPS 端口
if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
    echo "vhostHTTPPort = 80" >> /etc/frp/frps.toml
    echo "已在配置中启用 HTTP (80) 端口"
fi
if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
    echo "vhostHTTPSPort = 443" >> /etc/frp/frps.toml
    echo "已在配置中启用 HTTPS (443) 端口"
fi

# --- Firewall Configuration ---
echo "配置防火墙..."
FIREWALL_PORTS=("$BIND_PORT" "$DASH_PORT" "$MIN_PORT-$MAX_PORT")
if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then FIREWALL_PORTS+=("80"); fi
if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then FIREWALL_PORTS+=("443"); fi

OPENED_PORTS=()
FAILED_PORTS=()

configure_firewall() {
    local tool_name=$1
    local add_cmd=$2
    local reload_cmd=$3

    echo "使用 $tool_name 配置防火墙..."
    for port_spec in "${FIREWALL_PORTS[@]}"; do
        echo -n "尝试开放端口/范围: $port_spec/tcp... "
        eval "$add_cmd \"$port_spec\"" # Use eval to handle commands with arguments correctly
        if [ $? -eq 0 ]; then
            echo "成功 (待重载)"
            OPENED_PORTS+=("$port_spec")
        else
            echo "失败"
            FAILED_PORTS+=("$port_spec")
        fi
    done
    echo -n "重载 $tool_name 配置... "
    if $reload_cmd; then
        echo "成功"
    else
        echo "失败"
        # Attempt to remove ports that were added successfully but failed to reload
        # This part is complex and might need more robust error handling
    fi
}

if command -v firewall-cmd &> /dev/null; then
    # Format for firewall-cmd: port/tcp or start-end/tcp
    add_cmd_template='firewall-cmd --permanent --add-port=%s/tcp > /dev/null 2>&1'
    reload_cmd='firewall-cmd --reload > /dev/null 2>&1'
    
    firewall_add_cmd() {
        local port_spec=$1
        local formatted_spec=$(echo "$port_spec" | sed 's/:/-/') # Replace : with - if present
        local cmd=$(printf "$add_cmd_template" "$formatted_spec")
        eval "$cmd"
    }
    configure_firewall "firewall-cmd" firewall_add_cmd "$reload_cmd"

elif command -v ufw &> /dev/null; then
    add_cmd_template='ufw allow %s/tcp > /dev/null 2>&1'
    reload_cmd='ufw reload > /dev/null 2>&1'
    
    ufw_add_cmd() {
        local port_spec=$1
        local cmd=$(printf "$add_cmd_template" "$port_spec")
        eval "$cmd"
    }
    configure_firewall "ufw" ufw_add_cmd "$reload_cmd"

else
    echo "警告：未找到支持的防火墙工具 (firewall-cmd or ufw)。"
    FAILED_PORTS=("${FIREWALL_PORTS[@]}") # Mark all as failed for manual action
fi

if [ ${#FAILED_PORTS[@]} -gt 0 ]; then
     echo "警告：以下 TCP 端口未能自动打开，请手动配置防火墙："
     for port in "${FAILED_PORTS[@]}"; do echo "  - $port"; done
fi

# --- Systemd Service ---
echo "创建 systemd 服务: /etc/systemd/system/stellarfrps.service"
cat > /etc/systemd/system/stellarfrps.service << EOF
[Unit]
Description=FRP Server Service (StellarFrps - Node: ${NODE_NAME:-Unknown})
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Restart=on-failure
RestartSec=5s
ExecStart=/usr/local/bin/frps -c /etc/frp/frps.toml
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF

# 设置服务权限并启动
chmod 644 /etc/systemd/system/stellarfrps.service
echo "重新加载 systemd 配置..."
systemctl daemon-reload
echo "启用并启动 stellarfrps 服务..."
systemctl enable stellarfrps.service
systemctl start stellarfrps.service

# --- Final Output ---
echo "
==============================================
 StellarFrps 安装与配置完成!
==============================================
=== 重要配置信息 ===
节点名称: ${NODE_NAME:-未设置}
节点类型: ${NODE_TYPE:-未设置}
服务器IP: ${PUBLIC_IP:-无法获取}
服务绑定端口 (bindPort): $BIND_PORT
允许端口范围 (allowPorts): $MIN_PORT-$MAX_PORT
HTTP 端口 (vhostHTTPPort 80): $([[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]] && echo "已开启" || echo "未开启")
HTTPS 端口 (vhostHTTPSPort 443): $([[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]] && echo "已开启" || echo "未开启")
面板访问地址: http://${PUBLIC_IP:-你的服务器IP}:$DASH_PORT
面板用户名 (webServer.user): $DASH_USER
面板密码 (webServer.password): $DASH_PWD  (请务必妥善保管!)
==============================================
服务管理命令:
启动: systemctl start stellarfrps
停止: systemctl stop stellarfrps
重启: systemctl restart stellarfrps
状态: systemctl status stellarfrps
配置文件: /etc/frp/frps.toml
服务日志: journalctl -u stellarfrps -f --no-pager  或查看 /var/log/frps.log (如果配置了log.to)
==============================================
"

# 检查服务状态
echo "检查 stellarfrps 服务状态："
systemctl status stellarfrps.service --no-pager | cat

# --- Cleanup ---
echo "清理临时文件..."
cd .. > /dev/null 2>&1 # 返回上一级目录，忽略错误
rm -f frp.tar.gz
rm -rf frp_${LATEST_VERSION}_linux_${FRP_ARCH}
echo "脚本执行完毕!"

exit 0
