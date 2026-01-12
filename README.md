# ✍️ AP Manga Web

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/ap-manga-web)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/ap-manga-web)](https://github.com/shouni/ap-manga-web/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 概要 (About) - Webベースの画像生成オーケストレーター

**AP Manga Web** は、画像生成のコアライブラリ機能を **[Go Manga Kit](https://github.com/shouni/go-manga-kit)** を活用し、その機能を **Cloud Run** および **Google Cloud Tasks** を利用してWebアプリケーション化・オーケストレーションするためのプロジェクトです。

Webフォームを通じて画像生成処理を**非同期ワーカー**（Cloud Tasks）で実行します。処理完了時には **Slack** へ通知が飛び、生成された作品の確認や、キャラクター固定に必要な Seed 値の取得がスムーズに行えるのだ。

**Google OAuth 2.0** による認証機能を実装。Webフォームへのアクセスを承認されたユーザー（指定ドメイン/メールアドレス）のみに制限し、AIリソースの安全な利用を担保します。

---

## 🎨 5つのワークフロー (Workflows)

制作プロセスに応じて、以下の5つの機能をWeb UIから使い分けられるのだ。

| 画面 (Command) | 役割 | 主な出力 |
| --- | --- | --- |
| **Design** | DNA抽出。設定画を生成し、**固定用のSeed値を特定**する。 | Design Image, **Final Seed (via Slack)** |
| **Generate** | 一括生成。解析から全ページのパブリッシュまで一気通貫。 | HTML, Images, MD |
| **Script** | 台本生成。AIによる構成案（JSON）のみを出力。 | JSON (Script) |
| **Panel** | パネル作画。既存の台本から画像とHTMLを生成。 | Images, HTML, MD |
| **Page** | 最終錬成。プロット（Markdown）から漫画構成案を生成。 | Manga Structure (JSON) |

---

## ✨ 技術スタック (Technology Stack)

| 要素 | 技術 / ライブラリ | 役割 |
| --- | --- | --- |
| **言語** | **Go (Golang)** | Webサーバー（API/タスクワーカー）の開発言語。 |
| **認証・セッション** | **`x/oauth2`** / **`gorilla/sessions`** | **Google OAuth 2.0** フローとセッション管理。 |
| **Webフレームワーク** | **go-chi/chi/v5** | 軽量なルーティング処理。 |
| **非同期実行** | **Google Cloud Tasks** | 重い画像生成処理をキューイング。 |
| **結果保存** | **Google Cloud Storage (GCS)** | 生成物（HTML/画像）の永続化。 |
| **通知** | **Slack Webhook** | **SlackAdapter** による生成完了および Seed 値の報告。 |

---

## 🏗 システムアーキテクチャ (Internal Structure)

本プロジェクトは、拡張性を高めるために以下の3層構造で設計されています。

1. **Controller 層**: Web/Worker ハンドラーが外部との窓口となる。
2. **Pipeline 層**: `MangaPipeline` が全体の指揮官となり、台本・画像生成・公開・Slack通知を制御。
3. **Runner 層**: 画像生成やGCSアップロードなど、特定のタスクを実行する最小単位のコンポーネント。

---

## 🚀 使い方 (Usage) / セットアップ

### 1. 必要な環境変数

| 環境変数 | 説明 | デフォルト値 |
| --- | --- | --- |
| `SERVICE_URL` | アプリのルートURL（例: `https://myapp.run.app`） | `http://localhost:8080` |
| `GCP_PROJECT_ID` | GCPのプロジェクトID | - |
| `GCS_MANGA_BUCKET` | 漫画画像とHTMLを保存するバケット名 | - |
| `GEMINI_API_KEY` | Google Gemini APIキー | - |
| `GEMINI_MODEL` | 台本構成に使用するモデル名 | `gemini-3.0-flash-preview` |
| `IMAGE_MODEL` | 画像生成に使用するモデル名 | `gemini-3.0-pro-image-preview` |
| `GOOGLE_CLIENT_ID` | OAuthクライアントID | - |
| `GOOGLE_CLIENT_SECRET` | OAuthクライアントシークレット | - |
| `SESSION_SECRET` | セッション暗号化用のランダム文字列 | - |
| `ALLOWED_EMAILS` | 許可するメールアドレス（カンマ区切り） | - |
| `ALLOWED_DOMAINS` | 許可するドメイン（例: `example.com`） | - |
| `SLACK_WEBHOOK_URL` | 通知を送る先の Slack Webhook URL |

### 2. ローカルでの実行方法

```bash
# 必要な環境変数を設定（direnv推奨）
export SERVICE_URL="http://localhost:8080"
export GEMINI_API_KEY="your-api-key"
# ...その他の変数

# サーバー起動
go run main.go

```

---

## 📁 プロジェクトレイアウト

```text
ap-manga-web/
├── internal/
│   ├── adapters/     # Slack通知、Cloud Tasks連携等の外部アダプター
│   ├── builder/      # DIコンテナ、サーバー構築、Appコンテキスト
│   ├── config/       # 環境変数管理、キャラクター定義 (characters.json)
│   ├── controllers/
│   │   ├── auth/     # Google OAuth & OIDC検証
│   │   ├── web/      # 各ワークフローの画面表示・受付
│   │   └── worker/   # 非同期タスク実行
│   ├── domain/       # ドメインモデル (TaskPayload, NotificationRequest)
│   ├── pipeline/     # 全体の実行制御 (Script -> Image -> Publish -> Notify)
│   ├── prompts/      # 台本作成用テンプレート (Markdown)
│   └── runner/       # 各タスクの実行実体 (Design, Image, Script, etc.)
├── templates/        # UIテンプレート (layout.html, index.html 等)
└── main.go           # エントリーポイント

```

---

## 💻 ワークフロー (Workflow)

1. **Request**: ユーザーが Web フォームから URL やプロット、**Seed値**を送信。
2. **Enqueue**: `web.Handler` が Cloud Tasks にジョブを投入。
3. **Worker**: `worker.Handler` がリクエストを受け、`MangaPipeline` を起動。
4. **Pipeline**:
    * **Phase 1: Script/Page**: 台本生成や構成案の作成。
    * **Phase 2: Panel/Design**: 画像生成。指定された Seed またはランダム Seed を使用。
    * **Phase 3: Publish**: 成果物の保存。GCS へ保存し、公開用 URL を発行。
    * **Phase 4: Notification**: Slack へ完了報告。**Designモードの場合は決定された Seed 値を明記。**

---

## 💡 Tips: キャラクターを固定する方法

1. **Design** 画面でキャラクターID（例: `zundamon`）を入力し、Seedを `12345` で実行するのだ。
2. Slackに通知された **Final Seed**（例: `12345`）を確認するのだ。気に入った見た目ならその数値をメモ！
3. 次回 **Generate** や **Image** を実行する際、その Seed 値を入力欄に入れれば、同じビジュアルのキャラクターを登場させ続けられるのだ！

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

---
