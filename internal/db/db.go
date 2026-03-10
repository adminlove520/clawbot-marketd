package db

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		api_key TEXT UNIQUE NOT NULL,
		capabilities TEXT,
		rate REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT,
		reward REAL DEFAULT 0,
		status TEXT DEFAULT 'open',
		creator_id INTEGER,
		assignee_id INTEGER,
		claimed_at DATETIME,
		timeout_minutes INTEGER DEFAULT 60,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(creator_id) REFERENCES agents(id),
		FOREIGN KEY(assignee_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		amount REAL NOT NULL,
		balance REAL NOT NULL,
		reason TEXT,
		task_id INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(agent_id) REFERENCES agents(id),
		FOREIGN KEY(task_id) REFERENCES tasks(id)
	);

	CREATE TABLE IF NOT EXISTS channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id INTEGER NOT NULL,
		agent_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(channel_id) REFERENCES channels(id),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_ledger_agent ON ledger(agent_id);
	CREATE INDEX IF NOT EXISTS idx_posts_channel ON posts(channel_id);
	`
	_, err := db.Exec(schema)
	return err
}

func (db *DB) GetAgentByAPIKey(apiKey string) (int64, string, error) {
	var id int64
	var name string
	err := db.QueryRow("SELECT id, name FROM agents WHERE api_key = ?", apiKey).Scan(&id, &name)
	return id, name, err
}

func (db *DB) GetBalance(agentID int64) (float64, error) {
	var balance float64
	err := db.QueryRow("SELECT COALESCE(balance, 0) FROM ledger WHERE agent_id = ? ORDER BY id DESC LIMIT 1", agentID).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return balance, err
}

func (db *DB) AddLedgerEntry(agentID int64, amount float64, reason string, taskID *int64) error {
	balance, err := db.GetBalance(agentID)
	if err != nil {
		return err
	}
	newBalance := balance + amount
	if newBalance < 0 {
		return fmt.Errorf("insufficient balance")
	}
	_, err = db.Exec("INSERT INTO ledger (agent_id, amount, balance, reason, task_id) VALUES (?, ?, ?, ?, ?)",
		agentID, amount, newBalance, reason, taskID)
	return err
}
