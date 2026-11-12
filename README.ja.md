# new-api-plugin-gemini-decisions

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

[new-api](https://github.com/QuantumNous/new-api) で Google の [Gemini Decisions API](https://aistudio.google.com/docs/decisions-api) を利用するための Task Plugin です。ネイティブの JSON をそのまま転送し、`gemini-decision-1` のテキスト、画像、音声、動画、PDF 入力と、`noul`、`choice`、`score` による判定に対応します。

本プラグインは独立した `gemini-decisions` プラグインであり、new-api 組み込みの Google Veo プラグインを置き換えるものではありません。同期型のネイティブインターフェースを使用し、Chat Completions への変換は提供しません。

## 動作環境とインストール

本プロジェクトは、new-api **v1.0.0-rc.49** の Task Plugin v1 インターフェースを対象としています。このプラグインインターフェースは現在も開発が進められており、古いバージョンではカスタムのネイティブルートや `retainResult` に対応していない場合があります。

1. **Root スーパー管理者**アカウント（Task Plugin のアップロードと有効化に必要）で new-api の **Task Plugins / タスクプラグイン** ページを開き、タスクプラグインシステムを有効にします。リポジトリのルートにある `plugin.js` をアップロードし、プラグインを有効にしてください。アップロードダイアログで [plugin.js の Raw ファイル URL](https://raw.githubusercontent.com/huangdijia/new-api-plugin-gemini-decisions/main/plugin.js) を指定することもできます。
2. **Task Plugin** タイプのチャネルを作成し、`gemini-decisions` を選択します。
   - Base URL：`https://generativelanguage.googleapis.com`（空欄の場合の既定値）。上流が Google と new-api のどちらの場合も、サービスのルート URL または末尾に `/v1beta` が付いた URL を指定できます。どちらも末尾のスラッシュに対応します。
   - キー：Google AI Studio の Gemini API key。
   - モデル：`gemini-decision-1`。呼び出しを許可するグループも設定してください。
3. モデルに以下のタスク使用量に基づく課金式を設定し、new-api トークンで呼び出します。Google API key はチャネルにのみ設定し、クライアントは new-api トークンを使用します。

インストール済みの場合は、プラグイン **0.2.0** をアップロードして有効にしてください。new-api は同じプラグインの key/version に異なるソースコードを登録することを許可しません。既存の課金式と固定の 262,144-token の事前差し引きは変更されません。

上流が**本プラグインもインストール済み**の別の new-api である場合は、New API タイプのチャネルを使用して上流のタスクプラグインを紐付け、チャネルのキーに上流ゲートウェイのトークンを設定できます。

## 課金

Google が返す入力トークン数に基づいて精算するには、モデルの料金設定で **タスク使用量式** を使用します。例えば、Google が 2026-11-12 に公開した入力 100 万トークンあたり 0.10 米ドルで計算する場合は、次の式を使用します。

```text
tier("base", u("input_tokens") * 0.1 / 1000000)
```

Task Plugin の式は米ドル建ての金額を直接返すため、`/ 1000000` を省略してはいけません。同じ名前のモデルを複数のプラグインが提供している場合は、`gemini-decisions::gemini-decision-1` にプロバイダー別の式を設定できます。実際の料金、グループ倍率、無料枠は、ご自身の設定と [Google の料金表](https://aistudio.google.com/docs/pricing#gemini-decision)に従います。従来のリクエスト単位の料金設定は引き続きリクエスト単位で課金され、トークン単位の課金へ自動的に切り替わることはありません。

ゲートウェイはリクエスト送信前にすべてのマルチモーダル入力のトークン数を確実に計算できないため、プラグインはモデルのコンテキスト上限である **262,144 入力トークン** 分を事前に差し引きます。成功後は `usage.total_input_tokens` に基づいて精算します。使用量が欠落している場合、負数や整数以外の場合、または上限を超える場合は、事前に差し引いた金額をそのまま維持し、ホストが警告を記録します。上記の式でグループ倍率が 1 の場合、事前差し引き額は約 0.0262144 米ドルです。この金額を賄う残高がない場合、リクエストは Google に送信されません。

Google によると、**400 または 500** を返す失敗したリクエストにはトークン料金は発生しませんが、quota にはカウントされます。これは Google 上流の課金ポリシーであり、ゲートウェイの事前差し引きや精算とは別です。[課金 FAQ](https://aistudio.google.com/docs/billing#am-i-charged-for-failed-requests)を参照してください。

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

- Google の制限は、1 回あたり 1–1,000 件の質問と合計コンテキスト 262,144 tokens です。質問の key は英字、数字、`_`、`-` のみからなる **1–64 文字の ASCII 文字列**にしてください。対応する type は `noul`、`choice`、`score` です。`choice` と `score` はそれぞれ **2–255 項目**が必要で、各 `score` 項目は空でない文字列にしてください。`instructions` は必須で、文字列、TextContent オブジェクト、または空でない TextContent オブジェクトの配列を指定できます。プラグインはこれらの要件を明示的に検証し、その他の形式上の制約は Google が検証します。[公式の制限](https://aistudio.google.com/docs/decisions-api#limits)を参照してください。
- `input` は Google のテキスト、マルチモーダルコンテンツブロック、公開または署名付きの外部 HTTPS URL、既存の Google Files URI に対応します。Google の[入力制限](https://aistudio.google.com/docs/decisions-api#input)は、**1 リクエストの inline data が最大 100 MB**、**PDF が最大 50 MB**、**外部 HTTPS URL ごとに最大 15 MB**です。大きなファイルは、先に Google Files API へ直接アップロードし、返された URI を使用してください。本プラグインは Files API のアップロードを実装していません。任意の外部 URL で 15 MB の制限を回避することはできません。
- new-api **v1.0.0-rc.49** の `MAX_REQUEST_BODY_MB` は既定で **128 MiB** であり、展開後の JSON リクエストボディ全体を制限します。Base64 は元ファイルより約 3 分の 1 大きくなり、さらに JSON のオーバーヘッドが加わります。例えば、**100 MiB** の元ファイルは Base64 への変換だけで 128 MiB を超えます。Google の 100 MB の inline 制限とホストの 128 MiB のボディ制限は単位と対象が異なるため、100 MB の元ファイルが必ずホストの制限を超えるわけではありません。
- 結果は同期的に返されます。new-api **v1.0.0-rc.49** の `RELAY_TIMEOUT` は既定で **0** であり、送信 HTTP クライアントの全体タイムアウトを設定しません。`RELAY_RESPONSE_HEADER_TIMEOUT` は既定で **1800 秒**で、レスポンスヘッダーの待機時間を制限します。クライアントやリバースプロキシのタイムアウトが適用される場合もあります。
- ホストが正常に読み取り、プラグインに渡した上流の **2xx** レスポンスでは、回答が欠落したり回答の type が一致しなかったりしても、usage に基づいて精算し、上流の回答を保持します。`answers` オブジェクトがない場合は、使用量の精算後に **502** を返します。使用量が不正な場合は上記のとおり事前差し引き額を維持するため、502 だけで返金されたとは判断できません。ホストがレスポンスをプラグインに渡す前のエラーでは、返金される場合があります。
- 複数の new-api ゲートウェイを連結した場合、上流のゲートウェイは使用量を精算した後、`answers` オブジェクトがないため 502 を返すことがあります。下流の v1.0.0-rc.49 ゲートウェイは非 2xx レスポンスに対して呼び出し元に返金するため、上流への料金を下流の運営者が負担する場合があります。
- プラグインは `retainResult: false` を設定し、new-api はこの呼び出しの `task.data` を永続化しません。ただし、タスクと課金ログは保持されます。ログの具体的な内容はホストの設定によって決まります。
- HTTP エラーは new-api ホストが処理します。上流の一部の 4xx JSON エラー構造は保持できますが、バイト単位での転送は保証されず、5xx の詳細はサニタイズされる場合があります。プラグインはキーやリクエスト内容を記録しません。

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

[GitHub Actions ワークフロー](.github/workflows/test.yml)は、固定した new-api のリビジョンに対してこれらの検証を実行します。これらの検証は fixture またはローカルモックを使用しており、実際の Google API へのリクエスト成功を示すものではありません。

## ライセンス

本プロジェクトは [Apache License 2.0](LICENSE) を採用しています。プラグインインターフェースと使用量精算の実装は、[QuantumNous/new-api-plugins の TypeSafe プラグイン](https://github.com/QuantumNous/new-api-plugins/tree/main/plugins/tasks/typesafe)を参考にしています。
