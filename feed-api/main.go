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
    rdb *redis.Client
    firebaseClient *firestore.Client
    once sync.Once
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

    // User API Caching Redis Server
    redisInit("USERAPI-CACHING-API-KEY", "USERAPI-CACHING-DB", "USERAPI-CACHING-PASSWORD")

    // Fresh News Caching Redis Server
    redisInit("NEWS-CACHING-API-KEY", "NEWS-CACHING-DB", "NEWS-CACHING-PASSWORD")

    firebaseClient := initializeFirebase("../secrets/stockic-b6c89-firebase-adminsdk-wr64l-cb6a7b150d.json")

    go func() {
		<-context.Background().Done()
		firebaseClient.Close()
	}()
}

func initializeFirebase(credentialsPath string) *firestore.Client {

	once.Do(func() {
		opt := option.WithCredentialsFile(credentialsPath)
		app, err := firebase.NewApp(context.Background(), nil, opt)
		if err != nil {
			log.Fatalf("Failed to initialize Firebase app: %v", err)
		}

		client, err := app.Firestore(context.Background())
		if err != nil {
			log.Fatalf("Failed to create Firestore client: %v", err)
		}

		firebaseClient = client
	})

	return firebaseClient
}

func redisInit(redisAddress string, redisDB string, redisPassword string) {

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

    rdb = redis.NewClient(&redis.Options{
        Addr:     address,
        Password: password,
        DB:       db,
    })

    _, err = rdb.Ping(rdb.Context()).Result()
    if err != nil {
        logMessage("Failed to connect to Redis", "red", err)
    }

    logMessage("Redis client initialized successfully", "green")

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

func validateUserAPIKey(apiKey string) (bool, bool) {

	docRef := firebaseClient.Collection("users").Doc(apiKey)
	docSnapshot, err := docRef.Get(context.Background())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, false
		}
		log.Fatalf("Failed to get document: %v", err)
	}

	if !docSnapshot.Exists() {
		return false, false
	}

	premiumStatus, ok := docSnapshot.Data()["premium-status"].(bool)
	if !ok {
		return true, false
	}

	return true, premiumStatus
}

// Middleware for validating API Keys (Admin and User)
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(httpHandler http.ResponseWriter, request *http.Request) {
        
        apiKey := request.Header.Get("X-API-Key")
        if apiKey == "" {
            deliverJsonError(httpHandler, "User API key is missing", http.StatusUnauthorized)
            return
        }

        var userExists, isPremium = validateUserAPIKey(apiKey)
        if !userExists {
            deliverJsonError(httpHandler, "User doesn't exist", http.StatusUnauthorized)
            return
        }

        if !isPremium {
            deliverJsonError(httpHandler, "User is not premium", http.StatusUnauthorized)
            return
        }

        // User exists and is premium
        next.ServeHTTP(httpHandler, request)
    }
}

func main() {

    client := initializeFirebase("../secrets/stockic-b6c89-firebase-adminsdk-wr64l-cb6a7b150d.json")

	defer func() {
		if err := client.Close(); err != nil {
			log.Fatalf("Failed to close Firestore client: %v", err)
		}
	}()

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
