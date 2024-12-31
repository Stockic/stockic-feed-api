package services

import (
	"time"

	"feed-curator/database"
	"feed-curator/models"
	"feed-curator/utils"
)

/*
Function to push app logs and news-archive to MinIO
News-archival: First get a event message from Redis about the update of data
Then push the data into MinIO bucket called news-archive
App-logs: Here we store the logs of the curator service - We push the summarizer.log into
MinIO as a version controlled file in batch routines.
*/

func PushAppLogToMinIO() {
    ticker := time.NewTicker(20 * time.Second) 
    defer ticker.Stop()
    utils.LogMessage("Started Log to MinIO Push Service", "green")

    for range ticker.C {
        utils.LogMessage("MinIO Push App Log Procedure started", "green") 
        err := database.UploadLogDataToMinIO(models.MinIOClient, "app-logs", "summarizer.log")
        if err != nil {
            utils.LogMessage("Error pushing log file to MinIO", "red", err)
        }
        utils.LogMessage("Done Uploading Procedure for App Logs to MinIO", "green")
    }
}
