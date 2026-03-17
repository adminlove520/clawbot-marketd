package x402

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var client *ethclient.Client
var privateKey *ecdsa.PrivateKey
var fromAddr common.Address

// Init 初始化 x402 支付
func Init(privateKeyHex, rpcURL, usdcAddr string) error {
	// 优先从文件加载
	if privateKeyHex == "" {
		privateKeyHex = loadKeyFromFile()
	}

	// 如果还没有，生成新的
	if privateKeyHex == "" {
		newKey, err := GenerateNewWallet()
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

func loadKeyFromFile() string {
	data, err := os.ReadFile(".eth.key")
	if err != nil {
		return ""
	}
	return string(data)
}

// GenerateNewWallet 生成新钱包并保存
func GenerateNewWallet() (string, error) {
	key, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", err
	}

	privateKeyHex := hex.EncodeToString(key.D.Bytes())

	err = os.WriteFile(".eth.key", []byte(privateKeyHex), 0600)
	if err != nil {
		log.Printf("Warning: failed to save key: %v", err)
	}

	addr := crypto.PubkeyToAddress(key.PublicKey).Hex()
	log.Printf("🆕 New wallet generated: %s", addr)
	log.Printf("   Private key saved to: .eth.key")

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

// SendUSDC 发送 USDC 到指定地址（用于抢红包）
func SendUSDC(toAddr string, amount float64) (string, error) {
	if !IsInitialized() {
		return "", fmt.Errorf("x402 not initialized")
	}

	if !common.IsHexAddress(toAddr) {
		return "", fmt.Errorf("invalid address: %s", toAddr)
	}

	// USDC 精度 6 位
	amountInt := new(big.Int).Mul(big.NewInt(int64(amount*math.Pow10(6))), big.NewInt(1))

	usdcAddr := common.HexToAddress("0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8")

	// 构建 transfer 数据
	toBytes := common.HexToAddress(toAddr).Bytes()
	data := make([]byte, 64)
	copy(data[12:32], toBytes)
	copy(data[32:64], amountInt.Bytes())

	// 估算 Gas
	gasLimit := uint64(100000)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %v", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %v", err)
	}

	// 构建交易
	tx := types.NewTransaction(0, usdcAddr, big.NewInt(0), gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %v", err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

// GetUSDCBalance 查询 USDC 余额
func GetUSDCBalance(addr string) (float64, error) {
	if !IsInitialized() {
		return 0, fmt.Errorf("x402 not initialized")
	}

	address := common.HexToAddress(addr)
	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		return 0, err
	}

	balanceFloat := new(big.Float).SetInt(balance)
	balanceFloat = balanceFloat.Quo(balanceFloat, big.NewFloat(math.Pow10(6)))

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
