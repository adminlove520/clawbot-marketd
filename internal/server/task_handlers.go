package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.createTask(w, r)
		return
	case "GET":
		// fall through to list
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := r.URL.Query().Get("status")
	query := "SELECT id, title, description, reward, status, creator_id, assignee_id FROM tasks"
	args := []interface{}{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []map[string]interface{}
	for rows.Next() {
		var id, creatorID int64
		var assigneeID sql.NullInt64
		var title, description, status string
		var reward float64
		rows.Scan(&id, &title, &description, &reward, &status, &creatorID, &assigneeID)
		task := map[string]interface{}{
			"id":          id,
			"title":       title,
			"description": description,
			"reward":      reward,
			"status":      status,
			"creator_id":  creatorID,
		}
		if assigneeID.Valid {
			task["assignee_id"] = assigneeID.Int64
		}
		tasks = append(tasks, task)
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID int64 `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec(`
		UPDATE tasks 
		SET status = 'claimed', assignee_id = ?, claimed_at = datetime('now') 
		WHERE id = ? AND status = 'open'
	`, agentID, req.TaskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Task not available", http.StatusConflict)
		return
	}

	_, err = s.DB.Exec("UPDATE tasks SET status = 'working' WHERE id = ?", req.TaskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "claimed"})
}

func (s *Server) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID int64  `json:"task_id"`
		Result string `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec(`
		UPDATE tasks 
		SET status = 'review' 
		WHERE id = ? AND assignee_id = ? AND status = 'working'
	`, req.TaskID, agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Task not found or not assigned to you", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		// allow admin too
		if !s.requireAdmin(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		agentID = 0
	}

	var req struct {
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		Reward         float64 `json:"reward"`
		TimeoutMinutes int     `json:"timeout_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.TimeoutMinutes == 0 {
		req.TimeoutMinutes = 60
	}

	result, err := s.DB.Exec(`
		INSERT INTO tasks (title, description, reward, timeout_minutes, creator_id)
		VALUES (?, ?, ?, ?, ?)
	`, req.Title, req.Description, req.Reward, req.TimeoutMinutes, agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	taskID, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": taskID, "status": "open"})
}

func (s *Server) handleApproveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID, err := s.authenticate(r)
	isAdmin := s.requireAdmin(r)
	if err != nil && !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID   int64 `json:"task_id"`
		Approved bool  `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var creatorID, assigneeID int64
	var reward float64
	err = s.DB.QueryRow("SELECT creator_id, assignee_id, reward FROM tasks WHERE id = ? AND status = 'review'",
		req.TaskID).Scan(&creatorID, &assigneeID, &reward)
	if err != nil {
		http.Error(w, "Task not found or not in review", http.StatusNotFound)
		return
	}

	// Only creator or admin can approve
	if !isAdmin && agentID != creatorID {
		http.Error(w, "Only task creator can approve", http.StatusForbidden)
		return
	}

	status := "failed"
	if req.Approved {
		status = "done"
		if err := s.DB.AddLedgerEntry(assigneeID, reward, "task completed", &req.TaskID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	_, err = s.DB.Exec("UPDATE tasks SET status = ? WHERE id = ?", status, req.TaskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}
