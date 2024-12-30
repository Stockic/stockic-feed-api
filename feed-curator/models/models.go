package models

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/minio/minio-go/v7"
)

var (
    LogFile = "summarizer.log"

    GeminiCtx context.Context

    FreshNewsRedisCtx context.Context
    FreshNewsRedisCtxCancel context.CancelFunc

    MinIOClient *minio.Client
    MinIOCtx = context.Background()

    FreshNewsRedis *redis.Client
    RedisChannel   = "__keyspace@0__:*"
)

type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Article struct {
	Source      Source `json:"source"`
	Author      string `json:"author"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	URLToImage  string `json:"urlToImage"`
	PublishedAt string `json:"publishedAt"`
	Content     string `json:"content"`
}

type APIResponse struct {
	Status       string    `json:"status"`
	TotalResults int       `json:"totalResults"`
	Articles     []Article `json:"articles"`
}

type SummarizedArticle struct {
    StockicID           string `json:"stockicID"`
    Source              string `json:"source"`
	Author              string `json:"author"`
	Title               string `json:"title"`
	// Description string `json:"description"`
	URL                 string `json:"url"`
	URLToImage          string `json:"urlToImage"`
	PublishedAt         string `json:"publishedAt"`
	SummarizedContent   string `json:"content"`
}

type SummarizedResponse struct {
	Status       string                 `json:"status"`
	TotalResults int                    `json:"totalResults"`
	Articles     []SummarizedArticle    `json:"articles"`
}
