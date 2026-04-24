package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	_ "github.com/mattn/go-sqlite3"
)

type TelemetryPayload struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	EntityID  string    `json:"entity_id"`
	IPAddress string    `json:"ip_address"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	RiskScore float64   `json:"risk_score"`
}

var ctx = context.Background()

func main() {
	time.Sleep(10 * time.Second) // wait for redpanda/redis to be ready

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
	
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	db, err := sql.Open("sqlite3", "/shared_data/alerts.db")
	if err != nil {
		log.Fatalf("Failed to open SQLite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		entity_id TEXT,
		ip_address TEXT,
		risk_score REAL,
		actions TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT DEFAULT 'PENDING_AGENT'
	);`)
	if err != nil {
		log.Fatalf("Failed to execute table creation: %v", err)
	}

	brokers := []string{"redpanda:9092"}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics("raw-telemetry"),
		kgo.ConsumerGroup("processor-group"),
	)
	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}
	defer client.Close()

	fmt.Println("Stream Processor started and listening...")

	for {
		fetches := client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(t string, p int32, err error) {
			log.Printf("Topic %s partition %d had error: %v", t, p, err)
		})

		fetches.EachRecord(func(record *kgo.Record) {
			var payload TelemetryPayload
			if err := json.Unmarshal(record.Value, &payload); err != nil {
				return // bypass malformed
			}

			// Simulating Vector Search Match: high risk threshold
			if payload.RiskScore > 0.75 {
				stateKey := fmt.Sprintf("state:%s", payload.EntityID)
				
				// Using redis for stateful correlation with 15min TTL
				chainLen, _ := rdb.Incr(ctx, stateKey).Result()
				if chainLen == 1 {
					rdb.Expire(ctx, stateKey, 15*time.Minute)
				}
				
				// Append action history
				historyKey := fmt.Sprintf("history:%s", payload.EntityID)
				rdb.RPush(ctx, historyKey, payload.Action)
				rdb.Expire(ctx, historyKey, 15*time.Minute)

				// Core logic: 3 chained anomalies trigger triage alert
				if chainLen >= 3 {
					actions, _ := rdb.LRange(ctx, historyKey, 0, -1).Result()
					
					alertMsg := map[string]interface{}{
						"entity_id":  payload.EntityID,
						"ip_address": payload.IPAddress,
						"chain_len":  chainLen,
						"actions":    actions,
						"timestamp":  time.Now().Format(time.RFC3339),
					}
					
					alertBytes, _ := json.Marshal(alertMsg)
					prodRecord := &kgo.Record{Topic: "triage-alerts", Value: alertBytes}
					client.Produce(ctx, prodRecord, nil)
					fmt.Printf("Emitted Triage Alert for %s\n", payload.EntityID)
					
					actionsStr, _ := json.Marshal(actions)
					db.Exec(`INSERT INTO alerts (entity_id, ip_address, risk_score, actions) VALUES (?, ?, ?, ?)`, 
						payload.EntityID, payload.IPAddress, payload.RiskScore, string(actionsStr))
					
					// Reset chain to avoid storm
					rdb.Del(ctx, stateKey)
					rdb.Del(ctx, historyKey)
				}
			}
		})
	}
}
