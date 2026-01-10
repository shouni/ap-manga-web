# --- Stage 1: ビルダー ---
FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/main ./main.go

# --- Stage 2: 最終イメージ ---
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

# テンプレートディレクトリを作業ディレクトリのサブディレクトリにコピー
COPY templates /app/templates

# ビルドされたバイナリをコピー
COPY --from=builder /app/main /app/main

EXPOSE 8080

# 起動コマンドも /app/main に修正
CMD ["/app/main"]
