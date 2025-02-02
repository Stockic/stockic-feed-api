package services

import (
   	"context"
	"encoding/json"
	"fmt"
    "time"
    
    "actions/utils"
    "actions/config"
    "actions/models"
    "actions/database"

    "github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func PushAppLogToMinIO() {
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
