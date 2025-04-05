# StellarFrp Web 控制台

本项目为StellarFrp内网穿透工具的Web控制台，包含frpc（客户端）和frps（服务端）两个界面。

## 技术栈

- Vue 3
- TypeScript
- Vite
- Naive UI (主题：浅蓝色)
- pnpm 包管理器

## 环境要求

- Node.js 16+
- pnpm 9.x 以上版本

检查pnpm版本：
```bash
pnpm --version  # 应该是 9.x 以上版本
```

如果还没有安装pnpm，可以使用以下命令安装：
```bash
npm install -g pnpm
```

## 安装依赖

```bash
# 在web目录下执行
pnpm install
```

## 开发

### 开发frpc客户端界面
```bash
pnpm run dev:frpc
```

### 开发frps服务端界面
```bash
pnpm run dev:frps
```

## 构建

### 构建frpc客户端界面
```bash
pnpm run build:frpc
```

### 构建frps服务端界面
```bash
pnpm run build:frps
```

## 界面特点

- 采用Naive UI组件库，支持浅色/深色主题切换
- 全中文界面，提供友好的用户体验
- 响应式设计，适配不同屏幕尺寸
- 主题色为浅蓝色，视觉效果清新舒适

## 项目结构

```
web/
├── frpc/         # 客户端界面
├── frps/         # 服务端界面
├── Makefile      # 构建脚本
└── pnpm-workspace.yaml
``` 