# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

为 [new-api](https://github.com/QuantumNous/new-api) 提供 Google [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) 的 Task Plugin。插件转发原生 JSON，支持 `gemini-decision-1` 的文本、图片、音频、视频和 PDF 输入，以及 `noul`、`choice`、`score` 判断。

插件是独立的 `gemini-decisions`，不会替换 new-api 内置的 Google Veo 插件。它使用同步原生接口，不提供 Chat Completions 转换。

## 环境与安装

本项目以 new-api **v1.0.0-rc.49** 的 Task Plugin v1 接口为目标。该插件接口仍在演进；较旧版本可能不支持自定义原生路由或 `retainResult`。

1. 用 **Root 超级管理员**账号（上传和启用 Task Plugin 必需）打开 new-api 的 **Task Plugins / 任务插件** 页面，启用任务插件系统，上传仓库根目录的 `plugin.js` 并启用。也可以在上传对话框中使用 [plugin.js 的原始文件地址](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js)。
2. 创建 **Task Plugin** 类型渠道，选择 `gemini-decisions`：
   - Base URL：`https://generativelanguage.googleapis.com`（留空时采用默认值）。无论上游为 Google 还是 new-api，均可填写服务根地址或以 `/v1beta` 结尾的地址，两种写法都支持结尾斜线。
   - 密钥：Google AI Studio 的 Gemini API key。
   - 模型：`gemini-decision-1`，并配置允许调用的分组。
3. 为模型配置下面的任务用量计费表达式，再使用 new-api 令牌调用。Google API key 只配置在渠道中，客户端使用 new-api 令牌。

已有安装请上传并启用插件 **0.2.0**。new-api 不允许不同源码复用同一插件 key/version。现有计费表达式和固定 262,144-token 预扣保持不变。

如果上游是另一台**也安装了本插件**的 new-api，可使用 New API 类型渠道并绑定该上游任务插件，渠道密钥填上游网关令牌。

## 计费

要按 Google 返回的输入 token 结算，在模型定价中使用 **任务用量表达式**。例如，按 Google 于 2026-11-12 公布的每百万输入 token 0.10 美元计算：

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin 的表达式直接返回美元金额，不能省略 `/ 1000000`。如果同名模型由多个插件提供，可为 `gemini-decisions::gemini-decision-1` 配置提供商独立表达式。实际价格、分组倍率和免费额度以你的配置及 [Google 定价](https://aistudio.google.com/docs/pricing#gemini-decision)为准。旧式按次定价仍按次收费，不会自动变成按 token 收费。

因为网关无法在请求前可靠计算所有多模态输入的 token 数，插件会按模型上下文上限 **262,144 输入 token** 预扣。成功后使用 `usage.total_input_tokens` 结算；缺失、负数、非整数或超过上限的用量会保留预扣并由宿主记录警告。按上例和分组倍率 1，预扣对应约 0.0262144 美元；余额不足以覆盖预扣时，请求不会发送给 Google。

Google 说明：返回 **400 或 500** 的失败请求不收取 token 费用，但仍计入 quota。这是 Google 上游的计费政策，与网关的预扣和结算行为不同。参见[计费 FAQ](https://aistudio.google.com/docs/billing#am-i-charged-for-failed-requests)。

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

- Google 限制每次 1–1,000 个问题、总上下文 262,144 tokens。问题 key 必须为 **1–64 个 ASCII 字符**，仅允许字母、数字、`_`、`-`。支持的 type 为 `noul`、`choice`、`score`；`choice` 和 `score` 均要求 **2–255 项**，每个 `score` 项必须为非空字符串。`instructions` 必填，可为字符串、TextContent 对象或非空的 TextContent 对象数组。插件明确验证这些要求，其余格式限制由 Google 校验。参见[官方限制](https://aistudio.google.com/docs/decisions-api#limits)。
- `input` 支持 Google 的文本、多模态内容块、公开或签名的外部 HTTPS URL，以及已有的 Google Files URI。Google 的[输入限制](https://aistudio.google.com/docs/decisions-api#input)为：**每请求 inline data 最多 100 MB**、**PDF 最多 50 MB**、**每个外部 HTTPS URL 最多 15 MB**。较大文件请先直连 Google Files API 上传，再使用返回的 URI。本插件不实现 Files API 上传；任意外部 URL 不能绕过 15 MB 限制。
- new-api **v1.0.0-rc.49** 的 `MAX_REQUEST_BODY_MB` 默认 **128 MiB**，限制的是完整解压后的 JSON 请求体。Base64 比原始文件约增大三分之一，还需加上 JSON 开销。例如，**100 MiB** 原始文件仅编码后就超过 128 MiB，尚未计入 JSON 开销。Google 的 100 MB inline 限制与宿主 128 MiB 请求体限制的单位和范围不同，不能笼统认定 100 MB 原始文件必然超过宿主限制。
- 结果同步返回。new-api **v1.0.0-rc.49** 的 `RELAY_TIMEOUT` 默认 **0**，即不设置出站 HTTP 客户端的总超时；`RELAY_RESPONSE_HEADER_TIMEOUT` 默认 **1800 秒**，限制等待响应头的时间。客户端或反向代理的超时仍可能生效。
- 对宿主成功读取并交给插件的上游 **2xx** 响应，答案缺项或答案 type 不符时仍按 usage 结算并保留上游答案；如果没有 `answers` 对象，则先结算用量再返回 **502**。用量异常时按上文保留预扣，不能仅凭 502 判断已退款。宿主将响应交给插件前发生的错误仍可能触发退款。
- 多层 new-api 网关串联时，上游网关可能先结算用量，再因缺少 `answers` 对象返回 502；下游 v1.0.0-rc.49 网关遇到该非 2xx 响应会退还调用方费用，因此下游运营方可能承担上游费用。
- 插件设置 `retainResult: false`：new-api 不持久化该调用的 `task.data`，但仍保留任务和计费日志，具体日志内容由宿主配置决定。
- HTTP 错误由 new-api 宿主处理。部分上游 4xx JSON 错误结构可保留，但不保证逐字节原样透传，5xx 细节可能被净化。插件不记录密钥或请求内容。

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

[GitHub Actions 工作流](.github/workflows/test.yml)使用固定版本的 new-api 执行这些检查。以上检查使用 fixture 或本机模拟服务，不代表已成功调用真实 Google API。

## 许可

本项目使用 [Apache License 2.0](LICENSE)。插件接口及用量结算实现参考了 [QuantumNous/new-api-plugins 的 TypeSafe 插件](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe)。
