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
    "context"
    "sync"
    
    "github.com/go-redis/redis/v8"
    "github.com/joho/godotenv"

    "firebase.google.com/go"
	"google.golang.org/api/option"
	"cloud.google.com/go/firestore"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Global variables for Redis Database
var (
    redisAPICacheCtx context.Context
    redisAPICacheCtxCancel context.CancelFunc

    redisNewsCacheCtx context.Context
    redisNewsCacheCtxCancel context.CancelFunc

    firebaseCtx context.Context

    redisAPICache *redis.Client
    redisNewsCache *redis.Client
    firebaseClient *firestore.Client

    once sync.Once
)


const (
    apiKeyCacheExpiration = 24 * time.Hour
)


type UserStatus struct {
    Exists  bool `json:"exists"`
    Premium bool `json:"premium"`
}

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

    logMessage("Before context setting", "")
    redisAPICacheCtx, redisAPICacheCtxCancel = context.WithCancel(context.Background())
    redisNewsCacheCtx, redisNewsCacheCtxCancel = context.WithCancel(context.Background()) 
    logMessage("After context setting", "")

    // User API Caching Redis Server
    redisAPICache = redisInit(redisAPICacheCtx, "USERAPI_CACHING_ADDRESS", "USERAPI_CACHING_DB", "USERAPI_CACHING_PASSWORD")

    // Fresh News Caching Redis Server
    redisNewsCache = redisInit(redisNewsCacheCtx, "NEWS_CACHING_ADDRESS", "NEWS_CACHING_DB", "NEWS_CACHING_PASSWORD")

    firebaseCtx = context.Background()

    firebaseClient := initializeFirebase("../secrets/stockic-b6c89-firebase-adminsdk-wr64l-0a181fa457.json")

    go func() {
		<-firebaseCtx.Done()
		firebaseClient.Close()
	}()

    go func() {
        // You might want to hook this to your app's shutdown logic
        defer redisAPICacheCtxCancel()
        defer redisNewsCacheCtxCancel()
    }()
}

func initializeFirebase(credentialsPath string) *firestore.Client {

	once.Do(func() {
		opt := option.WithCredentialsFile(credentialsPath)
		app, err := firebase.NewApp(firebaseCtx, nil, opt)
		if err != nil {
			log.Fatalf("Failed to initialize Firebase app: %v", err)
		}

		client, err := app.Firestore(firebaseCtx)
		if err != nil {
			log.Fatalf("Failed to create Firestore client: %v", err)
		}

		firebaseClient = client
	})

	return firebaseClient
}

func redisInit(redisContext context.Context, redisAddress string, redisDB string, redisPassword string) *redis.Client {

    address := os.Getenv(redisAddress)
    if address == "" {
        address = "localhost:6379"
    }

    dbStr := os.Getenv(redisDB)
    db, err := strconv.Atoi(dbStr)
    if err != nil {
        db = 0
        logMessage("Warning: Invalid REDIS_DB value, using default: 0", "red")
    }

    password := os.Getenv(redisPassword)

    rdb := redis.NewClient(&redis.Options{
        Addr:     address,
        Password: password,
        DB:       db,
    })

    _, err = rdb.Ping(redisContext).Result()
    if err != nil {
        logMessage(fmt.Sprintf("Failed to connect to Redis - Address: %s, redisDB: %s", address, dbStr), "red", err)
    }

    logMessage(fmt.Sprintf("Successfully initialized Redis: %s", address), "green")
    
    return rdb
}

// Logs messages on the console with color
func logMessage(message, color string, errs ...error) {

    var err error
	if len(errs) > 0 {
		err = errs[0]
	} else {
		err = nil
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
    
    log.Printf("[WIPER-LOG] [%s] %s ERROR: %v", timestamp, message, err)

    if color == "red" {
        fmt.Printf("\033[31m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    } else if color == "green" {
        fmt.Printf("\033[32m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    } else {
        fmt.Printf("\033[31m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
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


func getCachedUserStatus(ctx context.Context, apiKey string) (*UserStatus, error) {
    val, err := redisAPICache.Get(ctx, fmt.Sprintf("apikey:%s", apiKey)).Result()
    if err != nil {
        return nil, err
    }
    
    var status UserStatus
    if err := json.Unmarshal([]byte(val), &status); err != nil {
        return nil, err
    }
    return &status, nil
}

func cacheUserStatus(ctx context.Context, apiKey string, status UserStatus) {
    statusJson, err := json.Marshal(status)
    if err != nil {
        log.Printf("Failed to marshal user status: %v", err)
        return
    }
    
    // Store in Redis asynchronously
    go func() {
        err := redisAPICache.Set(ctx, fmt.Sprintf("apikey:%s", apiKey), statusJson, apiKeyCacheExpiration).Err()
        if err != nil {
            log.Printf("Failed to cache user status: %v", err)
        }
    }()
}

func validateUserAPIKey(apiKey string) (bool, bool) {

    logMessage("Validator started", "")

    if cachedStatus, err := getCachedUserStatus(redisAPICacheCtx, apiKey); err == nil {
        logMessage("Cache Hit!", "")
        return cachedStatus.Exists, cachedStatus.Premium
    }

    logMessage("Turning to firebase", "")
	docRef := firebaseClient.Collection("users").Doc(apiKey)
	docSnapshot, err := docRef.Get(firebaseCtx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
            cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: false, Premium: false})
			return false, false
		}
		log.Fatalf("Failed to get document: %v", err)
	}

	if !docSnapshot.Exists() {
        cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: false, Premium: false})
		return false, false
	}

	premiumStatus, ok := docSnapshot.Data()["premium-status"].(bool)
	if !ok {
        cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: true, Premium: false})
		return true, false
	}

    cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: true, Premium: premiumStatus})
	return true, premiumStatus
}

// Middleware for validating API Keys (Admin and User)
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(httpHandler http.ResponseWriter, request *http.Request) {
        
        startTime := time.Now()

        apiKey := request.Header.Get("X-API-Key")
        if apiKey == "" {
            deliverJsonError(httpHandler, "User API key is missing", http.StatusUnauthorized)
            logMessage("User with no API Key tried to access", "red")
            return
        }

        var userExists, isPremium = validateUserAPIKey(apiKey)
        if !userExists {
            deliverJsonError(httpHandler, "User doesn't exist", http.StatusUnauthorized)
            logMessage("User with no registeration tried to access", "red")
            return
        }

        if !isPremium {
            deliverJsonError(httpHandler, "User is not premium", http.StatusUnauthorized)
            return
        }

        // User exists and is premium
        next.ServeHTTP(httpHandler, request)
        duration := time.Since(startTime)
        logStatement := fmt.Sprintf("Request to %s took %v", request.URL.Path, duration)
        logMessage(logStatement, "green")
    }
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
    
    // Geolocation specific headlines endpoint
    // /api/<version>/headlines
    http.HandleFunc(versionPrefix + "/headlines", apiKeyMiddleware(headlinesHandler))

    // Geolocation specific pagenated newsfeed endpoint
    // /api/<version>/newsfeed/<page-number>
    http.HandleFunc(versionPrefix + "/newsfeed", apiKeyMiddleware(newsFeedHandler))

    // Category specific pagenated newsfeed endpoint
    // /api/<version>/discover/<category>/<page-number>
    http.HandleFunc(versionPrefix + "/discover", apiKeyMiddleware(discoverHandler))

    // Internal ID based detailed newsfeed endpoint
    // /api/<version>/detail/<news-id>
    http.HandleFunc(versionPrefix + "/detail", apiKeyMiddleware(detailHandler))
}

func headlinesHandler(httpHandler http.ResponseWriter, request *http.Request) {
    
    httpHandler.Header().Set("Content-Type", "application/json")
    
    // Define a struct for the response
    response := map[string]string{
        "status": "okay",
    }

    // Encode the response as JSON and write it to the response writer
    if err := json.NewEncoder(httpHandler).Encode(response); err != nil {
        http.Error(httpHandler, "Error encoding JSON response", http.StatusInternalServerError)
        return
    }
}

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

}

func discoverHandler(httpHandler http.ResponseWriter, request *http.Request) {
    
    // Extract page number from URL
    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 6 {
        http.Error(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    // categoryStr := pathParts[4]
    pageStr := pathParts[5]
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        http.Error(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    // Example response
    fmt.Fprintf(httpHandler, "News feed for page %d", page)

}

func detailHandler(httpHandler http.ResponseWriter, request *http.Request) {
    
    // Extract page number from URL
    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 5 {
        http.Error(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    newsIDStr := pathParts[4]
    newsID, err := strconv.Atoi(newsIDStr)
    if err != nil || newsID < 1 {
        http.Error(httpHandler, "Invalid news id", http.StatusBadRequest)
        return
    }

    // Example response
    fmt.Fprintf(httpHandler, "News feed for page %d", newsID)
}
