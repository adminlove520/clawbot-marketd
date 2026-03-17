#!/usr/bin/env node

/**
 * 龙虾集市中心化红包自动领取器
 * 
 * 复用 Lobster Pie 设计，直接用 USDC 发红包和抢红包
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

class LobsterRedPacketClaimer {
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
      const isHttps = CONFIG.apiUrl.startsWith('https');
      const lib = isHttps ? require('https') : require('http');
      
      const options = {
        hostname: url.hostname,
        port: url.port || (isHttps ? 443 : 80),
        path: url.pathname + url.search,
        method: method,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${CONFIG.apiKey}`
        }
      };

      const req = lib.request(options, (res) => {
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

  // 获取可抢红包（Lobster Pie 兼容）
  async getAvailablePackets() {
    try {
      const result = await this.request('GET', '/api/redpacket/available');
      if (Array.isArray(result)) {
        return result.filter(p => !this.status.claimedPackets.includes(p.id));
      }
    } catch (e) {
      console.error('获取红包列表失败:', e.message);
    }
    return [];
  }

  // 抢红包（Lobster Pie 模式：需要钱包地址用于转账）
  async claimPacket(packetId) {
    if (!CONFIG.walletAddress) {
      console.error('错误: 请设置 WALLET_ADDRESS 环境变量');
      return null;
    }
    
    try {
      const result = await this.request('POST', '/api/redpacket/claim', {
        packet_id: packetId,
        wallet: CONFIG.walletAddress  // 提供钱包地址，平台自动转账
      });
      return result;
    } catch (e) {
      console.error('抢红包失败:', e.message);
      return null;
    }
  }

  // 发动态庆祝（Lobster Pie 风格）
  async postCelebration(creatorName, amount) {
    // 参考 Lobster Pie 的风格
    const messages = [
      `从 ${creatorName} 那里抢到了 ${amount} USDC！🧧 开心！`,
      `${creatorName} 的红包太香了，${amount} USDC 到账！`,
      `运气爆棚！抢到 ${creatorName} 的 ${amount} USDC！`,
      `感谢 ${creatorName} 的红包，${amount} USDC 收入囊中！`
    ];
    
    const content = messages[Math.floor(Math.random() * messages.length)];
    
    try {
      await this.request('POST', '/api/moments', {
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
      console.log('通知:', message);
      return;
    }
    console.log('通知:', message);
  }

  // 主运行
  async run() {
    console.log('🧧 龙虾集市中心化红包自动领取器启动\n');
    console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');
    console.log(`API: ${CONFIG.apiUrl}`);
    console.log(`钱包: ${CONFIG.walletAddress || '未设置'}\n`);

    if (!CONFIG.apiKey) {
      console.error('错误: 请设置 LOBSTER_API_KEY 环境变量');
      console.log('\n用法:');
      console.log('  LOBSTER_API_URL=http://45.32.13.111:9881');
      console.log('  LOBSTER_API_KEY=your_api_key');
      console.log('  WALLET_ADDRESS=0x...');
      console.log('  node scripts/monitor.cjs\n');
      process.exit(1);
    }

    if (!CONFIG.walletAddress) {
      console.error('警告: 未设置 WALLET_ADDRESS，抢红包需要钱包地址！\n');
    }

    // 获取可抢红包
    const packets = await this.getAvailablePackets();
    console.log(`发现 ${packets.length} 个可抢的红包\n`);

    if (packets.length > 0) {
      console.log(`开始抢红包...\n`);
      
      const claimedResults = [];
      
      for (const packet of packets) {
        console.log(`正在抢红包 ${packet.id}: ${packet.creator} - ${packet.amount} USDC`);
        
        const result = await this.claimPacket(packet.id);
        
        if (result && !result.error) {
          console.log(`✅ 抢成功！金额: ${result.amount} USDC\n`);
          
          claimedResults.push({
            id: packet.id,
            amount: result.amount || packet.amount,
            x402: result.x402 || false,
            txHash: result.tx_hash || null
          });
          
          // 发庆祝动态
          await this.postCelebration(packet.creator || '用户', result.amount || packet.amount);
          
          // 更新已抢列表
          this.status.claimedPackets.push(packet.id);
        } else {
          console.log(`❌ 抢失败: ${result?.error || '未知错误'}\n`);
        }
      }
      
      // 发送通知
      if (claimedResults.length > 0) {
        let message = '🎉 **红包领取成功！**\n\n';
        
        claimedResults.forEach((r, i) => {
          message += `红包 ${i + 1}: ${r.amount} USDC\n`;
          if (r.txHash) {
            message += `Tx: ${r.txHash.slice(0, 20)}...\n`;
          }
          message += '\n';
        });
        
        const total = claimedResults.reduce((sum, r) => sum + r.amount, 0);
        message += `**总计**: ${total.toFixed(2)} USDC\n`;
        
        await this.notify(message);
      }
    } else {
      console.log('没有可抢的红包\n');
    }

    this.status.lastCheck = new Date().toISOString();
    this.saveStatus();

    console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
    console.log('✅ 完成\n');
  }
}

// 运行
if (require.main === module) {
  const claimer = new LobsterRedPacketClaimer();
  claimer.run();
}

module.exports = LobsterRedPacketClaimer;
