package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ========== 充值记录 ==========

type Deposit struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Amount        float64   `json:"amount"`       // USDC 金额
	TxHash        string    `json:"tx_hash"`      // 区块链交易哈希
	Status        string    `json:"status"`        // pending/confirmed/failed
	Credits       float64   `json:"credits"`      // 获得的积分
	CreatedAt     time.Time `json:"created_at"`
	ConfirmedAt   *time.Time `json:"confirmed_at"`
}

// CreateDeposit 创建充值记录
func (db *DB) CreateDeposit(userID int64, amount float64, txHash string) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO deposits (user_id, amount, tx_hash, status, credits) VALUES (?, ?, ?, 'pending', 0)",
		userID, amount, txHash,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetDepositByTxHash 通过tx_hash查询充值记录
func (db *DB) GetDepositByTxHash(txHash string) (*Deposit, error) {
	var d Deposit
	var confirmedAt sql.NullTime
	err := db.QueryRow(
		"SELECT id, user_id, amount, tx_hash, status, credits, created_at, confirmed_at FROM deposits WHERE tx_hash = ?",
		txHash,
	).Scan(&d.ID, &d.UserID, &d.Amount, &d.TxHash, &d.Status, &d.Credits, &d.CreatedAt, &confirmedAt)
	if err != nil {
		return nil, err
	}
	if confirmedAt.Valid {
		d.ConfirmedAt = &confirmedAt.Time
	}
	return &d, nil
}

// ConfirmDeposit 确认充值
func (db *DB) ConfirmDeposit(txHash string) error {
	// 获取充值记录
	deposit, err := db.GetDepositByTxHash(txHash)
	if err != nil {
		return err
	}

	if deposit.Status == "confirmed" {
		return fmt.Errorf("deposit already confirmed")
	}

	// 更新状态
	_, err = db.Exec(
		"UPDATE deposits SET status = 'confirmed', credits = ?, confirmed_at = ? WHERE tx_hash = ?",
		deposit.Amount, time.Now(), txHash,
	)
	if err != nil {
		return err
	}

	// 给用户加积分（1 USDC = 1 积分）
	db.AddBalance(deposit.UserID, deposit.Amount, "deposit", nil)

	return nil
}

// GetUserDeposits 获取用户充值记录
func (db *DB) GetUserDeposits(userID int64) ([]Deposit, error) {
	rows, err := db.Query(
		"SELECT id, user_id, amount, tx_hash, status, credits, created_at, confirmed_at FROM deposits WHERE user_id = ? ORDER BY id DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deposits []Deposit
	for rows.Next() {
		var d Deposit
		var confirmedAt sql.NullTime
		rows.Scan(&d.ID, &d.UserID, &d.Amount, &d.TxHash, &d.Status, &d.Credits, &d.CreatedAt, &confirmedAt)
		if confirmedAt.Valid {
			d.ConfirmedAt = &confirmedAt.Time
		}
		deposits = append(deposits, d)
	}
	return deposits, nil
}

// GetPendingDeposits 获取待确认的充值
func (db *DB) GetPendingDeposits() ([]Deposit, error) {
	rows, err := db.Query(
		"SELECT id, user_id, amount, tx_hash, status, credits, created_at, confirmed_at FROM deposits WHERE status = 'pending' ORDER BY id DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deposits []Deposit
	for rows.Next() {
		var d Deposit
		var confirmedAt sql.NullTime
		rows.Scan(&d.ID, &d.UserID, &d.Amount, &d.TxHash, &d.Status, &d.Credits, &d.CreatedAt, &confirmedAt)
		if confirmedAt.Valid {
			d.ConfirmedAt = &confirmedAt.Time
		}
		deposits = append(deposits, d)
	}
	return deposits, nil
}
