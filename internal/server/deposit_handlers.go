package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ========== 充值 ==========

// DepositRequest 充值请求
type DepositRequest struct {
	TxHash string  `json:"tx_hash"` // 区块链交易哈希
	Amount float64 `json:"amount"`  // 充值金额（可选，用于验证）
}

// DepositConfirmRequest 确认充值请求
type DepositConfirmRequest struct {
	TxHash string `json:"tx_hash"` // 区块链交易哈希
}

func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/deposit" && r.Method == "POST":
		s.createDeposit(w, r)
	case path == "/api/deposit" && r.Method == "GET":
		s.getDeposit(w, r)
	case path == "/api/deposit/confirm" && r.Method == "POST":
		s.confirmDeposit(w, r)
	case path == "/api/deposit/my" && r.Method == "GET":
		s.myDeposits(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// createDeposit 创建充值记录
func (s *Server) createDeposit(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req DepositRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.TxHash == "" {
		http.Error(w, "tx_hash required", http.StatusBadRequest)
		return
	}

	// 检查是否已存在
	existing, _ := s.DB.GetDepositByTxHash(req.TxHash)
	if existing != nil {
		http.Error(w, "tx_hash already exists", http.StatusBadRequest)
		return
	}

	// 如果提供了金额，记录金额（用于后续验证）
	amount := req.Amount
	if amount <= 0 {
		amount = 0 // 待确认
	}

	// 创建充值记录
	depositID, err := s.DB.CreateDeposit(agentID, amount, req.TxHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       depositID,
		"tx_hash":  req.TxHash,
		"status":   "pending",
		"message":  "充值记录已创建，请等待区块链确认",
	})
}

// getDeposit 查询充值记录
func (s *Server) getDeposit(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	txHash := r.URL.Query().Get("tx_hash")
	if txHash == "" {
		http.Error(w, "tx_hash required", http.StatusBadRequest)
		return
	}

	deposit, err := s.DB.GetDepositByTxHash(txHash)
	if err != nil {
		http.Error(w, "Deposit not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":           deposit.ID,
		"user_id":      deposit.UserID,
		"amount":       deposit.Amount,
		"tx_hash":      deposit.TxHash,
		"status":       deposit.Status,
		"credits":      deposit.Credits,
		"created_at":   deposit.CreatedAt,
		"confirmed_at":  deposit.ConfirmedAt,
	})
}

// confirmDeposit 确认充值（管理员调用）
func (s *Server) confirmDeposit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Admin only", http.StatusForbidden)
		return
	}

	var req DepositConfirmRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	if req.TxHash == "" {
		http.Error(w, "tx_hash required", http.StatusBadRequest)
		return
	}

	// 查询充值记录
	deposit, err := s.DB.GetDepositByTxHash(req.TxHash)
	if err != nil {
		http.Error(w, "Deposit not found", http.StatusNotFound)
		return
	}

	if deposit.Status == "confirmed" {
		http.Error(w, "Already confirmed", http.StatusBadRequest)
		return
	}

	// 确认充值
	err = s.DB.ConfirmDeposit(req.TxHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 记录日志
	s.DB.AddLog(deposit.UserID, "", "deposit_confirm", "deposit", deposit.ID, 
		fmt.Sprintf("amount:%.2f,credits:%.2f", deposit.Amount, deposit.Amount))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       deposit.ID,
		"status":   "confirmed",
		"credits": deposit.Amount,
		"message":  fmt.Sprintf("充值确认成功！%.2f 积分已到账", deposit.Amount),
	})
}

// myDeposits 我的充值记录
func (s *Server) myDeposits(w http.ResponseWriter, r *http.Request) {
	agentID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deposits, err := s.DB.GetUserDeposits(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result []map[string]interface{}
	for _, d := range deposits {
		result = append(result, map[string]interface{}{
			"id":           d.ID,
			"amount":       d.Amount,
			"tx_hash":      d.TxHash,
			"status":       d.Status,
			"credits":      d.Credits,
			"created_at":   d.CreatedAt,
			"confirmed_at": d.ConfirmedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// ========== 平台钱包地址 ==========

func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 返回平台钱包地址
	addr := "0x63cd57e88c4a7cAEDE11E1220Fd9Fe65040D81c0" // 默认地址
	
	// 如果x402已初始化，使用x402的地址
	if s.X402 != nil && s.X402.IsInitialized() {
		addr = s.X402.GetFromAddress()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"address":   addr,
		"network":   "base",
		"token":     "USDC",
		"contract":  "0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8",
	})
}
