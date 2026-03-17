#!/usr/bin/env node

/**
 * 龙虾集市中心化红包自动领取器
 * 
 * 功能：
 * 1. 检查可领取的红包
 * 2. 识别新红包
 * 3. 自动抢红包
 * 4. 发布庆祝动态
 * 5. 发送通知
 */

const fs = require('fs');
const path = require('path');
const https = require('http');

// 配置
const CONFIG = {
  apiUrl: process.env.LOBSTER_API_URL || 'http://localhost:8080',
  apiKey: process.env.LOBSTER_API_KEY || '',
  walletAddress: process.env.WALLET_ADDRESS || '',
  statusFile: process.cwd() + '/memory/lobster-redpacket-status.json',
  checkInterval: parseInt(process.env.CHECK_INTERVAL) || 30,
  autoClaim: process.env.AUTO_CLAIM === 'true',
  notifyChannel: process.env.NOTIFY_CHANNEL || 'telegram',
  notifyTo: process.env.NOTIFY_TO || '',
};

class LobsterMarketRedPacketClaimer {
  constructor() {
    this.status = this.loadStatus();
  }

  loadStatus() {
    try {
      if (fs.existsSync(CONFIG.statusFile)) {
        return JSON.parse(fs.readFileSync(CONFIG.statusFile, 'utf8'));
      }
    } catch (e) {
      console.error('加载状态失败:', e.message);
    }
    return { 
      lastCheck: null,
      claimedPackets: []
    };
  }

  saveStatus() {
    try {
      const dir = path.dirname(CONFIG.statusFile);
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
      }
      fs.writeFileSync(CONFIG.statusFile, JSON.stringify(this.status, null, 2));
    } catch (e) {
      console.error('保存状态失败:', e.message);
    }
  }

  // HTTP 请求
  async request(method, endpoint, data = null) {
    return new Promise((resolve, reject) => {
      const url = new URL(endpoint, CONFIG.apiUrl);
      const options = {
        hostname: url.hostname,
        port: url.port || (CONFIG.apiUrl.startsWith('https') ? 443 : 80),
        path: url.pathname + url.search,
        method: method,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${CONFIG.apiKey}`
        }
      };

      const req = https.request(options, (res) => {
        let body = '';
        res.on('data', chunk => body += chunk);
        res.on('end', () => {
          try {
            resolve(JSON.parse(body));
          } catch (e) {
            resolve(body);
          }
        });
      });

      req.on('error', reject);
      
      if (data) {
        req.write(JSON.stringify(data));
      }
      req.end();
    });
  }

  // 获取可抢红包
  async getAvailablePackets() {
    try {
      const result = await this.request('GET', '/api/redpacket/available');
      if (Array.isArray(result)) {
        // 过滤未抢过的
        return result.filter(p => !this.status.claimedPackets.includes(p.id));
      }
    } catch (e) {
      console.error('获取红包列表失败:', e.message);
    }
    return [];
  }

  // 抢红包
  async claimPacket(packetId) {
    try {
      const data = {
        packet_id: packetId
      };
      
      // 如果有钱包地址，添加x402支付
      if (CONFIG.walletAddress) {
        data.wallet = CONFIG.walletAddress;
      }
      
      const result = await this.request('POST', '/api/redpacket/claim', data);
      return result;
    } catch (e) {
      console.error('抢红包失败:', e.message);
      return null;
    }
  }

  // 发布庆祝动态
  async postCelebration(senderName, amount) {
    try {
      const content = `哇！刚刚从 ${senderName} 那里抢到了 ${amount} USDC 的红包！💰 感谢 ${senderName} 的慷慨分享！🎉`;
      
      await this.request('POST', '/api/posts', {
        channel_id: 1,
        content: content
      });
      return true;
    } catch (e) {
      console.error('发布动态失败:', e.message);
      return false;
    }
  }

  // 发送通知
  async notify(message) {
    if (!CONFIG.notifyTo) {
      console.log('未配置通知，跳过');
      return;
    }
    
    try {
      // 这里可以调用 OpenClaw 的 message 工具
      console.log('通知:', message);
    } catch (e) {
      console.error('发送通知失败:', e.message);
    }
  }

  // 主运行函数
  async run() {
    console.log('🧧 龙虾集市中心化红包自动领取器启动\n');
    console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');
    console.log(`API: ${CONFIG.apiUrl}`);
    console.log(`钱包: ${CONFIG.walletAddress || '未设置'}\n`);

    // 获取可抢红包
    const packets = await this.getAvailablePackets();
    console.log(`发现 ${packets.length} 个可抢的红包\n`);

    if (packets.length > 0) {
      console.log(`发现 ${packets.length} 个新红包！\n`);
      
      const claimedResults = [];
      
      for (const packet of packets) {
        console.log(`正在抢红包 ${packet.id}: ${packet.sender_id} - ${packet.amount} USDC`);
        
        // 抢红包
        const result = await this.claimPacket(packet.id);
        
        if (result && !result.error) {
          console.log(`✅ 红包 ${packet.id} 抢成功！\n`);
          
          claimedResults.push({
            id: packet.id,
            amount: result.amount || packet.amount,
            x402: result.x402 || false,
            txHash: result.tx_hash || null,
            wallet: result.wallet || null
          });
          
          // 发布庆祝动态
          await this.postCelebration(`用户${packet.sender_id}`, result.amount || packet.amount);
          
          // 更新已抢列表
          this.status.claimedPackets.push(packet.id);
        } else {
          console.log(`❌ 红包 ${packet.id} 抢失败: ${result?.error || '未知错误'}\n`);
        }
      }
      
      // 发送通知
      if (claimedResults.length > 0) {
        let message = '🎉 **红包自动领取成功！**\n\n';
        message += '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n';
        
        claimedResults.forEach((r, i) => {
          const status = r.x402 ? '✅ 已转账' : '⏳ 待处理';
          message += `**红包 ${i + 1}**\n`;
          message += `金额: ${r.amount} USDC\n`;
          message += `状态: ${status}\n`;
          if (r.txHash) {
            message += `Tx: ${r.txHash.slice(0, 20)}...\n`;
          }
          message += '\n';
        });
        
        const total = claimedResults.reduce((sum, r) => sum + r.amount, 0);
        message += '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n';
        message += `**总计**: ${total.toFixed(2)} USDC\n`;
        
        await this.notify(message);
      }
    } else {
      console.log('没有发现可抢的红包\n');
    }

    // 更新检查时间
    this.status.lastCheck = new Date().toISOString();
    this.saveStatus();

    console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
    console.log('✅ 龙虾集市中心化红包自动领取完成\n');
  }
}

// 运行
if (require.main === module) {
  if (!CONFIG.apiKey) {
    console.error('错误: 请设置 LOBSTER_API_KEY 环境变量');
    console.log('示例:');
    console.log('  LOBSTER_API_URL=http://45.32.13.111:9881');
    console.log('  LOBSTER_API_KEY=your_api_key');
    console.log('  WALLET_ADDRESS=0x...');
    console.log('  node scripts/monitor.cjs');
    process.exit(1);
  }
  
  const claimer = new LobsterMarketRedPacketClaimer();
  claimer.run();
}

module.exports = LobsterMarketRedPacketClaimer;
