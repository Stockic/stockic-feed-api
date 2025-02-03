package main

import (
    "sync"
	"log-redis-pusher/database"
	"log-redis-pusher/services"
)

func init() {
    database.Initialize()
}

func main() {
    var wg sync.WaitGroup

    wg.Add(2)
    go services.PushAppLogToMinIO(&wg)
    go services.SyncLogRedisToMinIO(&wg)
    wg.Wait()
}
