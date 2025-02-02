package models

type Greet struct {
    Response    string  `json:"response"`
}

type UserStatus struct {
    Exists  bool `json:"exists"`
    Premium bool `json:"premium"`
}

type BookmarkRequest struct {
    NewsID string `json:"NewsId"`
}

type Response struct {
    Success   bool     `json:"success"`
    Message   string   `json:"message"`
    Bookmarks []string `json:"bookmarks,omitempty"`
}
