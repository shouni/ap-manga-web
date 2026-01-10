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
	cfg       config.Config
	publisher *publisher.MangaPublisher
}

func NewDefaultPublisherRunner(cfg config.Config, pub *publisher.MangaPublisher) *DefaultPublisherRunner {
	return &DefaultPublisherRunner{
		cfg:       cfg,
		publisher: pub,
	}
}

func (pr *DefaultPublisherRunner) Run(ctx context.Context, manga domain.MangaResponse, images []*imagedom.ImageResponse, outputFile, outputGCS string) error {
	opts := publisher.Options{
		OutputFile:     outputFile,
		OutputImageDir: outputGCS,
		ImageDirName:   "images",
	}

	return pr.publisher.Publish(ctx, manga, images, opts)
}
