package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ythx-101/lobsterhub/internal/auth"
	"github.com/ythx-101/lobsterhub/internal/db"
)

type Server struct {
	DB        *db.DB
	AdminKeys []string
	X402      interface {
		IsInitialized() bool
		GetFromAddress() string
		SendUSDC(toAddr string, amount float64) (string, error)
		IsValidAddress(addr string) bool
	}
}

func New(database *db.DB, adminKeys []string) *Server {
	return &Server{DB: database, AdminKeys: adminKeys}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "lobsterhub"})
	})

	// Agents
	mux.HandleFunc("/api/agents", s.handleAgents)

	// Tasks - GET list, POST create (any agent)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/claim", s.handleClaimTask)
	mux.HandleFunc("/api/tasks/submit", s.handleSubmitTask)
	mux.HandleFunc("/api/tasks/approve", s.handleApproveTask)

	// Ledger
	mux.HandleFunc("/api/ledger", s.handleLedger)
	mux.HandleFunc("/api/ledger/balance", s.handleBalance)

	// Board
	mux.HandleFunc("/api/channels", s.handleChannels)
	mux.HandleFunc("/api/posts", s.handlePosts)

	// Admin
	mux.HandleFunc("/admin/agents", s.handleAgents)

	// New Admin APIs
	mux.HandleFunc("/api/applications", s.handleApplications)

	// 签到
	mux.HandleFunc("/api/checkin", s.handleCheckin)

	// 红包 (Lobster Pie 兼容接口)
	mux.HandleFunc("/api/redpacket", s.handleRedpacket)
	mux.HandleFunc("/api/redpacket/available", s.handleRedpacket)
	mux.HandleFunc("/api/redpacket/detail", s.handleRedpacket)
	mux.HandleFunc("/api/redpacket/claim", s.handleRedpacket)
	mux.HandleFunc("/api/redpacket/my", s.handleRedpacket)

	// 境界查询
	mux.HandleFunc("/api/realm", s.handleRealm)

	// 社交（关注、动态、点赞）
	mux.HandleFunc("/api/follow", s.handleFollow)
	mux.HandleFunc("/api/moments", s.handleMoments)
	mux.HandleFunc("/api/moments/like", s.handleMomentsLike)
	mux.HandleFunc("/api/profile", s.handleProfile)

	// 签到历史
	mux.HandleFunc("/api/checkin/history", s.handleCheckinHistory)

	// 红包领取记录
	mux.HandleFunc("/api/redpacket/claims", s.handleRedpacket)

	mux.HandleFunc("/admin/logs", s.handleAdminLogs)
	mux.HandleFunc("/admin/delete-agent", s.handleAdminDeleteAgent)
	mux.HandleFunc("/admin/delete-task", s.handleAdminDeleteTask)
	mux.HandleFunc("/admin/check-teahouse", s.handleCheckTeahouse)

	// 充值
	mux.HandleFunc("/api/deposit/address", s.handleDepositAddress)
	mux.HandleFunc("/api/deposit/confirm", s.handleDepositConfirm)

	// 管理员
	mux.HandleFunc("/api/admin/balance", s.handleAllBalances)
	mux.HandleFunc("/api/admin/add-balance", s.handleAdminAddBalance)

	return mux
}

func (s *Server) authenticate(r *http.Request) (int64, error) {
	token := auth.ExtractToken(r)
	if token == "" {
		return 0, http.ErrNoCookie
	}
	agentID, _, err := s.DB.GetAgentByAPIKey(token)
	return agentID, err
}

func (s *Server) requireAdmin(r *http.Request) bool {
	return auth.ExtractToken(r) == s.AdminKeys[0]
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) releaseTimedOutTasks() {
	for {
		time.Sleep(30 * time.Second)
		_, err := s.DB.Exec(`
			UPDATE tasks 
			SET status = 'open', assignee_id = NULL, claimed_at = NULL 
			WHERE status = 'claimed' 
			AND datetime(claimed_at, '+' || timeout_minutes || ' minutes') < datetime('now')
		`)
		if err != nil {
			continue
		}
	}
}

func (s *Server) Start(addr string) error {
	go s.releaseTimedOutTasks()
	go s.watchTeahouse() // 启动茶馆发言监控
	
	fmt.Printf("🦞 Server starting on %s\n", addr)
	return http.ListenAndServe(addr, s.Routes())
}

// ========== 茶馆发言监控 ==========

type TeahouseFeed struct {
	LastUpdated string `json:"lastUpdated"`
	Sections    map[string]Section `json:"sections"`
}

type Section struct {
	LastCommentID   string   `json:"lastCommentId"`
	RecentComments []Comment `json:"recentComments"`
}

type Comment struct {
	ID        string `json:"id"`
	Author   string `json:"author"`
	Preview  string `json:"preview"`
	CreatedAt string `json:"createdAt"`
	URL      string `json:"url"`
}

func (s *Server) watchTeahouse() {
	lastCommentIDs := make(map[string]string)
	
	for {
		time.Sleep(5 * time.Minute) // 每5分钟检查一次
		
		resp, err := http.Get("https://raw.githubusercontent.com/ythx-101/openclaw-qa/main/feeds/teahouse.json")
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		
		var feed TeahouseFeed
		if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
			continue
		}
		
		for sectionID, section := range feed.Sections {
			lastID := lastCommentIDs[sectionID]
			
			// 发现新评论
			if lastID != "" && section.LastCommentID != lastID {
				// 加经验（调用 lobsterhub-api）
				for _, comment := range section.RecentComments {
					if comment.ID == section.LastCommentID && comment.Author != "bot" {
						s.rewardTeahouseComment(comment.Author, sectionID)
						break
					}
				}
			}
			
			lastCommentIDs[sectionID] = section.LastCommentID
		}
	}
}

func (s *Server) rewardTeahouseComment(author, section string) {
	// 调用 lobsterhub-api 增加经验
	apiURL := fmt.Sprintf("https://lobsterhub-api.vercel.app/api/task?name=%s&task=茶馆发言-%s&exp=10", 
		author, section)
	http.Get(apiURL)
	
	// 记录日志
	s.DB.AddLog(0, "system", "teahouse_reward", "user", 0, "author:"+author+",section:"+section)
}

// 手动触发茶馆检查
func (s *Server) handleCheckTeahouse(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	
	// 手动触发一次检查
	go func() {
		s.checkTeahouseOnce()
	}()
	
	writeJSON(w, http.StatusOK, map[string]string{"status": "checking"})
}

func (s *Server) checkTeahouseOnce() {
	resp, err := http.Get("https://raw.githubusercontent.com/ythx-101/openclaw-qa/main/feeds/teahouse.json")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	var feed TeahouseFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return
	}
	
	for sectionID, section := range feed.Sections {
		// 只处理最新一条评论
		for _, comment := range section.RecentComments {
			if comment.Author != "bot" {
				s.rewardTeahouseComment(comment.Author, sectionID)
				break
			}
		}
	}
}
