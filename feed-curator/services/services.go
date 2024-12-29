package services

import (
	"encoding/json"
	"fmt"
    "bytes"
	"time"

    "github.com/minio/minio-go/v7"

	"feed-curator/models"
)

func uploadToMinIO(minioClient *minio.Client, key string, value string, MinIOBucket string) error {
	
    payload := map[string]interface{}{
		"key":   key,
		"value": value,
		"time":  time.Now().Format(time.RFC3339),
	}
    
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

    reader := bytes.NewReader(payloadBytes)

	objectName := fmt.Sprintf("%s.json", key)
	_, err = minioClient.PutObject(models.MinIOCtx, 
        MinIOBucket,                            // Bucket Name 
        objectName,                             // Object Name
		reader,                                 // Reader of JSON file 
        int64(len(payloadBytes)),               // Size of file
        minio.PutObjectOptions{                 // Metadata
            ContentType: "application/json",
    })

    if err != nil {
        return fmt.Errorf("Error uploading file: %s Error: %v", key, err)
    }

	return nil
}
