package server

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

// 验证 ETH 地址
func isValidETHAddress(address string) bool {
	// ETH 地址格式: 0x 开头 + 40位 hex = 42字符
	ethRegex := regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)
	return ethRegex.MatchString(address)
}

// ========== Applications ==========

func (s *Server) handleApplications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.createApplication(w, r)
	case "GET":
		s.listApplications(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Tags    string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 验证钱包地址
	if req.Address != "" && !isValidETHAddress(req.Address) {
		http.Error(w, "Invalid ETH address format", http.StatusBadRequest)
		return
	}

	id, err := s.DB.CreateApplication(req.Name, req.Address, req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "status": "pending"})
}

func (s *Server) listApplications(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	status := r.URL.Query().Get("status")
	apps, err := s.DB.GetApplications(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, a := range apps {
		app := map[string]interface{}{
			"id":         a.ID,
			"name":       a.Name,
			"address":   a.Address,
			"tags":       a.Tags,
			"status":     a.Status,
			"created_at": a.CreatedAt,
		}
		result = append(result, app)
	}
	writeJSON(w, http.StatusOK, result)
}

// ========== Admin Operations ==========

// 操作日志
func (s *Server) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	logs, err := s.DB.GetLogs(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, l := range logs {
		log := map[string]interface{}{
			"id":            l.ID,
			"operator_id":   l.OperatorID,
			"operator_name": l.OperatorName,
			"action":        l.Action,
			"target_type":   l.TargetType,
			"target_id":     l.TargetID,
			"details":       l.Details,
			"created_at":    l.CreatedAt,
		}
		result = append(result, log)
	}
	writeJSON(w, http.StatusOK, result)
}

// 删除 Agent
func (s *Server) handleAdminDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		AgentID int64 `json:"agent_id"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.Unmarshal(body, &req)

	name, err := s.DB.GetAgentByID(req.AgentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}
	if err := s.DB.DeleteAgent(req.AgentID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(0, "admin", "delete_agent", "agent", req.AgentID, "name:"+name)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// 删除任务
func (s *Server) handleAdminDeleteTask(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		TaskID int64 `json:"task_id"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.Unmarshal(body, &req)

	// 直接删除，不获取任务详情
	if err := s.DB.DeleteTask(req.TaskID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
