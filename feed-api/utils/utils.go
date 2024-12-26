package utils

import (
    "fmt"
    "log"
    "net"
    "net/http"
    "strings"
    "time"
    "encoding/json"
)

func LogMessage(message, color string, errs ...error) {
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

func DeliverJsonError(httpHandler http.ResponseWriter, message string, statusCode int) {
    httpHandler.Header().Set("Content-Type", "application/json")
    httpHandler.WriteHeader(statusCode)
    jsonResponse := map[string]string{"error": message}
    
    LogMessage(message, "red")

    jsonEncoder := json.NewEncoder(httpHandler) 
    if err := jsonEncoder.Encode(jsonResponse); err != nil {
        LogMessage("jsonError: Failed to encode JSON response: %v" + err.Error(), "red")
        http.Error(httpHandler, `{"error": "internal server error"}`, http.StatusInternalServerError)
    }
}

func GetClientIP(request *http.Request) string {
    ip := request.RemoteAddr
    if strings.Contains(ip, ":") {
        ip, _, _ = net.SplitHostPort(ip)
    }
    return ip
}
