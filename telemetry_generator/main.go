package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
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

func main() {
	time.Sleep(5 * time.Second) // wait for redpanda
	
	brokers := []string{"redpanda:9092"}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kafka client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	ctx := context.Background()
	ticker := time.NewTicker(200 * time.Millisecond) // Throttling: 5 events per second for light weight
	defer ticker.Stop()

	fmt.Println("Starting Telemetry Generator...")

	entities := []string{"user_A", "user_B", "service_X", "db_admin"}
	ips := []string{"192.168.1.5", "10.0.0.12", "172.16.0.4", "198.51.100.14"}
	actionsNorm := []string{"LOGIN", "READ_FILE", "DB_QUERY", "LOGOUT"}
	
	// Anomalous actions chain example
	anomalousActions := []string{"FAILED_LOGIN", "PORT_SCAN", "READ_SENSITIVE_FILE", "ALTER_IAM_POLICY"}

	var anomalyState int = 0
	var anomalyEntity string = ""
	var anomalyIP string = ""

	for {
		<-ticker.C

		var payload TelemetryPayload
		
		isAnomaly := rand.Float32() < 0.05 // 5% chance to start/continue anomaly

		if isAnomaly || anomalyState > 0 {
			if anomalyState == 0 {
				anomalyEntity = entities[rand.Intn(len(entities))]
				anomalyIP = ips[rand.Intn(len(ips))]
			}
			
			payload = TelemetryPayload{
				EventID:   fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				Timestamp: time.Now(),
				EntityID:  anomalyEntity,
				IPAddress: anomalyIP,
				Action:    anomalousActions[anomalyState],
				Status:    "SUCCESS",
				RiskScore: 0.8 + rand.Float64()*0.2, // high risk
			}
			
			if payload.Action == "FAILED_LOGIN" {
				payload.Status = "FAILURE"
			}
			
			anomalyState++
			if anomalyState >= len(anomalousActions) {
				anomalyState = 0 // reset chain
			}

		} else {
			payload = TelemetryPayload{
				EventID:   fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				Timestamp: time.Now(),
				EntityID:  entities[rand.Intn(len(entities))],
				IPAddress: ips[rand.Intn(len(ips))],
				Action:    actionsNorm[rand.Intn(len(actionsNorm))],
				Status:    "SUCCESS",
				RiskScore: 0.0 + rand.Float64()*0.1, // low risk
			}
		}

		data, _ := json.Marshal(payload)
		
		record := &kgo.Record{
			Topic: "raw-telemetry",
			Value: data,
		}
		
		client.Produce(ctx, record, func(r *kgo.Record, err error) {
			if err == nil {
				fmt.Printf("Produced record to raw-telemetry: %s\n", string(data))
			} else {
				fmt.Printf("Produce error: %v\n", err)
			}
		})
	}
}
