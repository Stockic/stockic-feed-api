package config

import (
    "context"
    "sync"
    "time"

    "github.com/go-redis/redis/v8"
    "cloud.google.com/go/firestore"
)

var (
    RedisAPICacheCtx context.Context
    RedisAPICacheCtxCancel context.CancelFunc

    RedisNewsCacheCtx context.Context
    RedisNewsCacheCtxCancel context.CancelFunc

    RedisLogCtx context.Context
    RedisLogCtxCancel context.CancelFunc

    FirebaseCtx context.Context

    RedisAPICache *redis.Client
    RedisNewsCache *redis.Client
    RedisLog *redis.Client
    FirebaseClient *firestore.Client

    Once sync.Once
)

const (
    APIKeyCacheExpiration = 24 * time.Hour
    VersionPrefix = "/api/v1"
)
