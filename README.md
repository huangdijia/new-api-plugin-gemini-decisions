# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

A Task Plugin that adds Google's [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) to [new-api](https://github.com/QuantumNous/new-api). It forwards native JSON and supports text, image, audio, video, and PDF inputs for `gemini-decision-1`, along with `noul`, `choice`, and `score` decisions.

The plugin is a standalone `gemini-decisions` plugin and does not replace new-api's built-in Google Veo plugin. It uses the synchronous native API and does not provide Chat Completions conversion.

## Requirements and Installation

Plugin **0.3.0** supports the Task Plugin v1 interface in official new-api **v1.0.0-rc.47** and **v1.0.0-rc.49**. Real host integration tests cover both releases. Compatibility with other releases or customized forks is not guaranteed.

1. Sign in to new-api as a **Root super administrator** (required to upload and enable Task Plugins), open **Task Plugins**, enable the task plugin system, then upload and enable `plugin.js` from the repository root. You can also use the [raw URL for plugin.js](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js) in the upload dialog.
2. Create a channel of type **Task Plugin** and select `gemini-decisions`:
   - Base URL: `https://generativelanguage.googleapis.com` (the default when blank). For Google or an upstream new-api instance, use the service root URL or one ending in `/v1beta`; both forms accept a trailing slash.
   - Key: a Gemini API key from Google AI Studio.
   - Model: `gemini-decision-1`. Configure the groups allowed to use it.
3. Configure the task usage billing expression below for the model, then make requests with a new-api token. Configure the Google API key only in the channel; clients use a new-api token.

For an existing installation, upload and enable plugin **0.3.0**. new-api does not allow different source code to reuse the same plugin key/version. Existing billing expressions and the fixed 262,144-token reservation remain unchanged.

If the upstream is another new-api instance **with this plugin installed**, you can use a channel of type New API, bind it to the upstream task plugin, and set the channel key to the upstream gateway token.

## Billing

To bill by the input token count returned by Google, use a **task usage expression** in the model's pricing settings. For example, using Google's published price of USD 0.10 per million input tokens as of 2026-11-12:

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin expressions return a US dollar amount directly, so `/ 1000000` must not be omitted. If multiple plugins provide a model with the same name, you can configure a provider-specific expression for `gemini-decisions::gemini-decision-1`. Actual prices, group multipliers, and free allowances depend on your configuration and [Google's pricing](https://aistudio.google.com/docs/pricing#gemini-decision). Legacy per-request pricing continues to charge per request; it does not automatically switch to token-based billing.

Because the gateway cannot reliably calculate token counts for all multimodal inputs before a request, the plugin reserves quota for the model's context limit of **262,144 input tokens**. After a successful request, it settles the charge using `usage.total_input_tokens`. If usage is missing, negative, non-integer, or above the limit, the reservation is retained and the host records a warning. With the expression above and a group multiplier of 1, the reservation is approximately USD 0.0262144. If the balance cannot cover the reservation, the request is not sent to Google.

Google states that failed requests returning **400 or 500** are not charged for tokens, but still count toward quota. This is Google's upstream billing policy, distinct from the gateway's reservation and settlement behavior. See the [billing FAQ](https://aistudio.google.com/docs/billing#am-i-charged-for-failed-requests).

## REST Requests

Set `NEW_API_BASE_URL` to your gateway URL and `NEW_API_TOKEN` to the token for that gateway:

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

The plugin explicitly registers both `/v1beta/decisions` and `/gemini-decisions/v1beta/decisions` on both supported releases. The request's `input` and `questions` and the response's `answers` and `usage` retain Google's JSON structure. Model mapping only changes the `model` sent upstream.

## Google Python SDK

The SDK retries 5xx responses automatically by default. The example sets `retry_options.attempts` to `1` because rc.49 can return 502 after usage has already been settled; retrying that request may charge it again. Check gateway logs and charges before retrying. Other 502 responses may occur before settlement.

On rc.47, a missing `answers` object can arrive with HTTP 200 after usage has been settled. SDK callers must check for missing or empty `answers`, a missing requested answer, and missing `usage` before reading them; do not rely on HTTP success alone. The example below includes these checks.

Install `google-genai` 3.2.0 or later. Third-party plugin routes require a Bearer gateway token, so you must also explicitly set `Authorization`. The `x-goog-api-key` header sent by the SDK by default is not sufficient on its own for gateway authentication with this plugin.

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
        "retry_options": {"attempts": 1},
    },
)

result = client.decisions.create(
    model="gemini-decision-1",
    input="The mug arrived cracked.",
    questions={"damaged": {"type": "noul", "instructions": "Did the item arrive damaged?"}},
)
if not result.answers or result.answers.get("damaged") is None:
    raise RuntimeError("Decision response is missing the damaged answer")
if result.usage is None:
    raise RuntimeError("Decision response is missing usage")
print(result.answers["damaged"].noul, result.usage.total_input_tokens)
```

## Behavior and Limitations

- Google allows 1–1,000 questions per request and a total context of 262,144 tokens. Question keys must contain **1–64 ASCII characters** using only letters, digits, `_`, or `-`. Supported types are `noul`, `choice`, and `score`; `choice` and `score` each require **2–255 entries**, and each `score` entry must be a nonempty string. `instructions` is required and accepts a string, a TextContent object, or a nonempty array of TextContent objects. The plugin validates these requirements; Google validates the remaining format constraints. See the [official limits](https://aistudio.google.com/docs/decisions-api#limits).
- `input` supports Google's text, multimodal content blocks, public or signed external HTTPS URLs, and existing Google Files URIs. Google's [input limits](https://aistudio.google.com/docs/decisions-api#input) allow up to **100 MB of inline data per request**, **50 MB for PDF**, and **15 MB per external HTTPS URL**. For larger files, upload directly to the Google Files API first and use the resulting URI. This plugin does not implement Files API uploads; an arbitrary external URL does not bypass the 15 MB limit.
- In new-api **v1.0.0-rc.49**, `MAX_REQUEST_BODY_MB` defaults to **128 MiB** and limits the entire decompressed JSON request body. Base64 adds approximately one third to the raw file size, plus JSON overhead. For example, a **100 MiB** raw file exceeds 128 MiB after Base64 encoding, even before JSON overhead. Google's 100 MB inline limit and the host's 128 MiB body limit use different units and scopes; a 100 MB raw file does not necessarily exceed the host limit.
- Results are synchronous. In new-api **v1.0.0-rc.49**, `RELAY_TIMEOUT` defaults to **0**, which sets no overall timeout on the outbound HTTP client. `RELAY_RESPONSE_HEADER_TIMEOUT` defaults to **1800 seconds** and limits the wait for response headers. Client or reverse-proxy timeouts may still apply.
- For upstream **2xx** responses successfully read by the host and passed to the plugin, both releases return normal `answers` and `usage` as native JSON. Missing answer entries or mismatched answer types do not prevent usage settlement; the plugin preserves the upstream answers. If there is no `answers` object, it settles usage first. On **rc.49**, it then returns **HTTP 502** with `{"error":{"code":502,"message":"Decision response is missing answers"}}`. On **rc.47**, the host cannot change the status at this stage, so it returns **HTTP 200** with the same required `error` object and, if the upstream `usage` is an object, that object copied unchanged as an optional `usage` field. Clients must detect the missing `answers` rather than treating HTTP 200 as success. Invalid usage retains the reservation as described above; an error response alone does not imply a refund. Failures before the host passes the response to the plugin may still cause a refund.
- When new-api gateways are chained, an upstream rc.49 gateway may settle usage and then return 502 for a missing `answers` object. A downstream rc.49 gateway refunds its caller on that non-2xx response, so the downstream operator may bear the upstream charge. An upstream rc.47 gateway instead returns HTTP 200 with the error body described above. Its optional `usage` lets a downstream gateway running this plugin settle using a valid `usage.total_input_tokens`; if usage is missing or invalid, that downstream gateway retains the reservation.
- The plugin sets `retainResult: false`: new-api does not persist `task.data` for the call, but still retains task records and billing logs. The exact log contents depend on the host configuration.
- HTTP errors are processed by the new-api host. On both releases, upstream 400 and 5xx errors retain the host-selected HTTP status through `native.error`. Some upstream 4xx JSON error envelopes can be preserved, but byte-for-byte passthrough is not guaranteed, and 5xx details may be sanitized. The plugin does not log keys or request contents.
- The plugin uses the presence of `ctx.requestHeaders` only as a host capability feature distinguishing these two official releases. This does not guarantee compatibility with customized forks.

## Verification

Run the following on a new-api installation that supports the plugin CLI:

```bash
new-api plugin lint plugin.js
new-api plugin test plugin.js --fixture fixtures/decisions.json
```

The host integration tests use a local mock Google service, SQLite, and the real new-api routing and billing flow. No real key is required. Provide the official source directory for the corresponding release and Python 3. The script uses a Go overlay and does not modify the host source checkout; Python 3 generates the overlay:

```bash
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.47 rc.47
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49 rc.49
```

The second argument selects the expected host behavior and defaults to `rc.49` when omitted. After installing `google-genai==3.2.0`, set `GEMINI_DECISIONS_SDK_PYTHON` to that Python interpreter to run SDK requests through the real host as part of integration testing:

```bash
GEMINI_DECISIONS_SDK_PYTHON=/path/to/venv/bin/python \
  tests/run-integration.sh /path/to/new-api-v1.0.0-rc.47 rc.47
```

You can also run `python tests/sdk_compat_test.py` independently to check the SDK's request path, Bearer header, and response parsing against a local mock service.

The [GitHub Actions workflow](.github/workflows/test.yml) runs lint, fixtures, real host integration, and Google Python SDK 3.2.0 checks for both releases at pinned official revisions. These checks use fixtures or local upstream mocks; they do not demonstrate a successful request to the live Google API.

## License

This project is licensed under the [Apache License 2.0](LICENSE). The plugin interface and usage settlement implementation were informed by the [TypeSafe plugin in QuantumNous/new-api-plugins](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe).
