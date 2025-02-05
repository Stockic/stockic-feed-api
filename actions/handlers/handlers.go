package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
    "encoding/base64"
    "bytes"

	"actions/config"
	"actions/models"
	"actions/utils"

	"cloud.google.com/go/firestore"
)

func AddBookmarksHandlers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-API-Key")
	if userID == "" {
		http.Error(w, "Missing X-API-Key header", http.StatusBadRequest)
		return
	}

	var req models.BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userRef := config.FirebaseClient.Collection("users").Doc(userID)

	_, err := userRef.Update(config.FirebaseCtx, []firestore.Update{
		{
			Path:  "bookmarks",
			Value: firestore.ArrayUnion(req.NewsID),
		},
	})

	if err != nil {
		_, err = userRef.Set(config.FirebaseCtx, map[string]interface{}{
			"bookmarks": []string{req.NewsID},
		})
		if err != nil {
			http.Error(w, "Failed to create bookmark", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
    if err = json.NewEncoder(w).Encode(map[string]string{
		"message": "Bookmark added successfully",
	}); err != nil {
        utils.LogMessage("jsonError: Failed to encode JSON response: %v" + err.Error(), "red")
        http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
    }
}

func Ping(w http.ResponseWriter, r *http.Request) {
 	w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "pong",
	}); err != nil {
        utils.LogMessage("jsonError: Failed to encode JSON response: %v" + err.Error(), "red")
        http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
    }
}

func RemoveBookmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-API-Key")
	if userID == "" {
		http.Error(w, "Missing X-API-Key header", http.StatusBadRequest)
		return
	}

	var req models.BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userRef := config.FirebaseClient.Collection("users").Doc(userID)

	_, err := userRef.Update(config.FirebaseCtx, []firestore.Update{
		{
			Path:  "bookmarks",
			Value: firestore.ArrayRemove(req.NewsID),
		},
	})
	if err != nil {
		utils.LogMessage("firebaseError: Failed to delete bookmark: "+err.Error(), "red")
		http.Error(w, "Failed to delete bookmark", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(map[string]string{
		"message": "Bookmark deleted successfully",
	}); err != nil {
		utils.LogMessage("jsonError: Failed to encode JSON response: "+err.Error(), "red")
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
	}
}

func ListBookmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendResponse(w, models.BookmarkResponse{
			Success: false,
			Message: "Method not allowed",
		}, http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		sendResponse(w, models.BookmarkResponse{
			Success: false,
			Message: "Missing API key",
		}, http.StatusUnauthorized)
		return
	}

	docRef := config.FirebaseClient.Collection("users").Doc(apiKey)
	doc, err := docRef.Get(config.FirebaseCtx)
	if err != nil {
		utils.LogMessage("Failed to get user document", "red", err)
		sendResponse(w, models.BookmarkResponse{
			Success: false,
			Message: "User not found",
		}, http.StatusNotFound)
		return
	}

	var bookmarks []string
	bookmarksData, err := doc.DataAt("bookmarks")
	if err != nil {
		bookmarks = []string{}
	} else {
		bookmarksInterface, ok := bookmarksData.([]interface{})
		if !ok {
			utils.LogMessage("Failed to convert bookmarks data: invalid format", "red")
			sendResponse(w, models.BookmarkResponse{
				Success: false,
				Message: "Internal server error",
			}, http.StatusInternalServerError)
			return
		}

		for _, newsID := range bookmarksInterface {
			if strNewsID, ok := newsID.(string); ok {
				bookmarks = append(bookmarks, strNewsID)
			}
		}
	}

	sendResponse(w, models.BookmarkResponse{
		Success:   true,
		Message:   "Bookmarks retrieved successfully",
		Bookmarks: bookmarks,
	}, http.StatusOK)
}

func FallbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "Fallback Handler")
}

func sendResponse(w http.ResponseWriter, response models.BookmarkResponse, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
    if err := json.NewEncoder(w).Encode(response); err != nil {
        utils.LogMessage("Failed sending response", "red", err)
    }
}

/*
This endpoint creates a sign in session for the user to proceed with oauth. 
We set a random number for the state which is the session id for login.
The session id (state) is stored in redis in format "session id": "X-API-Key". 
The session id has a timeout specified in config module. 
If the user allows access, we move ahead and Callback function gets the state. 
We get the session id and find the X-API-Key from Redis and store it into firebase for the user.
Incase data is not found in Redis, we pop up the page "Session Timeout, please try again."
*/
func OauthNotionCreateAuthSession(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    var jsonResponse models.OauthNotionResponse

    if jsonResponse.OauthURL = os.Getenv("NOTION_OAUTH_URL"); jsonResponse.OauthURL != "" {
        
        /* Create a Session ID Token */
        sessionToken, err := utils.GenerateSessionKey(config.NotionSessionKeyLength, config.NotionSessionCharset)
        if err != nil {
            utils.LogMessage("Failed to create Notion Session Token", "red", err)
            jsonResponse.Success = false 
            jsonResponse.OauthURL = "nil"
            w.WriteHeader(http.StatusNotFound)
            if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
                utils.LogMessage("Failed to create Notion Session Token and Error sending response about failure", "red", err)
            }
        }

        jsonResponse.Success = true
        jsonResponse.OauthURL = jsonResponse.OauthURL + fmt.Sprintf("&state=%s", sessionToken)

        apiKey := r.Header.Get("X-API-Key")
        fmt.Println(sessionToken)

        // Session stored in Redis -> sessionKey: apiKey
        err = config.RedisSessionCache.Set(config.RedisAPICacheCtx, sessionToken, apiKey, config.NotionSessionExpiration).Err()
        if err != nil {
            utils.LogMessage("Error storing session information into session redis", "red", err)
        }

        w.WriteHeader(http.StatusOK)
        if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
            utils.LogMessage("Faild to send Notion Oauth URL : URL Found but Not Delivered", "red", err) 
            w.WriteHeader(http.StatusNotFound)
            fmt.Fprintln(w, "No Notion Oauth URL set in backend")
        }
    } else {
        jsonResponse.Success = false 
        jsonResponse.OauthURL = "URL not found"
        utils.LogMessage("No Notion Oauth URL Found", "red")
        w.WriteHeader(http.StatusNotFound)
        if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
            utils.LogMessage("No Notion Oauth URL Found and Error sending response about failure", "red", err)
        }
    }
}

func OauthNotionCallback(w http.ResponseWriter, r *http.Request) {
    utils.LogMessage("Recieved callback", "green")
    w.Header().Set("Content-Type", "application/json")
    params := r.URL.Query()

	NotionAuthcode := params.Get("code") 
    if NotionAuthcode == "" {
        utils.LogMessage("No Notion Auth Code in Callback", "red")
    }

    NotionAuthState := params.Get("state") 
    if NotionAuthcode == "" {
        utils.LogMessage("No Notion Auth Code in Callback", "red")
    }

    utils.LogMessage(fmt.Sprintf("Notion Auth Code: %s, Notion Auth State: %s", NotionAuthcode, NotionAuthState), "green")
    
    clientID := os.Getenv("NOTION_CLIENT_ID")
    clientSecret := os.Getenv("NOTION_CLIENT_SECRET")
    RedirectURI := os.Getenv("NOTION_REDIRECT_URI")

    credentials := clientID + ":" + clientSecret

    encodedCredentials := base64.StdEncoding.EncodeToString([]byte(credentials))
    
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"code":          NotionAuthcode,
		"redirect_uri":  RedirectURI,
	}

    jsonData, err := json.Marshal(payload)
	if err != nil {
		utils.LogMessage("Error encoding JSON", "red", err)
		return
	}

    url := "https://api.notion.com/v1/oauth/token"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		utils.LogMessage("Error creating request:", "red", err)
		return
	}

    req.Header.Set("Authorization", "Basic " + encodedCredentials)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

    client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		utils.LogMessage("Error making request", "red", err)
        http.Error(w, "Error making request to Notion", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var authResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&authResult); err != nil {
		utils.LogMessage("Error decoding response", "red", err)
        http.Error(w, "Error Decoding Notion Response" + err.Error(), http.StatusUnauthorized)
		return
	}
	fmt.Println("Response Body:", authResult)

    apiKey, err := config.RedisSessionCache.Get(config.RedisSessionCacheCtx, NotionAuthState).Result()
	if err != nil {
		utils.LogMessage("Could not get key. Session might went expired", "red", err)
        http.Error(w, "Session expired or invalid: "+err.Error(), http.StatusUnauthorized)
        return
	}

    docRef := config.FirebaseClient.Collection("users").Doc(apiKey)

	_, err = docRef.Set(config.FirebaseCtx, map[string]interface{}{
		"NotionCredentials": authResult,
	})
	if err != nil {
		utils.LogMessage("Failed to store data in Firestore", "red", err)
	}
}
