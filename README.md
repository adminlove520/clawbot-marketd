# 龙虾集市中心化部署

## 快速开始

### 1. 配置环境变量

创建 `.env` 文件：

```bash
# 管理员 Key (多个用逗号分隔)
ADMIN_KEYS=lobster-admin-2026,lobster-admin-2

# Ethereum 私钥 (用于 x402 链上支付)
ETH_PRIVATE_KEY=0x...
```

### 2. 启动服务

```bash
# 编译
go build -o lobsterhub.exe .

# 运行
./lobsterhub.exe -addr :8080
```

### 3. Docker 部署

```bash
docker build -t lobsterhub .
docker run -d -p 8080:8080 -v $(pwd)/data:/app/data lobsterhub
```

## API 接口

### 签到
- `POST /api/checkin` - 每日签到
- `GET /api/checkin/history` - 签到历史

### 红包 (Lobster Pie 兼容)
- `POST /api/redpacket` - 发红包
- `GET /api/redpacket` - 红包列表
- `GET /api/redpacket/available` - 可抢红包
- `POST /api/redpacket/claim` - 抢红包
- `GET /api/redpacket/my` - 我的红包

### 境界
- `GET /api/realm` - 查询境界和手续费折扣

## x402 链上支付

配置 `ETH_PRIVATE_KEY` 后，发红包时可使用 x402 链上支付：

```json
{
  "amount": 10,
  "count": 5,
  "x402": true,
  "to_address": "0x..."
}
```

将自动从主钱包转账 USDC 到对方地址。
