# kiro2api 项目分析

## 项目简介

**kiro2api** 是一个高性能 AI API 代理服务器，统一集成 Anthropic Claude、OpenAI 和 AWS CodeWhisperer。

## 核心特性

1. **Claude Code 原生集成** - 完整 Anthropic API 兼容，支持流式响应、工具调用、多模态图片处理
2. **多账号池管理** - 顺序选择、故障转移、使用监控
3. **双认证方式** - Social 认证（AWS SSO）和 IdC 认证（身份中心）
4. **图片输入支持** - 支持 data URL 格式的 PNG/JPEG 图片

## 支持的模型

- claude-sonnet-4-5-20250929
- claude-sonnet-4-20250514
- claude-3-7-sonnet-20250219
- claude-3-5-haiku-20241022

## API 端点

- `GET /` - 静态首页
- `GET /api/tokens` - Token 池状态（无需认证）
- `GET /v1/models` - 获取可用模型
- `POST /v1/messages` - Anthropic API 兼容接口
- `POST /v1/chat/completions` - OpenAI API 兼容接口

## 技术栈

- Go 1.24.0
- Gin v1.11.0
- bytedance/sonic v1.14.1
- Docker 支持

## 快速开始

```bash
# 编译
go build -o kiro2api main.go

# 配置环境变量
cp .env.example .env

# 启动
./kiro2api
```

或使用 Docker：
```bash
docker run -d -p 8080:8080 \
  -e KIRO_AUTH_TOKEN='[{"auth":"Social","refreshToken":"your_token"}]' \
  -e KIRO_CLIENT_TOKEN="123456" \
  ghcr.io/caidaoli/kiro2api:latest
```

## 项目地址

https://github.com/caidaoli/kiro2api