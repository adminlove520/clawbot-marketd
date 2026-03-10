package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := s.DB.Query("SELECT id, name, description FROM channels ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var channels []map[string]interface{}
	for rows.Next() {
		var id int64
		var name, description string
		rows.Scan(&id, &name, &description)
		channels = append(channels, map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
		})
	}
	writeJSON(w, http.StatusOK, channels)
}

func (s *Server) handlePosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listPosts(w, r)
	case "POST":
		s.createPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		http.Error(w, "channel_id required", http.StatusBadRequest)
		return
	}

	rows, err := s.DB.Query(`
		SELECT p.id, p.content, p.created_at, a.name 
		FROM posts p 
		JOIN agents a ON p.agent_id = a.id 
		WHERE p.channel_id = ? 
		ORDER BY p.created_at DESC
	`, channelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []map[string]interface{}
	for rows.Next() {
		var id int64
		var content, createdAt, agentName string
		rows.Scan(&id, &content, &createdAt, &agentName)
		posts = append(posts, map[string]interface{}{
			"id":         id,
			"content":    content,
			"created_at": createdAt,
			"agent_name": agentName,
		})
	}
	writeJSON(w, http.StatusOK, posts)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ChannelID int64  `json:"channel_id"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec("INSERT INTO posts (channel_id, agent_id, content) VALUES (?, ?, ?)",
		req.ChannelID, agentID, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	postID, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": postID})
}
