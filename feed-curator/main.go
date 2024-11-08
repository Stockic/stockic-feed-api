package main

import (
    "fmt"
    "log"
    "context"
    "os"
    "encoding/json"
    "io"
    "net/http"

    "github.com/google/generative-ai-go/genai"
    "google.golang.org/api/option"
    "github.com/joho/godotenv"

)

var (
    geminiCtx context.Context
)

type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Article struct {
	Source      Source `json:"source"`
	Author      string `json:"author"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	URLToImage  string `json:"urlToImage"`
	PublishedAt string `json:"publishedAt"`
	Content     string `json:"content"`
}

type APIResponse struct {
	Status       string    `json:"status"`
	TotalResults int       `json:"totalResults"`
	Articles     []Article `json:"articles"`
}

func init() {
    
    logFile, err := os.OpenFile("summarizer.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
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

}

func printResponse(resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
}

func summarizer(modelName string, title string, text string) *genai.GenerateContentResponse {
    
    geminiCtx := context.Background()
    // Access your API key as an environment variable (see "Set up your API key" above)
    client, err := genai.NewClient(geminiCtx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    model := client.GenerativeModel(modelName)
    promptInput := fmt.Sprintf("Focus on content, main financial points and not on source/author. Summarize given news only called %s in the url: %s", title, text)
    response, err := model.GenerateContent(geminiCtx, genai.Text(promptInput))
    if err != nil {
        log.Fatal(err)
    }

    return response
}

func newsAPICaller(url string) APIResponse {

    resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

    var response APIResponse 

    err = json.Unmarshal(body, &response)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

    if response.Status != "ok" {
		log.Fatalf("Error: Received non-ok status: %s", response.Status)
	}

    return response
 
}

func newsAPIHeadlineCaller(country string, category string, page string, pageSize string) APIResponse {
    url := fmt.Sprintf("https://newsapi.org/v2/top-headlines?country=%s&category=%s&page=%s&pageSize=%s&apiKey", country, category, page, pageSize, os.Getenv("NEWSAPI_API_KEY"))

    return newsAPICaller(url)

}

// searchIn = title,description,content
func newsAPIEverything(q string, searchIn string, sortBy string, from string, to string, page string, pageSize string) APIResponse {
    url := fmt.Sprintf("https://newsapi.org/v2/everything?q=%s&searchIn=%s&sortBy=%s&from=%s&to=%s&page=%s&pageSize=%s&apiKey=%s", q, searchIn, sortBy, from, to, page, os.Getenv("NEWS_API_KEY"))

    return newsAPICaller(url)
}

func main() {

    url := fmt.Sprintf("https://newsapi.org/v2/everything?q=bitcoin&apiKey=%s", os.Getenv("NEWSAPI_API_KEY"))

    resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

    var response APIResponse 

    err = json.Unmarshal(body, &response)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

    if response.Status != "ok" {
		log.Fatalf("Error: Received non-ok status: %s", response.Status)
	}

    fmt.Printf("Total Results: %d\n\n", response.TotalResults)
	for _, article := range response.Articles {
		fmt.Printf("Title: %s\n", article.Title)
		fmt.Printf("Author: %s\n", article.Author)
		fmt.Printf("URL: %s\n", article.URL)
		fmt.Printf("Published At: %s\n", article.PublishedAt)
		fmt.Printf("Description: %s\n", article.Description)
		fmt.Println("---")
	}

    // printResponse(summarizer("gemini-1.5-pro", "Unmasking Bitcoin Creator Satoshi Nakamoto—Again", "https://www.wired.com/story/unmasking-bitcoin-creator-satoshi-nakamoto-again/"))
}
