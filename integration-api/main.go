package main 

import (
    "fmt"
    "net/http"

    "integration-api/notion"
)

func main() {

    setupRoutes()

    port := ":4040"

    err := http.ListenAndServe(port, nil)
    if err != nil {
        fmt.Printf("Failed to run the server")
    }

}

func setupRoutes() {
    http.HandleFunc("/integration", notion.AuthenticateNotion)
    http.HandleFunc("/write", notion.WriteToNotionDatabase)
}
