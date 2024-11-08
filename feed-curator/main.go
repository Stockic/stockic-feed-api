package main

import (
    "fmt"
    "log"
    "context"
    "os"

    "github.com/google/generative-ai-go/genai"
    "google.golang.org/api/option"

)

func main() {

    ctx := context.Background()
    // Access your API key as an environment variable (see "Set up your API key" above)
    client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("API_KEY")))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    model := client.GenerativeModel("gemini-1.5-flash")
    resp, err := model.GenerateContent(ctx, genai.Text("Write a story about a magic backpack."))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Printing Response")
    fmt.Println(resp)
}
