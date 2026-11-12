# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

為 [new-api](https://github.com/QuantumNous/new-api) 提供 Google [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) 的 Task Plugin。外掛轉送原生 JSON，支援 `gemini-decision-1` 的文字、圖片、音訊、影片和 PDF 輸入，以及 `noul`、`choice`、`score` 判斷。

本外掛是獨立的 `gemini-decisions`，不會取代 new-api 內建的 Google Veo 外掛。它使用同步原生介面，不提供 Chat Completions 轉換。

## 環境與安裝

本專案以 new-api **v1.0.0-rc.49** 的 Task Plugin v1 介面為目標。該外掛介面仍在演進；較舊版本可能不支援自訂原生路由或 `retainResult`。

1. 使用管理員帳號開啟 new-api 的 **Task Plugins / 任務外掛** 頁面，啟用任務外掛系統，上傳儲存庫根目錄的 `plugin.js` 並啟用。也可以在上傳對話框中使用 [plugin.js 的原始檔案網址](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js)。
2. 建立 **Task Plugin** 類型的通道，選擇 `gemini-decisions`：
   - Base URL：`https://generativelanguage.googleapis.com`（留空時採用外掛預設值）。
   - 金鑰：Google AI Studio 的 Gemini API key。
   - 模型：`gemini-decision-1`，並設定允許呼叫的分組。
3. 為模型設定下方的任務用量計費表達式，再使用 new-api 權杖呼叫。Google API key 只設定在通道中，用戶端使用 new-api 權杖。

如果上游是另一台**也安裝了本外掛**的 new-api，可使用 New API 類型通道並綁定該上游任務外掛，通道金鑰填入上游閘道權杖。

## 計費

若要按照 Google 回傳的輸入 token 結算，請在模型定價中使用 **任務用量表達式**。例如，依 Google 於 2026-11-12 公布的每百萬輸入 token 0.10 美元計算：

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin 的表達式直接回傳美元金額，不能省略 `/ 1000000`。如果同名模型由多個外掛提供，可為 `gemini-decisions::gemini-decision-1` 設定供應商獨立表達式。實際價格、分組倍率和免費額度以你的設定及 [Google 定價](https://aistudio.google.com/docs/pricing#gemini-decision)為準。舊式按次定價仍按次收費，不會自動變成按 token 收費。

由於閘道無法在請求前可靠計算所有多模態輸入的 token 數，外掛會按模型上下文上限 **262,144 輸入 token** 預扣。成功後使用 `usage.total_input_tokens` 結算；用量缺失、為負數、非整數或超過上限時，會保留預扣並由宿主記錄警告。以上例和分組倍率 1 計算，預扣約為 0.0262144 美元；餘額不足以支付預扣時，請求不會傳送給 Google。

## REST 呼叫

將 `NEW_API_BASE_URL` 設為你的閘道網址，`NEW_API_TOKEN` 設為該閘道權杖：

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

也可呼叫帶外掛前綴的 `/gemini-decisions/v1beta/decisions`。請求的 `input`、`questions` 和回應的 `answers`、`usage` 保留 Google 的 JSON 結構，模型映射僅修改傳送至上游的 `model`。

## Google Python SDK

安裝 `google-genai` 3.2.0 或更高版本。第三方外掛路由需要 Bearer 閘道權杖，因此還要明確設定 `Authorization`；SDK 預設傳送的 `x-goog-api-key` 無法單獨完成此外掛的閘道驗證。

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

## 行為與限制

- Google 的限制包括每次 1–1,000 個問題、總上下文 262,144 tokens；問題格式和媒體限制以[官方文件](https://aistudio.google.com/docs/decisions-api#limits)為準。外掛只做基本請求檢查，其餘驗證由 Google 執行。
- `input` 可使用官方支援的文字、多模態內容區塊、公開或簽署的 HTTPS URL，以及既有的 Google Files URI。本外掛只實作 Decisions，不提供 Files API 上傳；請求主體大小也受 new-api 宿主限制。
- 結果同步回傳。外掛設定 `retainResult: false`：new-api 不會持久化該次呼叫的 `task.data`，但仍保留任務和計費日誌，具體日誌內容由宿主設定決定。
- HTTP 錯誤由 new-api 宿主處理；原始上游錯誤詳情可能經過淨化，無法保證原樣轉送。外掛不記錄金鑰或請求內容。

## 驗證

在支援外掛 CLI 的 new-api 上執行：

```bash
new-api plugin lint plugin.js
new-api plugin test plugin.js --fixture fixtures/decisions.json
```

宿主整合測試使用本機模擬 Google、SQLite 和實際的 new-api 路由及計費流程，不需要真實金鑰。請使用獨立的 `v1.0.0-rc.49` 原始碼檢出執行：

```bash
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49
```

安裝 `google-genai` 3.2.0 後，還可以執行 `python tests/sdk_compat_test.py`，透過本機模擬服務檢查 SDK 的路徑、Bearer 請求標頭及回應解析。此測試獨立於宿主整合測試。

## 授權

本專案使用 [Apache License 2.0](LICENSE)。外掛介面及用量結算實作參考了 [QuantumNous/new-api-plugins 的 TypeSafe 外掛](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe)。
