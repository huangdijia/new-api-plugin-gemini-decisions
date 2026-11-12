# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

A Task Plugin that adds Google's [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) to [new-api](https://github.com/QuantumNous/new-api). It forwards native JSON and supports text, image, audio, video, and PDF inputs for `gemini-decision-1`, along with `noul`, `choice`, and `score` decisions.

The plugin is a standalone `gemini-decisions` plugin and does not replace new-api's built-in Google Veo plugin. It uses the synchronous native API and does not provide Chat Completions conversion.

## Requirements and Installation

This project targets the Task Plugin v1 interface in new-api **v1.0.0-rc.49**. The plugin interface is still evolving; older versions may not support custom native routes or `retainResult`.

1. Sign in to new-api as an administrator, open **Task Plugins**, enable the task plugin system, then upload and enable `plugin.js` from the repository root. You can also use the [raw URL for plugin.js](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js) in the upload dialog.
2. Create a channel of type **Task Plugin** and select `gemini-decisions`:
   - Base URL: `https://generativelanguage.googleapis.com` (the plugin uses this default when left blank).
   - Key: a Gemini API key from Google AI Studio.
   - Model: `gemini-decision-1`. Configure the groups allowed to use it.
3. Configure the task usage billing expression below for the model, then make requests with a new-api token. Configure the Google API key only in the channel; clients use a new-api token.

If the upstream is another new-api instance **with this plugin installed**, you can use a channel of type New API, bind it to the upstream task plugin, and set the channel key to the upstream gateway token.

## Billing

To bill by the input token count returned by Google, use a **task usage expression** in the model's pricing settings. For example, using Google's published price of USD 0.10 per million input tokens as of 2026-11-12:

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin expressions return a US dollar amount directly, so `/ 1000000` must not be omitted. If multiple plugins provide a model with the same name, you can configure a provider-specific expression for `gemini-decisions::gemini-decision-1`. Actual prices, group multipliers, and free allowances depend on your configuration and [Google's pricing](https://aistudio.google.com/docs/pricing#gemini-decision). Legacy per-request pricing continues to charge per request; it does not automatically switch to token-based billing.

Because the gateway cannot reliably calculate token counts for all multimodal inputs before a request, the plugin reserves quota for the model's context limit of **262,144 input tokens**. After a successful request, it settles the charge using `usage.total_input_tokens`. If usage is missing, negative, non-integer, or above the limit, the reservation is retained and the host records a warning. With the expression above and a group multiplier of 1, the reservation is approximately USD 0.0262144. If the balance cannot cover the reservation, the request is not sent to Google.

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

You can also use the plugin-prefixed route `/gemini-decisions/v1beta/decisions`. The request's `input` and `questions` and the response's `answers` and `usage` retain Google's JSON structure. Model mapping only changes the `model` sent upstream.

## Google Python SDK

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
    },
)

result = client.decisions.create(
    model="gemini-decision-1",
    input="The mug arrived cracked.",
    questions={"damaged": {"type": "noul", "instructions": "Did the item arrive damaged?"}},
)
print(result.answers["damaged"].noul, result.usage.total_input_tokens)
```

## Behavior and Limitations

- Google's limits include 1–1,000 questions per request and a total context of 262,144 tokens. Refer to the [official documentation](https://aistudio.google.com/docs/decisions-api#limits) for question formats and media limits. The plugin performs only basic request checks; Google handles the remaining validation.
- `input` can contain officially supported text, multimodal content blocks, public or signed HTTPS URLs, and existing Google Files URIs. This plugin implements only Decisions and does not provide Files API uploads. Request body size is also subject to the new-api host's limits.
- Results are returned synchronously. The plugin sets `retainResult: false`: new-api does not persist `task.data` for the call, but still retains task records and billing logs. The exact log contents depend on the host configuration.
- HTTP errors are processed by the new-api host. Raw upstream error details may be sanitized and are not guaranteed to pass through unchanged. The plugin does not log keys or request contents.

## Verification

Run the following on a new-api installation that supports the plugin CLI:

```bash
new-api plugin lint plugin.js
new-api plugin test plugin.js --fixture fixtures/decisions.json
```

The host integration tests use a local mock Google service, SQLite, and the real new-api routing and billing flow. No real key is required. Run them against a separate source checkout of `v1.0.0-rc.49`:

```bash
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49
```

After installing `google-genai` 3.2.0, you can also run `python tests/sdk_compat_test.py` to check the SDK's request path, Bearer header, and response parsing against a local mock service. This test is independent of the host integration tests.

## License

This project is licensed under the [Apache License 2.0](LICENSE). The plugin interface and usage settlement implementation were informed by the [TypeSafe plugin in QuantumNous/new-api-plugins](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe).
