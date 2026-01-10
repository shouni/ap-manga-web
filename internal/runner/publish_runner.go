package runner

import (
	"context"

	"ap-manga-web/internal/config"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// PublisherRunner はパブリッシュ処理のインターフェースです。
type PublisherRunner interface {
	Run(ctx context.Context, manga domain.MangaResponse, images []*imagedom.ImageResponse, outputFile, outputGCS string) error
}

// DefaultPublisherRunner は pkg/publisher を利用した標準実装なのだ。
type DefaultPublisherRunner struct {
	cfg       config.Config // CLI版の options ではなく Config 全体を保持する形に合わせるのだ
	publisher *publisher.MangaPublisher
}

func NewDefaultPublisherRunner(cfg config.Config, pub *publisher.MangaPublisher) *DefaultPublisherRunner {
	return &DefaultPublisherRunner{
		cfg:       cfg,
		publisher: pub,
	}
}

func (pr *DefaultPublisherRunner) Run(ctx context.Context, manga domain.MangaResponse, images []*imagedom.ImageResponse, outputFile, outputGCS string) error {
	// Web版の Config (または GCS 保存先設定) を pkg/publisher 用の構造体に詰め替えるのだ。
	// ここで設定する OutputFile は、GCS のバケットパス (gs://...) などになるのだ。
	opts := publisher.Options{
		OutputFile:     outputFile,
		OutputImageDir: outputGCS,
		ImageDirName:   "images",
	}

	return pr.publisher.Publish(ctx, manga, images, opts)
}
