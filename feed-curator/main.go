package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
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

type SummarizedArticle struct {
    stockicID           string `json:"stockicID"`
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

func newsAPICaller(url string) (APIResponse, error) {
    var response APIResponse

    resp, err := http.Get(url)
    if err != nil {
        log.Printf("Error making request: %v", err)
        return response, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        log.Printf("Received non-200 status code: %d", resp.StatusCode)
        return response, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Error reading response body: %v", err)
        return response, err
    }

    err = json.Unmarshal(body, &response)
    if err != nil {
        log.Printf("Error unmarshalling JSON: %v", err)
        return response, err
    }

    if response.Status != "ok" {
        log.Printf("Error: Received non-ok status: %s", response.Status)
        return response, fmt.Errorf("non-ok status: %s", response.Status)
    }

    return response, nil
}

func newsAPIHeadlineCaller(country string, page string, pageSize string) (APIResponse, error) {
    url := fmt.Sprintf(
        "https://newsapi.org/v2/top-headlines?country=%s&page=%s&pageSize=%s&apiKey=%s",
        country, page, pageSize, os.Getenv("NEWSAPI_API_KEY"),
    )
    return newsAPICaller(url)
}

func newsAPIEverything(q, searchIn, language, sortBy, from, to, page, pageSize string) (APIResponse, error) {
    url := fmt.Sprintf(
        "https://newsapi.org/v2/everything?q=%s&searchIn=%s&language=%s&sortBy=%s&from=%s&to=%s&page=%s&pageSize=%s&apiKey=%s",
        q, searchIn, language, sortBy, from, to, page, pageSize, os.Getenv("NEWSAPI_API_KEY"),
    )

    return newsAPICaller(url)
}

func fetchHeadlinesByCountry(countries []string, pageSize string) map[string]APIResponse {
    countryHeadlines := make(map[string]APIResponse)

    // Iterate over the countries
    for _, country := range countries {
        var allArticles []Article
        page := 1

        // Keep fetching pages until all articles for the country are fetched
        for {
            response, err := newsAPIHeadlineCaller(country, fmt.Sprintf("%d", page), pageSize)
            time.Sleep(1 * time.Second)
            if err != nil {
                log.Printf("Error fetching headlines for country '%s': %v", country, err)
                break
            }

            allArticles = append(allArticles, response.Articles...)
            
            // If the number of articles fetched is less than the total results, continue fetching next pages
            if len(allArticles) >= response.TotalResults {
                break
            }

            // If there are more results, increase the page number and continue fetching
            page++
        }

        // Store the fetched articles for the country in the map
        countryHeadlines[country] = APIResponse{
            Status:       "ok",
            TotalResults: len(allArticles),
            Articles:     allArticles,
        }
    }

    return countryHeadlines
}

func newsDiscoverySummarizer(language, sortBy, from, to string) map[string]APIResponse {
    discoverTags := []string{
        "gainers", "losers", "software", "finance", "stocks",
        "bonds", "corporate", "banking", "technology", "tax", "geopolitics",
    }

    categorizedResponses := make(map[string]APIResponse)
    pageSize := "20"

    for _, category := range discoverTags {
        page := 1
        var categoryArticles []Article

        for {
            response, err := newsAPIEverything(category, "", language, sortBy, from, to, fmt.Sprintf("%d", page), pageSize)
            if err != nil {
                log.Printf("Error fetching news for category '%s': %v", category, err)
                break
            }

            categoryArticles = append(categoryArticles, response.Articles...)

            if len(categoryArticles) >= response.TotalResults {
                break
            }

            page++
        }

        categorizedResponses[category] = APIResponse{
            Status:       "ok",
            TotalResults: len(categoryArticles),
            Articles:     categoryArticles,
        }
    }

    return categorizedResponses
}

func main() {
    // Define country codes for each region (North America, Europe, Asia, Australia)
    northAmerica := []string{"us", "ca", "mx"}
    europe := []string{"gb", "de", "fr", "it", "es", "pl", "nl"}
    asia := []string{"cn", "in", "jp", "kr", "sg", "hk"}
    australia := []string{"au", "nz"}

    // Combine all countries into one list
    allCountries := append(append(northAmerica, europe...), append(asia, australia...)...)

    // Fetch headlines from all countries (passing "20" articles per page)
    categorizedHeadlines := fetchHeadlinesByCountry(allCountries, "20")

    // Iterate over the categorized headlines and print them
    for country, response := range categorizedHeadlines {
        fmt.Printf("Headlines for country: %s\n", country)
        fmt.Printf("Total Results: %d\n\n", response.TotalResults)
        for _, article := range response.Articles {
            fmt.Printf("Title: %s\n", article.Title)
            fmt.Printf("Author: %s\n", article.Author)
            fmt.Printf("URL: %s\n", article.URL)
            fmt.Printf("URLImage: %s\n", article.URLToImage)
            fmt.Printf("Published At: %s\n", article.PublishedAt)
            fmt.Printf("Description: %s\n", article.Description)
            fmt.Printf("Content: %s\n", article.Content)
            fmt.Println("---")
        }
        fmt.Println("=========")
    }
}

