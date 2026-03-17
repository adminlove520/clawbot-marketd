package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ========== 签到 ==========

func (s *Server) handleCheckin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 检查今天是否已签到
	checkin, err := s.DB.GetOrCreateCheckin(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	today := time.Now().Format("2006-01-02")
	if checkin.LastCheckin == today {
		http.Error(w, "Already checked in today", http.StatusBadRequest)
		return
	}

	// 计算奖励
	baseReward := 10.0
	streakBonus := float64(checkin.Streak) * 2
	reward := baseReward + streakBonus

	newStreak, err := s.DB.ProcessCheckin(agentID, reward)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "checkin", "agent", agentID, fmt.Sprintf("streak:%d,reward:%.2f", newStreak, reward))

	// 联动龙虾文明
	go func() {
		agentName := fmt.Sprintf("agent_%d", agentID)
		apiURL := fmt.Sprintf("https://lobsterhub-api.vercel.app/api/checkin?name=%s&realm=cyber", agentName)
		http.Get(apiURL)
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"streak":  newStreak,
		"reward":  reward,
		"message": fmt.Sprintf("签到成功！连续 %d 天，奖励 %.0f 积分", newStreak, reward),
	})
}

// ========== 红包 ==========

func (s *Server) handleRedPackets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.createRedPacket(w, r)
	case "GET":
		s.listRedPackets(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createRedPacket(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Amount float64 `json:"amount"`
		Count  int     `json:"count"`
		Realm  string  `json:"realm"`
	}
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.Amount <= 0 || req.Count <= 0 {
		http.Error(w, "Invalid amount or count", http.StatusBadRequest)
		return
	}

	// 检查余额
	balance, _ := s.DB.GetBalance(agentID)
	if balance < req.Amount {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// 创建红包
	packetID, err := s.DB.CreateRedPacket(agentID, req.Amount, req.Count, req.Realm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "create_red_packet", "red_packet", packetID, fmt.Sprintf("amount:%.2f,count:%d", req.Amount, req.Count))

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      packetID,
		"message": "红包创建成功",
	})
}

func (s *Server) listRedPackets(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 获取所有有效红包
	rows, err := s.DB.Query("SELECT id, sender_id, amount, count, remaining, realm, created_at FROM red_packets WHERE remaining > 0 AND sender_id != ? ORDER BY id DESC", agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var packets []map[string]interface{}
	for rows.Next() {
		var id, senderID int64
		var amount, remaining float64
		var count int
		var realm, createdAt string
		rows.Scan(&id, &senderID, &amount, &count, &remaining, &realm, &createdAt)
		packets = append(packets, map[string]interface{}{
			"id":          id,
			"sender_id":   senderID,
			"amount":      amount,
			"count":      count,
			"remaining":   remaining,
			"realm":       realm,
			"created_at":  createdAt,
		})
	}
	writeJSON(w, http.StatusOK, packets)
}

// 抢红包
func (s *Server) handleClaimRedPacket(w http.ResponseWriter, r *http.Request) {
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
		PacketID int64 `json:"packet_id"`
	}
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	amount, err := s.DB.ClaimRedPacket(req.PacketID, agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "claim_red_packet", "red_packet", req.PacketID, fmt.Sprintf("amount:%.2f", amount))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"amount":  amount,
		"message": fmt.Sprintf("抢到 %.2f 积分！", amount),
	})
}
