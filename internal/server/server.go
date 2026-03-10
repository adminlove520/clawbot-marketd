package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ythx-101/lobsterhub/internal/auth"
	"github.com/ythx-101/lobsterhub/internal/db"
)

type Server struct {
	DB       *db.DB
	AdminKey string
}

func New(database *db.DB, adminKey string) *Server {
	return &Server{DB: database, AdminKey: adminKey}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "lobsterhub"})
	})

	// Agents
	mux.HandleFunc("/api/agents", s.handleAgents)

	// Tasks - GET list, POST create (any agent)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/claim", s.handleClaimTask)
	mux.HandleFunc("/api/tasks/submit", s.handleSubmitTask)
	mux.HandleFunc("/api/tasks/approve", s.handleApproveTask)

	// Ledger
	mux.HandleFunc("/api/ledger", s.handleLedger)
	mux.HandleFunc("/api/ledger/balance", s.handleBalance)

	// Board
	mux.HandleFunc("/api/channels", s.handleChannels)
	mux.HandleFunc("/api/posts", s.handlePosts)

	// Admin
	mux.HandleFunc("/admin/agents", s.handleAgents)
	mux.HandleFunc("/admin/channels/create", s.adminCreateChannel)

	return mux
}

func (s *Server) authenticate(r *http.Request) (int64, error) {
	token := auth.ExtractToken(r)
	if token == "" {
		return 0, http.ErrNoCookie
	}
	agentID, _, err := s.DB.GetAgentByAPIKey(token)
	return agentID, err
}

func (s *Server) requireAdmin(r *http.Request) bool {
	return auth.ExtractToken(r) == s.AdminKey
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) releaseTimedOutTasks() {
	for {
		time.Sleep(30 * time.Second)
		_, err := s.DB.Exec(`
			UPDATE tasks 
			SET status = 'open', assignee_id = NULL, claimed_at = NULL 
			WHERE status = 'claimed' 
			AND datetime(claimed_at, '+' || timeout_minutes || ' minutes') < datetime('now')
		`)
		if err != nil {
			continue
		}
	}
}

func (s *Server) Start(addr string) error {
	go s.releaseTimedOutTasks()
	return http.ListenAndServe(addr, s.Routes())
}
