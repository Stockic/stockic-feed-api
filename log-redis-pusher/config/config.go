package config

import (
    "context"

    "github.com/go-redis/redis/v8"
    "github.com/minio/minio-go/v7"
)

var (
    RedisLogCtx context.Context
    RedisLogCtxCancel context.CancelFunc

    MinIOCtx = context.Background()
    RedisLog *redis.Client
    MinIOClient *minio.Client
)

const (
    Logfile = "log-redis-pusher.log"
)
