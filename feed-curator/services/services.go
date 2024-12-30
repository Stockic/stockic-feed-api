package services

import (
	"time"

	"github.com/go-redis/redis/v8"

	"feed-curator/database"
	"feed-curator/models"
	"feed-curator/utils"
)

/*
Function to push app logs and news-archive to MinIO
News-archival: First get a event message from Redis about the update of data
Then push the data into MinIO bucket called news-archive
App-logs: Here we store the logs of the curator service - We push the summarizer.log into
MinIO as a version controlled file in batch routines.
*/

func BucketStoreMinIOService() {
    go PushAppLogToMinIO()
    PushNewsToMinIOArchive()
}

func PushAppLogToMinIO() {
    ticker := time.NewTicker(20 * time.Second) 
    defer ticker.Stop()
    utils.LogMessage("Started Log to MinIO Push Service", "green")

    for range ticker.C {
        utils.LogMessage("MinIO Push App Log Procedure started", "green") 
        err := database.UploadLogDataToMinIO(models.MinIOClient, "app-logs", "summarizer.log")
        if err != nil {
            utils.LogMessage("Error pushing log file to MinIO", "red", err)
        }
        utils.LogMessage("Done Uploading Procedure for App Logs to MinIO", "green")
    }
}

// func PushNewsToMinIOArchive() {
// 
//     /*
//     Timer doesn't work here, I need to get an interrupt from Redis Database
//     once the feed is updated
//     */
// 
//     pubsub := models.FreshNewsRedis.PSubscribe(models.FreshNewsRedisCtx, models.RedisChannel)
//     defer pubsub.Close()
// 
//     fmt.Println("Listening for Redis Updates on the Channel")
// 
//     utils.LogMessage("Started News Archival Service", "green")
//     for msg := range pubsub.Channel() {
//         utils.LogMessage("Fresh News Redis Update Detected - Starting News Archival Procedure", "green")
//         go HandleRedisEvent(msg)
//     }
// }
// 
// func HandleRedisEvent(msg *redis.Message) {
//     key := strings.TrimPrefix(msg.Channel, "__keyspace@0__:*") 
//     if key == "" {
//         utils.LogMessage("Failed to extract key from event channel", "red")
//         return
//     }
// 
//     value, err := models.FreshNewsRedis.Get(models.FreshNewsRedisCtx, key).Result()
//     if err != nil {
//         utils.LogMessage("Failed to get key", "red", err)
//         return
//     }
// 
//     err = database.UploadRedisDataToMinIO(models.MinIOClient, key, value, "news-archive")
//     if err != nil {
//         utils.LogMessage("Failed to upload key to MiniIO", "red", err)
//     }
// }

func PushNewsToMinIOArchive() {
    pubsub := models.FreshNewsRedis.PSubscribe(models.FreshNewsRedisCtx, "__keyspace@0__:headlines")
    defer pubsub.Close()
    
    utils.LogMessage("Started News Archival Service - Monitoring headlines", "green")
    
    for msg := range pubsub.Channel() {
        if msg.Payload == "set" || msg.Payload == "hset" {
            utils.LogMessage("Headlines update detected - Starting archival", "green")
            go HandleRedisEvent(msg)
        }
    }
}

func HandleRedisEvent(msg *redis.Message) {
    key := "headlines"
    value, err := models.FreshNewsRedis.Get(models.FreshNewsRedisCtx, key).Result()
    if err != nil {
        utils.LogMessage("Failed to get headlines", "red", err)
        return
    }
    
    err = database.UploadRedisDataToMinIO(models.MinIOClient, key, value, "news-archive")
    if err != nil {
        utils.LogMessage("Failed to upload headlines to MinIO", "red", err)
    }
}
