# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

[new-api](https://github.com/QuantumNous/new-api) で Google の [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) を利用するための Task Plugin です。ネイティブの JSON をそのまま転送し、`gemini-decision-1` のテキスト、画像、音声、動画、PDF 入力と、`noul`、`choice`、`score` による判定に対応します。

本プラグインは独立した `gemini-decisions` プラグインであり、new-api 組み込みの Google Veo プラグインを置き換えるものではありません。同期型のネイティブインターフェースを使用し、Chat Completions への変換は提供しません。

## 動作環境とインストール

本プロジェクトは、new-api **v1.0.0-rc.49** の Task Plugin v1 インターフェースを対象としています。このプラグインインターフェースは現在も開発が進められており、古いバージョンではカスタムのネイティブルートや `retainResult` に対応していない場合があります。

1. 管理者アカウントで new-api の **Task Plugins / タスクプラグイン** ページを開き、タスクプラグインシステムを有効にします。リポジトリのルートにある `plugin.js` をアップロードし、プラグインを有効にしてください。アップロードダイアログで [plugin.js の Raw ファイル URL](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js) を指定することもできます。
2. **Task Plugin** タイプのチャネルを作成し、`gemini-decisions` を選択します。
   - Base URL：`https://generativelanguage.googleapis.com`（空欄の場合はプラグインの既定値を使用）。
   - キー：Google AI Studio の Gemini API key。
   - モデル：`gemini-decision-1`。呼び出しを許可するグループも設定してください。
3. モデルに以下のタスク使用量に基づく課金式を設定し、new-api トークンで呼び出します。Google API key はチャネルにのみ設定し、クライアントは new-api トークンを使用します。

上流が**本プラグインもインストール済み**の別の new-api である場合は、New API タイプのチャネルを使用して上流のタスクプラグインを紐付け、チャネルのキーに上流ゲートウェイのトークンを設定できます。

## 課金

Google が返す入力トークン数に基づいて精算するには、モデルの料金設定で **タスク使用量式** を使用します。例えば、Google が 2026-11-12 に公開した入力 100 万トークンあたり 0.10 米ドルで計算する場合は、次の式を使用します。

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin の式は米ドル建ての金額を直接返すため、`/ 1000000` を省略してはいけません。同じ名前のモデルを複数のプラグインが提供している場合は、`gemini-decisions::gemini-decision-1` にプロバイダー別の式を設定できます。実際の料金、グループ倍率、無料枠は、ご自身の設定と [Google の料金表](https://aistudio.google.com/docs/pricing#gemini-decision)に従います。従来のリクエスト単位の料金設定は引き続きリクエスト単位で課金され、トークン単位の課金へ自動的に切り替わることはありません。

ゲートウェイはリクエスト送信前にすべてのマルチモーダル入力のトークン数を確実に計算できないため、プラグインはモデルのコンテキスト上限である **262,144 入力トークン** 分を事前に差し引きます。成功後は `usage.total_input_tokens` に基づいて精算します。使用量が欠落している場合、負数や整数以外の場合、または上限を超える場合は、事前に差し引いた金額をそのまま維持し、ホストが警告を記録します。上記の式でグループ倍率が 1 の場合、事前差し引き額は約 0.0262144 米ドルです。この金額を賄う残高がない場合、リクエストは Google に送信されません。

## REST での呼び出し

`NEW_API_BASE_URL` にゲートウェイのアドレスを、`NEW_API_TOKEN` にそのゲートウェイのトークンを設定します。

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

プラグイン名をプレフィックスに持つ `/gemini-decisions/v1beta/decisions` も呼び出せます。リクエストの `input`、`questions` とレスポンスの `answers`、`usage` は Google の JSON 構造を維持します。モデルマッピングによって変更されるのは、上流へ送信する `model` のみです。

## Google Python SDK

`google-genai` 3.2.0 以降をインストールしてください。サードパーティープラグインのルートではゲートウェイトークンによる Bearer 認証が必要なため、`Authorization` も明示的に設定する必要があります。SDK が既定で送信する `x-goog-api-key` だけでは、本プラグインのゲートウェイ認証はできません。

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

## 動作と制限

- Google の制限には、1 回あたり 1–1,000 件の質問、合計コンテキスト 262,144 tokens が含まれます。質問の形式とメディアの制限は [公式ドキュメント](https://aistudio.google.com/docs/decisions-api#limits)に従ってください。プラグインは基本的なリクエストチェックのみを行い、それ以外の検証は Google が実施します。
- `input` には、公式にサポートされているテキスト、マルチモーダルコンテンツブロック、公開または署名付き HTTPS URL、既存の Google Files URI を使用できます。本プラグインは Decisions のみを実装し、Files API によるアップロードは提供しません。リクエストボディのサイズには new-api ホスト側の制限も適用されます。
- 結果は同期的に返されます。プラグインは `retainResult: false` を設定しているため、new-api はこの呼び出しの `task.data` を永続化しません。ただし、タスクと課金ログは保持されます。ログの具体的な内容はホストの設定によって決まります。
- HTTP エラーは new-api ホストによって処理されます。上流からの元のエラー詳細はサニタイズされる場合があり、そのまま転送される保証はありません。プラグインはキーやリクエスト内容を記録しません。

## 検証

プラグイン CLI に対応した new-api で次を実行します。

```bash
new-api plugin lint plugin.js
new-api plugin test plugin.js --fixture fixtures/decisions.json
```

ホスト統合テストは、ローカルの Google モック、SQLite、実際の new-api のルーティングと課金処理を使用し、実際のキーは必要ありません。独立した `v1.0.0-rc.49` のソースコードのチェックアウトを使用して実行してください。

```bash
tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49
```

`google-genai` 3.2.0 をインストールした後は、`python tests/sdk_compat_test.py` を実行し、ローカルのモックサービスで SDK のパス、Bearer リクエストヘッダー、レスポンス解析を確認することもできます。このテストはホスト統合テストとは独立しています。

## ライセンス

本プロジェクトは [Apache License 2.0](LICENSE) を採用しています。プラグインインターフェースと使用量精算の実装は、[QuantumNous/new-api-plugins の TypeSafe プラグイン](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe)を参考にしています。
