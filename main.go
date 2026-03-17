package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ythx-101/lobsterhub/internal/db"
	"github.com/ythx-101/lobsterhub/internal/server"
)

// 生成随机 admin key
func generateAdminKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "lobster-admin-" + hex.EncodeToString(bytes)[:16]
}

// 从环境变量或文件加载 admin keys
func loadAdminKeys() []string {
	// 优先从环境变量读取
	keysStr := os.Getenv("ADMIN_KEYS")
	if keysStr != "" {
		keys := strings.Split(keysStr, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
		return keys
	}

	// 尝试读取 .env 文件
	envFile := ".env"
	if data, err := os.ReadFile(envFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ADMIN_KEYS=") {
				keysStr = strings.TrimSpace(strings.Split(line, "=")[1])
				if keysStr != "" {
					keys := strings.Split(keysStr, ",")
					for i := range keys {
						keys[i] = strings.TrimSpace(keys[i])
					}
					return keys
				}
			}
		}
	}

	// 自动生成
	key := generateAdminKey()
	fmt.Printf("🦞 Auto-generated admin key: %s\n", key)
	fmt.Printf("   Save this key! Set ADMIN_KEYS env or create .env file to persist.\n")
	return []string{key}
}

func main() {
	addr := flag.String("addr", ":8080", "server address")
	dbPath := flag.String("db", "lobsterhub.db", "database path")
	ethKey := flag.String("eth-key", "", "Ethereum private key for x402 payments")
	rpcURL := flag.String("rpc", "https://base.llamarpc.com", "Ethereum RPC URL")
	flag.Parse()

	// 处理 admin keys
	adminKeys := loadAdminKeys()

	// 初始化 x402 支付
	if *ethKey != "" {
		err := InitX402(*ethKey, *rpcURL, "0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8")
		if err != nil {
			log.Printf("Warning: x402 init failed: %v", err)
		} else {
			fmt.Printf("   x402: enabled, from %s\n", GetX402FromAddress())
		}
	} else if envKey := os.Getenv("ETH_PRIVATE_KEY"); envKey != "" {
		err := InitX402(envKey, *rpcURL, "0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8")
		if err != nil {
			log.Printf("Warning: x402 init failed: %v", err)
		} else {
			fmt.Printf("   x402: enabled, from %s\n", GetX402FromAddress())
		}
	} else {
		fmt.Printf("   x402: disabled (no ETH_PRIVATE_KEY)\n")
	}

	// 初始化数据库
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// 初始化表
	if err := database.Init(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 创建服务器
	srv := server.New(database, adminKeys)

	// 启动服务器
	fmt.Printf("🦞 LobsterHub starting on %s\n", *addr)
	fmt.Printf("   Admin keys: %d key(s)\n", len(adminKeys))
	fmt.Printf("   Database: %s\n", *dbPath)

	if err := srv.Start(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
