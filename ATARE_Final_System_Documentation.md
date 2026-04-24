# ATARE: Distributed Agentic Telemetry & Autonomous Response Engine

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/Python-3.11+-3776AB?style=flat&logo=python)](https://www.python.org/)
[![Redpanda](https://img.shields.io/badge/Redpanda-Event_Streaming-black?style=flat)](https://redpanda.com/)
[![Redis](https://img.shields.io/badge/Redis-State_Store-DC382D?style=flat&logo=redis)](https://redis.io/)
[![Ollama](https://img.shields.io/badge/Ollama-Local_LLM-white?style=flat&logo=ollama)](https://ollama.com/)
[![TUI](https://img.shields.io/badge/UI-Terminal_Dashboard-4CAF50?style=flat)](https://github.com/charmbracelet/bubbletea)

## Overview
ATARE is a high-throughput, horizontally scalable distributed system designed to ingest, correlate, and autonomously triage streaming data. Engineered strictly as a Software Development Engineering (SDE) initiative, it abandons static rule-based processing in favor of semantic vector retrieval and localized Machine Learning inference. 

The architecture leverages a Go/Redpanda pipeline for low-latency event streaming, Redis for distributed state correlation, an entirely local LLM operating as an autonomous agent via the Model Context Protocol (MCP), and a Terminal User Interface (TUI) for real-time visualization. 

This system operates with **zero external API dependencies**, running entirely on local containerized infrastructure engineered specifically to run smoothly on an 8 GB RAM edge machine.

---

## 1. System Architecture

The system relies on decoupled microservices to handle high-volume data ingestion, real-time stream processing, asynchronous ML inference, and ultra-lightweight visualization.

### Tech Stack
* **Ingestion & Stream Processing:** Go, Redpanda (C++ Kafka alternative)
* **State Management:** Redis (Session state & deduplication)
* **Vector Search:** SQLite-VSS (Serverless local vector store)
* **Agentic Engine:** Python, Ollama (Phi-3-Mini / Qwen2.5-1.5B), Model Context Protocol (MCP)
* **Visualization Layer:** Go (BubbleTea) or Python (Textual) for the TUI Dashboard

---

## 2. Module Design

### Module 1: Telemetry Generator (Go)
Generates high-concurrency synthetic JSON payloads simulating distributed system logs (e.g., AWS CloudTrail, VPC Flow Logs). 
* **Execution:** Publishes directly to the `raw-telemetry` Redpanda topic.
* **SDE Implementation:** Uses a probabilistic distribution model where 99% of logs represent baseline system noise, and 1% represent simulated, multi-stage anomalous event chains.

### Module 2: Stream Processor & Vector Correlator (Go + Redpanda + SQLite-VSS)
The primary backend processing engine.
* **Execution:** Consumes `raw-telemetry`. Embeds payload data into vectors and queries a local SQLite-VSS database containing known behavioral anomalies.
* **Stateful Correlation:** If an event threshold is met, context is cached in Redis with a strict Time-To-Live (TTL) of 15 minutes. Subsequent anomalies from the same entity within the TTL are chained.
* **Routing:** Correlated, high-risk event chains are serialized and published to the `triage-alerts` topic.

### Module 3: Agentic Triage Engine (Python + Ollama + MCP)
An asynchronous Python service that acts as the decision engine.
* **Execution:** Consumes `triage-alerts`. Prompts a localized, quantized LLM via Ollama.
* **MCP Integration:** The LLM accesses a custom Python MCP server exposing specific functional tools:
    1.  `query_redis_state(entity_id)`: Fetches rolling event history.
    2.  `check_reputation(ip_address)`: Queries a local mock database for entity reputation.
    3.  `simulate_remediation(action, resource)`: Executes deterministic state-change functions.
* **Validation:** Output is strictly enforced via Pydantic schemas. If the LLM output violates the JSON schema, the system catches the exception and initiates a retry loop.

### Module 4: Autonomous Responder
Closes the loop by executing commands deterministically chosen by the LLM. 
* **Execution:** Records the exact CLI commands or API calls required to isolate the anomalous entity (e.g., modifying IAM policies or security groups), proving idempotency and execution flow. Output is piped to a local database for the dashboard to read.

### Module 5: The TUI Dashboard (Terminal User Interface)
The ultra-lightweight visualization layer that replaces a heavy web frontend.
* **Execution:** A terminal-based dashboard built with Go (BubbleTea) or Python (Textual). It streams the final autonomous decisions from the local SQLite database.
* **UX/UI:** Renders an interactive, auto-updating grid directly in the terminal, displaying active alerts, LLM confidence scores, and the automated remediation actions taken.
* **SDE Impact:** Proves deep command-line proficiency and an understanding of extreme resource constraints, operating at a ~15 MB memory footprint.

---

## 3. Bottleneck Mitigation & Zero-Cost Edge Optimization (8 GB RAM Targeting)

To ensure this enterprise-grade architecture runs flawlessly on an 8 GB RAM laptop without lagging the host OS or incurring cloud costs, the following strict SDE constraints are implemented:

* **TUI over Web UI:** Eliminating Node.js, React, and browser-based rendering drops the dashboard memory footprint from ~1 GB to ~15 MB, preserving crucial RAM for the ML engine.
* **JVM Elimination (Redpanda over Kafka):** Apache Kafka requires the Java Virtual Machine and Zookeeper. Redpanda is a C++ binary compatible with Kafka APIs, reducing idle memory consumption by over 80%.
* **Quantized Edge ML Models:** Running 8B+ parameter models causes massive memory paging. ATARE defaults to **Phi-3-Mini (3.8B)** or **Qwen2.5-1.5B**, reducing the VRAM footprint to < 2.5GB while maintaining sufficient reasoning capabilities for MCP tool utilization.
* **Serverless Vector Storage:** Swapping ChromaDB for **SQLite-VSS** eliminates an entire Python server process, embedding vector search directly into the existing Python application runtime.
* **Container Resource Quotas:** `docker-compose.yml` strictly enforces CPU and memory limits (e.g., capping Redpanda at 1GB RAM) to prevent host OS starvation.
* **Ingestion Throttling:** The Go telemetry generator utilizes `time.Ticker` to rate-limit synthetic payload generation, preventing consumer lag and backend pipeline collapse.

---

## 4. Edge Cases & Reliability Testing

The system is designed to handle the following fault conditions:

1.  **State Expiry & Slow-Drip Chains:** Time-based event correlations expiring from Redis are mitigated by granting the LLM historical query access via MCP tools.
2.  **Backpressure & Consumer Lag:** Graceful degradation is built into the Go stream processor. If Redpanda consumer lag exceeds a defined threshold, the processor dynamically drops vector embedding for known low-priority event types.
3.  **LLM Hallucinations:** Handled via strict Pydantic validation. The system allows 3 retry attempts for malformed JSON outputs before defaulting the payload to a "Human Review" dead-letter queue.
4.  **Race Conditions & Alert Storms:** A deduplication cache in Redis ensures that identical rapid-fire events do not trigger redundant, expensive LLM inference cycles.
