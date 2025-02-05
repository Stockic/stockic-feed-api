package main

// User actions backend logics are handled by this service
import (
	"fmt"
	"net/http"

	"actions/config"
	"actions/database"
	"actions/services"
    "actions/middleware"
    "actions/handlers"
)

// Firebase is gonna be used since it's user specific data
// MinIO for storing logs - app logs and user logs
func init() {
    database.Initialize()
}

func main() {   
    go services.PushAppLogToMinIO()

    setupRoutes()

    port := "localhost:9090"
    fmt.Printf("\033[36m Starting server on port %s...\033[0m \n", port)
    if err := http.ListenAndServe(port, nil); err != nil {
        fmt.Printf("\033[31m Could not start server: %s \033[0m \n", err)
    }
}

func setupRoutes() {
    http.HandleFunc(config.VersionPrefix + "/ping", middleware.RequestMiddleware(handlers.Ping))
    http.HandleFunc(config.VersionPrefix + "/bookmarks-add", middleware.RequestMiddleware(handlers.AddBookmarksHandlers))
    http.HandleFunc(config.VersionPrefix + "/bookmarks-remove", middleware.RequestMiddleware(handlers.RemoveBookmarks))
    http.HandleFunc(config.VersionPrefix + "/bookmarks-list", middleware.RequestMiddleware(handlers.ListBookmarks))
    http.HandleFunc(config.VersionPrefix + "/notion/oauth/auth-session", middleware.RequestMiddleware(handlers.OauthNotionCreateAuthSession))
    http.HandleFunc(config.VersionPrefix + "/notion/oauth/callback", handlers.OauthNotionCallback)
    http.HandleFunc("/", handlers.FallbackHandler)
}
