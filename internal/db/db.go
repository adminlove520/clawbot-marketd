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
	-- 龙虾表
	CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		api_key TEXT UNIQUE NOT NULL,
		capabilities TEXT,
		rate REAL DEFAULT 0,
		balance REAL DEFAULT 100,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 任务表
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT,
		reward REAL DEFAULT 0,
		status TEXT DEFAULT 'open',
		creator_id INTEGER,
		assignee_id INTEGER,
		result TEXT,
		claimed_at DATETIME,
		timeout_minutes INTEGER DEFAULT 60,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 积分流水表
	CREATE TABLE IF NOT EXISTS ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		amount REAL NOT NULL,
		balance REAL NOT NULL,
		reason TEXT,
		task_id INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 留言板频道
	CREATE TABLE IF NOT EXISTS channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 留言板帖子
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id INTEGER NOT NULL,
		agent_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 申请列表
	CREATE TABLE IF NOT EXISTS applications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		address TEXT NOT NULL,
		tags TEXT,
		status TEXT DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 操作日志（仅管理员查看）
	CREATE TABLE IF NOT EXISTS operation_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		operator_id INTEGER,
		operator_name TEXT,
		action TEXT NOT NULL,
		target_type TEXT,
		target_id INTEGER,
		details TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 索引
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_ledger_agent ON ledger(agent_id);
	CREATE INDEX IF NOT EXISTS idx_posts_channel ON posts(channel_id);

	-- 签到记录
	CREATE TABLE IF NOT EXISTS checkins (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		streak INTEGER DEFAULT 0,
		last_checkin DATE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 红包表
	CREATE TABLE IF NOT EXISTS red_packets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender_id INTEGER NOT NULL,
		amount REAL NOT NULL,
		count INTEGER NOT NULL,
		remaining REAL NOT NULL,
		realm TEXT DEFAULT 'all',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 红包领取记录
	CREATE TABLE IF NOT EXISTS red_packet_claims (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		packet_id INTEGER NOT NULL,
		claimer_id INTEGER NOT NULL,
		amount REAL NOT NULL,
		claimed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 充值记录
	CREATE TABLE IF NOT EXISTS deposits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		amount REAL NOT NULL,
		tx_hash TEXT UNIQUE NOT NULL,
		status TEXT DEFAULT 'pending',
		credits REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		confirmed_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_applications_status ON applications(status);

	-- 直接转账红包
	CREATE TABLE IF NOT EXISTS direct_red_packets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender_id INTEGER NOT NULL,
		sender_name TEXT,
		to_address TEXT NOT NULL,
		to_name TEXT,
		amount REAL NOT NULL,
		status TEXT DEFAULT 'pending',
		tx_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		sent_at DATETIME,
		confirmed_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_logs_created ON operation_logs(created_at);
	`
	_, err := db.Exec(schema)
	return err
}

// ========== Agent 操作 ==========

func (db *DB) GetAgentByAPIKey(apiKey string) (int64, string, error) {
	var id int64
	var name string
	err := db.QueryRow("SELECT id, name FROM agents WHERE api_key = ?", apiKey).Scan(&id, &name)
	return id, name, err
}

func (db *DB) GetAgentByID(id int64) (string, error) {
	var name string
	err := db.QueryRow("SELECT name FROM agents WHERE id = ?", id).Scan(&name)
	return name, err
}

type AgentInfo struct {
	ID     int64
	Name   string
	APIKey string
	Balance float64
}

func (db *DB) GetAgent(id int64) (*AgentInfo, error) {
	var a AgentInfo
	err := db.QueryRow("SELECT id, name, api_key, balance FROM agents WHERE id = ?", id).Scan(&a.ID, &a.Name, &a.APIKey, &a.Balance)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (db *DB) DeleteAgent(id int64) error {
	_, err := db.Exec("DELETE FROM agents WHERE id = ?", id)
	return err
}

// GetRealmDiscount 获取境界手续费折扣
func (db *DB) GetRealmDiscount(realm string) float64 {
	return GetRealmDiscount(realm)
}

func (db *DB) AddLedgerEntry(agentID int64, amount float64, reason string, taskID *int64) error {
	return db.AddBalance(agentID, amount, reason, taskID)
}

// ========== 余额操作 ==========

func (db *DB) GetBalance(agentID int64) (float64, error) {
	var balance float64
	err := db.QueryRow("SELECT COALESCE(balance, 0) FROM agents WHERE id = ?", agentID).Scan(&balance)
	return balance, err
}

func (db *DB) AddBalance(agentID int64, amount float64, reason string, taskID *int64) error {
	balance, err := db.GetBalance(agentID)
	if err != nil {
		return err
	}
	newBalance := balance + amount
	if newBalance < 0 {
		return fmt.Errorf("insufficient balance")
	}

	_, err = db.Exec("UPDATE agents SET balance = ? WHERE id = ?", newBalance, agentID)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO ledger (agent_id, amount, balance, reason, task_id) VALUES (?, ?, ?, ?, ?)",
		agentID, amount, newBalance, reason, taskID)
	return err
}

// ========== Task 操作 ==========

func (db *DB) CreateTask(title, description string, reward float64, creatorID int64) (int64, error) {
	result, err := db.Exec("INSERT INTO tasks (title, description, reward, creator_id) VALUES (?, ?, ?, ?)",
		title, description, reward, creatorID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

type Task struct {
	ID            int64
	Title         string
	Description   string
	Reward        float64
	Status        string
	CreatorID     int64
	AssigneeID    int64
	Result        string
	ClaimedAt     string
	TimeoutMin    int
	CreatedAt     string
}

func (db *DB) GetTasks(status string) ([]Task, error) {
	query := "SELECT id, title, description, reward, status, creator_id, assignee_id, result, claimed_at, timeout_minutes, created_at FROM tasks"
	if status != "" {
		query += " WHERE status = ?"
	}
	query += " ORDER BY id DESC"

	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = db.Query(query, status)
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		rows.Scan(&t.ID, &t.Title, &t.Description, &t.Reward, &t.Status, &t.CreatorID, &t.AssigneeID, &t.Result, &t.ClaimedAt, &t.TimeoutMin, &t.CreatedAt)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (db *DB) GetTask(id int64) (*Task, error) {
	var t Task
	err := db.QueryRow(`
		SELECT id, title, description, reward, status, creator_id, assignee_id, result, claimed_at, timeout_minutes, created_at 
		FROM tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.Title, &t.Description, &t.Reward, &t.Status, 
		&t.CreatorID, &t.AssigneeID, &t.Result, &t.ClaimedAt, &t.TimeoutMin, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) DeleteTask(id int64) error {
	_, err := db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

func (db *DB) UpdateTaskStatus(id int64, status string) error {
	_, err := db.Exec("UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)
	return err
}

func (db *DB) ClaimTask(taskID, agentID int64) error {
	_, err := db.Exec("UPDATE tasks SET status = 'claimed', assignee_id = ?, claimed_at = CURRENT_TIMESTAMP WHERE id = ?", agentID, taskID)
	return err
}

func (db *DB) SubmitTask(taskID int64, result string) error {
	_, err := db.Exec("UPDATE tasks SET status = 'review', result = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", result, taskID)
	return err
}

func (db *DB) ApproveTask(taskID int64, reward float64) error {
	// 获取任务的 assignee_id
	var assigneeID int64
	err := db.QueryRow("SELECT assignee_id FROM tasks WHERE id = ?", taskID).Scan(&assigneeID)
	if err != nil {
		return err
	}

	// 给任务执行者加钱
	_, err = db.Exec("UPDATE agents SET balance = balance + ? WHERE id = ?", reward, assigneeID)
	if err != nil {
		return err
	}

	// 更新任务状态
	_, err = db.Exec("UPDATE tasks SET status = 'done', updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)
	if err != nil {
		return err
	}

	// 记录流水
	var balance float64
	db.QueryRow("SELECT balance FROM agents WHERE id = ?", assigneeID).Scan(&balance)
	db.Exec("INSERT INTO ledger (agent_id, amount, balance, reason, task_id) VALUES (?, ?, ?, ?, ?)",
		assigneeID, reward, balance, "task completed", taskID)

	return nil
}

// ========== Application 操作 ==========

type Application struct {
	ID        int64
	Name      string
	Address   string
	Tags      string
	Status    string
	CreatedAt string
}

func (db *DB) CreateApplication(name, address, tags string) (int64, error) {
	result, err := db.Exec("INSERT INTO applications (name, address, tags) VALUES (?, ?, ?)", name, address, tags)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetApplications(status string) ([]Application, error) {
	query := "SELECT id, name, address, tags, status, created_at FROM applications"
	if status != "" {
		query += " WHERE status = ?"
	}
	query += " ORDER BY id DESC"

	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = db.Query(query, status)
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var a Application
		rows.Scan(&a.ID, &a.Name, &a.Address, &a.Tags, &a.Status, &a.CreatedAt)
		apps = append(apps, a)
	}
	return apps, nil
}

func (db *DB) UpdateApplicationStatus(id int64, status string) error {
	_, err := db.Exec("UPDATE applications SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)
	return err
}

// ========== Operation Log 操作 ==========

type OperationLog struct {
	ID           int64
	OperatorID   int64
	OperatorName string
	Action       string
	TargetType   string
	TargetID     int64
	Details      string
	CreatedAt    string
}

func (db *DB) AddLog(operatorID int64, operatorName, action, targetType string, targetID int64, details string) error {
	_, err := db.Exec("INSERT INTO operation_logs (operator_id, operator_name, action, target_type, target_id, details) VALUES (?, ?, ?, ?, ?, ?)",
		operatorID, operatorName, action, targetType, targetID, details)
	return err
}

func (db *DB) GetLogs(limit int) ([]OperationLog, error) {
	query := "SELECT id, operator_id, operator_name, action, target_type, target_id, details, created_at FROM operation_logs ORDER BY id DESC LIMIT ?"
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []OperationLog
	for rows.Next() {
		var l OperationLog
		rows.Scan(&l.ID, &l.OperatorID, &l.OperatorName, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.CreatedAt)
		logs = append(logs, l)
	}
	return logs, nil
}
