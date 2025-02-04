package config

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-redis/redis/v8"
	"github.com/minio/minio-go/v7"
)

var (
    RedisAPICacheCtx context.Context
    RedisAPICacheCtxCancel context.CancelFunc

    MinIOCtx = context.Background()

    FirebaseCtx context.Context

    RedisAPICache *redis.Client
    FirebaseClient *firestore.Client
    MinIOClient *minio.Client
    
    Once sync.Once
)

const (
    APIKeyCacheExpiration = 24 * time.Hour 
    VersionPrefix = "/api/v2/actions"
    Logfile = "actions.log"
    FirebaseConfigFile = "./secrets/stockic-b6c89-firebase-adminsdk-wr64l-a8e3bdf5e7.json"
)
