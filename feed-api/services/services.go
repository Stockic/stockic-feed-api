package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"feed-api/config"
	"feed-api/models"
	"feed-api/utils"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateUserAPIKey(apiKey string) (bool, bool) {
    if cachedStatus, err := GetCachedUserStatus(config.RedisAPICacheCtx, apiKey); err == nil {
        return cachedStatus.Exists, cachedStatus.Premium
    }

	docRef := config.FirebaseClient.Collection("users").Doc(apiKey)
	docSnapshot, err := docRef.Get(config.FirebaseCtx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
            err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: false, Premium: false})
            if err != nil {
                utils.LogMessage("Failed to Cache", "red", err)
            }
			return false, false
		}
		log.Fatalf("Failed to get document: %v", err)
	}

	if !docSnapshot.Exists() {
        err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: false, Premium: false})
        if err != nil {
            utils.LogMessage("Failed to Cache", "red", err)
        }
		return false, false
	}

	premiumStatus, ok := docSnapshot.Data()["premium-status"].(bool)
	if !ok {
        err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: true, Premium: false})
        if err != nil {
            utils.LogMessage("Failed to Cache", "red", err)
        }
		return true, false
	}

    err = CacheUserStatus(config.RedisAPICacheCtx, apiKey, models.UserStatus{Exists: true, Premium: premiumStatus})
    if err != nil {
        utils.LogMessage("Failed to Cache", "red", err)
    }

	return true, premiumStatus
}

func GetCachedUserStatus(ctx context.Context, apiKey string) (*models.UserStatus, error) {
    pong, err := config.RedisAPICache.Ping(ctx).Result()
    if err != nil {
        log.Printf("Redis connection error: %v", err)
    } else {
        log.Printf("Redis ping response: %s", pong)
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
        log.Printf("Redis connection error: %v", err)
        utils.LogMessage("Redis connection error", "red", err)
    } else {
        log.Printf("Redis ping response: %s", pong)
        utils.LogMessage(fmt.Sprintf("Redis ping response: %s", pong), "green")
    }

    statusJson, err := json.Marshal(status)
    if err != nil {
        utils.LogMessage("Failed to marshal user status", "red", err)
        return err
    }
    
    // Store in Redis asynchronously
    err = config.RedisAPICache.Set(ctx, fmt.Sprintf("apikey:%s", apiKey), statusJson, config.APIKeyCacheExpiration).Err()
    if err != nil {
        utils.LogMessage("Failed to cache user status: %v", "red", err)
        return err
    } 

    return err
}

// services/sync_service.go
func SyncLogRedisToFirebase() {
    ticker := time.NewTicker(1 * time.Minute) // Sync every 1 minutes
	defer ticker.Stop()

    utils.LogMessage("Started the Goroutine for Firebase Sync", "green")
	for range ticker.C {
        utils.LogMessage("Syncing Procedure: Log Redis to Firebase", "green")
		keys, err := config.RedisLog.Keys(config.RedisLogCtx, "endpoint:/detail/news:*").Result()
		if err != nil {
			log.Printf("Error fetching Redis keys: %v", err)
			continue
		}

		for _, key := range keys {
			parts := strings.Split(key, "/")
			if len(parts) < 4 {
				log.Printf("Invalid key format: %s", key)
				continue
			}
			newsID := strings.TrimPrefix(parts[2], "news:")
			apiKey := strings.TrimPrefix(parts[3], "user:")
            utils.LogMessage(fmt.Sprintf("Pushing Redis Keys to Firebase: News ID: %s, API Key: %s", newsID, apiKey), "green")

			// Get and reset the count atomically
			accessCount, err := config.RedisLog.GetDel(config.RedisLogCtx, key).Result()
			if err != nil {
				log.Printf("Error fetching and deleting key %s: %v", key, err)
				continue
			}

			// Create the log data for Firestore
			logData := map[string]interface{}{
				"accessCount": accessCount,
				"lastSynced":  time.Now().UTC().Format(time.RFC3339),
			}

			// Push log to Firestore under the user's API Key
			_, err = config.FirebaseClient.Collection("users").Doc(apiKey).Collection("logs").Doc(newsID).Set(config.FirebaseCtx, logData)
			if err != nil {
				utils.LogMessage(fmt.Sprintf("Error writing to Firestore for key %s", key), "red", err)
				config.RedisLog.Set(config.RedisLogCtx, key, accessCount, 0)
				continue
			}
		}
	}
}

// services/article_service.go
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
