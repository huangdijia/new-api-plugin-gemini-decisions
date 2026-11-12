# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

為 [new-api](https://github.com/QuantumNous/new-api) 提供 Google [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) 的 Task Plugin。外掛轉送原生 JSON，支援 `gemini-decision-1` 的文字、圖片、音訊、影片和 PDF 輸入，以及 `noul`、`choice`、`score` 判斷。

本外掛是獨立的 `gemini-decisions`，不會取代 new-api 內建的 Google Veo 外掛。它使用同步原生介面，不提供 Chat Completions 轉換。

## 環境與安裝

外掛 **0.3.0** 支援官方 new-api **v1.0.0-rc.47** 和 **v1.0.0-rc.49** 的 Task Plugin v1 介面，並以真實宿主整合測試涵蓋這兩個版本。其他版本或客製 fork 的相容性不予保證。

1. 使用 **Root 超級管理員**帳號（上傳和啟用 Task Plugin 必需）開啟 new-api 的 **Task Plugins / 任務外掛** 頁面，啟用任務外掛系統，上傳儲存庫根目錄的 `plugin.js` 並啟用。也可以在上傳對話框中使用 [plugin.js 的原始檔案網址](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js)。
2. 建立 **Task Plugin** 類型的通道，選擇 `gemini-decisions`：
   - Base URL：`https://generativelanguage.googleapis.com`（留空時採用預設值）。無論上游為 Google 或 new-api，皆可填寫服務根網址或以 `/v1beta` 結尾的網址，兩種寫法都支援結尾斜線。
   - 金鑰：Google AI Studio 的 Gemini API key。
   - 模型：`gemini-decision-1`，並設定允許呼叫的分組。
3. 為模型設定下方的任務用量計費表達式，再使用 new-api 權杖呼叫。Google API key 只設定在通道中，用戶端使用 new-api 權杖。

既有安裝請上傳並啟用外掛 **0.3.0**。new-api 不允許不同原始碼重複使用相同外掛 key/version。既有計費表達式和固定 262,144-token 預扣維持不變。

如果上游是另一台**也安裝了本外掛**的 new-api，可使用 New API 類型通道並綁定該上游任務外掛，通道金鑰填入上游閘道權杖。

## 計費

若要按照 Google 回傳的輸入 token 結算，請在模型定價中使用 **任務用量表達式**。例如，依 Google 於 2026-11-12 公布的每百萬輸入 token 0.10 美元計算：

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin 的表達式直接回傳美元金額，不能省略 `/ 1000000`。如果同名模型由多個外掛提供，可為 `gemini-decisions::gemini-decision-1` 設定供應商獨立表達式。實際價格、分組倍率和免費額度以你的設定及 [Google 定價](https://aistudio.google.com/docs/pricing#gemini-decision)為準。舊式按次定價仍按次收費，不會自動變成按 token 收費。

由於閘道無法在請求前可靠計算所有多模態輸入的 token 數，外掛會按模型上下文上限 **262,144 輸入 token** 預扣。成功後使用 `usage.total_input_tokens` 結算；用量缺失、為負數、非整數或超過上限時，會保留預扣並由宿主記錄警告。以上例和分組倍率 1 計算，預扣約為 0.0262144 美元；餘額不足以支付預扣時，請求不會傳送給 Google。

Google 說明：回傳 **400 或 500** 的失敗請求不收取 token 費用，但仍計入 quota。這是 Google 上游的計費政策，與閘道的預扣和結算行為不同。參見[計費 FAQ](https://aistudio.google.com/docs/billing#am-i-charged-for-failed-requests)。

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

外掛在兩個支援版本中皆明確註冊 `/v1beta/decisions` 和 `/gemini-decisions/v1beta/decisions`。請求的 `input`、`questions` 和回應的 `answers`、`usage` 保留 Google 的 JSON 結構，模型映射僅修改傳送至上游的 `model`。

## Google Python SDK

SDK 預設會自動重試 5xx 回應。下例設定 `retry_options.attempts` 為 `1`，因為 rc.49 可能在結算用量後回傳 502，自動重試可能導致重複收費。用戶端重試前應核對閘道日誌與扣費紀錄；其他 502 也可能發生在結算之前。

rc.47 可能在結算用量後，以 HTTP 200 回傳缺少 `answers` 的回應。SDK 呼叫端讀取結果前，必須檢查 `answers` 是否缺失或為空、所需問題的答案是否缺失，以及 `usage` 是否缺失，不能僅憑 HTTP 成功判斷；下例已加入這些檢查。

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

## 行為與限制

- Google 限制每次 1–1,000 個問題、總上下文 262,144 tokens。問題 key 必須為 **1–64 個 ASCII 字元**，僅允許字母、數字、`_`、`-`。支援的 type 為 `noul`、`choice`、`score`；`choice` 和 `score` 均要求 **2–255 項**，每個 `score` 項必須為非空字串。`instructions` 必填，可為字串、TextContent 物件或非空的 TextContent 物件陣列。外掛明確驗證這些要求，其餘格式限制由 Google 驗證。參見[官方限制](https://aistudio.google.com/docs/decisions-api#limits)。
- `input` 支援 Google 的文字、多模態內容區塊、公開或簽署的外部 HTTPS URL，以及既有的 Google Files URI。Google 的[輸入限制](https://aistudio.google.com/docs/decisions-api#input)為：**每請求 inline data 最多 100 MB**、**PDF 最多 50 MB**、**每個外部 HTTPS URL 最多 15 MB**。較大檔案請先直接呼叫 Google Files API 上傳，再使用回傳的 URI。本外掛不實作 Files API 上傳；任意外部 URL 不能繞過 15 MB 限制。
- new-api **v1.0.0-rc.49** 的 `MAX_REQUEST_BODY_MB` 預設 **128 MiB**，限制的是完整解壓後的 JSON 請求主體。Base64 比原始檔案約增大三分之一，還需加上 JSON 開銷。例如，**100 MiB** 原始檔案僅編碼後就超過 128 MiB，尚未計入 JSON 開銷。Google 的 100 MB inline 限制與宿主 128 MiB 請求主體限制的單位和範圍不同，不能籠統認定 100 MB 原始檔案必然超過宿主限制。
- 結果同步回傳。new-api **v1.0.0-rc.49** 的 `RELAY_TIMEOUT` 預設 **0**，即不設定出站 HTTP 用戶端的總逾時；`RELAY_RESPONSE_HEADER_TIMEOUT` 預設 **1800 秒**，限制等待回應標頭的時間。用戶端或反向代理的逾時仍可能生效。
- 對宿主成功讀取並交給外掛的上游 **2xx** 回應，兩個版本皆以原生 JSON 回傳正常的 `answers`、`usage`。答案缺項或答案 type 不符時仍按 usage 結算並保留上游答案。如果沒有 `answers` 物件，則先結算用量。**rc.49 隨後回傳 HTTP 502**，回應主體為 `{"error":{"code":502,"message":"Decision response is missing answers"}}`。**rc.47** 的宿主在此階段無法修改狀態，因此回傳 **HTTP 200**，回應主體必含相同的 `error` 物件；若上游 `usage` 為物件，還會將其原樣複製到可選的 `usage` 欄位。用戶端必須檢查 `answers` 是否存在，不能將 HTTP 200 直接視為成功。用量異常時依上文保留預扣，不能僅憑錯誤回應判斷已退款。宿主將回應交給外掛前發生的錯誤仍可能觸發退款。
- 多層 new-api 閘道串接時，上游 rc.49 閘道可能先結算用量，再因缺少 `answers` 物件回傳 502；下游 rc.49 閘道遇到該非 2xx 回應會退還呼叫方費用，因此下游營運方可能承擔上游費用。上游 rc.47 閘道則回傳 HTTP 200 和上述錯誤回應主體，其中可選的 `usage` 可供執行本外掛的下游閘道按合法的 `usage.total_input_tokens` 結算；用量缺失或不合法時，下游閘道保留預扣。
- 外掛設定 `retainResult: false`：new-api 不會持久化該次呼叫的 `task.data`，但仍保留任務和計費日誌，具體日誌內容由宿主設定決定。
- HTTP 錯誤由 new-api 宿主處理。兩個版本的上游 400 和 5xx 錯誤透過 `native.error` 回傳時，皆保留宿主選擇的 HTTP 狀態。部分上游 4xx JSON 錯誤結構可保留，但不保證逐位元組原樣轉送，5xx 詳情可能經過淨化。外掛不記錄金鑰或請求內容。
- 外掛僅將 `ctx.requestHeaders` 是否存在用作區分這兩個官方版本的宿主能力特徵，不保證客製 fork 的相容性。

## 驗證

在支援外掛 CLI 的 new-api 上執行：

```bash
new-api plugin lint plugin.js
new-api plugin test plugin.js --fixture fixtures/decisions.json
```

宿主整合測試使用本機模擬 Google、SQLite 和實際的 new-api 路由及計費流程，不需要真實金鑰。請提供對應官方版本的原始碼目錄及 Python 3。指令碼使用 Go overlay，不修改宿主原始碼檢出；Python 3 用於產生 overlay：

```bash
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.47 rc.47
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49 rc.49
```

第二個參數指定預期的宿主行為，省略時預設為 `rc.49`。安裝 `google-genai==3.2.0` 後，將 `GEMINI_DECISIONS_SDK_PYTHON` 設為該 Python 解譯器路徑，即可在整合測試中透過真實宿主執行 SDK 請求：

```bash
GEMINI_DECISIONS_SDK_PYTHON=/path/to/venv/bin/python \
  tests/run-integration.sh /path/to/new-api-v1.0.0-rc.47 rc.47
```

也可獨立執行 `python tests/sdk_compat_test.py`，透過本機模擬服務檢查 SDK 的路徑、Bearer 請求標頭及回應解析。

[GitHub Actions 工作流程](.github/workflows/test.yml)使用兩個官方版本的固定 revision，涵蓋 lint、fixtures、真實宿主整合及 Google Python SDK 3.2.0 測試。以上檢查使用 fixture 或本機模擬上游，不代表已成功呼叫真實 Google API。

## 授權

本專案使用 [Apache License 2.0](LICENSE)。外掛介面及用量結算實作參考了 [QuantumNous/new-api-plugins 的 TypeSafe 外掛](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe)。
