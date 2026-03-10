package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) adminCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Reward          float64 `json:"reward"`
		TimeoutMinutes  int     `json:"timeout_minutes"`
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
		VALUES (?, ?, ?, ?, 0)
	`, req.Title, req.Description, req.Reward, req.TimeoutMinutes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	taskID, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": taskID})
}

func (s *Server) adminReviewTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		TaskID   int64  `json:"task_id"`
		Approved bool   `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var assigneeID int64
	var reward float64
	err := s.DB.QueryRow("SELECT assignee_id, reward FROM tasks WHERE id = ? AND status = 'review'", req.TaskID).Scan(&assigneeID, &reward)
	if err != nil {
		http.Error(w, "Task not found or not in review", http.StatusNotFound)
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

func (s *Server) adminCreateChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec("INSERT INTO channels (name, description) VALUES (?, ?)", req.Name, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	channelID, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": channelID})
}
