package x402

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/pbkdf2"
)

var client *ethclient.Client
var privateKey *ecdsa.PrivateKey
var fromAddr common.Address

// Base mainnet USDC contract address
const USDCAddress = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"

// ERC20 transfer function selector: transfer(address,uint256)
var transferSelector = common.Hex2Bytes("a9059cbb")

// EncryptedWallet 加密钱包文件结构
type EncryptedWallet struct {
	Address    string `json:"address"`
	Ciphertext string `json:"ciphertext"`
	Salt       string `json:"salt"`
	IV         string `json:"iv"`
}

// Init 初始化 x402 支付
func Init(privateKeyHex, rpcURL, usdcAddr string) error {
	// 优先从环境变量加载明文 key（开发用）
	if privateKeyHex == "" {
		privateKeyHex = loadKeyFromEncryptedFile()
	}

	// 兼容旧版明文文件（迁移用，启动后应删除）
	if privateKeyHex == "" {
		privateKeyHex = loadKeyFromPlainFile()
		if privateKeyHex != "" {
			log.Printf("⚠️  WARNING: Using plaintext .eth.key — migrate to encrypted wallet ASAP")
		}
	}

	// 如果还没有，生成新的（加密存储）
	if privateKeyHex == "" {
		password := os.Getenv("WALLET_PASSWORD")
		if password == "" {
			password = "changeme" // 默认密码，生产环境必须修改
			log.Printf("⚠️  WARNING: Using default wallet password — set WALLET_PASSWORD env var")
		}
		newKey, err := GenerateEncryptedWallet(password)
		if err != nil {
			return fmt.Errorf("failed to generate wallet: %v", err)
		}
		privateKeyHex = newKey
	}

	var err error
	client, err = ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect RPC: %v", err)
	}

	privateKey, err = crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return fmt.Errorf("invalid private key: %v", err)
	}

	fromAddr = crypto.PubkeyToAddress(privateKey.PublicKey)
	log.Printf("x402 initialized, from: %s", fromAddr.Hex())
	return nil
}

// loadKeyFromEncryptedFile 从加密钱包文件加载私钥
func loadKeyFromEncryptedFile() string {
	data, err := os.ReadFile(".wallet.json")
	if err != nil {
		return ""
	}

	password := os.Getenv("WALLET_PASSWORD")
	if password == "" {
		log.Printf("⚠️  .wallet.json found but WALLET_PASSWORD not set")
		return ""
	}

	var wallet EncryptedWallet
	if err := json.Unmarshal(data, &wallet); err != nil {
		return ""
	}

	ciphertext, _ := hex.DecodeString(wallet.Ciphertext)
	salt, _ := hex.DecodeString(wallet.Salt)
	iv, _ := hex.DecodeString(wallet.IV)

	// Derive key using PBKDF2-SHA256
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}

	aesGCM, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return ""
	}

	plaintext, err := aesGCM.Open(nil, iv, ciphertext, nil)
	if err != nil {
		log.Printf("⚠️  Failed to decrypt wallet — wrong password?")
		return ""
	}

	return string(plaintext)
}

// loadKeyFromPlainFile 兼容旧版明文文件（迁移用）
func loadKeyFromPlainFile() string {
	data, err := os.ReadFile(".eth.key")
	if err != nil {
		return ""
	}
	return string(data)
}

// GenerateEncryptedWallet 生成新钱包并加密存储
func GenerateEncryptedWallet(password string) (string, error) {
	key, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", err
	}

	privateKeyHex := hex.EncodeToString(key.D.Bytes())
	addr := crypto.PubkeyToAddress(key.PublicKey).Hex()

	// Generate salt and IV
	salt := make([]byte, 32)
	rand.Read(salt)
	iv := make([]byte, 12) // GCM standard nonce size
	rand.Read(iv)

	// Derive encryption key
	encKey := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nil, iv, []byte(privateKeyHex), nil)

	wallet := EncryptedWallet{
		Address:    addr,
		Ciphertext: hex.EncodeToString(ciphertext),
		Salt:       hex.EncodeToString(salt),
		IV:         hex.EncodeToString(iv),
	}

	data, _ := json.MarshalIndent(wallet, "", "  ")
	err = os.WriteFile(".wallet.json", data, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to save encrypted wallet: %v", err)
	}

	log.Printf("🆕 New wallet generated: %s", addr)
	log.Printf("   Encrypted wallet saved to: .wallet.json")

	return privateKeyHex, nil
}

// IsInitialized 检查是否已初始化
func IsInitialized() bool {
	return client != nil && privateKey != nil
}

// GetFromAddress 获取发送地址（平台钱包）
func GetFromAddress() string {
	if fromAddr == (common.Address{}) {
		return ""
	}
	return fromAddr.Hex()
}

// SendUSDC 发送 USDC 到指定地址
func SendUSDC(toAddr string, amount float64) (string, error) {
	if !IsInitialized() {
		return "", fmt.Errorf("x402 not initialized")
	}

	if !common.IsHexAddress(toAddr) {
		return "", fmt.Errorf("invalid address: %s", toAddr)
	}

	// 金额上限检查
	if amount > 1000 {
		return "", fmt.Errorf("amount exceeds maximum (1000 USDC)")
	}
	if amount <= 0 {
		return "", fmt.Errorf("amount must be positive")
	}

	to := common.HexToAddress(toAddr)
	usdcContract := common.HexToAddress(USDCAddress)

	// USDC 精度 6 位 — 使用精确整数运算避免浮点误差
	amountMicro := int64(math.Round(amount * 1e6))
	amountInt := big.NewInt(amountMicro)

	// 构建 ERC20 transfer(address,uint256) calldata
	data := make([]byte, 4+32+32) // selector + address + amount
	copy(data[0:4], transferSelector)
	copy(data[4+12:4+32], to.Bytes()) // address 左填充到 32 字节
	amountInt.FillBytes(data[4+32 : 4+64])

	// 获取真实 nonce
	ctx := context.Background()
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	// 估算 Gas
	gasLimit := uint64(100000)
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %v", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %v", err)
	}

	// 构建交易（EIP-155）
	tx := types.NewTransaction(nonce, usdcContract, big.NewInt(0), gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %v", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send: %v", err)
	}

	log.Printf("💸 USDC sent: %.2f to %s, tx: %s", amount, toAddr, signedTx.Hash().Hex())
	return signedTx.Hash().Hex(), nil
}

// GetUSDCBalance 查询 USDC 余额（通过 ERC20 balanceOf 调用）
func GetUSDCBalance(addr string) (float64, error) {
	if !IsInitialized() {
		return 0, fmt.Errorf("x402 not initialized")
	}

	address := common.HexToAddress(addr)
	usdcContract := common.HexToAddress(USDCAddress)

	// balanceOf(address) selector: 0x70a08231
	balanceOfSelector := common.Hex2Bytes("70a08231")
	callData := make([]byte, 4+32)
	copy(callData[0:4], balanceOfSelector)
	copy(callData[4+12:4+32], address.Bytes())

	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &usdcContract,
		Data: callData,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call balanceOf: %v", err)
	}

	balance := new(big.Int).SetBytes(result)
	balanceFloat := new(big.Float).SetInt(balance)
	balanceFloat = balanceFloat.Quo(balanceFloat, big.NewFloat(1e6))

	f, _ := balanceFloat.Float64()
	return f, nil
}

// IsValidAddress 验证地址是否有效
func IsValidAddress(addr string) bool {
	return common.IsHexAddress(addr)
}

// GetETHBalance 查询 ETH 余额
func GetETHBalance(addr string) (float64, error) {
	if !IsInitialized() {
		return 0, fmt.Errorf("x402 not initialized")
	}

	address := common.HexToAddress(addr)
	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		return 0, err
	}

	balanceFloat := new(big.Float).SetInt(balance)
	balanceFloat = balanceFloat.Quo(balanceFloat, big.NewFloat(math.Pow10(18)))

	f, _ := balanceFloat.Float64()
	return f, nil
}
