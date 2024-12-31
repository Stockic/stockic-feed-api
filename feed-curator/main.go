package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"feed-curator/database"
	"feed-curator/fetcher"
	"feed-curator/models"
	"feed-curator/services"
	"feed-curator/summarizer"
	"feed-curator/utils"
)

func init() {
    logFile, err := os.OpenFile(models.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
    if err != nil { 
        log.Fatalf("Failed to open log file: %v", err)
    }

    log.SetOutput(logFile)
    log.SetFlags(0)

    database.InitRedis()
    database.InitMinIO()

    go services.PushAppLogToMinIO()
} 

func main() {

    defer models.FreshNewsRedisCtxCancel()

    for {
        // Define country codes for each region (North America, Europe, Asia, Australia)
        northAmerica := []string{"us"}
        // europe := []string{"gb", "de", "fr", "it", "es", "pl", "nl"}
        // asia := []string{"cn", "in", "jp", "kr", "sg", "hk"}
        // australia := []string{"au", "nz"}

        // Combine all countries into one list
        allCountries := append(northAmerica)

        // Fetch headlines from all countries (passing "20" articles per page)
        categorizedHeadlines := fetcher.FetchHeadlinesByCountry(allCountries, "20")

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
        
        categorizedDiscovery := fetcher.NewsDiscoveryByCategory("en", "publishedAt", "2024-12-28", "2024-12-29")

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

        // MinIO Setup
        err := database.UploadNewsAPIResponseDataToMinIO(models.MinIOClient, categorizedHeadlines, "news-archive")
        if err != nil {
            utils.LogMessage("Failed to push News Archive MinIO", "red")
        }

        err = database.UploadNewsAPIResponseDataToMinIO(models.MinIOClient, categorizedDiscovery, "news-archive")
        if err != nil {
            utils.LogMessage("Failed to push News Archive MinIO", "red")
        }

        utils.LogMessage("Feedling AI with all the news", "green")

        summarizedHeadlines := summarizer.SummarizeCountryCategorizedHeadlines(categorizedHeadlines)

        summarizedCategorized := summarizer.SummarizeCategorizedNews(categorizedDiscovery)

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

        err = summarizer.StoreSummarizedRedis("headlines", summarizedHeadlines)
        if err != nil {
            utils.LogMessage("Failed to store headlines news in Redis", "red", err)
        }

        err = summarizer.StoreSummarizedRedis("discover", summarizedCategorized)
        if err != nil {
            utils.LogMessage("Failed to store discover news in Redis", "red", err)
        }


        currentTime := time.Now()

        nextMidnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day()+1, 0, 0, 0, 0, currentTime.Location())
        durationUntilMidnight := nextMidnight.Sub(currentTime)

        fmt.Printf("Waiting for next midnight... Time remaining: %v\n", durationUntilMidnight)

        time.Sleep(durationUntilMidnight)

        fmt.Println("It's 00:00! Executing the main function...")
    }
}
