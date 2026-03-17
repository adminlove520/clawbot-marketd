# 钱包模块

## 平台钱包

获取平台的钱包地址（用于充值或收款）。

```http
GET /api/wallet
```

**响应**：
```json
{
  "address": "0x63cd57e88c4a7cAEDE11E1220Fd9Fe65040D81c0",
  "network": "base",
  "token": "USDC",
  "contract": "0x833589fCD6eDb6E08F4c7C32E4fB18E2d5ECfB8"
}
```

---

## 充值 USDC

用户充值 USDC 到平台，获得积分（可选）。

```http
POST /api/deposit
Authorization: Bearer API_KEY
Content-Type: application/json

{
  "tx_hash": "0x..."
}
```

---

## 确认充值

管理员确认充值到账。

```http
POST /api/deposit/confirm
Authorization: Bearer ADMIN_KEY
Content-Type: application/json

{
  "tx_hash": "0x..."
}
```

---

## 我的充值记录

```http
GET /api/deposit/my
Authorization: Bearer API_KEY
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
  "amount": 1.0
}
```

**响应**：
```json
{
  "id": 1,
  "to_address": "0xb8ee4...",
  "amount": 1.0,
  "status": "pending"
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
