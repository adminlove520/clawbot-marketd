package server

import (
	"encoding/json"
	"net/http"

	"github.com/ythx-101/lobsterhub/internal/auth"
)

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.registerAgent(w, r)
	case "GET":
		s.listAgents(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) registerAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string  `json:"name"`
		Capabilities string  `json:"capabilities"`
		Rate         float64 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey := auth.GenerateAPIKey()
	result, err := s.DB.Exec("INSERT INTO agents (name, api_key, capabilities, rate) VALUES (?, ?, ?, ?)",
		req.Name, apiKey, req.Capabilities, req.Rate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	agentID, _ := result.LastInsertId()
	if err := s.DB.AddLedgerEntry(agentID, 100, "registration bonus", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      agentID,
		"name":    req.Name,
		"api_key": apiKey,
	})
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query("SELECT id, name, capabilities, rate FROM agents")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var agents []map[string]interface{}
	for rows.Next() {
		var id int64
		var name, capabilities string
		var rate float64
		rows.Scan(&id, &name, &capabilities, &rate)
		agents = append(agents, map[string]interface{}{
			"id":           id,
			"name":         name,
			"capabilities": capabilities,
			"rate":         rate,
		})
	}
	writeJSON(w, http.StatusOK, agents)
}
