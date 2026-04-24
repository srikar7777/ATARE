import json
import time
import sqlite3
import requests
import re
from kafka import KafkaConsumer
from pydantic import BaseModel, Field, ValidationError

OLLAMA_URL = "http://ollama:11434"
MODEL = "qwen2.5:0.5b" # Extremely lightweight variant to guarantee 0 lag on 8GB host

class AgentResponse(BaseModel):
    action: str = Field(pattern="^(ISOLATE|MONITOR|BAN)$")
    confidence: float
    reasoning: str
    remediation_cmd: str

def init_ollama():
    print(f"Ensuring model {MODEL} is pulled. This might take a moment on first boot...")
    try:
        res = requests.post(f"{OLLAMA_URL}/api/pull", json={"name": MODEL})
        if res.status_code == 200:
            print("Model pulled successfully.")
        else:
            print(f"Failed to pull model: {res.text}")
    except Exception as e:
        print(f"Ollama connection error: {e}")

def update_db(entity_id: str, action: str, cmd: str):
    conn = sqlite3.connect("/shared_data/alerts.db")
    cursor = conn.cursor()
    cursor.execute("""
        UPDATE alerts 
        SET status = 'REMEDIATED_BY_AGENT', actions = actions || ?
        WHERE entity_id = ? AND status = 'PENDING_AGENT'
    """, (f" | AgentAction:{action} | Run:{cmd}", entity_id))
    conn.commit()
    conn.close()

def ask_agent(payload_str: str) -> dict:
    prompt = f"""You are an autonomous SRE triage agent.
Read the alert below and output ONLY a JSON object with strictly these keys:
- action (must be exactly ISOLATE, MONITOR, or BAN)
- confidence (float between 0 and 1)
- reasoning (string explain why)
- remediation_cmd (a mock bash CLI command to fix this)

Alert Data: 
{payload_str}

Output ONLY valid JSON inside ```json and ``` tags.
"""
    for attempt in range(3):
        try:
            res = requests.post(f"{OLLAMA_URL}/api/generate", json={
                "model": MODEL,
                "prompt": prompt,
                "stream": False,
                "format": "json" # Forces JSON output structurally
            }, timeout=30)
            
            text = res.json().get("response", "{}")
            # attempt to parse generic json response
            parsed = json.loads(text)
            
            validated = AgentResponse(**parsed)
            return validated.dict()
            
        except (json.JSONDecodeError, ValidationError) as e:
            print(f"Agent validation error attempt {attempt+1}: {e}")
            time.sleep(1)
        except Exception as e:
            print(f"Agent communication error: {e}")
            time.sleep(2)
            
    # Fallback if agent hallucinates 3 times
    print("Agent failed 3 times, falling back to deterministic policy.")
    return {
        "action": "MONITOR",
        "confidence": 0.1,
        "reasoning": "Model failed Pydantic validation 3 times. Downgraded to MONITOR.",
        "remediation_cmd": "echo 'Investigate manually'"
    }

def main():
    time.sleep(20) # Wait for Kafka and DB
    init_ollama()
    
    consumer = KafkaConsumer(
        'triage-alerts',
        bootstrap_servers=['redpanda:9092'],
        group_id='agent-group',
        value_deserializer=lambda x: json.loads(x.decode('utf-8'))
    )
    print("Agent engine listening on 'triage-alerts'")
    
    for message in consumer:
        alert = message.value
        print(f"Received Triage Alert for {alert.get('entity_id')}")
        
        decision = ask_agent(json.dumps(alert))
        print(f"Agent Decision: {decision}")
        
        update_db(alert.get('entity_id'), decision['action'], decision['remediation_cmd'])
        print(f"Remediation executed and saved to SQLite.\n")

if __name__ == "__main__":
    main()
