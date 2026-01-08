# ✍️ AP Manga Web

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/ap-manga-web)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/ap-manga-web)](https://github.com/shouni/ap-manga-web/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 概要 (About) - Webベースの画像作成オーケストレーター

**AP Manga Web** は、画像作成の**コアライブラリ機能**を **[Go Manga Kit](https://github.com/shouni/go-manga-kit)** を活用し、その機能を **Cloud Run** および **Google Cloud Tasks** を利用してWebアプリケーション化・オーケストレーションするためのプロジェクトです。

Webフォームを通じて画像作成処理を**非同期ワーカー**（Cloud Tasks）で実行します。

**Google OAuth 2.0** による認証機能を実装。Webフォームへのアクセスを承認されたユーザー（指定ドメイン/メールアドレス）のみに制限し、AIリソースの安全な利用を担保します。

---

## ✨ 技術スタック (Technology Stack)

| 要素 | 技術 / ライブラリ | 役割 |
| --- | --- | --- |
| **言語** | **Go (Golang)** | Webサーバー（API/タスクワーカー）の開発言語。 |
| **認証・セッション** | **`x/oauth2`** / **`gorilla/sessions`** | **Google OAuth 2.0** フローとCookieベースのセッション管理。 |
| **Webフレームワーク** | **go-chi/chi/v5** | 軽量でモジュール化されたルーティング処理。 |
| **アーキテクチャ** | **DI (Dependency Injection)** | `builder/server.go` で全ての依存関係を統合する保守性の高い設計。 |
| **非同期実行** | **Google Cloud Tasks** | 重いレビュー処理を非同期キューに投入し、タイムアウトを回避。 |
| **結果保存** | **Google Cloud Storage (GCS)** | AIが出力したレビュー結果（HTML）を保存。 |
| **I/O抽象化** | **[`go-remote-io`](https://github.com/shouni/go-remote-io)** | GCSへのI/O操作および署名付きURLの生成。 |

---

## 🚀 使い方 (Usage) / セットアップ

### 1. GCPコンソールでの事前準備 (OAuth) 🔐

1. **「APIとサービス」 > 「認証情報」** で **OAuth クライアント ID** を作成します。
2. **承認済みのリダイレクト URI**:
* ローカル: `http://localhost:8080/auth/callback`
* 本番: `https://[あなたのCloud Run URL]/auth/callback`


3. **クライアント ID** と **シークレット** を取得します。

### 2. 必要な環境変数

| 環境変数 | 説明 |
| --- | --- |
| `SERVICE_URL` | アプリのルートURL（例: `https://myapp.run.app`） |
| `GCP_PROJECT_ID` | GCPのプロジェクトID |
| `GCS_REVIEW_BUCKET` | レビュー結果を保存するバケット名 |
| `GEMINI_API_KEY` | Google Gemini APIキー |
| `GOOGLE_CLIENT_ID` | OAuthクライアントID |
| `GOOGLE_CLIENT_SECRET` | OAuthクライアントシークレット |
| `SESSION_SECRET` | セッション暗号化用のランダム文字列 |
| `ALLOWED_EMAILS` | 許可するメールアドレス（カンマ区切り） |
| `ALLOWED_DOMAINS` | 許可するドメイン（例: `example.com`） |

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

本アプリケーションをGoogle Cloud RunとCloud Tasksで安全に運用するためには、各サービスアカウント（SA）に対し、**正確な権限付与**が必要です。設定が不足していると `403 Forbidden` エラーが発生します。

### A. Cloud Run サービスアカウント (アプリケーション実行用)

*Webフロントエンドおよびワーカーとして動作するサービスアカウントです。*

| 権限（IAMロール） | 目的 |
| :--- | :--- |
| **Cloud Tasks エンキューア**<br>(`roles/cloudtasks.enqueuer`) | Webフォーム受付時に、タスクを Cloud Tasks キューに**追加**するために必要です。 |
| **サービス アカウント ユーザー**<br>(`roles/iam.serviceAccountUser`) | **重要:** タスク投入時、そのタスクを実行するID（Cloud Tasks SA）として振る舞う（ActAs）ために必要です。これがないとOIDCトークン付きのタスクを作成できません。 |
| **Storage オブジェクト管理者**<br>(`roles/storage.objectAdmin`) | AIレビュー結果のHTMLファイルを **GCS** バケットに書き込むために必要です。 |
| **Secret Manager のシークレット アクセサー**<br>(`roles/secretmanager.secretAccessor`) | `GEMINI_API_KEY` を Secret Manager から安全に取得する場合に推奨されます。 |

### B. Cloud Tasks サービスアカウント (タスク実行ID)

*Cloud Tasks がワーカー（Cloud Run）を呼び出す際に使用するIDです。アプリケーションSAと同じものを使うことも可能ですが、セキュリティ上分けることを推奨します。*

| 権限（IAMロール） | 目的 |
| :--- | :--- |
| **Cloud Run 起動元**<br>(`roles/run.invoker`) | Cloud Tasks が、ワーカーエンドポイント (`/tasks/execute_review`) を認証付きで呼び出すために必要です。 |

---

## 📁 プロジェクトレイアウト

```text
ap-manga-web/
├── internal/
│   ├── adapters/     # Slack通知等の外部連携
│   ├── builder/      # DIコンテナ、サーバー構築 (server.go)
│   ├── config/       # 環境変数・設定管理 (config.go)
│   ├── controllers/
│   │   ├── auth/     # Google OAuth & セッション管理
│   │   ├── web/      # フォーム受付・ハンドラ
│   │   └── worker/   # 非同期タスク実行
│   ├── domain/       # ドメインモデル (review, response)
│   └── runner/       # レビュー実行パイプライン
├── templates/        # HTMLテンプレート (review_form.html等)
├── main.go           # エントリーポイント
└── go.mod            # モジュール定義

```

---

## 💻 処理のフロー

1. **リクエスト:** 認証済みユーザーがフォームよりGitHub URL等を送信。
2. **署名付きURL生成:** `go-remote-io` を使い、未来の保存先URLを即時生成。
3. **タスク投入:** Cloud Tasks へレビュー処理を委譲し、ユーザーには `202 Accepted` を返却。
4. **非同期実行:** ワーカーが `gemini-reviewer-core` を実行し、結果をGCSへ保存＆Slack通知。

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

---
