#!/bin/bash

# 检查root权限
if [ "$EUID" -ne 0 ]; then
    echo "请使用root权限运行此脚本"
    exit 1
fi

# 检查是否已安装stellarfrps
if systemctl is-active --quiet stellarfrps; then
    echo "检测到已安装stellarfrps服务，正在卸载..."
    # 停止并禁用服务
    systemctl stop stellarfrps
    systemctl disable stellarfrps
    
    # 删除文件
    rm -f /usr/local/bin/frps
    rm -f /etc/systemd/system/stellarfrps.service
    rm -rf /etc/frp
    
    # 重新加载systemd
    systemctl daemon-reload
    
    echo "已完成旧版本卸载，开始安装新版本..."
fi

# 获取系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)    FRP_ARCH="amd64" ;;
    aarch64)   FRP_ARCH="arm64" ;;
    armv7l)    FRP_ARCH="arm" ;;
    *)          echo "不支持的架构: $ARCH"; exit 1 ;;
esac

# 获取最新版本
echo "正在获取最新版本..."
LATEST_VERSION=$(curl -s https://api.github.com/repos/fatedier/frp/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
if [ -z "$LATEST_VERSION" ]; then
    echo "无法获取最新版本"
    exit 1
fi

# 下载并解压
DOWNLOAD_URL="https://gh.llkk.cc/https://github.com/fatedier/frp/releases/download/v${LATEST_VERSION}/frp_${LATEST_VERSION}_linux_${FRP_ARCH}.tar.gz"
echo "正在下载: $DOWNLOAD_URL"
wget -q --show-progress $DOWNLOAD_URL -O frp.tar.gz || { echo "下载失败"; exit 1; }
tar zxf frp.tar.gz || { echo "解压失败"; exit 1; }
cd frp_${LATEST_VERSION}_linux_${FRP_ARCH}

# 安装frps
echo "安装frps到/usr/local/bin"
cp frps /usr/local/bin/ && chmod +x /usr/local/bin/frp*

# 创建配置目录
mkdir -p /etc/frp

# 获取用户输入
read -p "请输入服务绑定端口: " BIND_PORT
read -p "请输入开放端口范围最小值: " MIN_PORT
read -p "请输入开放端口范围最大值: " MAX_PORT
read -p "请输入面板端口: " DASH_PORT
# 生成随机字符串函数
generate_random_string() {
    tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w ${1:-32} | head -n 1
}

read -p "请输入面板用户 (留空随机生成): " DASH_USER
read -p "请输入面板密码 (留空随机生成): " DASH_PWD
read -p "请输入节点类型 (例如free/VIP): " NODE_TYPE
read -p "请输入节点名称 (例如HK-1): " NODE_NAME

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

# 验证输入
validate_number() {
    [[ "$1" =~ ^[0-9]+$ ]] || { echo "$2 必须为数字"; exit 1; }
}
validate_number $BIND_PORT "服务绑定端口"
validate_number $MIN_PORT "最小开放端口"
validate_number $MAX_PORT "最大开放端口"
validate_number $DASH_PORT "面板端口"
[ "$MIN_PORT" -le "$MAX_PORT" ] || { echo "最小端口不能大于最大端口"; exit 1; }

# 询问用户是否开启 HTTP/HTTPS 端口
read -p "是否开启 HTTP 端口 (80)? (y/n): " ENABLE_HTTP
read -p "是否开启 HTTPS 端口 (443)? (y/n): " ENABLE_HTTPS

# 生成配置文件
echo "生成配置文件..."
cat > /etc/frp/frps.toml << EOF
bindPort = $BIND_PORT
allowPorts = [
  { start = $MIN_PORT, end = $MAX_PORT },
]
webServer.addr = "0.0.0.0"
webServer.port = $DASH_PORT
webServer.user = "$DASH_USER"
webServer.password = "$DASH_PWD"
[[httpPlugins]]
addr = "https://api.stellarfrp.top"
path = "/$NODE_TYPE"
ops = ["Login"]
[[httpPlugins]]
addr = "https://api.stellarfrp.top"
path = "/check_proxy"
ops = ["NewProxy"]
EOF

# 根据用户选择，添加 HTTP/HTTPS 端口
if [[ "$ENABLE_HTTP" == "y" || "$ENABLE_HTTP" == "Y" ]]; then
    echo "vhostHTTPPort = 80" >> /etc/frp/frps.toml
fi

if [[ "$ENABLE_HTTPS" == "y" || "$ENABLE_HTTPS" == "Y" ]]; then
    echo "vhostHTTPSPort = 443" >> /etc/frp/frps.toml
fi


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
ExecStart=/usr/local/bin/frps -c /etc/frp/frps.toml

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
节点名称: $NODE_NAME
服务绑定端口: $BIND_PORT
开放端口范围: $MIN_PORT-$MAX_PORT
面板访问地址: http://${PUBLIC_IP}:$DASH_PORT
面板用户名: $DASH_USER
面板密码: $DASH_PWD
==================
"

# 安装 ak 探针
echo "开始安装 ak 探针..."
wget --no-check-certificate -O setup-ak.sh https://file.stellarfrp.top/d/StellarFrp/frps/setup-ak.sh
chmod +x setup-ak.sh
./setup-ak.sh "NingMeng123" "wss://statusapi.stellarfrp.top/monitor" "${NODE_NAME}"
echo "安装完成！服务状态："
systemctl status stellarfrps.service

# 清理临时文件
cd ..
rm -rf frp.tar.gz frp_${LATEST_VERSION}_linux_${FRP_ARCH}
rm -f setup-ak.sh