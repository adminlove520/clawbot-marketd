package db

import (
	"database/sql"
	"time"
)

// ========== 红包（直接转账模式）==========

// DirectRedPacket 直接转账红包
type DirectRedPacket struct {
	ID          int64     `json:"id"`
	SenderID    int64     `json:"sender_id"`    // 发红包者ID
	SenderName  string    `json:"sender_name"`  // 发红包者名称
	ToAddress   string    `json:"to_address"`   // 接收者钱包地址
	ToName      string    `json:"to_name"`     // 接收者名称
	Amount      float64   `json:"amount"`      // 金额（USDC）
	Status      string    `json:"status"`      // pending(待转账)/sent(已转账)/confirmed(已确认)
	TxHash      string    `json:"tx_hash"`     // 交易哈希
	CreatedAt   time.Time `json:"created_at"`
	SentAt      *time.Time `json:"sent_at"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
}

// CreateDirectRedPacket 创建直接转账红包
func (db *DB) CreateDirectRedPacket(senderID int64, senderName, toAddress, toName string, amount float64) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO direct_red_packets (sender_id, sender_name, to_address, to_name, amount, status) VALUES (?, ?, ?, ?, ?, 'pending')",
		senderID, senderName, toAddress, toName, amount,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetDirectRedPacket 获取红包详情
func (db *DB) GetDirectRedPacket(id int64) (*DirectRedPacket, error) {
	var rp DirectRedPacket
	var sentAt, confirmedAt sql.NullTime
	err := db.QueryRow(
		"SELECT id, sender_id, sender_name, to_address, to_name, amount, status, tx_hash, created_at, sent_at, confirmed_at FROM direct_red_packets WHERE id = ?",
		id,
	).Scan(&rp.ID, &rp.SenderID, &rp.SenderName, &rp.ToAddress, &rp.ToName, &rp.Amount, &rp.Status, &rp.TxHash, &rp.CreatedAt, &sentAt, &confirmedAt)
	if err != nil {
		return nil, err
	}
	if sentAt.Valid {
		rp.SentAt = &sentAt.Time
	}
	if confirmedAt.Valid {
		rp.ConfirmedAt = &confirmedAt.Time
	}
	return &rp, nil
}

// GetPendingDirectRedPackets 获取待转账的红包
func (db *DB) GetPendingDirectRedPackets() ([]DirectRedPacket, error) {
	rows, err := db.Query(
		"SELECT id, sender_id, sender_name, to_address, to_name, amount, status, tx_hash, created_at, sent_at, confirmed_at FROM direct_red_packets WHERE status = 'pending' ORDER BY id DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packets []DirectRedPacket
	for rows.Next() {
		var rp DirectRedPacket
		var sentAt, confirmedAt sql.NullTime
		rows.Scan(&rp.ID, &rp.SenderID, &rp.SenderName, &rp.ToAddress, &rp.ToName, &rp.Amount, &rp.Status, &rp.TxHash, &rp.CreatedAt, &sentAt, &confirmedAt)
		if sentAt.Valid {
			rp.SentAt = &sentAt.Time
		}
		if confirmedAt.Valid {
			rp.ConfirmedAt = &confirmedAt.Time
		}
		packets = append(packets, rp)
	}
	return packets, nil
}

// UpdateDirectRedPacketStatus 更新红包状态
func (db *DB) UpdateDirectRedPacketStatus(id int64, status, txHash string) error {
	var err error
	if status == "sent" {
		_, err = db.Exec("UPDATE direct_red_packets SET status = ?, tx_hash = ?, sent_at = ? WHERE id = ?", status, txHash, time.Now(), id)
	} else if status == "confirmed" {
		_, err = db.Exec("UPDATE direct_red_packets SET status = ?, confirmed_at = ? WHERE id = ?", status, time.Now(), id)
	} else {
		_, err = db.Exec("UPDATE direct_red_packets SET status = ? WHERE id = ?", status, id)
	}
	return err
}

// GetUserDirectRedPackets 获取用户的红包记录
func (db *DB) GetUserDirectRedPackets(userID int64) ([]DirectRedPacket, error) {
	rows, err := db.Query(
		"SELECT id, sender_id, sender_name, to_address, to_name, amount, status, tx_hash, created_at, sent_at, confirmed_at FROM direct_red_packets WHERE sender_id = ? OR to_address LIKE ? ORDER BY id DESC",
		userID, "%%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packets []DirectRedPacket
	for rows.Next() {
		var rp DirectRedPacket
		var sentAt, confirmedAt sql.NullTime
		rows.Scan(&rp.ID, &rp.SenderID, &rp.SenderName, &rp.ToAddress, &rp.ToName, &rp.Amount, &rp.Status, &rp.TxHash, &rp.CreatedAt, &sentAt, &confirmedAt)
		if sentAt.Valid {
			rp.SentAt = &sentAt.Time
		}
		if confirmedAt.Valid {
			rp.ConfirmedAt = &confirmedAt.Time
		}
		packets = append(packets, rp)
	}
	return packets, nil
}
