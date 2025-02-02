package main

import (
    "fmt"
    "net/http"

    "feed-api/config"
    "feed-api/handlers"
    "feed-api/services"
    "feed-api/middleware"
    "feed-api/database"
)

func init() {
    database.Initialize()
}

func main() {
    // go services.SyncLogRedisToFirebase()
    go services.SyncLogRedisToMinIO()
    go services.PushAppLogToMinIO()

    defer config.RedisAPICacheCtxCancel()
    defer config.RedisNewsCacheCtxCancel()
    defer config.RedisLogCtxCancel()

    setupRoutes()

    port := ":8080"
    fmt.Printf("\033[36m Starting server on port %s...\033[0m \n", port)
    err := http.ListenAndServe(port, nil)
    if err != nil {
        fmt.Printf("\033[31m Could not start server: %s \033[0m \n", err)
    }
}

func setupRoutes() {
    http.HandleFunc(config.VersionPrefix + "/ping", middleware.RequestMiddleware(handlers.Ping))
    http.HandleFunc(config.VersionPrefix + "/headlines/", middleware.RequestMiddleware(handlers.HeadlinesHandler))
    http.HandleFunc(config.VersionPrefix + "/newsfeed/", middleware.RequestMiddleware(handlers.NewsFeedHandler))
    http.HandleFunc(config.VersionPrefix + "/discover/", middleware.RequestMiddleware(handlers.DiscoverHandler))
    http.HandleFunc(config.VersionPrefix + "/detail/", middleware.RequestMiddleware(handlers.DetailHandler))
    http.HandleFunc("/", handlers.FallbackHandler)
}
