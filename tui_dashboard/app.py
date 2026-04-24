import sqlite3
import os
import time
from textual.app import App, ComposeResult
from textual.widgets import Header, Footer, DataTable, Static
from textual.containers import Container
from textual.reactive import reactive

DB_PATH = "../shared_data/alerts.db"

class StatsWidget(Static):
    # Responsive statistic
    content = reactive("Waiting for system data...")
    def render(self) -> str:
        return self.content

class AtareTUI(App):
    CSS = """
    Screen {
        background: $surface-darken-1;
    }
    #stats-bar {
        height: 3;
        dock: top;
        content-align: center middle;
        background: $primary-background;
        color: $text;
        text-style: bold;
    }
    DataTable {
        height: 100%;
        margin-top: 1;
    }
    """
    
    BINDINGS = [
        ("q", "quit", "Quit"),
        ("r", "refresh", "Manual Refresh")
    ]

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        self.stats = StatsWidget(id="stats-bar")
        yield self.stats
        self.table = DataTable()
        yield self.table
        yield Footer()

    def on_mount(self) -> None:
        self.table.add_columns("ID", "Entity", "IP", "Actions/Events", "Status")
        self.fetch_data()
        self.set_interval(2.0, self.fetch_data) # Auto refresh every 2 seconds

    def action_refresh(self) -> None:
        self.fetch_data()

    def fetch_data(self) -> None:
        if not os.path.exists(DB_PATH):
            self.stats.content = "SYSTEM STATUS: BOOTING... [Database not initialized]"
            return

        try:
            conn = sqlite3.connect(DB_PATH)
            cur = conn.cursor()
            
            # Fetch latest 20 alerts
            cur.execute("SELECT id, entity_id, ip_address, actions, status FROM alerts ORDER BY id DESC LIMIT 20")
            rows = cur.fetchall()
            
            # Count stats
            cur.execute("SELECT count(*) FROM alerts")
            total = cur.fetchone()[0]
            
            cur.execute("SELECT count(*) FROM alerts WHERE status = 'PENDING_AGENT'")
            pending = cur.fetchone()[0]
            
            conn.close()

            self.stats.content = f"🔥 ATARE ACTIVE | Total Anomalies: {total} | ⏳ Pending Agent Triage: {pending} | 🧠 Running Model: Qwen2.5:0.5B (Edge Quantized)"
            
            self.table.clear()
            for row in rows:
                id_col = f"[bold green]#{row[0]}[/bold green]"
                entity = f"[bold white]{row[1]}[/bold white]"
                ip = f"[cyan]{row[2]}[/cyan]"
                
                status_color = "red" if "PENDING" in row[4] else "green"
                status = f"[{status_color}]{row[4]}[/{status_color}]"
                
                # Truncate action chain if too long
                actions = row[3]
                if len(actions) > 50:
                    actions = actions[:47] + "..."
                
                self.table.add_row(id_col, entity, ip, actions, status)

        except sqlite3.OperationalError:
            # DB locked or mid-creation
            pass

if __name__ == "__main__":
    app = AtareTUI()
    app.run()
