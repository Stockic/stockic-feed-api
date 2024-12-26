package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"feed-api/config"
	"feed-api/services"
	"feed-api/utils"
)

func RequestMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(httpHandler http.ResponseWriter, request *http.Request) {
        startTime := time.Now()
        
        apiKey := request.Header.Get("X-API-Key")
        if apiKey == "" {
            http.Error(httpHandler, "No API Key", http.StatusNotFound)
            utils.LogMessage("User with no API Key tried to access", "red")
            return
        }

        var userExists, _ = services.ValidateUserAPIKey(apiKey)
        if !userExists {
            http.Error(httpHandler, "User doesn't exist", http.StatusNotFound)
            utils.LogMessage("User with no registeration tried to access", "red")
            return
        }

        /*
        It's important to keep the premium status check in the code
        since this middleware is going to be used in premium integrations.

        PLEASE DO NOT REMOVE PREMIUM STATUS CHECKS
        */

        // Check if /detail/ is being accessed and store the newsID async to redis

        // Move this to discover endpoint

        urlPath := request.URL.Path
        if strings.HasPrefix(urlPath, fmt.Sprintf("%s/detail", config.VersionPrefix)) {
            utils.LogMessage("Started logging detail request", "green")
            parts := strings.Split(urlPath, "/")
            if len(parts) >= 5 {
                newsID := parts[4]

                redisKey := "endpoint:/detail/news:" + newsID + "/user:" + apiKey
                utils.LogMessage(fmt.Sprintf("Redis Key: %s", redisKey), "green")

                go func() {
                    err := config.RedisLog.Incr(context.Background(), redisKey).Err()
                    if err != nil {
                        utils.LogMessage(fmt.Sprintf("Failed to increment Redis key %s: %v", redisKey, err), "red")
                    } else {
                        utils.LogMessage(fmt.Sprintf("Successfully incremented Redis key %s", redisKey), "green")
                    }
                }()
            }
        }

        next.ServeHTTP(httpHandler, request)
        duration := time.Since(startTime)
        logStatement := fmt.Sprintf("Request to %s took %v", request.URL.Path, duration)
        utils.LogMessage(logStatement, "green")
    }
}
