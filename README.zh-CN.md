# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

为 [new-api](https://github.com/QuantumNous/new-api) 提供 Google [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) 的 Task Plugin。插件转发原生 JSON，支持 `gemini-decision-1` 的文本、图片、音频、视频和 PDF 输入，以及 `noul`、`choice`、`score` 判断。

插件是独立的 `gemini-decisions`，不会替换 new-api 内置的 Google Veo 插件。它使用同步原生接口，不提供 Chat Completions 转换。

## 环境与安装

本项目以 new-api **v1.0.0-rc.49** 的 Task Plugin v1 接口为目标。该插件接口仍在演进；较旧版本可能不支持自定义原生路由或 `retainResult`。

1. 用管理员账号打开 new-api 的 **Task Plugins / 任务插件** 页面，启用任务插件系统，上传仓库根目录的 `plugin.js` 并启用。也可以在上传对话框中使用 [plugin.js 的原始文件地址](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js)。
2. 创建 **Task Plugin** 类型渠道，选择 `gemini-decisions`：
   - Base URL：`https://generativelanguage.googleapis.com`（留空时采用插件默认值）。
   - 密钥：Google AI Studio 的 Gemini API key。
   - 模型：`gemini-decision-1`，并配置允许调用的分组。
3. 为模型配置下面的任务用量计费表达式，再使用 new-api 令牌调用。Google API key 只配置在渠道中，客户端使用 new-api 令牌。

如果上游是另一台**也安装了本插件**的 new-api，可使用 New API 类型渠道并绑定该上游任务插件，渠道密钥填上游网关令牌。

## 计费

要按 Google 返回的输入 token 结算，在模型定价中使用 **任务用量表达式**。例如，按 Google 于 2026-11-12 公布的每百万输入 token 0.10 美元计算：

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin 的表达式直接返回美元金额，不能省略 `/ 1000000`。如果同名模型由多个插件提供，可为 `gemini-decisions::gemini-decision-1` 配置提供商独立表达式。实际价格、分组倍率和免费额度以你的配置及 [Google 定价](https://aistudio.google.com/docs/pricing#gemini-decision)为准。旧式按次定价仍按次收费，不会自动变成按 token 收费。

因为网关无法在请求前可靠计算所有多模态输入的 token 数，插件会按模型上下文上限 **262,144 输入 token** 预扣。成功后使用 `usage.total_input_tokens` 结算；缺失、负数、非整数或超过上限的用量会保留预扣并由宿主记录警告。按上例和分组倍率 1，预扣对应约 0.0262144 美元；余额不足以覆盖预扣时，请求不会发送给 Google。

## REST 调用

将 `NEW_API_BASE_URL` 设为你的网关地址，`NEW_API_TOKEN` 设为该网关令牌：

```bash
curl "$NEW_API_BASE_URL/v1beta/decisions" \
  -H "Authorization: Bearer $NEW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemini-decision-1",
    "input": "The mug arrived cracked.",
    "questions": {
      "damaged": {
        "type": "noul",
        "instructions": "Did the item arrive damaged?"
      }
    }
  }'
```

也可调用带插件前缀的 `/gemini-decisions/v1beta/decisions`。请求的 `input`、`questions` 和响应的 `answers`、`usage` 保留 Google 的 JSON 结构，模型映射仅修改发往上游的 `model`。

## Google Python SDK

安装 `google-genai` 3.2.0 或更高版本。第三方插件路由需要 Bearer 网关令牌，因此还要显式设置 `Authorization`；SDK 默认发送的 `x-goog-api-key` 不能单独完成此插件的网关鉴权。

```python
import os
from google import genai

token = os.environ["NEW_API_TOKEN"]
client = genai.Client(
    api_key=token,
    http_options={
        "base_url": os.environ["NEW_API_BASE_URL"],
        "api_version": "v1beta",
        "headers": {"Authorization": f"Bearer {token}"},
    },
)

result = client.decisions.create(
    model="gemini-decision-1",
    input="The mug arrived cracked.",
    questions={"damaged": {"type": "noul", "instructions": "Did the item arrive damaged?"}},
)
print(result.answers["damaged"].noul, result.usage.total_input_tokens)
```

## 行为与限制

- Google 的限制包括每次 1–1,000 个问题、总上下文 262,144 tokens；问题格式和媒体限制以 [官方文档](https://aistudio.google.com/docs/decisions-api#limits)为准。插件只做基本请求检查，其余校验由 Google 执行。
- `input` 可使用官方支持的文本、多模态内容块、公开或签名 HTTPS URL，以及已有的 Google Files URI。本插件只实现 Decisions，不提供 Files API 上传；请求体大小还受 new-api 宿主限制。
- 结果同步返回。插件设置 `retainResult: false`：new-api 不持久化该调用的 `task.data`，但仍保留任务和计费日志，具体日志内容由宿主配置决定。
- HTTP 错误经过 new-api 宿主处理；原始上游错误细节可能被净化，不能保证原样透传。插件不记录密钥或请求内容。

## 验证

在支持插件 CLI 的 new-api 上运行：

```bash
new-api plugin lint plugin.js
new-api plugin test plugin.js --fixture fixtures/decisions.json
```

宿主集成测试使用本机模拟 Google、SQLite 和真实的 new-api 路由及计费流程，不需要真实密钥。请使用独立的 `v1.0.0-rc.49` 源码检出运行：

```bash
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49
```

安装 `google-genai` 3.2.0 后，还可以运行 `python tests/sdk_compat_test.py`，通过本机模拟服务检查 SDK 的路径、Bearer 请求头及响应解析。此测试独立于宿主集成测试。

## 许可

本项目使用 [Apache License 2.0](LICENSE)。插件接口及用量结算实现参考了 [QuantumNous/new-api-plugins 的 TypeSafe 插件](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe)。
