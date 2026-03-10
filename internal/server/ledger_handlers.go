package server

import (
	"net/http"
	"strconv"
)

func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentIDParam := r.URL.Query().Get("agent_id")
	if agentIDParam != "" {
		id, _ := strconv.ParseInt(agentIDParam, 10, 64)
		if id > 0 {
			agentID = id
		}
	}

	rows, err := s.DB.Query(`
		SELECT id, amount, balance, reason, task_id, created_at 
		FROM ledger 
		WHERE agent_id = ? 
		ORDER BY id DESC
	`, agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []map[string]interface{}
	for rows.Next() {
		var id, taskID int64
		var amount, balance float64
		var reason, createdAt string
		rows.Scan(&id, &amount, &balance, &reason, &taskID, &createdAt)
		entries = append(entries, map[string]interface{}{
			"id":         id,
			"amount":     amount,
			"balance":    balance,
			"reason":     reason,
			"task_id":    taskID,
			"created_at": createdAt,
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var balance float64
	err = s.DB.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM ledger WHERE agent_id = ?", agentID).Scan(&balance)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"agent_id": agentID, "balance": balance})
}
