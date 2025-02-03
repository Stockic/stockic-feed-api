package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"

	"log-redis-pusher/config"
	"log-redis-pusher/database"
	"log-redis-pusher/utils"
)

func PushAppLogToMinIO(wg *sync.WaitGroup) {
    defer wg.Done()

    ticker := time.NewTicker(20 * time.Second) 
    defer ticker.Stop()
    utils.LogMessage("Started Log to MinIO Push Service", "green")

    for range ticker.C {
        utils.LogMessage("MinIO Push App Log Procedure started", "green") 
        err := database.UploadLogDataToMinIO(config.MinIOClient, "actions-app-logs", config.Logfile)
        if err != nil {
            utils.LogMessage("Error pushing log file to MinIO", "red", err)
        }
        utils.LogMessage("Done Uploading Procedure for App Logs to MinIO", "green")
    }
}

func SyncLogRedisToMinIO(wg *sync.WaitGroup) {
    defer wg.Done()

    ticker := time.NewTicker(30 * time.Minute) // Sync every 1 minute
    defer ticker.Stop()
    utils.LogMessage("Started the Goroutine for MinIO Sync", "green")

    for range ticker.C {
        utils.LogMessage("Syncing Procedure: Log Redis to MinIO", "green")

        if _, err := config.RedisLog.Ping(config.RedisLogCtx).Result(); err != nil {
            utils.LogMessage("RedisLog connection failed - Cannot Sync to MinIO", "red", err)
            continue
        }

        keys, err := config.RedisLog.Keys(config.RedisLogCtx, "endpoint:/detail/news:*").Result()
        if err != nil {
            utils.LogMessage("Error fetching Redis keys", "red", err)
            continue
        }

        for _, key := range keys {
            fmt.Println(key)
        }

        groupedLogs := make(map[string][]map[string]interface{})

        for _, key := range keys {
            parts := strings.Split(key, "/")
            if len(parts) < 4 {
                utils.LogMessage(fmt.Sprintf("Invalid key format: %s", key), "red", nil)
                continue
            }
            newsID := strings.TrimPrefix(parts[2], "news:")
            apiKey := strings.TrimPrefix(parts[3], "user:")

            accessCount, err := config.RedisLog.GetDel(config.RedisLogCtx, key).Result()
            if err != nil {
                utils.LogMessage(fmt.Sprintf("Error fetching and deleting key %s", key), "red", err)
                continue
            }

            logData := map[string]interface{}{
                "newsID":      newsID,
                "accessCount": accessCount,
                "lastSynced":  time.Now().UTC().Format(time.RFC3339),
            }

            groupedLogs[apiKey] = append(groupedLogs[apiKey], logData)
        }

        for apiKey, logs := range groupedLogs {
            timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
            objectName := fmt.Sprintf("%s/detail-log-%s.json", apiKey, timestamp)

            jsonData, err := json.Marshal(logs)
            if err != nil {
                utils.LogMessage(fmt.Sprintf("Error marshaling JSON for apiKey %s", apiKey), "red", err)
                continue
            }

            _, err = config.MinIOClient.PutObject(
                config.MinIOCtx,
                "user-logs",
                objectName,
                bytes.NewReader(jsonData),
                int64(len(jsonData)),
                minio.PutObjectOptions{
                    ContentType: 
                    "application/json",
                },
            )
            if err != nil {
                utils.LogMessage(fmt.Sprintf("Error uploading JSON to MinIO for apiKey %s", apiKey), "red", err)
                continue
            }

            utils.LogMessage(fmt.Sprintf("Successfully uploaded logs for apiKey %s to MinIO", apiKey), "green")
        }
    }
}
