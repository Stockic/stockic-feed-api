package fetcher

import (
    "fmt"
    "log" 
    "net/http"
    "os"
    "time"
    "io"
    "encoding/json"

    "feed-curator/models"
)

func NewsAPICaller(url string) (models.APIResponse, error) {
    var response models.APIResponse

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

func NewsAPIHeadlineCaller(country string, page string, pageSize string) (models.APIResponse, error) {
    url := fmt.Sprintf(
        "https://newsapi.org/v2/top-headlines?country=%s&page=%s&pageSize=%s&apiKey=%s",
        country, page, pageSize, os.Getenv("NEWSAPI_API_KEY"),
    )
    return NewsAPICaller(url)
}

func NewsAPIEverythingCaller(q, searchIn, language, sortBy, from, to, page, pageSize string) (models.APIResponse, error) {
    url := fmt.Sprintf(
        "https://newsapi.org/v2/everything?q=%s&searchIn=%s&language=%s&sortBy=%s&from=%s&to=%s&page=%s&pageSize=%s&apiKey=%s",
        q, searchIn, language, sortBy, from, to, page, pageSize, os.Getenv("NEWSAPI_API_KEY"),
    )

    return NewsAPICaller(url)
}

func FetchHeadlinesByCountry(countries []string, pageSize string) map[string]models.APIResponse {
    countryHeadlines := make(map[string]models.APIResponse)

    // Iterate over the countries
    for _, country := range countries {
        var allArticles []models.Article
        page := 1

        // Keep fetching pages until all articles for the country are fetched
        for {
            response, err := NewsAPIHeadlineCaller(country, fmt.Sprintf("%d", page), pageSize)
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
        countryHeadlines[country] = models.APIResponse{
            Status:       "ok",
            TotalResults: len(allArticles),
            Articles:     allArticles,
        }
    }

    return countryHeadlines
}

func NewsDiscoveryByCategory(language, sortBy, from, to string) map[string]models.APIResponse {
    discoverTags := []string{
        "gainers", "losers", "software", "finance", "stocks",
        "bonds", "corporate", "banking", "technology", "tax", "geopolitics",
    }

    categorizedResponses := make(map[string]models.APIResponse)
    pageSize := "20"

    for _, category := range discoverTags {
        page := 1
        var categoryArticles []models.Article

        for {
            response, err := NewsAPIEverythingCaller(category, "", language, sortBy, from, to, fmt.Sprintf("%d", page), pageSize)
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

        categorizedResponses[category] = models.APIResponse{
            Status:       "ok",
            TotalResults: len(categoryArticles),
            Articles:     categoryArticles,
        }
    }

    return categorizedResponses
}
