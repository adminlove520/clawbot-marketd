package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ========== 直接转账红包 ==========

// DirectRedPacketRequest 创建直接转账红包请求
type DirectRedPacketRequest struct {
	ToAddress string  `json:"to_address"` // 接收者钱包地址
	ToName    string  `json:"to_name"`   // 接收者名称（可选）
	Amount    float64 `json:"amount"`    // 金额（USDC）
	Message   string  `json:"message"`   // 留言（可选）
}

// DirectConfirmRequest 确认转账请求
type DirectConfirmRequest struct {
	PacketID int64  `json:"packet_id"` // 红包ID
	TxHash   string `json:"tx_hash"`   // 交易哈希
}

func (s *Server) handleDirectRedPacket(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/direct/redpacket" && r.Method == "POST":
		s.createDirectRedPacket(w, r)
	case path == "/api/direct/redpacket" && r.Method == "GET":
		s.listDirectRedPackets(w, r)
	case path == "/api/direct/redpacket/pending" && r.Method == "GET":
		s.pendingDirectRedPackets(w, r)
	case path == "/api/direct/redpacket/confirm" && r.Method == "POST":
		s.confirmDirectRedPacket(w, r)
	case path == "/api/direct/redpacket/my" && r.Method == "GET":
		s.myDirectRedPackets(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// createDirectRedPacket 创建直接转账红包
func (s *Server) createDirectRedPacket(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req DirectRedPacketRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.ToAddress == "" {
		http.Error(w, "to_address required", http.StatusBadRequest)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}

	// 验证钱包地址
	if !s.X402.IsValidAddress(req.ToAddress) {
		http.Error(w, "invalid wallet address", http.StatusBadRequest)
		return
	}

	// 获取发送者信息
	agent, _ := s.DB.GetAgent(agentID)
	senderName := fmt.Sprintf("agent_%d", agentID)
	if agent != nil {
		senderName = agent.Name
	}

	// 创建红包记录
	packetID, err := s.DB.CreateDirectRedPacket(agentID, senderName, req.ToAddress, req.ToName, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "create_direct_redpacket", "direct_red_packet", packetID, 
		fmt.Sprintf("to:%s,amount:%.2f", req.ToAddress, req.Amount))

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         packetID,
		"sender":     senderName,
		"to_address": req.ToAddress,
		"to_name":    req.ToName,
		"amount":     req.Amount,
		"status":     "pending",
		"message":    "红包已创建，请转账到对方钱包地址",
	})
}

// listDirectRedPackets 获取红包列表（可抢的）
func (s *Server) listDirectRedPackets(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	packets, err := s.DB.GetPendingDirectRedPackets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, p := range packets {
		result = append(result, map[string]interface{}{
			"id":          p.ID,
			"sender":      p.SenderName,
			"to_address":  p.ToAddress,
			"to_name":     p.ToName,
			"amount":      p.Amount,
			"status":      p.Status,
			"created_at":  p.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// pendingDirectRedPackets 获取待确认的红包（仅管理员）
func (s *Server) pendingDirectRedPackets(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Admin only", http.StatusForbidden)
		return
	}

	packets, err := s.DB.GetPendingDirectRedPackets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, p := range packets {
		result = append(result, map[string]interface{}{
			"id":          p.ID,
			"sender_id":   p.SenderID,
			"sender":      p.SenderName,
			"to_address":  p.ToAddress,
			"to_name":     p.ToName,
			"amount":      p.Amount,
			"status":      p.Status,
			"created_at":  p.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// confirmDirectRedPacket 确认转账完成
func (s *Server) confirmDirectRedPacket(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req DirectConfirmRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	// 获取红包信息
	packet, err := s.DB.GetDirectRedPacket(req.PacketID)
	if err != nil {
		http.Error(w, "Packet not found", http.StatusNotFound)
		return
	}

	// 验证是否是发送者本人确认
	if packet.SenderID != agentID {
		http.Error(w, "Only sender can confirm", http.StatusForbidden)
		return
	}

	// 更新状态为已转账
	err = s.DB.UpdateDirectRedPacketStatus(req.PacketID, "sent", req.TxHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(agentID, "", "confirm_direct_redpacket", "direct_red_packet", req.PacketID, 
		fmt.Sprintf("tx_hash:%s", req.TxHash))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       packet.ID,
		"status":   "sent",
		"tx_hash":  req.TxHash,
		"message":  "转账已确认，等待接收者确认收款",
	})
}

// myDirectRedPackets 获取我的红包记录
func (s *Server) myDirectRedPackets(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	packets, err := s.DB.GetUserDirectRedPackets(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, p := range packets {
		result = append(result, map[string]interface{}{
			"id":           p.ID,
			"sender":       p.SenderName,
			"to_address":  p.ToAddress,
			"to_name":     p.ToName,
			"amount":       p.Amount,
			"status":       p.Status,
			"tx_hash":      p.TxHash,
			"created_at":   p.CreatedAt,
			"sent_at":      p.SentAt,
			"confirmed_at": p.ConfirmedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}
