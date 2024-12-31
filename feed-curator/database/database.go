package database

import (
    "context" 
    "os"
    "strconv"
    "fmt"
    "log"
    "bytes"
    "time"
    "path/filepath"
    "encoding/json"
    
    "feed-curator/utils"
    "feed-curator/models"

    "github.com/go-redis/redis/v8" 
    "github.com/joho/godotenv"

    "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func UploadNewsAPIResponseDataToMinIO(minioClient *minio.Client, jsonNewsData map[string]models.APIResponse, MinIOBucket string) error {

    utils.LogMessage("Started Uploading Archive News to MinIO", "green")
	
    payload := map[string]interface{}{
        "time":  time.Now().Format(time.RFC3339),
		"news-data": jsonNewsData,
	}
    
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

    reader := bytes.NewReader(payloadBytes)

    currentTime := time.Now()
    formattedTime := currentTime.Format("2006-01-02T15-04-05")

    objectName := fmt.Sprintf("news-%s.json", formattedTime)
	_, err = minioClient.PutObject(models.MinIOCtx, 
        MinIOBucket,                            // Bucket Name 
        objectName,                             // Object Name
		reader,                                 // Reader of JSON file 
        int64(len(payloadBytes)),               // Size of file
        minio.PutObjectOptions{                 // Metadata
            ContentType: "application/json",
    })

    if err != nil {
        return fmt.Errorf("Error uploading file: %v", err)
    }

	return nil
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
        models.MinIOCtx,
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

func InitRedis() {

    err := godotenv.Load()
    if err != nil {
        utils.LogMessage("Warning: Error loading .env file", "red", err)
    }

    models.FreshNewsRedisCtx, models.FreshNewsRedisCtxCancel = context.WithCancel(context.Background())

    models.FreshNewsRedis, err = RedisInit(models.FreshNewsRedisCtx, "FRESHNEWS_REDIS_ADDRESS", "FRESHNEWS_REDIS_DB", "FRESHNEWS_REDIS_PASSWORD", "FREASNEWS_REDIS_CHANNEL")
    if err != nil {
        utils.LogMessage("FRESH NEWS Redis Server Setup Failed!", "red", err)
    }
}

func RedisInit(redisContext context.Context, redisAddress, redisDB, redisPassword, redisChannel string) (*redis.Client, error) {

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

    // Enable Redis keyspace notifications
    err = rdb.ConfigSet(models.FreshNewsRedisCtx, "notify-keyspace-events", "Ex").Err()
	if err != nil {
		utils.LogMessage("Failed to enable keyspace notifications", "green", err)
	}

    _, err = rdb.Ping(redisContext).Result()
    if err != nil {
        utils.LogMessage(fmt.Sprintf("Failed to connect to Redis - Address: %s, redisDB: %s", address, dbStr), "red", err)
        return nil, err
    }

    utils.LogMessage(fmt.Sprintf("Successfully initialized Redis: %s", address), "green")
    
    return rdb, err
}

func InitMinIO() {
    stores := []string{"user-logs", "news-archive", "app-logs"}

    models.MinIOClient = MinIOInit("MINIO_ENDPOINT", "MINIO_ACCESSKEY", "MINIO_SECRETKEY", stores)
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
        exists, err := minioClient.BucketExists(models.MinIOCtx, MinIOBucket)
        if err != nil {
            utils.LogMessage(fmt.Sprintf("Error checking bucket existence: %s", MinIOBucket), "red", err)
        }
        if !exists {
            utils.LogMessage(fmt.Sprintf("Bucket was not found, creating... %s", MinIOBucket), "red")
            err := minioClient.MakeBucket(models.MinIOCtx, MinIOBucket, minio.MakeBucketOptions{})
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
