package config

import (
    "context"
    "sync"
    "time"

    "github.com/go-redis/redis/v8"
    "cloud.google.com/go/firestore"
    "github.com/minio/minio-go/v7"
)

var (
    RedisAPICacheCtx context.Context
    RedisAPICacheCtxCancel context.CancelFunc

    RedisNewsCacheCtx context.Context
    RedisNewsCacheCtxCancel context.CancelFunc

    RedisLogCtx context.Context
    RedisLogCtxCancel context.CancelFunc

    MinIOCtx = context.Background()

    FirebaseCtx context.Context

    RedisAPICache *redis.Client
    RedisNewsCache *redis.Client
    RedisLog *redis.Client
    FirebaseClient *firestore.Client
    MinIOClient *minio.Client

    Once sync.Once
)

const (
    APIKeyCacheExpiration = 24 * time.Hour
    VersionPrefix = "/api/v2"
    Logfile = "feed-api.log"
)
