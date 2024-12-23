package main

import (
	"context"
    "strconv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
    "crypto/sha256"
    "encoding/hex"

    "github.com/go-redis/redis/v8"
	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

var (
    geminiCtx context.Context

    freshNewsRedisCtx context.Context
    freshNewsRedisCtxCancel context.CancelFunc

    freshNewsRedis *redis.Client
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

    freshNewsRedisCtx, freshNewsRedisCtxCancel = context.WithCancel(context.Background())

    freshNewsRedis, err = redisInit(freshNewsRedisCtx, "FRESHNEWS_REDIS_ADDRESS", "FRESHNEWS_REDIS_DB", "FRESHNEWS_REDIS_PASSWORD")
    if err != nil {
        logMessage("FRESH NEWS Redis Server Setup Failed!", "red", err)
    }
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

func summarizer(modelName string, title string, text string) (*genai.GenerateContentResponse, error) {
    
    geminiCtx := context.Background()
    // Access your API key as an environment variable (see "Set up your API key" above)
    client, err := genai.NewClient(geminiCtx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
    if err != nil {
        return nil, err
    }
    defer client.Close()

    model := client.GenerativeModel(modelName)

    promptInput := fmt.Sprintf("Focus on content, main financial points and not on source/author. Summarize given news only called %s in the url: %s", title, text)
    response, err := model.GenerateContent(geminiCtx, genai.Text(promptInput))
    if err != nil {
        return nil, err
    }

    return response, err
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

func newsAPIEverythingCaller(q, searchIn, language, sortBy, from, to, page, pageSize string) (APIResponse, error) {
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

func newsDiscoveryByCategory(language, sortBy, from, to string) map[string]APIResponse {
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
            response, err := newsAPIEverythingCaller(category, "", language, sortBy, from, to, fmt.Sprintf("%d", page), pageSize)
            time.Sleep(1 * time.Second)
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

func printResponse(resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
}

func summarizeCountryCategorizedHeadlines(categorizedHeadlines map[string]APIResponse) map[string]SummarizedResponse {
    summarizedResponses := make(map[string]SummarizedResponse)

    for category, apiResponse := range categorizedHeadlines {
        var summarizedArticles []SummarizedArticle

        for _, article := range apiResponse.Articles {
            // summaryResp, err := summarizer("gemini-1.5-flash", article.Title, article.Content)
            // if err != nil {
            //    logMessage(fmt.Sprintf("AI Failed to process: %s", article.Title), "red", err)
            //    continue
            // }
            // time.Sleep(30 * time.Second)
            // logMessage("Feeding AI with 1 news", "green")

            // var contentString string = ""
            // for _, candidate := range summaryResp.Candidates {
            //     if candidate.Content != nil {
            //         for _, part := range candidate.Content.Parts {
            //             contentString = fmt.Sprintf("%s%s", contentString, part) 
            //         }
            //     }
            // }
            
            contentString := article.Content

            // logMessage("===== AI NEWS! ====", "green")
            fmt.Println(contentString)
            // logMessage("===================", "green")

            if article.URLToImage == "" {
                logMessage(fmt.Sprintf("Skipping article without image: %s", article.Title), "yellow")
                continue
            }

            // Create summarized article
            summarizedArticle := SummarizedArticle{
                Source:             article.Source.Name,
                Author:             article.Author,
                Title:              article.Title,
                URL:                article.URL,
                URLToImage:         article.URLToImage,
                PublishedAt:        article.PublishedAt,
                SummarizedContent:  contentString,
            }

            // Concatenate fields to generate StockicID
            concatenatedFields := fmt.Sprintf("%s%s%s%s%s%s%s",
                summarizedArticle.Source,
                summarizedArticle.Author,
                summarizedArticle.Title,
                summarizedArticle.URL,
                summarizedArticle.URLToImage,
                summarizedArticle.PublishedAt,
                summarizedArticle.SummarizedContent,
            )

            // Generate SHA256 hash
            hash := sha256.Sum256([]byte(concatenatedFields))
            summarizedArticle.StockicID = hex.EncodeToString(hash[:])

            // Append to list of summarized articles
            summarizedArticles = append(summarizedArticles, summarizedArticle)
        }
        summarizedResponses[category] = SummarizedResponse{

            Status:       "ok",
            TotalResults: len(summarizedArticles),
            Articles:     summarizedArticles,
        }
    }

    return summarizedResponses
}

func summarizeCategorizedNews(categorizedNews map[string]APIResponse) map[string]SummarizedResponse {
    summarizedResponses := make(map[string]SummarizedResponse)

    for category, apiResponse := range categorizedNews {
        var summarizedArticles []SummarizedArticle

        for _, article := range apiResponse.Articles {
            // summaryResp, err := summarizer("gemini_model_name", article.Title, article.Content)
            // if err != nil {
            //      logMessage(fmt.Sprintf("AI Failed to process: %s", article.Title), "red", err)
            //      continue
            // }
            // time.Sleep(20 * time.Second)

            // logMessage("Feeding AI with 1 news", "green")

            // var contentString string = ""
            // for _, candidate := range summaryResp.Candidates {
            //     if candidate.Content != nil {
            //         for _, part := range candidate.Content.Parts {
            //             contentString = fmt.Sprintf("%s%s", contentString, part) 
            //         }
            //     }
            // }

            contentString := article.Content

            // logMessage("===== AI NEWS! ====", "green")
            fmt.Println(contentString)
            // logMessage("===================", "green")

            if article.URLToImage == "" {
                logMessage(fmt.Sprintf("Skipping article without image: %s", article.Title), "yellow")
                continue
            }

            if article.Author == "" {
                logMessage("No Author, replacing with Aditya Patil", "red")
                article.Author = "Aditya Patil"
            }

            if article.Source.Name == "" {
                logMessage("No source Name - Replacing with Stockic Editors", "red")
                article.Source.Name = "Stockic Editors"
            }

            if article.Source.Name == "The Washington Post" {
                logMessage("The Washington Post News, changing URL", "red")
                article.URLToImage = "https://www.washingtonpost.com/wp-apps/imrs.php?src=https%3A%2F%2Farc-anglerfish-washpost-prod-washpost%252Es3%252Eamazonaws%252Ecom%2Fpublic%2FBA3LQ27PFVG5RCTQ7P2D2SMBJU%252Ejpg&w=924&h=694"
            }

            summarizedArticle := SummarizedArticle{
                Source:             article.Source.Name,
                Author:             article.Author,
                Title:              article.Title,
                URL:                article.URL,
                URLToImage:         article.URLToImage,
                PublishedAt:        article.PublishedAt,
                SummarizedContent:  contentString,
            }

            concatenatedFields := fmt.Sprintf("%s%s%s%s%s%s%s",
                summarizedArticle.Source,
                summarizedArticle.Author,
                summarizedArticle.Title,
                summarizedArticle.URL,
                summarizedArticle.URLToImage,
                summarizedArticle.PublishedAt,
                summarizedArticle.SummarizedContent,
            )

            hash := sha256.Sum256([]byte(concatenatedFields))
            summarizedArticle.StockicID = hex.EncodeToString(hash[:])

            summarizedArticles = append(summarizedArticles, summarizedArticle)
        }

        summarizedResponses[category] = SummarizedResponse{
            Status:       "ok",
            TotalResults: len(summarizedArticles),
            Articles:     summarizedArticles,
        }
    }

    return summarizedResponses
}

func storeSummarizedRedis(redisKey string, summarizedHeadlines map[string]SummarizedResponse) error {
    summarizedJSONData, err := json.Marshal(summarizedHeadlines)
	if err != nil {
        logMessage("Error serializing data", "red", err)
        return err
	}

	err = freshNewsRedis.Set(freshNewsRedisCtx, redisKey, summarizedJSONData, 0).Err()
	if err != nil {
		log.Fatalf("Error setting data in Redis: %v", err)
        return err
	}

    logMessage(fmt.Sprintf("Data stored in Redis successfully: %s", redisKey), "green", err)
    return nil
}

func main() {

    for {

        // Define country codes for each region (North America, Europe, Asia, Australia)
        northAmerica := []string{"us"}
        // europe := []string{"gb", "de", "fr", "it", "es", "pl", "nl"}
        // asia := []string{"cn", "in", "jp", "kr", "sg", "hk"}
        // australia := []string{"au", "nz"}

        // Combine all countries into one list
        allCountries := append(northAmerica)

        // Fetch headlines from all countries (passing "20" articles per page)
        categorizedHeadlines := fetchHeadlinesByCountry(allCountries, "20")

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
        
        categorizedDiscovery := newsDiscoveryByCategory("en", "publishedAt", "2024-11-07", "2024-11-08")

        for category, response := range categorizedDiscovery {
            fmt.Printf("Category: %s\n", category)
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

        logMessage("Feedling AI with all the news", "green")

        summarizedHeadlines := summarizeCountryCategorizedHeadlines(categorizedHeadlines)

        summarizedCategorized := summarizeCategorizedNews(categorizedDiscovery)

        for category, summarizedResponse := range summarizedHeadlines {
            fmt.Printf("Summarized Headline Category: %s, Total Articles: %d\n", category, summarizedResponse.TotalResults)
            for _, article := range summarizedResponse.Articles {
                fmt.Printf("Summarized Article Title: %s\n", article.Title)
            }
        }

        for category, summarizedResponse := range summarizedCategorized {
            fmt.Printf("Summarized News Category: %s, Total Articles: %d\n", category, summarizedResponse.TotalResults)
            for _, article := range summarizedResponse.Articles {
                fmt.Printf("Summarized Article Title: %s\n", article.Title)
            }
        }

        err := storeSummarizedRedis("headlines", summarizedHeadlines)
        if err != nil {
            logMessage("Failed to store headlines news in Redis", "red", err)
        }

        err = storeSummarizedRedis("discover", summarizedCategorized)
        if err != nil {
            logMessage("Failed to store discover news in Redis", "red", err)
        }

        defer freshNewsRedisCtxCancel()


        currentTime := time.Now()

	nextMidnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day()+1, 0, 0, 0, 0, currentTime.Location())
	durationUntilMidnight := nextMidnight.Sub(currentTime)

	fmt.Printf("Waiting for next midnight... Time remaining: %v\n", durationUntilMidnight)

	time.Sleep(durationUntilMidnight)

	fmt.Println("It's 00:00! Executing the main function...")
    }
}

