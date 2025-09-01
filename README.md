# OpenAI兼容API代理 for Z.ai GLM-4.5

这是一个为Z.ai GLM-4.5模型提供OpenAI兼容API接口的代理服务器。

## Render部署

1. Fork这个仓库到你的GitHub账户

2. 在Render上创建新的Web Service：
   - 连接你的GitHub仓库
   - 选择Docker作为环境
   - 设置以下环境变量：
   - `UPSTREAM_TOKEN`: Z.ai 的访问令牌 (必需)
   - `DEFAULT_KEY`: 客户端API密钥 (可选，默认: sk-your-key)
   - `MODEL_NAME`: 显示的模型名称 (可选，默认: GLM-4.5)

   - `PORT`: 服务监听端口 (Render会自动设置)

3. 部署完成后，使用Render提供的URL作为OpenAI API的base_url

## 使用示例

```python
import openai

client = openai.OpenAI(
    api_key="your-api-key",  # 对应 DEFAULT_KEY
    base_url="https://your-app.onrender.com/v1"
)

response = client.chat.completions.create(
    model="GLM-4.5",
    messages=[{"role": "user", "content": "你好"}],
    stream=True
)

for chunk in response:
    print(chunk.choices[0].delta.content or "", end="")
```

## 贡献指南

欢迎提交 Issue 和 Pull Request！请确保：

1. 代码符合 Go 的代码风格
2. 提交前运行测试
3. 更新相关文档

## 许可证

LICENSE

## 免责声明

本项目与 Z.ai 官方无关，使用前请确保遵守 Z.ai 的服务条款。