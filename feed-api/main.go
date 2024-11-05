package main

import (
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "encoding/json"
    "time"
    "log"
    "os"
    
    "github.com/go-redis/redis/v8"
    "github.com/joho/godotenv"
)

// Global variables for Redis Database
var (
    rdb *redis.Client
)

// Init Function - For initial configurations and setting up Redis Database
func init() {

    // Setup Logging files and configurations
    logFile, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	log.SetOutput(logFile)
	log.SetFlags(0)

    // Load Environment Variables with .env file
    err = godotenv.Load()
    if err != nil {
        log.Printf("Warning: Error loading .env file: %v", err)
    }
    
    // Load API key from .env file 
    if os.Getenv("ADMIN_API_KEY") == "" {
        log.Fatal("ADMIN_API_KEY is not set in the environment")
    }

    // Extracting Redis configuration from environment variables and setting it up
    password := os.Getenv("REDIS_PASSWORD")

    dbStr := os.Getenv("REDIS_DB")
    db, err := strconv.Atoi(dbStr)
    if err != nil {
        // Default Value
        db = 0          
        log.Printf("Warning: Invalid REDIS_DB value, using default: 0")
    }

    address := os.Getenv("REDIS_ADDRESS")
    if address == "" {
        address = "localhost:6379" // local database default value  
    }

    rdb = redis.NewClient(&redis.Options{
        Addr:     address,
        Password: password,
        DB:       db,
    })

    _, err = rdb.Ping(rdb.Context()).Result()
    if err != nil {
        log.Fatalf("Failed to connect to Redis: %v", err)
    }

    logMessage("Redis client initialized successfully", "green")
}

// Check if a given string is JSON
func isValidJSON(str string) bool {
    var js json.RawMessage
    return json.Unmarshal([]byte(str), &js) == nil
}

// Logs messages on the console with color
func logMessage(message, color string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
    
    log.Printf("[WIPER-LOG] [%s] %s", timestamp, message)

    if color == "red" {
        fmt.Printf("\033[31m [%s] %s \033[0m \n", timestamp, message)
    } else if color == "green" {
        fmt.Printf("\033[32m [%s] %s \033[0m \n", timestamp, message)
    } else {
        fmt.Printf("\033[31m [%s] %s \033[0m \n", timestamp, message)
    }

}

// Delivers JSON Error to the user in cases of errors
func deliverJsonError(httpHandler http.ResponseWriter, message string, statusCode int) {
    httpHandler.Header().Set("Content-Type", "application/json")
    httpHandler.WriteHeader(statusCode)
    jsonResponse := map[string]string{"error": message}
    
    logMessage(message, "red")

    jsonEncoder := json.NewEncoder(httpHandler) 
    if err := jsonEncoder.Encode(jsonResponse); err != nil {
        logMessage("jsonError: Failed to encode JSON response: %v" + err.Error(), "red")
		http.Error(httpHandler, `{"error": "internal server error"}`, http.StatusInternalServerError)
    }

}

// Validate User API Key - From Database
func validateUserAPIKey(apiKey string) bool {
    return false
}

// Handler for Newsfeed API Endpoint
func newsFeedHandler(httpHandler http.ResponseWriter, request *http.Request) {

    // Extract page number from URL
    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 5 {
        http.Error(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    pageStr := pathParts[4]
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        http.Error(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    // Example response
    fmt.Fprintf(httpHandler, "News feed for page %d", page)

    // News would be fetched through Redis Server
}

// Middleware for validating API Keys (Admin and User)
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        
        apiKey := r.Header.Get("X-API-Key")
        if apiKey == "" {
            deliverJsonError(w, "User API key is missing", http.StatusUnauthorized)
            return
        }

        if !validateUserAPIKey(apiKey) {
            deliverJsonError(w, "Invalid User API key", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    }
}

// Validate Admin API Key
func validateAdminAPIKey(apiKey string) bool {
    validKey := os.Getenv("ADMIN_API_KEY")
    return apiKey == validKey
}

func main() {
    setupRoutes()

    port := ":80"
    fmt.Printf("\033[36m Starting server on port %s...\033[0m \n", port)
    err := http.ListenAndServe(port, nil)
    if err != nil {
        fmt.Printf("\033[31m Could not start server: %s \033[0m \n", err)
    }
}

// Setting up API endpoints
func setupRoutes() {
    versionPrefix := "/api/v1"    
    
    // Endpoint for news feed with pagination, returns JSON data with newsfeed - User Privilege
    http.HandleFunc(versionPrefix + "/newsfeed/page", apiKeyMiddleware(newsFeedHandler))
}
