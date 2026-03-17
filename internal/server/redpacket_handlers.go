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

// ========== 红包 (复用 Lobster Pie 接口) ==========

// RedpacketRequest 发红包请求
type RedpacketRequest struct {
	Amount    float64 `json:"amount"`     // 金额
	Count     int     `json:"count"`       // 数量
	Realm     string  `json:"realm"`       // 流派限制
	X402      bool    `json:"x402"`        // 是否使用x402链上支付
	ToAddress string  `json:"to_address"`  // 对方钱包地址(x402时必填)
}

// RedpacketClaimRequest 抢红包请求
type RedpacketClaimRequest struct {
	PacketID int64  `json:"packet_id"`
	Wallet   string `json:"wallet"` // 钱包地址，用于x402支付
}

func (s *Server) handleRedpacket(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/redpacket" && r.Method == "POST":
		s.createRedpacket(w, r)
	case path == "/api/redpacket" && r.Method == "GET":
		s.listRedpackets(w, r)
	case path == "/api/redpacket/available" && r.Method == "GET":
		s.availableRedpackets(w, r)
	case path == "/api/redpacket/detail" && r.Method == "GET":
		s.redpacketDetail(w, r)
	case path == "/api/redpacket/claim" && r.Method == "POST":
		s.claimRedpacket(w, r)
	case path == "/api/redpacket/my" && r.Method == "GET":
		s.myRedpackets(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *Server) createRedpacket(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req RedpacketRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.Amount <= 0 || req.Count <= 0 {
		http.Error(w, "Invalid amount or count", http.StatusBadRequest)
		return
	}

	// 如果使用x402链上支付
	if req.X402 {
		if req.ToAddress == "" {
			http.Error(w, "to_address required for x402 payment", http.StatusBadRequest)
			return
		}
		// TODO: 调用x402支付API
		// 这里先模拟成功
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id":       time.Now().UnixNano(),
			"x402":     true,
			"tx_hash":  "0x" + fmt.Sprintf("%x", time.Now().UnixNano()),
			"message":  "x402支付请求已发起",
		})
		return
	}

	// 积分支付
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
	s.DB.AddLog(agentID, "", "create_redpacket", "red_packet", packetID, fmt.Sprintf("amount:%.2f,count:%d", req.Amount, req.Count))

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      packetID,
		"message": "红包创建成功",
	})
}

func (s *Server) listRedpackets(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := s.DB.Query("SELECT id, sender_id, amount, count, remaining, realm, created_at FROM red_packets WHERE remaining > 0 ORDER BY id DESC")
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
			"count":       count,
			"remaining":   remaining,
			"realm":       realm,
			"created_at":  createdAt,
		})
	}
	writeJSON(w, http.StatusOK, packets)
}

func (s *Server) availableRedpackets(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 获取当前用户的境界
	agent, _ := s.DB.GetAgent(agentID)
	userRealm := "xianxia"
	if agent != nil {
		if r, err := s.DB.GetPlayerRealm(agent.Name); err == nil {
			userRealm = r
		}
	}

	rows, err := s.DB.Query("SELECT id, sender_id, amount, count, remaining, realm, created_at FROM red_packets WHERE remaining > 0 AND sender_id != ? AND (realm = 'all' OR realm = ?) ORDER BY id DESC", agentID, userRealm)
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
		
		// 计算手续费折扣
		discount := s.DB.GetRealmDiscount(realm)
		
		packets = append(packets, map[string]interface{}{
			"id":          id,
			"sender_id":   senderID,
			"amount":      amount,
			"count":       count,
			"remaining":   remaining,
			"realm":       realm,
			"discount":    discount,
			"created_at":  createdAt,
		})
	}
	writeJSON(w, http.StatusOK, packets)
}

func (s *Server) redpacketDetail(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	packetID := r.URL.Query().Get("id")
	if packetID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	packet, err := s.DB.GetRedPacket(1) // TODO
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, packet)
}

func (s *Server) claimRedpacket(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req RedpacketClaimRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	amount, err := s.DB.ClaimRedPacket(req.PacketID, agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 如果提供了钱包地址，说明要用x402支付
	response := map[string]interface{}{
		"amount":  amount,
		"message": fmt.Sprintf("抢到 %.2f 积分！", amount),
	}

	if req.Wallet != "" {
		// TODO: 触发x402支付
		response["x402"] = true
		response["wallet"] = req.Wallet
		response["message"] = fmt.Sprintf("抢到 %.2f 积分，x402支付已发起！", amount)
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "claim_redpacket", "red_packet", req.PacketID, fmt.Sprintf("amount:%.2f", amount))

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) myRedpackets(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 获取我发出的红包
	rows, err := s.DB.Query("SELECT id, sender_id, amount, count, remaining, realm, created_at FROM red_packets WHERE sender_id = ? ORDER BY id DESC", agentID)
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
			"count":       count,
			"remaining":   remaining,
			"realm":       realm,
			"created_at":  createdAt,
		})
	}
	writeJSON(w, http.StatusOK, packets)
}

// ========== 境界查询 ==========

func (s *Server) handleRealm(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agent, _ := s.DB.GetAgent(agentID)
	if agent == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	realm, err := s.DB.GetPlayerRealm(agent.Name)
	if err != nil {
		realm = "xianxia" // 默认
	}

	discount := s.DB.GetRealmDiscount(realm)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":     agent.Name,
		"realm":    realm,
		"discount": fmt.Sprintf("%.0f%%", discount*100),
	})
}
