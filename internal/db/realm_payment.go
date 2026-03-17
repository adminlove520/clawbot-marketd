package db

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ========== 境界特权 ==========

type PlayerInfo struct {
	Name  string `json:"name"`
	Realm string `json:"realm"`
	Level int    `json:"level"`
	Exp   int    `json:"exp"`
}

// 境界手续费折扣
var realmDiscounts = map[string]float64{
	"xianxia": 0.00, // 练气期
	"zhuji":   0.05, // 筑基期
	"jindan":  0.10, // 金丹期
	"yuanying": 0.15, // 元婴期
	"huashen": 0.20, // 化神期
	"dujie":   0.25, // 渡劫期
	"feisheng": 0.30, // 飞升
	"cyber":   0.10, // 赛博派
}

func GetRealmDiscount(realm string) float64 {
	if discount, ok := realmDiscounts[realm]; ok {
		return discount
	}
	return 0.0 // 默认无折扣
}

func (db *DB) GetPlayerRealm(name string) (string, error) {
	// 调用龙虾文明API获取玩家境界
	url := fmt.Sprintf("https://lobsterhub-api.vercel.app/api/player?name=%s", name)
	resp, err := http.Get(url)
	if err != nil {
		return "xianxia", err // 默认境界
	}
	defer resp.Body.Close()

	var player PlayerInfo
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return "xianxia", err
	}

	return player.Realm, nil
}

func (db *DB) CalculateFee(agentID int64, amount float64) (float64, float64) {
	// 获取玩家境界
	name, err := db.GetAgentByID(agentID)
	if err != nil {
		return amount, 0 // 无折扣
	}

	realm, err := db.GetPlayerRealm(name)
	if err != nil {
		return amount, 0
	}

	discount := GetRealmDiscount(realm)
	fee := amount * (1 - discount)
	discountAmount := amount * discount

	return fee, discountAmount
}

// ========== x402 支付 ==========

type PaymentRequest struct {
	To       string  `json:"to"`
	Amount   float64 `json:"amount"`
	Token    string  `json:"token"` // USDC
}

func (db *DB) ProcessX402Payment(req PaymentRequest) (string, error) {
	// TODO: 实现x402支付
	// 需要配置私钥和RPC
	// 返回 tx_hash
	return "", fmt.Errorf("x402 payment not implemented yet")
}
