# Dockerfile: StellarFrpc 独立多架构镜像

# 使用 Alpine 作为基础镜像
FROM docker.io/library/alpine:latest

# 维护者和描述信息
LABEL maintainer="ningmeng@stellarfrp.top"
LABEL description="带StellarFrpc自动下载、多架构兼容并支持环境变量启动的Alpine镜像"
LABEL version="1.0.0"

# 设置工作目录
WORKDIR /app

# 核心步骤：安装必要的工具，并使用 heredoc 生成 /app/entrypoint.sh 脚本
# Buildx 会在 RUN 阶段针对不同架构安装相应的工具链。
RUN apk update && apk add --no-cache bash curl jq tar \
    && echo "--- 正在生成 /app/entrypoint.sh 启动脚本 ---" \
    && cat > /app/entrypoint.sh << 'EOF'
#!/bin/sh

# $TARGETARCH 变量由 Docker Buildx 自动提供，用于判断需要下载的客户端架构。
TARGET_ARCH="${TARGETARCH:-amd64}" 
FRPC_BINARY_NAME="frpc" 
FRPC_PATH="/app/${FRPC_BINARY_NAME}"

BASE_API="https://resources.stellarfrp.top/api/fs/list?path="
BASE_DOWNLOAD_URL="https://resources.stellarfrp.top/api/fs/download?id="

echo "--- StellarFrpc 客户端启动检查 ---"

# 1. 检查 frpc 文件是否已经存在。如果不存在，则执行下载逻辑。
if [ ! -f "$FRPC_PATH" ]; then
    echo "❌ StellarFrpc 客户端不存在，开始自动下载..."
    echo "   -> 目标架构: ${TARGET_ARCH}"

    # 获取下载源列表
    SOURCE_API="${BASE_API}StellarCore/"
    SOURCE_RESPONSE=$(curl -s "$SOURCE_API")
    if [ $? -ne 0 ]; then echo "致命错误: 无法连接到下载源 API。" ; exit 1 ; fi

    # 优先选择 "蓝奏云" 作为下载源
    SELECTED_PROVIDER=$(echo "$SOURCE_RESPONSE" | jq -r '.data.content[] | select(.name == "蓝奏云") | .name' | head -n 1)
    if [ -z "$SELECTED_PROVIDER" ]; then
        SELECTED_PROVIDER=$(echo "$SOURCE_RESPONSE" | jq -r '.data.content[] | select(.is_dir == true) | .name' | head -n 1)
    fi

    if [ -z "$SELECTED_PROVIDER" ]; then echo "致命错误: 未找到可用的下载源目录。" ; exit 1 ; fi

    echo "   -> 选定的下载源: ${SELECTED_PROVIDER}"
    
    # 获取客户端文件列表
    ENCODED_PROVIDER=$(echo "$SELECTED_PROVIDER" | sed 's/ /%20/g')
    CLIENT_API="${BASE_API}StellarCore/${ENCODED_PROVIDER}/frpc"

    CLIENT_RESPONSE=$(curl -s "$CLIENT_API")
    if [ $? -ne 0 ]; then echo "致命错误: 无法连接到客户端文件 API。" ; exit 1 ; fi

    # 解析 JSON，查找匹配 linux_$TARGET_ARCH 的最新文件
    DOWNLOAD_INFO=$(echo "$CLIENT_RESPONSE" | \
        jq -r --arg ARCH "$TARGET_ARCH" '.data.content[] | select(.name | 
        test("StellarFrpc_.*_linux_" + $ARCH + "_.*\\.tar\\.gz$")) | 
        .id + "|" + .name' | head -n 1)

    if [ -z "$DOWNLOAD_INFO" ]; then echo "致命错误: 未找到适用于 linux_${TARGET_ARCH} 的 StellarFrpc 下载链接。" ; exit 1 ; fi

    FILE_ID=$(echo "$DOWNLOAD_INFO" | cut -d '|' -f 1)
    FILE_NAME=$(echo "$DOWNLOAD_INFO" | cut -d '|' -f 2)
    DOWNLOAD_LINK="https://resources.stellarfrp.top/d/StellarCore/${ENCODED_PROVIDER}/frpc/${FILE_NAME}"

    echo "   -> 构造的下载路径: $DOWNLOAD_LINK"

    echo "   -> 匹配到的文件: $FILE_NAME"
    
    # 下载、解压和设置
    TEMP_FILE="/tmp/${FILE_NAME}"
    echo "   -> 正在下载: $FILE_NAME"
    # 使用 -f 选项，遇到 HTTP 错误时会失败并返回非零状态码
    curl -fL -o "$TEMP_FILE" "$DOWNLOAD_LINK"

    if [ $? -ne 0 ]; then echo "致命错误: 下载文件失败或链接不可用，请检查下载链接: $DOWNLOAD_LINK" ; exit 1 ; fi


    echo "   -> 正在解压..."
    # 注意：tar -C /app 可能会在 /app 目录下创建一个新的文件夹，然后文件在文件夹内
    tar -xzf "$TEMP_FILE" -C /app
    
    if [ $? -ne 0 ]; then rm -f "$TEMP_FILE" ; echo "致命错误: 解压文件失败。" ; exit 1 ; fi
    rm -f "$TEMP_FILE"
    
    # *** 新增容错逻辑 ***
    # 查找 /app 目录下是否存在名为 StellarFrpc 的文件，并将其重命名为 frpc
    ACTUAL_FRPC_NAME="StellarFrpc" 
    
    if [ -f "/app/$ACTUAL_FRPC_NAME" ]; then
        echo "   -> 发现可执行文件 $ACTUAL_FRPC_NAME，正在重命名为 ${FRPC_BINARY_NAME}..."
        mv "/app/$ACTUAL_FRPC_NAME" "$FRPC_PATH"
    elif [ -f "/app/${FRPC_BINARY_NAME}" ]; then
        echo "   -> 发现可执行文件 ${FRPC_BINARY_NAME}。"
    else
        # 如果以上两种名字都不存在，则尝试查找解压后的文件夹（例如：StellarFrpc_0.61.2_linux_amd64/StellarFrpc）
        # 为了简洁和避免引入复杂递归，我们先假设它在 /app 根目录，如果失败用户需提供解压后的目录结构
        
        # 再次检查：确保 /app/frpc 存在
        if [ ! -f "$FRPC_PATH" ]; then 
            echo "致命错误: 解压后未在 $FRPC_PATH 找到 ${FRPC_BINARY_NAME} 可执行文件。请检查压缩包内的文件名或目录结构。" 
            # 可以在这里添加一个 ls -l /app 命令来输出目录结构辅助调试
            ls -l /app 
            exit 1 
        fi
    fi

    chmod +x "$FRPC_PATH"
    echo "✅ StellarFrpc 下载、重命名和设置完成。"
else
    echo "✅ StellarFrpc 客户端已存在 ($FRPC_PATH)。"
fi


# 2. 检查启动参数：确保 FRPC_TOKEN 和 FRPC_IDS 环境变量已设置
if [ -z "$FRPC_TOKEN" ] || [ -z "$FRPC_IDS" ]; then
    echo "--- 启动参数缺失 ---"
    echo "警告: 环境变量 FRPC_TOKEN (Token) 和 FRPC_IDS (隧道ID) 必须设置才能启动 StellarFrpc。"
    echo "将执行默认 CMD: /bin/sh..."
    exec "$@" 
fi


# 3. 构造并执行 StellarFrpc 启动命令
echo "--- 启动 StellarFrpc ---"
echo "启动命令: ${FRPC_PATH} -u ***** -t ${FRPC_IDS}"
# 使用 exec 启动客户端，并将环境变量中的值传递给 -u 和 -t 参数。
exec "$FRPC_PATH" -u "$FRPC_TOKEN" -t "$FRPC_IDS"

echo "致命错误: 无法执行 StellarFrpc 客户端。"
exit 1
EOF

# 赋予脚本执行权限
# ！！！重要：先使用 sed 确保脚本是 Linux 换行符格式！！！
RUN sed -i 's/\r//g' /app/entrypoint.sh \
    && chmod +x /app/entrypoint.sh
    
# ENTRYPOINT 定义了容器启动的主逻辑
ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]

# CMD 作为备用默认命令
CMD ["/bin/sh"]