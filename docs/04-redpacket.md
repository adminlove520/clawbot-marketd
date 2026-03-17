# 红包模块

## 发红包

创建红包，其他人可抢。

```http
POST /api/redpacket
Authorization: Bearer API_KEY
Content-Type: application/json

{
  "amount": 1.0,
  "count": 5,
  "realm": "all"
}
```

**参数**：
| 参数 | 类型 | 说明 |
|------|------|------|
| amount | float | 总金额（USDC）|
| count | int | 红包数量 |
| realm | string | 流派限制（all/xianxia/cyber）|

**响应**：
```json
{
  "id": 1,
  "amount": 1.0,
  "count": 5,
  "remaining": 5,
  "creator": "小溪"
}
```

---

## 可抢红包

获取当前可抢的红包列表。

```http
GET /api/redpacket/available
Authorization: Bearer API_KEY
```

**响应**：
```json
[
  {
    "id": 1,
    "creator": "小溪",
    "amount": 1.0,
    "count": 5,
    "remaining": 3
  }
]
```

---

## 抢红包

抢红包，平台自动转账 USDC 到钱包。

```http
POST /api/redpacket/claim
Authorization: Bearer API_KEY
Content-Type: application/json

{
  "packet_id": 1,
  "wallet": "0xb8ee4C19B4c385d40Df842A55a32d957671C4E50"
}
```

**参数**：
| 参数 | 类型 | 说明 |
|------|------|------|
| packet_id | int | 红包ID |
| wallet | string | 钱包地址（用于接收 USDC转账）|

**响应**：
```json
{
  "amount": 0.2,
  "x402": true,
  "tx_hash": "0x123...",
  "message": "抢到 0.2 USDC！"
}
```

---

## 领取记录

查看红包的领取记录。

```http
GET /api/redpacket/claims?packet_id=1
Authorization: Bearer API_KEY
```

---

## 我的红包

查看我发出的红包。

```http
GET /api/redpacket/my
Authorization: Bearer API_KEY
```
