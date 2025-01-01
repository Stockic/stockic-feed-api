package database

import (
    "context"
    "fmt"
    "log"
    "os"
    "strconv"
    "sync"
    "time"
    "path/filepath"

    "github.com/go-redis/redis/v8"
    firebase "firebase.google.com/go"
    "google.golang.org/api/option"
    "cloud.google.com/go/firestore"
    "github.com/joho/godotenv"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"

    "feed-api/utils"
    "feed-api/config"
)

func Initialize() {
    // Setup Logging files and configurations
    logFile, err := os.OpenFile(config.Logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
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
    
    // Initialize MinIO
    InitMinIO()
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

func InitMinIO() {
    stores := []string{"user-logs", "feed-api-app-logs"}

    config.MinIOClient = MinIOInit("MINIO_ENDPOINT", "MINIO_ACCESSKEY", "MINIO_SECRETKEY", stores)
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

func MinIOInit(MinIOEndpoint string, MinIOAccessKey string, MinIOSecretKey string, BucketList []string) *minio.Client {
    utils.LogMessage("MinIO Init Started ...", "green")

    endpoint := os.Getenv(MinIOEndpoint)
    accessKey := os.Getenv(MinIOAccessKey)
    secretKey := os.Getenv(MinIOSecretKey)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		utils.LogMessage("Failed to initialize MinIO client", "red", err)
	}

    for _, MinIOBucket := range BucketList {
        exists, err := minioClient.BucketExists(config.MinIOCtx, MinIOBucket)
        if err != nil {
            utils.LogMessage(fmt.Sprintf("Error checking bucket existence: %s", MinIOBucket), "red", err)
        }
        if !exists {
            utils.LogMessage(fmt.Sprintf("Bucket was not found, creating... %s", MinIOBucket), "red")
            err := minioClient.MakeBucket(config.MinIOCtx, MinIOBucket, minio.MakeBucketOptions{})
            if err != nil {
                log.Fatalf("Failed to create bucket: %v", err)
                utils.LogMessage(fmt.Sprintf("Failed to create bucket: %s", MinIOBucket), "red", err)
            }
        }
    }

    /*
    minioClient is object of data type *minio.Client which is a pointer. 
    When it would be called in the code, like client := minioClient, client will be a pointer too. 
    Hence, each time client is accessed, throughout the code, the code would be executed residing 
    in the particular memory address and then returned back to the execution stack. This is how 
    you make a function accessible universally and allow it to retain it's state. 
    */

    return minioClient
}

func UploadLogDataToMinIO(minioClient *minio.Client, BucketName, localFilePath string) error {

    // Open the log file
    file, err := os.Open(localFilePath)
    if err != nil {
        return fmt.Errorf("error opening file: %w", err)
    }
    defer file.Close()

    // Get file stats
    stat, err := file.Stat()
    if err != nil {
        return fmt.Errorf("error getting file stats: %w", err)
    }

    timestamp := time.Now().Format("20060102_150405")
    fileName := filepath.Base(localFilePath)
    fileExt := filepath.Ext(fileName)
    fileNameWithoutExt := fileName[:len(fileName)-len(fileExt)]
    objectName := fmt.Sprintf("logs/%s/%s_%s%s",
        time.Now().Format("2006/01"),
        fileNameWithoutExt,
        timestamp,
        fileExt,
    )

    // Upload the file
    _, err = minioClient.PutObject(
        config.MinIOCtx,
        BucketName,
        objectName,
        file,
        stat.Size(),
        minio.PutObjectOptions{
            ContentType: "text/plain",
        },
    )
    if err != nil {
        return fmt.Errorf("error uploading file: %w", err)
    }
    
    return nil
}
