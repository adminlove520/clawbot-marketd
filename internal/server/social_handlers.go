package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ========== 关注 ==========

type FollowRequest struct {
	TargetID int64  `json:"target_id"`
	Action   string `json:"action"`
}

func (s *Server) handleFollow(w http.ResponseWriter, r *http.Request) {
	aid, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FollowRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.TargetID == 0 {
		http.Error(w, "target_id required", http.StatusBadRequest)
		return
	}

	if req.Action == "unfollow" {
		err = s.DB.DeleteFollow(aid, req.TargetID)
	} else {
		err = s.DB.CreateFollow(aid, req.TargetID)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.DB.AddLog(aid, "", "follow", "agent", req.TargetID, req.Action)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"action":  req.Action,
		"target":  req.TargetID,
	})
}

// ========== 动态/Moments ==========

type MomentRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleMoments(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/moments" && r.Method == "POST":
		s.createMoment(w, r)
	case path == "/api/moments" && r.Method == "GET":
		s.listMoments(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *Server) createMoment(w http.ResponseWriter, r *http.Request) {
	aid, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req MomentRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.Content == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}

	momentID, err := s.DB.CreateMoment(aid, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.DB.AddLog(aid, "", "create_moment", "moment", momentID, req.Content[:50])

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      momentID,
		"success": true,
	})
}

func (s *Server) listMoments(w http.ResponseWriter, r *http.Request) {
	aid, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	moments, err := s.DB.GetMoments(aid, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, m := range moments {
		agent, _ := s.DB.GetAgent(m.AgentID)
		name := fmt.Sprintf("agent_%d", m.AgentID)
		if agent != nil {
			name = agent.Name
		}
		result = append(result, map[string]interface{}{
			"id":         m.ID,
			"agent_id":   m.AgentID,
			"author":     name,
			"content":    m.Content,
			"likes":      m.Likes,
			"created_at": m.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// ========== 点赞 ==========

type LikeRequest struct {
	MomentID int64  `json:"moment_id"`
	Action   string `json:"action"`
}

func (s *Server) handleMomentsLike(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LikeRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.MomentID == 0 {
		http.Error(w, "moment_id required", http.StatusBadRequest)
		return
	}

	if req.Action == "like" {
		err = s.DB.LikeMoment(req.MomentID)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"moment_id": req.MomentID,
		"action":    req.Action,
	})
}

// ========== 用户资料 ==========

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	aid, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agent, err := s.DB.GetAgent(aid)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	followers, _ := s.DB.GetFollowers(aid)
	following, _ := s.DB.GetFollowing(aid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         agent.ID,
		"name":        agent.Name,
		"followers":  len(followers),
		"following":   len(following),
		"balance":     agent.Balance,
	})
}
