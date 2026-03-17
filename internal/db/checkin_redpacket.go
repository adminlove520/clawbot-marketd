package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ========== 签到操作 ==========

type Checkin struct {
	ID         int64
	AgentID    int64
	Streak    int
	LastCheckin string
}

func (db *DB) GetOrCreateCheckin(agentID int64) (*Checkin, error) {
	var c Checkin
	var lastCheckin sql.NullString
	err := db.QueryRow("SELECT id, agent_id, streak, last_checkin FROM checkins WHERE agent_id = ?", agentID).Scan(&c.ID, &c.AgentID, &c.Streak, &lastCheckin)
	if err == sql.ErrNoRows {
		// 创建签到记录
		result, err := db.Exec("INSERT INTO checkins (agent_id, streak, last_checkin) VALUES (?, 0, NULL)", agentID)
		if err != nil {
			return nil, err
		}
		id, _ := result.LastInsertId()
		return &Checkin{ID: id, AgentID: agentID, Streak: 0, LastCheckin: ""}, nil
	}
	if err != nil {
		return nil, err
	}
	c.LastCheckin = lastCheckin.String
	return &c, nil
}

func (db *DB) ProcessCheckin(agentID int64, reward float64) (int, error) {
	// 获取签到记录
	c, err := db.GetOrCreateCheckin(agentID)
	if err != nil {
		return 0, err
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	
	isContinuous := c.LastCheckin == yesterday
	isFirstTime := c.LastCheckin == ""
	
	newStreak := c.Streak
	if isContinuous {
		newStreak = c.Streak + 1
	} else if isFirstTime {
		newStreak = 1
	} else {
		newStreak = 1 // 断开，重新开始
	}

	// 更新签到
	_, err = db.Exec("UPDATE checkins SET streak = ?, last_checkin = ? WHERE agent_id = ?", newStreak, today, agentID)
	if err != nil {
		return 0, err
	}

	// 加积分
	db.AddBalance(agentID, reward, "daily checkin", nil)

	return newStreak, nil
}

// ========== 红包操作 ==========

type RedPacket struct {
	ID            int64
	SenderID      int64
	Amount       float64
	Count        int
	Remaining    float64
	Realm        string
	CreatedAt    string
}

type RedPacketClaim struct {
	ID         int64
	PacketID   int64
	ClaimerID  int64
	Amount     float64
	ClaimedAt  string
}

func (db *DB) CreateRedPacket(senderID int64, amount float64, count int, realm string) (int64, error) {
	result, err := db.Exec("INSERT INTO red_packets (sender_id, amount, count, remaining, realm) VALUES (?, ?, ?, ?, ?)",
		senderID, amount, count, amount, realm)
	if err != nil {
		return 0, err
	}
	
	// 扣积分
	db.AddBalance(senderID, -amount, "create red packet", nil)
	
	return result.LastInsertId()
}

func (db *DB) GetRedPacket(id int64) (*RedPacket, error) {
	var r RedPacket
	err := db.QueryRow("SELECT id, sender_id, amount, count, remaining, realm, created_at FROM red_packets WHERE id = ?", id).
		Scan(&r.ID, &r.SenderID, &r.Amount, &r.Count, &r.Remaining, &r.Realm, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *DB) ClaimRedPacket(packetID, claimerID int64) (float64, error) {
	// 获取红包
	r, err := db.GetRedPacket(packetID)
	if err != nil {
		return 0, err
	}
	
	// 不能抢自己的
	if r.SenderID == claimerID {
		return 0, fmt.Errorf("cannot claim your own red packet")
	}
	
	// 检查是否已抢过
	var existing int
	err = db.QueryRow("SELECT COUNT(*) FROM red_packet_claims WHERE packet_id = ? AND claimer_id = ?", packetID, claimerID).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if existing > 0 {
		return 0, fmt.Errorf("already claimed")
	}
	
	// 计算随机金额
	claimAmount := r.Remaining / float64(r.Count)
	if claimAmount < 0.01 {
		claimAmount = 0.01
	}
	
	// 更新剩余
	_, err = db.Exec("UPDATE red_packets SET remaining = remaining - ? WHERE id = ?", claimAmount, packetID)
	if err != nil {
		return 0, err
	}
	
	// 记录领取
	db.Exec("INSERT INTO red_packet_claims (packet_id, claimer_id, amount) VALUES (?, ?, ?)", packetID, claimerID, claimAmount)
	
	// 加积分
	db.AddBalance(claimerID, claimAmount, "red packet claimed", nil)
	
	return claimAmount, nil
}

func (db *DB) GetRedPacketClaims(packetID int64) ([]RedPacketClaim, error) {
	rows, err := db.Query("SELECT id, packet_id, claimer_id, amount, claimed_at FROM red_packet_claims WHERE packet_id = ? ORDER BY id", packetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var claims []RedPacketClaim
	for rows.Next() {
		var c RedPacketClaim
		rows.Scan(&c.ID, &c.PacketID, &c.ClaimerID, &c.Amount, &c.ClaimedAt)
		claims = append(claims, c)
	}
	return claims, nil
}
