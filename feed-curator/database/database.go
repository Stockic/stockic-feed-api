package database

import (
    "context" 
    "os"
    "strconv"
    "fmt"
    "log"
    "bytes"
    "time"
    "encoding/json"
    
    "feed-curator/utils"
    "feed-curator/models"

    "github.com/go-redis/redis/v8" 
    "github.com/joho/godotenv"

    "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func InitRedis() {

    err := godotenv.Load()
    if err != nil {
        log.Printf("Warning: Error loading .env file: %v", err)
    }

    models.FreshNewsRedisCtx, models.FreshNewsRedisCtxCancel = context.WithCancel(context.Background())

    models.FreshNewsRedis, err = RedisInit(models.FreshNewsRedisCtx, "FRESHNEWS_REDIS_ADDRESS", "FRESHNEWS_REDIS_DB", "FRESHNEWS_REDIS_PASSWORD")
    if err != nil {
        utils.LogMessage("FRESH NEWS Redis Server Setup Failed!", "red", err)
    }
}

func InitMinIO() {
    stores := []string{"user-logs", "news-archive"}

    models.MinIOClient = MinIOInit("MINIOENDPOINT", "MINIOACCESSKEY", "MINIOSECRETKEY", stores)
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

func MinIOInit(MinIOEndpoint string, MinIOAccessKey string, MinIOSecretKey string, BucketList []string) *minio.Client {

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

func uploadToMinIO(minioClient *minio.Client, key string, value string, MinIOBucket string) error {
	
    payload := map[string]interface{}{
		"key":   key,
		"value": value,
		"time":  time.Now().Format(time.RFC3339),
	}
    
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

    reader := bytes.NewReader(payloadBytes)

	objectName := fmt.Sprintf("%s.json", key)
	_, err = minioClient.PutObject(models.MinIOCtx, 
        MinIOBucket,                            // Bucket Name 
        objectName,                             // Object Name
		reader,                                 // Reader of JSON file 
        int64(len(payloadBytes)),               // Size of file
        minio.PutObjectOptions{                 // Metadata
            ContentType: "application/json",
    })

    if err != nil {
        return fmt.Errorf("Error uploading file: %s Error: %v", key, err)
    }

	return nil
}
