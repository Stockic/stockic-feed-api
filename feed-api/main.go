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

type SummarizedArticle struct {
    StockicID           string `json:"stockicID"`
    Source              string `json:"source"`
	Author              string `json:"author"`
	Title               string `json:"title"`
	// Description string `json:"description"`
	URL                 string `json:"url"`
	URLToImage          string `json:"urlToImage"`
	PublishedAt         string `json:"publishedAt"`
	SummarizedContent   string `json:"content"`
}

type SummarizedResponse struct {
	Status       string                 `json:"status"`
	TotalResults int                    `json:"totalResults"`
	Articles     []SummarizedArticle    `json:"articles"`
}

type Greet struct {
    Response    string  `json:"response"`
}

// Global variables for Redis Database
var (
    redisAPICacheCtx context.Context
    redisAPICacheCtxCancel context.CancelFunc

    redisNewsCacheCtx context.Context
    redisNewsCacheCtxCancel context.CancelFunc

    redisLogCtx context.Context
    redisLogCtxCancel context.CancelFunc

    firebaseCtx context.Context

    redisAPICache *redis.Client
    redisNewsCache *redis.Client
    redisLog *redis.Client
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

    redisAPICacheCtx, redisAPICacheCtxCancel = context.WithCancel(context.Background())
    redisNewsCacheCtx, redisNewsCacheCtxCancel = context.WithCancel(context.Background())
    redisLogCtx, redisLogCtxCancel = context.WithCancel(context.Background())

    // User API Caching Redis Server
    redisAPICache, err = redisInit(redisAPICacheCtx, "USERAPI_CACHING_ADDRESS", "USERAPI_CACHING_DB", "USERAPI_CACHING_PASSWORD")
    if err != nil {
        logMessage("API Cache Server Setup Failed!", "red", err)
    }

    // Fresh News Caching Redis Server
    redisNewsCache, err = redisInit(redisNewsCacheCtx, "NEWS_CACHING_ADDRESS", "NEWS_CACHING_DB", "NEWS_CACHING_PASSWORD")
    if err != nil {
        logMessage("News Cache Server Setup Failed", "red", err)
    }

    redisLog, err = redisInit(redisLogCtx, "LOGREDIS_ADDRESS", "LOGREDIS_DB", "LOGREDIS_PASSWORD")
    if err != nil {
        logMessage("Log Redis Server Setup Failed", "red", err)
    }

    firebaseCtx = context.Background()

    firebaseClient := initializeFirebase("./secrets/stockic-b6c89-firebase-adminsdk-wr64l-a8e3bdf5e7.json")

    go func() {
		<-firebaseCtx.Done()
		firebaseClient.Close()
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

func redisInit(redisContext context.Context, redisAddress string, redisDB string, redisPassword string) (*redis.Client, error) {

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
        return nil, err
    }

    logMessage(fmt.Sprintf("Successfully initialized Redis: %s", address), "green")
    
    return rdb, err
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

    pong, err := redisAPICache.Ping(ctx).Result()
    if err != nil {
        log.Printf("Redis connection error: %v", err)
    } else {
        log.Printf("Redis ping response: %s", pong)
    }

    val, err := redisAPICache.Get(ctx, fmt.Sprintf("apikey:%s", apiKey)).Result()
    if err == redis.Nil {
        return nil, err
    } else if err != nil {
        return nil, err
    }
    
    var status UserStatus
    if err := json.Unmarshal([]byte(val), &status); err != nil {
        return nil, err
    }

    return &status, nil
}

func cacheUserStatus(ctx context.Context, apiKey string, status UserStatus) error {

    pong, err := redisAPICache.Ping(ctx).Result()
    if err != nil {
        log.Printf("Redis connection error: %v", err)
        logMessage("Redis connection error", "red", err)
    } else {
        log.Printf("Redis ping response: %s", pong)
        logMessage(fmt.Sprintf("Redis ping response: %s", pong), "green")
    }

    statusJson, err := json.Marshal(status)
    if err != nil {
        logMessage("Failed to marshal user status", "red", err)
        return err
    }
    
    // Store in Redis asynchronously
    err = redisAPICache.Set(ctx, fmt.Sprintf("apikey:%s", apiKey), statusJson, apiKeyCacheExpiration).Err()
    if err != nil {
        logMessage("Failed to cache user status: %v", "red", err)
        return err
    } 

    return err
}

func validateUserAPIKey(apiKey string) (bool, bool) {

    if cachedStatus, err := getCachedUserStatus(redisAPICacheCtx, apiKey); err == nil {
        // logMessage("Cache Hit!", "")
        return cachedStatus.Exists, cachedStatus.Premium
    }

	docRef := firebaseClient.Collection("users").Doc(apiKey)
	docSnapshot, err := docRef.Get(firebaseCtx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
            err = cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: false, Premium: false})
            if err != nil {
                logMessage("Failed to Cache", "red", err)
            }
			return false, false
		}
		log.Fatalf("Failed to get document: %v", err)
	}

	if !docSnapshot.Exists() {
        err = cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: false, Premium: false})
        if err != nil {
            logMessage("Failed to Cache", "red", err)
        }
		return false, false
	}

	premiumStatus, ok := docSnapshot.Data()["premium-status"].(bool)
	if !ok {
        err = cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: true, Premium: false})
        if err != nil {
            logMessage("Failed to Cache", "red", err)
        }
		return true, false
	}

    err = cacheUserStatus(redisAPICacheCtx, apiKey, UserStatus{Exists: true, Premium: premiumStatus})
    if err != nil {
        logMessage("Failed to Cache", "red", err)
    }

	return true, premiumStatus
}

// Syncing Log Redis to Firebase 
func SyncLogRedisToFirebase() {
	ticker := time.NewTicker(1 * time.Minute) // Sync every 1 minutes
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()

		keys, err := redisLog.Keys(ctx, "endpoint:/detail/news:*").Result()
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
			newsID := strings.TrimPrefix(parts[1], "news:")
			apiKey := strings.TrimPrefix(parts[3], "user:")

			// Get and reset the count atomically
			accessCount, err := redisLog.GetDel(ctx, key).Result()
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
			_, err = firebaseClient.Collection("Users").Doc(apiKey).Collection("logs").Doc(newsID).Set(firebaseCtx, logData)
			if err != nil {
				logMessage(fmt.Sprintf("Error writing to Firestore for key %s", key), "red", err)
				redisLog.Set(ctx, key, accessCount, 0)
				continue
			}
		}
	}
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
            logMessage("Non-Premium user is trying to access", "red")
            return
        }

        // Check if /detail/ is being accessed and store the newsID async to redis
        urlPath := request.URL.Path
        if strings.HasPrefix(urlPath, "/detail/") {
            parts := strings.Split(urlPath, "/")
            if len(parts) >= 3 {
                newsID := parts[2]

                redisKey := "endpoint:/detail/news:" + newsID + "/user:" + apiKey

                go func() {
                    err := redisLog.Incr(context.Background(), redisKey).Err()
                    if err != nil {
                        logMessage(fmt.Sprintf("Failed to increment Redis key %s: %v", redisKey, err), "red")
                    } else {
                        logMessage(fmt.Sprintf("Successfully incremented Redis key %s", redisKey), "green")
                    }
                }()
            }
        }

        // User exists and is premium
        next.ServeHTTP(httpHandler, request)
        duration := time.Since(startTime)
        logStatement := fmt.Sprintf("Request to %s took %v", request.URL.Path, duration)
        logMessage(logStatement, "green")
    }
}

// Helper function to paginate articles
func paginateArticles(articles []SummarizedArticle, page, pageSize int) *SummarizedResponse {
    startIndex := (page - 1) * pageSize
    endIndex := startIndex + pageSize

    if startIndex >= len(articles) {
        return &SummarizedResponse{
            Status:       "ok",
            TotalResults: len(articles),
            Articles:     []SummarizedArticle{},
        }
    }

    if endIndex > len(articles) {
        endIndex = len(articles)
    }

    return &SummarizedResponse{
        Status:       "ok",
        TotalResults: len(articles),
        Articles:     articles[startIndex:endIndex],
    }
}

func findArticleByID(id, headlinesData, discoverData string) *SummarizedArticle {
    var headlines, discover map[string]SummarizedResponse
    
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

func main() {

    go SyncLogRedisToFirebase()

    setupRoutes()

    port := ":8080"
    fmt.Printf("\033[36m Starting server on port %s...\033[0m \n", port)
    err := http.ListenAndServe(port, nil)
    if err != nil {
        fmt.Printf("\033[31m Could not start server: %s \033[0m \n", err)
    }

    defer redisAPICacheCtxCancel()
    defer redisNewsCacheCtxCancel()
}

// Setting up API endpoints
func setupRoutes() {
    versionPrefix := "/api/v1"    

    http.HandleFunc(versionPrefix + "/ping", apiKeyMiddleware(greeter))
    
    // Geolocation specific headlines endpoint
    // /api/<version>/headlines/<page-size>
    http.HandleFunc(versionPrefix + "/headlines/", apiKeyMiddleware(headlinesHandler))

    // Geolocation specific pagenated newsfeed endpoint
    // /api/<version>/newsfeed/<page-size>/<page-number>
    http.HandleFunc(versionPrefix + "/newsfeed/", apiKeyMiddleware(newsFeedHandler))

    // Category specific pagenated newsfeed endpoint
    // /api/<version>/discover/<category>/<page-number>/<page-number>
    http.HandleFunc(versionPrefix + "/discover/", apiKeyMiddleware(discoverHandler))

    // Internal ID based detailed newsfeed endpoint
    // /api/<version>/detail/<news-id>
    http.HandleFunc(versionPrefix + "/detail/", apiKeyMiddleware(detailHandler))
}

func greeter(httpHandler http.ResponseWriter, request *http.Request) {
    response := Greet{
        Response: "pong! does it work?",
    }

    httpHandler.Header().Set("Content-Type", "application/json")
    json.NewEncoder(httpHandler).Encode(response)
}

func headlinesHandler(httpHandler http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 5 {
        deliverJsonError(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    pageSizeStr := pathParts[4]
    pageSize, err := strconv.Atoi(pageSizeStr)
    if err != nil || pageSize < 1 {
        deliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    headlinesData, err := redisNewsCache.Get(redisNewsCacheCtx, "headlines").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch headlines", http.StatusInternalServerError)
        return
    }

    var headlines map[string]SummarizedResponse
    if err := json.Unmarshal([]byte(headlinesData), &headlines); err != nil {
        http.Error(httpHandler, "Failed to parse headlines data", http.StatusInternalServerError)
        return
    }

    response := paginateArticles(headlines["us"].Articles, 1, pageSize)
    
    httpHandler.Header().Set("Content-Type", "application/json")
    json.NewEncoder(httpHandler).Encode(response)
}

// Newsfeed handler - returns paginated news
func newsFeedHandler(httpHandler http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 6 {
        deliverJsonError(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    pageSizeStr := pathParts[4]
    pageSize, err := strconv.Atoi(pageSizeStr)
    if err != nil || pageSize < 1 {
        deliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    pageStr := pathParts[5]
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        deliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    // Fetch headlines from Redis
    headlinesData, err := redisNewsCache.Get(redisNewsCacheCtx, "headlines").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch news", http.StatusInternalServerError)
        return
    }

    var headlines map[string]SummarizedResponse
    if err := json.Unmarshal([]byte(headlinesData), &headlines); err != nil {
        http.Error(httpHandler, "Failed to parse news data", http.StatusInternalServerError)
        return
    }

    // Return paginated articles
    response := paginateArticles(headlines["us"].Articles, page, pageSize)
    
    httpHandler.Header().Set("Content-Type", "application/json")
    json.NewEncoder(httpHandler).Encode(response)
}

func discoverHandler(httpHandler http.ResponseWriter, request *http.Request) {

    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 7 {
        deliverJsonError(httpHandler, "Less than 8 parts", http.StatusBadRequest)
        return
    }

    category := pathParts[4]
    if category == "" {
        deliverJsonError(httpHandler, "Category is empty", http.StatusBadRequest)
        return
    }

    pageSizeStr := pathParts[5]
    pageSize, err := strconv.Atoi(pageSizeStr)
    if err != nil || pageSize < 1 {
        deliverJsonError(httpHandler, "Invalid page size number", http.StatusBadRequest)
        return
    }

    pageStr := pathParts[6]
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        deliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    discoverData, err := redisNewsCache.Get(redisNewsCacheCtx, "discover").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch category news", http.StatusInternalServerError)
        return
    }

    var categorizedNews map[string]SummarizedResponse
    if err := json.Unmarshal([]byte(discoverData), &categorizedNews); err != nil {
        http.Error(httpHandler, "Failed to parse category news data", http.StatusInternalServerError)
        return
    }

    fmt.Println(categorizedNews)

    categoryNews, exists := categorizedNews[category]
    if !exists {
        http.Error(httpHandler, "Category not found", http.StatusNotFound)
        return
    }

    response := paginateArticles(categoryNews.Articles, page, pageSize)
    
    httpHandler.Header().Set("Content-Type", "application/json")
    json.NewEncoder(httpHandler).Encode(response)
}

func detailHandler(httpHandler http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 5 {
        http.Error(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    newsID := pathParts[len(pathParts)-1]

    headlinesData, err := redisNewsCache.Get(redisNewsCacheCtx, "headlines").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch headlines", http.StatusInternalServerError)
        return
    }

    discoverData, err := redisNewsCache.Get(redisAPICacheCtx, "discover").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch discover news", http.StatusInternalServerError)
        return
    }

    article := findArticleByID(newsID, headlinesData, discoverData)
    if article == nil {
        http.Error(httpHandler, "Article not found", http.StatusNotFound)
        return
    }

    httpHandler.Header().Set("Content-Type", "application/json")
    json.NewEncoder(httpHandler).Encode(article)
}
