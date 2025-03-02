package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"feed-api/config"
	"feed-api/models"
	"feed-api/services"
	"feed-api/utils"
)

func Ping(httpHandler http.ResponseWriter, request *http.Request) {
    response := models.Greet{
        Response: "pong!",
    }

    httpHandler.Header().Set("Content-Type", "application/json")
    err := json.NewEncoder(httpHandler).Encode(response)
    if err != nil {
        utils.LogMessage("JSON Encoder in ping() Failed", "red", err)    
    }
}

func HeadlinesHandler(httpHandler http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 5 {
        utils.DeliverJsonError(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    pageSizeStr := pathParts[4]
    pageSize, err := strconv.Atoi(pageSizeStr)
    if err != nil || pageSize < 1 {
        utils.DeliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    headlinesData, err := config.RedisNewsCache.Get(config.RedisNewsCacheCtx, "headlines").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch headlines", http.StatusInternalServerError)
        return
    }

    var headlines map[string]models.SummarizedResponse
    if err := json.Unmarshal([]byte(headlinesData), &headlines); err != nil {
        http.Error(httpHandler, "Failed to parse headlines data", http.StatusInternalServerError)
        return
    }

    response := services.PaginateArticles(headlines["us"].Articles, 1, pageSize)
    
    httpHandler.Header().Set("Content-Type", "application/json")
    err = json.NewEncoder(httpHandler).Encode(response)
    if err != nil {
        utils.LogMessage("JSON Encoder in headlinesHandler() Failed", "red", err)
    }
}

func NewsFeedHandler(httpHandler http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 6 {
        utils.DeliverJsonError(httpHandler, "Invalid URL", http.StatusBadRequest)
        return
    }

    pageSizeStr := pathParts[4]
    pageSize, err := strconv.Atoi(pageSizeStr)
    if err != nil || pageSize < 1 {
        utils.DeliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    pageStr := pathParts[5]
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        utils.DeliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    // Fetch headlines from Redis
    headlinesData, err := config.RedisNewsCache.Get(config.RedisNewsCacheCtx, "headlines").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch news", http.StatusInternalServerError)
        return
    }

    var headlines map[string]models.SummarizedResponse
    if err := json.Unmarshal([]byte(headlinesData), &headlines); err != nil {
        http.Error(httpHandler, "Failed to parse news data", http.StatusInternalServerError)
        return
    }

    // Return paginated articles
    response := services.PaginateArticles(headlines["us"].Articles, page, pageSize)
    
    httpHandler.Header().Set("Content-Type", "application/json")
    err = json.NewEncoder(httpHandler).Encode(response)
    if err != nil {
        utils.LogMessage("JSON Encoder in newsFeedHandler() Failed", "red", err)
    }
}

func DiscoverHandler(httpHandler http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        http.Error(httpHandler, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    pathParts := strings.Split(request.URL.Path, "/")
    if len(pathParts) < 7 {
        utils.DeliverJsonError(httpHandler, "Less than 8 parts", http.StatusBadRequest)
        return
    }

    category := pathParts[4]
    if category == "" {
        utils.DeliverJsonError(httpHandler, "Category is empty", http.StatusBadRequest)
        return
    }

    pageSizeStr := pathParts[5]
    pageSize, err := strconv.Atoi(pageSizeStr)
    if err != nil || pageSize < 1 {
        utils.DeliverJsonError(httpHandler, "Invalid page size number", http.StatusBadRequest)
        return
    }

    pageStr := pathParts[6]
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        utils.DeliverJsonError(httpHandler, "Invalid page number", http.StatusBadRequest)
        return
    }

    discoverData, err := config.RedisNewsCache.Get(config.RedisNewsCacheCtx, "discover").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch category news", http.StatusInternalServerError)
        return
    }

    var categorizedNews map[string]models.SummarizedResponse
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

    response := services.PaginateArticles(categoryNews.Articles, page, pageSize)
    
    httpHandler.Header().Set("Content-Type", "application/json")
    err = json.NewEncoder(httpHandler).Encode(response)
    if err != nil {
        utils.LogMessage("JSON Encoder in discoverHandler() Failed", "red", err)
    }
}

func DetailHandler(httpHandler http.ResponseWriter, request *http.Request) {
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

    headlinesData, err := config.RedisNewsCache.Get(config.RedisNewsCacheCtx, "headlines").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch headlines", http.StatusInternalServerError)
        return
    }

    discoverData, err := config.RedisNewsCache.Get(config.RedisAPICacheCtx, "discover").Result()
    if err != nil {
        http.Error(httpHandler, "Failed to fetch discover news", http.StatusInternalServerError)
        return
    }

    article := services.FindArticleByID(newsID, headlinesData, discoverData)
    if article == nil {
        http.Error(httpHandler, "Article not found", http.StatusNotFound)
        return
    }

    httpHandler.Header().Set("Content-Type", "application/json")
    err = json.NewEncoder(httpHandler).Encode(article)
    if err != nil {
        utils.LogMessage("JSON Encoder in detailHandler() Failed", "red", err)
    }

    // Log Request
    utils.LogMessage("Started logging detail request", "green")
    apiKey := request.Header.Get("X-API-Key")
    redisKey := "endpoint:/detail/news:" + newsID + "/user:" + apiKey 
    utils.LogMessage(fmt.Sprintf("Redis Key:%s", redisKey), "green")

    go func() {
        err := config.RedisLog.Incr(context.Background(), redisKey).Err()
        if err != nil {
            utils.LogMessage(fmt.Sprintf("Failed to increment Redis key %s: %v", redisKey, err), "red")
        } else {
            utils.LogMessage(fmt.Sprintf("Successfully incremented Redis Key %s", redisKey), "green")
        }
    }()

    /*
    Write a goroutine function to add XPs to Firebase in 
    */
}

func FallbackHandler(httpHandler http.ResponseWriter, request *http.Request) {
    clientIP := utils.GetClientIP(request)
    utils.LogMessage(fmt.Sprintf("Intrusion IP detected: %s", clientIP), "red")
	if services.IsBlocked(clientIP) {
		http.Error(httpHandler, "You are Blocked", http.StatusForbidden)
		return
	}

    /* Now this would have been practical if I want to unblock
    suspected IPs after sometime like 1 hour. But since we know that
    legit IPs don't have any chance of accessing invalid endpoints, we 
    are blocking them permenantly and storing them in Firebase. 
    We can allow them from backend only in case we need to. */
    // blockIP(clientIP)

    utils.LogMessage(fmt.Sprintf("IP: %s wasn't blocked", clientIP), "red")

	err := services.SaveBlockedIPToFirebase(clientIP)
	if err != nil {
		utils.LogMessage("Failed to store blocked IP in Firebase", "red", err)
		http.Error(httpHandler, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Error(httpHandler, "Invalid Endpoint Accessed, You are Blocked", http.StatusNotFound)
	utils.LogMessage(fmt.Sprintf("Blocked IP: %s\n", clientIP), "green")
}
