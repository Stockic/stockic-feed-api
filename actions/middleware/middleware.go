package middleware

import (
	"fmt"
	"net/http"
	"time"

	"actions/utils"
    "actions/services"
)

func RequestMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(httpHandler http.ResponseWriter, request *http.Request) {
        startTime := time.Now() 

        apiKey := request.Header.Get("X-API-Key")
        if apiKey == "" {
            http.Error(httpHandler, "No API Key", http.StatusNotFound)
            utils.LogMessage("Users with no API Key tried to access", "red")
            return
        }

        var userExists, _ = services.ValidateUserAPIKey(apiKey)
        if !userExists {
            http.Error(httpHandler, "User doesn't exist", http.StatusNotFound)
            utils.LogMessage("User with no registeration tried to access", "red")
            return
        }

        next.ServeHTTP(httpHandler, request)
        duration := time.Since(startTime)
        logStatement := fmt.Sprintf("Request to %s took %v", request.URL.Path, duration)
        utils.LogMessage(logStatement, "green")
    }
}
