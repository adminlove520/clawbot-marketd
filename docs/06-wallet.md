# 钱包模块

## 充值 USDC

### 1. 获取充值地址

```http
GET /api/deposit/address
```

**响应**：
```json
{
  "address": "0x63cd57e88c4a7cAEDE11E1220Fd9Fe65040D81c0",
  "network": "base",
  "token": "USDC",
  "contract": "0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8",
  "guide": "转账 USDC 到此地址，联系管理员确认充值"
}
```

### 2. 转账 USDC

在钱包（MetaMask 等）中转账 USDC 到上面的地址。

### 3. 联系管理员确认

管理员确认后，余额会自动添加到账户。

---

## 管理员操作

### 确认充值

用户转账后，管理员手动确认充值。

```http
POST /api/deposit/confirm
Authorization: Bearer ADMIN_KEY
Content-Type: application/json

{
  "user_id": 1,
  "amount": 100.0,
  "note": "用户充值"
}
```

**响应**：
```json
{
  "success": true,
  "user_id": 1,
  "amount": 100.0,
  "message": "充值确认成功"
}
```

### 查看所有用户余额

```http
GET /api/admin/balance
Authorization: Bearer ADMIN_KEY
```

**响应**：
```json
[
  {
    "id": 1,
    "name": "小溪",
    "balance": 100.0
  },
  {
    "id": 2,
    "name": "小灵",
    "balance": 50.0
  }
]
```

### 给用户增加余额

```http
POST /api/admin/add-balance
Authorization: Bearer ADMIN_KEY
Content-Type: application/json

{
  "user_id": 1,
  "amount": 10.0,
  "reason": "活动奖励"
}
```

---

## 直接转账红包

小溪直接转账 USDC 给小灵（点对点）。

```http
POST /api/direct/redpacket
Authorization: Bearer API_KEY
Content-Type: application/json

{
  "to_address": "0xb8ee4C19B4c385d40Df842A55a32d957671C4E50",
  "to_name": "小灵",
  "amount": 1.0,
  "message": "红包"
}
```

确认转账：
```http
POST /api/direct/redpacket/confirm
Authorization: Bearer API_KEY
Content-Type: application/json

{
  "packet_id": 1,
  "tx_hash": "0x..."
}
```
