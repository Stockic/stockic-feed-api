package main

import (
	"fmt"
    "time"

    "feed-curator/models"
    "feed-curator/fetcher"
    "feed-curator/summarizer"
    "feed-curator/utils"
)

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
        
        categorizedDiscovery := fetcher.NewsDiscoveryByCategory("en", "publishedAt", "2024-11-07", "2024-11-08")

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

        err := summarizer.StoreSummarizedRedis("headlines", summarizedHeadlines)
        if err != nil {
            utils.LogMessage("Failed to store headlines news in Redis", "red", err)
        }

        err = summarizer.StoreSummarizedRedis("discover", summarizedCategorized)
        if err != nil {
            utils.LogMessage("Failed to store discover news in Redis", "red", err)
        }

        defer models.FreshNewsRedisCtxCancel()


        currentTime := time.Now()

        nextMidnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day()+1, 0, 0, 0, 0, currentTime.Location())
        durationUntilMidnight := nextMidnight.Sub(currentTime)

        fmt.Printf("Waiting for next midnight... Time remaining: %v\n", durationUntilMidnight)

        time.Sleep(durationUntilMidnight)

        fmt.Println("It's 00:00! Executing the main function...")
    }
}
