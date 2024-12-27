package database

import (
    "context" 
    "os"
    "strconv"
    "fmt"
    "log"
    
    "feed-curator/utils"
    "feed-curator/models"

    "github.com/go-redis/redis/v8" 
    "github.com/joho/godotenv"
)

func InitRedis() {
        logFile, err := os.OpenFile("summarizer.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

    log.SetOutput(logFile)
	log.SetFlags(0)

    err = godotenv.Load()
    if err != nil {
        log.Printf("Warning: Error loading .env file: %v", err)
    }

    models.FreshNewsRedisCtx, models.FreshNewsRedisCtxCancel = context.WithCancel(context.Background())

    models.FreshNewsRedis, err = RedisInit(models.FreshNewsRedisCtx, "FRESHNEWS_REDIS_ADDRESS", "FRESHNEWS_REDIS_DB", "FRESHNEWS_REDIS_PASSWORD")
    if err != nil {
        utils.LogMessage("FRESH NEWS Redis Server Setup Failed!", "red", err)
    }
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
