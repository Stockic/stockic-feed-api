package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
    "bytes"

	"feed-api/config"
	"feed-api/models"
	"feed-api/utils"
    "feed-api/database"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
    "github.com/minio/minio-go/v7"
)

func PushAppLogToMinIO() {
    ticker := time.NewTicker(20 * time.Second) 
    defer ticker.Stop()
    utils.LogMessage("Started Log to MinIO Push Service", "green")

    for range ticker.C {
        utils.LogMessage("MinIO Push App Log Procedure started", "green") 
        err := database.UploadLogDataToMinIO(config.MinIOClient, "feed-api-app-logs", config.Logfile)
        if err != nil {
            utils.LogMessage("Error pushing log file to MinIO", "red", err)
        }
        utils.LogMessage("Done Uploading Procedure for App Logs to MinIO", "green")
    }
}

func ValidateUserAPIKey(apiKey string) (bool, bool) {
    // Check for Redis Cache -> If error == nil means Cache Hit for User -> Return the Existance and Premium Status
    if cachedStatus, err := GetCachedUserStatus(config.RedisAPICacheCtx, apiKey); err == nil {
        return cachedStatus.Exists, cachedStatus.Premium
    }

    // docRef is reference to the data specified by apiKey 
	docRef := config.FirebaseClient.Collection("users").Doc(apiKey)

    // docSnapshot is contains data of the user associated with the API Key
	docSnapshot, err := docRef.Get(config.FirebaseCtx)

    // If an error has been caught, it is now impossible to move forward and false, false is going to be served 
	if err != nil {
        // This means API Key is nowhere in the database -> Return false, false
		if status.Code(err) == codes.NotFound {
            utils.LogMessage("User Doesn't Exist in FireStore", "green", err)

            // Update cache for non existence  
            err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: false, Premium: false})
            if err != nil {
                utils.LogMessage("Failed to Cache", "red", err)
            }

			return false, false
		}

        /* 
        Not Found was not recieved, it's issue with Firebase connection so log it
        At this point, we don't know if user exists or not since connection is the issue
        Log this thing in Logs and we can't return anything specific
        It's I think better to return false, false for obvious reasons
        */

		utils.LogMessage("Failed to get document: %v", "red", err)
        return false, false 
	}

    /*
    If we reach here, no error has been returned and we have the docSnapshot
    Now we check if user exists or not.
    */
	if !docSnapshot.Exists() {
        // User does not exist! -> We update cache to set existence to false -> It would be better if we remove the key from Redis 
        err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: false, Premium: false})

        if err != nil {
            utils.LogMessage("Failed to Cache", "red", err)
        }
		return false, false
	}

    // Check for premium status 
	premiumStatus, ok := docSnapshot.Data()["premium-status"].(bool)
	if !ok {
        err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: true, Premium: false})
        if err != nil {
            utils.LogMessage("Failed to Cache", "red", err)
        }
		return true, false
	}

    // If user exists, store it to cache
    err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: true, Premium: premiumStatus})
    if err != nil {
        utils.LogMessage("Failed to Cache", "red", err)
    }

	return true, premiumStatus
}

func GetCachedUserStatus(ctx context.Context, apiKey string) (*models.UserStatus, error) {
    pong, err := config.RedisAPICache.Ping(ctx).Result()
    if err != nil {
        utils.LogMessage("Redis connection error", "red", err)
    } else {
        utils.LogMessage(fmt.Sprintf("Redis ping response: %s", pong), "green", nil)
    }

    val, err := config.RedisAPICache.Get(ctx, fmt.Sprintf("apikey:%s", apiKey)).Result()
    if err == redis.Nil {
        return nil, err
    } else if err != nil {
        return nil, err
    }
    
    var status models.UserStatus
    if err := json.Unmarshal([]byte(val), &status); err != nil {
        return nil, err
    }

    return &status, nil
}

func CacheUserStatus(ctx context.Context, apiKey string, status models.UserStatus) error {
    pong, err := config.RedisAPICache.Ping(ctx).Result()
    if err != nil {
        utils.LogMessage("Redis connection error", "red", err)
    } else {
        utils.LogMessage(fmt.Sprintf("Redis ping response: %s", pong), "green")
    }

    statusJson, err := json.Marshal(status)
    if err != nil {
        utils.LogMessage("Failed to marshal user status", "red", err)
        return err
    }
    
    err = config.RedisAPICache.Set(ctx, fmt.Sprintf("apikey:%s", apiKey), statusJson, config.APIKeyCacheExpiration).Err()
    if err != nil {
        utils.LogMessage("Failed to cache user status: %v", "red", err)
        return err
    } 

    return err
}

func SyncLogRedisToMinIO() {
    ticker := time.NewTicker(30 * time.Second) // Sync every 1 minute
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

        // Map to group logs by apiKey
        groupedLogs := make(map[string][]map[string]interface{})

        for _, key := range keys {
            parts := strings.Split(key, "/")
            if len(parts) < 4 {
                utils.LogMessage(fmt.Sprintf("Invalid key format: %s", key), "red", nil)
                continue
            }
            newsID := strings.TrimPrefix(parts[2], "news:")
            apiKey := strings.TrimPrefix(parts[3], "user:")

            // Get and reset the count atomically
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

        // Upload each apiKey's logs to MinIO
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

func FindArticleByID(id, headlinesData, discoverData string) *models.SummarizedArticle {
    var headlines, discover map[string]models.SummarizedResponse
    
    if err := json.Unmarshal([]byte(headlinesData), &headlines); err == nil {
        // Search in headlines
        for _, response := range headlines {
            for _, article := range response.Articles {
                if article.StockicID == id {
                    return &article
                }
            }
        }
    }

    if err := json.Unmarshal([]byte(discoverData), &discover); err == nil {
        // Search in discover news
        for _, response := range discover {
            for _, article := range response.Articles {
                if article.StockicID == id {
                    return &article
                }
            }
        }
    }

    return nil
}

func PaginateArticles(articles []models.SummarizedArticle, page, pageSize int) *models.SummarizedResponse {
    startIndex := (page - 1) * pageSize
    endIndex := startIndex + pageSize

    if startIndex >= len(articles) {
        return &models.SummarizedResponse{
            Status:       "ok",
            TotalResults: len(articles),
            Articles:     []models.SummarizedArticle{},
        }
    }

    if endIndex > len(articles) {
        endIndex = len(articles)
    }

    return &models.SummarizedResponse{
        Status:       "ok",
        TotalResults: len(articles),
        Articles:     articles[startIndex:endIndex],
    }
}

func IsBlocked(ip string) bool {
    utils.LogMessage("isBlocked() checking for blocked IPs", "red")
	if config.RedisAPICache.Exists(config.RedisAPICacheCtx, ip).Val() > 0 {
        utils.LogMessage("Blocked Cache Hit!", "green")
		return true
	}

	iter := config.FirebaseClient.Collection("blocked").Where("ip", "==", ip).Documents(config.FirebaseCtx)
	defer iter.Stop()

	if doc, err := iter.Next(); err == nil && doc.Exists() {
		go func() {
            err := config.RedisAPICache.Set(config.RedisAPICacheCtx, ip, "blocked", time.Hour).Err()
            if err != nil {
                utils.LogMessage("Failed to write firebase blocked IP to Redis", "red")
            }
		}()
		return true
	}

	return false
}

func SaveBlockedIPToFirebase(ip string) error {
    utils.LogMessage(fmt.Sprintf("Storing Blocked IP to Firebase: %s", ip), "green")
	doc := map[string]interface{}{
		"ip":        ip,
		"blockedAt": time.Now(),
	}

    _, _, err := config.FirebaseClient.Collection("blocked").Add(config.FirebaseCtx, doc)
	if err != nil {
		return fmt.Errorf("failed to store IP in Firebase: %w", err)
	}

	return nil
}
