package utils

import (
    "fmt"
    "time"
    "log"
)

func LogMessage(message, color string, errs ...error) {
    var err error
    if len(errs) > 0 {
        err = errs[0]
    } else {
        err = nil
    }

    timestamp := time.Now().Format("2006-01-02 15:04:05")
    
    log.Printf("[FEED-API-LOG] [%s] %s ERROR: %v", timestamp, message, err)

    if color == "red" {
        fmt.Printf("\033[31m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    } else if color == "green" {
        fmt.Printf("\033[32m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    } else {
        fmt.Printf("\033[31m [%s] %s \033[0m ERROR: %v\n", timestamp, message, err)
    }
}
