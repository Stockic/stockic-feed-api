package summarizer

import (
    "fmt"
    "context"
    "os"
    "log"
    "time"
    "encoding/hex"
    "encoding/json"
    "crypto/sha256"

    "feed-curator/models"
    "feed-curator/utils"

    "github.com/google/generative-ai-go/genai"
    "google.golang.org/api/option"
)

func summarizer(modelName string, title string, text string) (*genai.GenerateContentResponse, error) {
    geminiCtx := context.Background()
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

func articleTagger(modelName string, title string, text string) (*genai.GenerateContentResponse, error) {
    geminiCtx := context.Background()
    client, err := genai.NewClient(geminiCtx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
    if err != nil {
        return nil, err 
    }
    defer client.Close() 

    model := client.GenerativeModel(modelName)

    promptInput := fmt.Sprintf("Extract important works like names of companies in JSON format ['{companies: []}']: Title: %s Content: %s", title, text)
    response, err := model.GenerateContent(geminiCtx, genai.Text(promptInput))
    if err != nil {
        return nil, err
    }

    return response, err
}

func SummarizeCountryCategorizedHeadlines(categorizedHeadlines map[string]models.APIResponse) map[string]models.SummarizedResponse {
    summarizedResponses := make(map[string]models.SummarizedResponse)

    for category, apiResponse := range categorizedHeadlines {
        var summarizedArticles []models.SummarizedArticle
        var tagsString string = ""
        for _, article := range apiResponse.Articles {

            utils.LogMessage("Feeding AI with 1 news", "green")
            summaryResp, err := summarizer("gemini-1.5-flash", article.Title, article.Content)
            if err != nil {
               utils.LogMessage(fmt.Sprintf("AI Failed to process: %s", article.Title), "red", err)
               continue
            }
            time.Sleep(30 * time.Second)

            companiesTags, err := articleTagger("gemini-1.5-flash", article.Title, article.Content)
            if err == nil {
                for _,candidate := range companiesTags.Candidates {
                    if candidate.Content != nil {
                        for _, part := range candidate.Content.Parts {
                            tagsString = fmt.Sprintf("%s%s", tagsString, part)
                        }
                    }
                }
                utils.LogMessage(tagsString, "green")
            } else {
                utils.LogMessage(fmt.Sprintf("AI Tagging Failed: %s", article.Title), "red", err)
            }
            time.Sleep(30 * time.Second)

            var contentString string = ""
            for _, candidate := range summaryResp.Candidates {
                if candidate.Content != nil {
                    for _, part := range candidate.Content.Parts {
                        contentString = fmt.Sprintf("%s%s", contentString, part) 
                    }
                }
            }

            var companiesTagsDeserialized models.CompaniesTags
            if err = json.Unmarshal([]byte(tagsString), &companiesTagsDeserialized); err != nil {
                utils.LogMessage("Faied to deserialize companies json tags", "red", err)
            }
           
            // contentString := article.Content

            utils.LogMessage("===== AI NEWS! ====", "green")
            fmt.Println(contentString)
            utils.LogMessage("===================", "green")

            if article.URLToImage == "" {
                utils.LogMessage(fmt.Sprintf("Skipping article without image: %s", article.Title), "yellow")
                continue
            }

            if article.Author == "" {
                utils.LogMessage("No Author, replacing with Aditya Patil", "red")
                article.Author = "Aditya Patil"
            }

            if article.Source.Name == "" {
                utils.LogMessage("No source Name - Replacing with Stockic Editors", "red")
                article.Source.Name = "Stockic Editors"
            }

            if article.Source.Name == "The Washington Post" {
                utils.LogMessage("The Washington Post News, changing URL", "red")
                article.URLToImage = "https://theintercept.com/wp-content/uploads/2017/01/the-washington-post-newspaper-2-1484771977.jpg"
            }


            // Create summarized article
            summarizedArticle := models.SummarizedArticle{
                Source:             article.Source.Name,
                Author:             article.Author,
                Title:              article.Title,
                URL:                article.URL,
                URLToImage:         article.URLToImage,
                PublishedAt:        article.PublishedAt,
                SummarizedContent:  contentString,
                CompaniesTags:      companiesTagsDeserialized,
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
        summarizedResponses[category] = models.SummarizedResponse{

            Status:       "ok",
            TotalResults: len(summarizedArticles),
            Articles:     summarizedArticles,
        }
    }

    return summarizedResponses
}

func SummarizeCategorizedNews(categorizedNews map[string]models.APIResponse) map[string]models.SummarizedResponse {
    summarizedResponses := make(map[string]models.SummarizedResponse)

    for category, apiResponse := range categorizedNews {
        var summarizedArticles []models.SummarizedArticle

        for _, article := range apiResponse.Articles {
            summaryResp, err := summarizer("gemini_model_name", article.Title, article.Content)
            if err != nil {
                 utils.LogMessage(fmt.Sprintf("AI Failed to process: %s", article.Title), "red", err)
                 continue
            }
            time.Sleep(30 * time.Second)

            utils.LogMessage("Feeding AI with 1 news", "green")

            var contentString string = ""
            for _, candidate := range summaryResp.Candidates {
                if candidate.Content != nil {
                    for _, part := range candidate.Content.Parts {
                        contentString = fmt.Sprintf("%s%s", contentString, part) 
                    }
                }
            }

            // contentString := article.Content

            utils.LogMessage("===== AI NEWS! ====", "green")
            fmt.Println(contentString)
            utils.LogMessage("===================", "green")

            if article.URLToImage == "" {
                utils.LogMessage(fmt.Sprintf("Skipping article without image: %s", article.Title), "yellow")
                continue
            }

            if article.Author == "" {
                utils.LogMessage("No Author, replacing with Aditya Patil", "red")
                article.Author = "Aditya Patil"
            }

            if article.Source.Name == "" {
                utils.LogMessage("No source Name - Replacing with Stockic Editors", "red")
                article.Source.Name = "Stockic Editors"
            }

            if article.Source.Name == "The Washington Post" {
                utils.LogMessage("The Washington Post News, changing URL", "red")
                article.URLToImage = "https://www.washingtonpost.com/wp-apps/imrs.php?src=https%3A%2F%2Farc-anglerfish-washpost-prod-washpost%252Es3%252Eamazonaws%252Ecom%2Fpublic%2FBA3LQ27PFVG5RCTQ7P2D2SMBJU%252Ejpg&w=924&h=694"
            }

            summarizedArticle := models.SummarizedArticle{
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

        summarizedResponses[category] = models.SummarizedResponse{
            Status:       "ok",
            TotalResults: len(summarizedArticles),
            Articles:     summarizedArticles,
        }
    }

    return summarizedResponses
}

func StoreSummarizedRedis(redisKey string, summarizedHeadlines map[string]models.SummarizedResponse) error {
    summarizedJSONData, err := json.Marshal(summarizedHeadlines)
	if err != nil {
        utils.LogMessage("Error serializing data", "red", err)
        return err
	}

	err = models.FreshNewsRedis.Set(models.FreshNewsRedisCtx, redisKey, summarizedJSONData, 0).Err()
	if err != nil {
		log.Fatalf("Error setting data in Redis: %v", err)
        return err
	}

    utils.LogMessage(fmt.Sprintf("Data stored in Redis successfully: %s", redisKey), "green", err)
    return nil
}
