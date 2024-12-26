package models

type SummarizedArticle struct {
    StockicID           string `json:"stockicID"`
    Source              string `json:"source"`
    Author              string `json:"author"`
    Title               string `json:"title"`
    URL                 string `json:"url"`
    URLToImage          string `json:"urlToImage"`
    PublishedAt         string `json:"publishedAt"`
    SummarizedContent   string `json:"content"`
}

type SummarizedResponse struct {
    Status       string              `json:"status"`
    TotalResults int                 `json:"totalResults"`
    Articles     []SummarizedArticle `json:"articles"`
}

type Greet struct {
    Response    string  `json:"response"`
}

type UserStatus struct {
    Exists  bool `json:"exists"`
    Premium bool `json:"premium"`
}
