package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ========== 充值 USDC ==========

// DepositConfirm 管理员确认充值
func (s *Server) handleDepositConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Admin only", http.StatusForbidden)
		return
	}

	var req struct {
		UserID int64   `json:"user_id"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.UserID == 0 || req.Amount <= 0 {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}

	// 给用户增加余额
	err := s.DB.AddBalance(req.UserID, req.Amount, "deposit", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(req.UserID, "", "deposit_confirm", "user", req.UserID, 
		fmt.Sprintf("amount:%.2f,note:%s", req.Amount, req.Note))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user_id": req.UserID,
		"amount":  req.Amount,
		"message": "充值确认成功",
	})
}

// GetDepositAddress 获取充值地址
func (s *Server) handleDepositAddress(w http.ResponseWriter, r *http.Request) {
	// 返回平台钱包地址
	addr := "0x63cd57e88c4a7cAEDE11E1220Fd9Fe65040D81c0" // 默认
	
	if s.X402 != nil && s.X402.IsInitialized() {
		addr = s.X402.GetFromAddress()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"address":   addr,
		"network":   "base",
		"token":     "USDC",
		"contract":  "0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8",
		"guide":     "转账 USDC 到此地址，联系管理员确认充值",
	})
}

// ========== 管理员操作 ==========

// AdminAddBalance 管理员给用户增加余额
func (s *Server) handleAdminAddBalance(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Admin only", http.StatusForbidden)
		return
	}

	var req struct {
		UserID int64   `json:"user_id"`
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.UserID == 0 || req.Amount == 0 {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}

	err := s.DB.AddBalance(req.UserID, req.Amount, req.Reason, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.DB.AddLog(req.UserID, "", "admin_add_balance", "user", req.UserID, 
		fmt.Sprintf("amount:%.2f,reason:%s", req.Amount, req.Reason))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user_id": req.UserID,
		"amount":  req.Amount,
	})
}

// GetAllBalances 查看所有用户余额
func (s *Server) handleAllBalances(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Admin only", http.StatusForbidden)
		return
	}

	rows, err := s.DB.Query("SELECT id, name, balance FROM agents ORDER BY balance DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id int64
		var name string
		var balance float64
		rows.Scan(&id, &name, &balance)
		users = append(users, map[string]interface{}{
			"id":      id,
			"name":    name,
			"balance": balance,
		})
	}

	writeJSON(w, http.StatusOK, users)
}
