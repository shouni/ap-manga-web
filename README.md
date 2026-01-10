# ✍️ AP Manga Web

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/ap-manga-web)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/ap-manga-web)](https://github.com/shouni/ap-manga-web/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 概要 (About) - Webベースの画像生成オーケストレーター

**AP Manga Web** は、画像生成のコアライブラリ機能を **[Go Manga Kit](https://github.com/shouni/go-manga-kit)** を活用し、その機能を **Cloud Run** および **Google Cloud Tasks** を利用してWebアプリケーション化・オーケストレーションするためのプロジェクトです。

Webフォームを通じて画像生成処理を**非同期ワーカー**（Cloud Tasks）で実行します。

**Google OAuth 2.0** による認証機能を実装。Webフォームへのアクセスを承認されたユーザー（指定ドメイン/メールアドレス）のみに制限し、AIリソースの安全な利用を担保します。

---

## 🎨 5つのワークフロー (Workflows)

本ツールは、制作プロセスに応じて5つの機能をWeb UIから使い分けられるのだ。

| 画面 (Command) | 役割 | 主な出力 |
| --- | --- | --- |
| **Design** | DNA抽出。設定画を生成し、固定用のSeed値を特定する。 | Design Image, Seed |
| **Generate** | 一括生成。解析から全ページのパブリッシュまで一気通貫。 | HTML, Images, MD |
| **Script** | 台本生成。AIによる構成案（JSON）のみを出力。 | JSON (Script) |
| **Image** | パネル作画。各コマの個別画像とHTMLを生成。 | Images, HTML, MD |
| **Story** | 最終錬成。Markdown/JSONから複数枚の統合漫画画像を生成。 | Final Images (PNGs) |

---

## ✨ 技術スタック (Technology Stack)

| 要素 | 技術 / ライブラリ | 役割 |
| --- | --- | --- |
| **言語** | **Go (Golang)** | Webサーバー（API/タスクワーカー）の開発言語。 |
| **認証・セッション** | **`x/oauth2`** / **`gorilla/sessions`** | **Google OAuth 2.0** フローとCookieベースのセッション管理。 |
| **Webフレームワーク** | **go-chi/chi/v5** | 軽量でモジュール化されたルーティング処理。 |
| **アーキテクチャ** | **DI (Dependency Injection)** | `builder/server.go` で全ての依存関係を統合。 |
| **非同期実行** | **Google Cloud Tasks** | 重い画像生成処理を非同期キューに投入し、タイムアウトを回避。 |
| **結果保存** | **Google Cloud Storage (GCS)** | 生成された画像を保存。 |
| **I/O抽象化** | **[`go-remote-io`](https://github.com/shouni/go-remote-io)** | GCSへのI/O操作および署名付きURLの生成。 |

---


## 🏗 システムアーキテクチャ (Internal Structure)

本プロジェクトは、インポートサイクルを回避しつつ、拡張性を高めるために以下の3層構造で設計されています。

1. **Controller 層**: Web/Worker ハンドラーが外部（ユーザー/Cloud Tasks）との窓口となる。
2. **Pipeline 層**: `MangaPipeline` が全体の指揮官となり、台本生成・画像生成・公開の順序を制御。
3. **Runner 層**: 最小単位の実行部隊。特定のタスク（画像1枚の生成など）に専念。

---

## 🚀 使い方 (Usage) / セットアップ

### 1. GCPコンソールでの事前準備 (OAuth) 🔐

1. **「APIとサービス」 > 「認証情報」** で **OAuth クライアント ID** を作成します。
2. **承認済みのリダイレクト URI**:
* ローカル: `http://localhost:8080/auth/callback`
* 本番: `https://[あなたのCloud Run URL]/auth/callback`


3. **クライアント ID** と **シークレット** を取得します。

### 2. 必要な環境変数

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

### 3. ローカルでの実行方法

```bash
# 必要な環境変数を設定（direnv推奨）
export SERVICE_URL="http://localhost:8080"
export GEMINI_API_KEY="your-api-key"
# ...その他の変数

# サーバー起動
go run main.go

```

---

## 🔐 4. 必要なIAMロールの設定（重要）

### A. Cloud Run サービスアカウント (アプリケーション実行用)

| 権限（IAMロール） | 目的 |
| --- | --- |
| **Cloud Tasks エンキューア** (`roles/cloudtasks.enqueuer`) | タスクを Cloud Tasks キューに**追加**するために必要です。 |
| **サービス アカウント ユーザー** (`roles/iam.serviceAccountUser`) | OIDCトークン付きのタスクを作成するために必要です。 |
| **Storage オブジェクト管理者**(`roles/storage.objectAdmin`) | 生成された画像やHTMLなどの成果物を **GCS** バケットに書き込むために必要です。 |

### B. Cloud Tasks サービスアカウント (タスク実行ID)

| 権限（IAMロール） | 目的 |
| --- | --- |
| **Cloud Run 起動元**<br>(`roles/run.invoker`) | Cloud Tasks が、ワーカーエンドポイント (`/tasks/generate`) を認証付きで呼び出すために必要です。 |

---

## 📁 プロジェクトレイアウト

```text
ap-manga-web/
├── internal/
│   ├── adapters/     # 外部システム連携を担うアダプター
│   ├── builder/      # DIコンテナ、サーバー構築 (server.go)
│   ├── config/       # 環境変数・設定管理 (config.go)
│   ├── controllers/
│   │   ├── auth/     # Google OAuth & セッション管理
│   │   ├── web/      # 画面表示・フォーム受付 (handler.go)
│   │   └── worker/   # 非同期タスク実行 (handler.go)
│   ├── domain/       # ドメインモデル (task.go, manga.go)
│   ├── pipeline/     # 実行制御 (Script -> Image -> Publish)
│   ├── prompt/       # 台本作成用テンプレート (Markdown)
│   └── runner/       # 画像生成実行パイプライン
├── templates/        # HTMLテンプレート (layout.html, index.html等)
├── main.go           # エントリーポイント
└── go.mod            # モジュール定義

```

---

## 💻 処理のフロー (Workflow Flow)

1. **Request**: ユーザーが Web フォームから URL を送信。
2. **Enqueue**: `web.Handler` が Cloud Tasks にジョブを投入。
3. **Worker**: `worker.Handler` がリクエストを受け、`MangaPipeline` を起動。
4. **Pipeline**:
* **Script Phase**: ウェブコンテンツを抽出し、Gemini で JSON 台本を構成。
* **Image Phase**: 各パネルに対し、設定されたキャラクタースタイルで画像生成。
* **Publish Phase**: 画像を GCS へ保存し、HTML/Markdown レポートを出力。

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

---
