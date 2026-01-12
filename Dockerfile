# --- Stage 1: ビルダー ---
FROM golang:1.23-alpine AS builder
# タイムゾーンデータと証明書をインストール
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/main ./main.go

# --- Stage 2: 最終イメージ ---
FROM scratch

# 1. SSL証明書をコピー (HTTPSリクエストに必須)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 2. タイムゾーンデータをコピー (Asia/Tokyo を認識させるために必須なのだ！)
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# 設定ファイルやテンプレートをコピー
COPY templates /app/templates
COPY internal/config/characters.json /app/internal/config/characters.json

# ビルドされたバイナリをコピー
COPY --from=builder /app/main /app/main

# 3. 環境変数でデフォルトのタイムゾーンを指定
ENV TZ=Asia/Tokyo

EXPOSE 8080

CMD ["/app/main"]
