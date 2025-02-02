package handlers

import (
	// "context"
	"encoding/json"
	"fmt"
	// "fmt"
	"net/http"
	// "strconv"
	// "strings"

	"actions/config"
	"actions/models"
	"actions/utils"

	// "actions/services"
	// "actions/utils"

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
	// Only allow DELETE requests
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from header
	userID := r.Header.Get("X-API-Key")
	if userID == "" {
		http.Error(w, "Missing X-API-Key header", http.StatusBadRequest)
		return
	}

	// Decode request body
	var req models.BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Reference to user document
	userRef := config.FirebaseClient.Collection("users").Doc(userID)

	// Remove newsID from bookmarks array
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

	// Send success response
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
		sendResponse(w, models.Response{
			Success: false,
			Message: "Method not allowed",
		}, http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		sendResponse(w, models.Response{
			Success: false,
			Message: "Missing API key",
		}, http.StatusUnauthorized)
		return
	}

	docRef := config.FirebaseClient.Collection("users").Doc(apiKey)
	doc, err := docRef.Get(config.FirebaseCtx)
	if err != nil {
		utils.LogMessage("Failed to get user document", "red", err)
		sendResponse(w, models.Response{
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
			sendResponse(w, models.Response{
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

	sendResponse(w, models.Response{
		Success:   true,
		Message:   "Bookmarks retrieved successfully",
		Bookmarks: bookmarks,
	}, http.StatusOK)
}

func FallbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "Fallback Handler")
}

func sendResponse(w http.ResponseWriter, response models.Response, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
