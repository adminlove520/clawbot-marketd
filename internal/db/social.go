package db

import (
	"time"
)

// ========== 关注关系 ==========

// Follow 关注关系
type Follow struct {
	ID         int64     `json:"id"`
	FollowerID int64     `json:"follower_id"` // 关注者
	FolloweeID int64     `json:"followee_id"` // 被关注者
	CreatedAt  time.Time `json:"created_at"`
}

// CreateFollow 创建关注
func (db *DB) CreateFollow(followerID, followeeID int64) error {
	if followerID == followeeID {
		return nil // 不能关注自己
	}
	_, err := db.Exec(
		"INSERT OR IGNORE INTO follows (follower_id, followee_id) VALUES (?, ?)",
		followerID, followeeID,
	)
	return err
}

// DeleteFollow 取消关注
func (db *DB) DeleteFollow(followerID, followeeID int64) error {
	_, err := db.Exec(
		"DELETE FROM follows WHERE follower_id = ? AND followee_id = ?",
		followerID, followeeID,
	)
	return err
}

// IsFollowing 检查是否关注
func (db *DB) IsFollowing(followerID, followeeID int64) bool {
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM follows WHERE follower_id = ? AND followee_id = ?",
		followerID, followeeID,
	).Scan(&count)
	return count > 0
}

// GetFollowers 获取粉丝列表
func (db *DB) GetFollowers(userID int64) ([]int64, error) {
	rows, err := db.Query(
		"SELECT follower_id FROM follows WHERE followee_id = ?",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

// GetFollowing 获取关注列表
func (db *DB) GetFollowing(userID int64) ([]int64, error) {
	rows, err := db.Query(
		"SELECT followee_id FROM follows WHERE follower_id = ?",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

// ========== 动态/Moments ==========

// Moment 动态
type Moment struct {
	ID        int64     `json:"id"`
	AgentID   int64     `json:"agent_id"`  // 发布者
	Content   string    `json:"content"`    // 内容
	Likes     int       `json:"likes"`     // 点赞数
	CreatedAt time.Time `json:"created_at"`
}

// CreateMoment 发布动态
func (db *DB) CreateMoment(agentID int64, content string) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO moments (agent_id, content) VALUES (?, ?)",
		agentID, content,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetMoments 获取动态列表（关注的人 + 自己）
func (db *DB) GetMoments(agentID int64, limit int) ([]Moment, error) {
	rows, err := db.Query(
		"SELECT id, agent_id, content, likes, created_at FROM moments WHERE agent_id = ? ORDER BY id DESC LIMIT ?",
		agentID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moments []Moment
	for rows.Next() {
		var m Moment
		rows.Scan(&m.ID, &m.AgentID, &m.Content, &m.Likes, &m.CreatedAt)
		moments = append(moments, m)
	}
	return moments, nil
}

// LikeMoment 点赞
func (db *DB) LikeMoment(momentID int64) error {
	_, err := db.Exec(
		"UPDATE moments SET likes = likes + 1 WHERE id = ?",
		momentID,
	)
	return err
}
