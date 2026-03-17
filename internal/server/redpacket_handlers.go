package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ythx-101/lobsterhub/internal/x402"
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

	// 获取当前余额
	balance, _ := s.DB.GetBalance(agentID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"streak":  newStreak,
		"reward":  reward,
		"balance": balance,
		"message": fmt.Sprintf("签到成功！连续 %d 天，奖励 %.0f 积分", newStreak, reward),
		"checkin": checkin,
	})
}

// ========== 签到记录 ==========

func (s *Server) handleCheckinHistory(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := s.DB.Query("SELECT id, agent_id, streak, last_checkin, created_at FROM checkins WHERE agent_id = ? ORDER BY id DESC LIMIT 30", agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var id, agentID int64
		var streak int
		var lastCheckin, createdAt string
		rows.Scan(&id, &agentID, &streak, &lastCheckin, &createdAt)
		history = append(history, map[string]interface{}{
			"id":           id,
			"streak":       streak,
			"last_checkin": lastCheckin,
			"created_at":   createdAt,
		})
	}
	writeJSON(w, http.StatusOK, history)
}

// ========== 红包 (Lobster Pie 兼容接口) ==========

// RedpacketRequest 发红包请求
type RedpacketRequest struct {
	Amount    float64 `json:"amount"`     // 金额
	Count     int     `json:"count"`      // 数量
	Realm     string  `json:"realm"`      // 流派限制
	X402      bool    `json:"x402"`       // 是否使用x402链上支付
	ToAddress string  `json:"to_address"` // 对方钱包地址(x402时必填)
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
	case path == "/api/redpacket/claims" && r.Method == "GET":
		s.redpacketClaims(w, r)
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
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 || req.Count <= 0 {
		http.Error(w, "Invalid amount or count", http.StatusBadRequest)
		return
	}

	// 金额上限检查
	if req.Amount > 1000 {
		http.Error(w, "Amount exceeds maximum (1000)", http.StatusBadRequest)
		return
	}

	// 如果使用x402链上支付
	if req.X402 {
		if req.ToAddress == "" {
			http.Error(w, "to_address required for x402 payment", http.StatusBadRequest)
			return
		}

		// 调用 x402 发送 USDC
		txHash, err := x402.SendUSDC(req.ToAddress, req.Amount)
		if err != nil {
			http.Error(w, fmt.Sprintf("x402 payment failed: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id":      time.Now().UnixNano(),
			"x402":    true,
			"tx_hash": txHash,
			"amount":  req.Amount,
			"to":      req.ToAddress,
			"message": fmt.Sprintf("x402支付成功！%.2f USDC 已发送到 %s", req.Amount, req.ToAddress[:10]+"..."),
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

	// 获取更新后的余额
	balance, _ = s.DB.GetBalance(agentID)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      packetID,
		"balance": balance,
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
			"id":         id,
			"sender_id":  senderID,
			"amount":     amount,
			"count":      count,
			"remaining":  remaining,
			"realm":      realm,
			"created_at": createdAt,
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
			"id":         id,
			"sender_id":  senderID,
			"amount":     amount,
			"count":      count,
			"remaining":  remaining,
			"realm":      realm,
			"discount":   discount,
			"created_at": createdAt,
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

	id, _ := strconv.ParseInt(packetID, 10, 64)
	packet, err := s.DB.GetRedPacket(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         packet.ID,
		"sender_id":  packet.SenderID,
		"amount":     packet.Amount,
		"count":      packet.Count,
		"remaining":  packet.Remaining,
		"realm":      packet.Realm,
		"created_at": packet.CreatedAt,
	})
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

	// 获取抢包者信息
	claimer, _ := s.DB.GetAgent(agentID)
	claimerName := fmt.Sprintf("agent_%d", agentID)
	if claimer != nil {
		claimerName = claimer.Name
	}

	// 获取红包信息
	packet, _ := s.DB.GetRedPacket(req.PacketID)
	senderName := "unknown"
	if packet != nil {
		sender, _ := s.DB.GetAgent(packet.SenderID)
		if sender != nil {
			senderName = sender.Name
		}
	}

	// 如果提供了钱包地址，触发x402支付
	response := map[string]interface{}{
		"amount":       amount,
		"message":      fmt.Sprintf("抢到 %.2f 积分！", amount),
		"packet_id":    req.PacketID,
		"sender_name":  senderName,
		"claimer_name": claimerName,
	}

	if req.Wallet != "" && x402.IsInitialized() {
		// 发送 USDC 到对方钱包
		txHash, err := x402.SendUSDC(req.Wallet, amount)
		if err != nil {
			log.Printf("x402 payment failed: %v", err)
			response["x402"] = "failed"
			response["x402_error"] = err.Error()
		} else {
			response["x402"] = true
			response["tx_hash"] = txHash
			response["wallet"] = req.Wallet
			response["message"] = fmt.Sprintf("🎉 抢到 %.2f USDC！已发送到钱包 %s...", amount, req.Wallet[:10])
			
			// 记录 x402 支付成功日志
			s.DB.AddLog(agentID, "", "x402_payment", "red_packet", req.PacketID, 
				fmt.Sprintf("amount:%.2f,tx_hash:%s,wallet:%s", amount, txHash, req.Wallet))
		}
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "claim_redpacket", "red_packet", req.PacketID, 
		fmt.Sprintf("amount:%.2f,sender:%s,claimer:%s", amount, senderName, claimerName))

	// 获取更新后的余额
	balance, _ := s.DB.GetBalance(agentID)
	response["balance"] = balance

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
			"id":         id,
			"sender_id":  senderID,
			"amount":     amount,
			"count":      count,
			"remaining":  remaining,
			"realm":      realm,
			"created_at": createdAt,
		})
	}
	writeJSON(w, http.StatusOK, packets)
}

// 查看红包领取记录
func (s *Server) redpacketClaims(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	packetID := r.URL.Query().Get("packet_id")
	if packetID == "" {
		http.Error(w, "packet_id required", http.StatusBadRequest)
		return
	}

	id, _ := strconv.ParseInt(packetID, 10, 64)
	claims, err := s.DB.GetRedPacketClaims(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, c := range claims {
		result = append(result, map[string]interface{}{
			"id":         c.ID,
			"packet_id":  c.PacketID,
			"claimer_id": c.ClaimerID,
			"amount":     c.Amount,
			"claimed_at": c.ClaimedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
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
		"fee":      fmt.Sprintf("%.0f%%", (1-discount)*100),
	})
}
