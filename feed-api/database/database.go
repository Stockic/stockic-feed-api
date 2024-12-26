package database

import (
    "context"
    "fmt"
    "log"
    "os"
    "strconv"
    "sync"

    "github.com/go-redis/redis/v8"
    firebase "firebase.google.com/go"
    "google.golang.org/api/option"
    "cloud.google.com/go/firestore"
    "github.com/joho/godotenv"

    "feed-api/utils"
    "feed-api/config"
)

func Initialize() {
    // Setup Logging files and configurations
    logFile, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
    if err != nil {
        log.Fatalf("Failed to open log file: %v", err)
    }
    
    log.SetOutput(logFile)
    log.SetFlags(0)

    // Load Environment Variables with .env file
    err = godotenv.Load()
    if err != nil {
        log.Printf("Warning: Error loading .env file: %v", err)
    }

    // Initialize contexts
    config.RedisAPICacheCtx, config.RedisAPICacheCtxCancel = context.WithCancel(context.Background())
    config.RedisNewsCacheCtx, config.RedisNewsCacheCtxCancel = context.WithCancel(context.Background())
    config.RedisLogCtx, config.RedisLogCtxCancel = context.WithCancel(context.Background())
    config.FirebaseCtx = context.Background()

    // Initialize Redis clients
    initRedisClients()

    // Initialize Firebase
    initFirebase()
}

func initRedisClients() {
    var err error
    
    // User API Caching Redis Server
    config.RedisAPICache, err = RedisInit(
        config.RedisAPICacheCtx,
        "USERAPI_CACHING_ADDRESS",
        "USERAPI_CACHING_DB",
        "USERAPI_CACHING_PASSWORD",
    )
    if err != nil {
        utils.LogMessage("API Cache Server Setup Failed!", "red", err)
    }

    // Fresh News Caching Redis Server
    config.RedisNewsCache, err = RedisInit(
        config.RedisNewsCacheCtx,
        "NEWS_CACHING_ADDRESS",
        "NEWS_CACHING_DB",
        "NEWS_CACHING_PASSWORD",
    )
    if err != nil {
        utils.LogMessage("News Cache Server Setup Failed", "red", err)
    }

    // Log Redis Server
    config.RedisLog, err = RedisInit(
        config.RedisLogCtx,
        "LOGREDIS_ADDRESS",
        "LOGREDIS_DB",
        "LOGREDIS_PASSWORD",
    )
    if err != nil {
        utils.LogMessage("Log Redis Server Setup Failed", "red", err)
    }
}

func initFirebase() {
    config.FirebaseClient = InitializeFirebase(
        config.FirebaseCtx,
        "./secrets/stockic-b6c89-firebase-adminsdk-wr64l-a8e3bdf5e7.json",
        &config.Once,
    )

    go func() {
        <-config.FirebaseCtx.Done()
        config.FirebaseClient.Close()
    }()
}

func RedisInit(redisContext context.Context, redisAddress string, redisDB string, redisPassword string) (*redis.Client, error) {
    address := os.Getenv(redisAddress)
    if address == "" {
        address = "localhost:6379"
    }

    dbStr := os.Getenv(redisDB)
    db, err := strconv.Atoi(dbStr)
    if err != nil {
        db = 0
        utils.LogMessage("Warning: Invalid REDIS_DB value, using default: 0", "red")
    }

    password := os.Getenv(redisPassword)

    rdb := redis.NewClient(&redis.Options{
        Addr:     address,
        Password: password,
        DB:       db,
    })

    _, err = rdb.Ping(redisContext).Result()
    if err != nil {
        utils.LogMessage(fmt.Sprintf("Failed to connect to Redis - Address: %s, redisDB: %s", address, dbStr), "red", err)
        return nil, err
    }

    utils.LogMessage(fmt.Sprintf("Successfully initialized Redis: %s", address), "green")
    
    return rdb, err
}

func InitializeFirebase(ctx context.Context, credentialsPath string, once *sync.Once) *firestore.Client {
    var client *firestore.Client

    once.Do(func() {
        opt := option.WithCredentialsFile(credentialsPath)
        app, err := firebase.NewApp(ctx, nil, opt)
        if err != nil {
            log.Fatalf("Failed to initialize Firebase app: %v", err)
        }

        client, err = app.Firestore(ctx)
        if err != nil {
            log.Fatalf("Failed to create Firestore client: %v", err)
        }
    })

    return client
}
