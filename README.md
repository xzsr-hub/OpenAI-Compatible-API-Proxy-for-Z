# OpenAI-Compatible API Proxy for Z.ai

## 项目简介

这是一个为 Z.ai 提供 OpenAI API 兼容接口的代理服务，允许开发者通过标准的 OpenAI API 格式访问 Z.ai 的 GLM-4.5 模型。

## 主要特性

- **OpenAI API 兼容**：支持标准的 `/v1/chat/completions` 和 `/v1/models` 端点
- **流式响应支持**：完整实现 Server-Sent Events (SSE) 流式传输
- **思考内容处理**：提供多种策略处理模型的思考过程（`<details>` 标签）
- **匿名会话支持**：可选使用匿名 token 避免共享对话历史
- **调试模式**：详细的请求/响应日志记录
- **CORS 支持**：内置跨域资源共享支持

## 使用场景

- 将 Z.ai 集成到支持 OpenAI API 的应用程序中
- 开发需要同时使用多个 AI 服务的应用
- 测试和评估 GLM-4.5 模型的能力

## 快速开始

1. 克隆仓库：
   ```bash
   git clone https://github.com/kbykb/OpenAI-Compatible-API-Proxy-for-Z.git
   cd OpenAI-Compatible-API-Proxy-for-Z
   ```

2. 修改配置（可选）：
   编辑 `main.go` 中的常量：
   - `UPSTREAM_URL`: Z.ai 的上游 API 地址
   - `DEFAULT_KEY`: 你的 API 密钥
   - `UPSTREAM_TOKEN`: Z.ai 的访问令牌
   - `MODEL_NAME`: 显示的模型名称
   - `PORT`: 服务监听端口

3. 运行服务：
   ```bash
   go run main.go
   ```

4. 使用 OpenAI 客户端库调用：
   ```python
   import openai

   client = openai.OpenAI(
       base_url="http://localhost:8080/v1",
       api_key="your-api-key"
   )

   response = client.chat.completions.create(
       model="GLM-4.5",
       messages=[{"role": "user", "content": "你好"}],
       stream=True
   )

   for chunk in response:
       print(chunk.choices[0].delta.content or "", end="")
   ```

## 配置选项

| 配置项 | 描述 | 默认值 |
|--------|------|--------|
| `UPSTREAM_URL` | Z.ai 的上游 API 地址 | `https://chat.z.ai/api/chat/completions` |
| `DEFAULT_KEY` | 下游客户端鉴权 key | `sk-your-key` |
| `UPSTREAM_TOKEN` | 上游 API 的 token | (示例 token) |
| `MODEL_NAME` | 显示的模型名称 | `GLM-4.5` |
| `PORT` | 服务监听端口 | `:8080` |
| `DEBUG_MODE` | 调试模式开关 | `true` |
| `THINK_TAGS_MODE` | 思考内容处理策略 | `strip` (可选: `think`, `raw`) |
| `ANON_TOKEN_ENABLED` | 是否使用匿名 token | `true` |

## 贡献指南

欢迎提交 Issue 和 Pull Request！请确保：
1. 代码符合 Go 的代码风格
2. 提交前运行测试
3. 更新相关文档

## 许可证

LICENSE

## 免责声明

本项目与 Z.ai 官方无关，使用前请确保遵守 Z.ai 的服务条款。
