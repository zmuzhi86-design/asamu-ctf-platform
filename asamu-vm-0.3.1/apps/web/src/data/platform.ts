import { assets, categoryKeyByLabel } from './assetManifest'

export const navigationItems = [
  { to: '/', label: '首页' }, { to: '/challenges', label: '题库' }, { to: '/competitions', label: '比赛' },
  { to: '/learning', label: '训练路线' }, { to: '/teams', label: '战队' }, { to: '/leaderboard', label: '排行榜' },
  { to: '/writeups', label: 'WriteUp' },
]

export const categoryLabels = ['Web', 'Pwn', 'Reverse', 'Crypto', 'Misc', 'Forensics', 'IoT', 'Mobile', 'Cloud', 'AI Security'] as const

export const categories = categoryLabels.map((label, index) => {
  const key = categoryKeyByLabel[label]
  const names = ['Web 实验室', 'Pwn 芯片工厂', 'Reverse 迷宫', 'Crypto 水晶密室', 'Misc 杂货铺', 'Forensics 档案室', 'IoT 路由器工坊', 'Mobile 移动实验室', 'Cloud 云岛', 'AI 机器人实验室']
  return { label, key, name: names[index], icon: assets.categories[key].icon, scene: assets.categories[key].scene }
})

export type Challenge = {
  id: string; title: string; category: string; difficulty: string; score: number; solves: number; solveRate: string;
  tags: string[]; dynamic: boolean; attachment: boolean; writeup: boolean; solved: boolean; favorite?: boolean
}

export const challengeCatalog: Challenge[] = [
  { id: 'sqli-art', title: 'SQLi 是门艺术', category: 'Web', difficulty: '中等', score: 300, solves: 2451, solveRate: '32.14%', tags: ['SQLi', 'Union', '报错注入'], dynamic: true, attachment: true, writeup: true, solved: false, favorite: true },
  { id: 'ret2win', title: 'ret2win 小火箭', category: 'Pwn', difficulty: '简单', score: 250, solves: 3678, solveRate: '45.22%', tags: ['ROP', 'ELF', 'NX'], dynamic: true, attachment: true, writeup: true, solved: true },
  { id: 'baby-router', title: 'Baby Router', category: 'IoT', difficulty: '中等', score: 300, solves: 1324, solveRate: '19.77%', tags: ['Firmware', 'MIPS', 'Router'], dynamic: true, attachment: true, writeup: false, solved: false },
  { id: 'baby-re', title: 'baby_re', category: 'Reverse', difficulty: '简单', score: 200, solves: 4112, solveRate: '51.82%', tags: ['反汇编', '字符串', '逆向'], dynamic: false, attachment: true, writeup: true, solved: true },
  { id: 'rsa-lab', title: 'RSA 小剧场', category: 'Crypto', difficulty: '中等', score: 350, solves: 2103, solveRate: '28.41%', tags: ['RSA', '数论', '低指数'], dynamic: false, attachment: false, writeup: true, solved: false },
  { id: 'cloud-walk', title: '云上攻防战', category: 'Cloud', difficulty: '中等', score: 300, solves: 1789, solveRate: '24.66%', tags: ['AWS', 'S3', 'IAM'], dynamic: true, attachment: false, writeup: true, solved: false },
  { id: 'heap-escape', title: '神秘的压缩包', category: 'Misc', difficulty: '简单', score: 150, solves: 5620, solveRate: '74.35%', tags: ['文件分析', '隐写', '压缩包'], dynamic: false, attachment: true, writeup: true, solved: true },
  { id: 'pcap-trace', title: '取证入门：日志迷踪', category: 'Forensics', difficulty: '中等', score: 250, solves: 2876, solveRate: '41.07%', tags: ['PCAP', 'ELK', '时间线'], dynamic: false, attachment: true, writeup: false, solved: false },
  { id: 'mobile-lock', title: '解锁你的手机', category: 'Mobile', difficulty: '中等', score: 300, solves: 2051, solveRate: '30.62%', tags: ['Android', 'Frida', 'Hook'], dynamic: true, attachment: true, writeup: false, solved: false },
  { id: 'prompt-lab', title: 'AI 模型越狱初探', category: 'AI Security', difficulty: '困难', score: 450, solves: 1102, solveRate: '16.87%', tags: ['LLM', 'Prompt', '越狱'], dynamic: true, attachment: false, writeup: true, solved: false },
  { id: 'osint-art', title: '社工的艺术', category: 'Misc', difficulty: '入门', score: 100, solves: 3945, solveRate: '58.13%', tags: ['OSINT', '信息收集'], dynamic: false, attachment: true, writeup: true, solved: true },
  { id: 'kernel-door', title: '内核的后门', category: 'Pwn', difficulty: '专家', score: 600, solves: 324, solveRate: '5.12%', tags: ['Kernel', 'UAF', '提权'], dynamic: true, attachment: true, writeup: false, solved: false },
]

export const featuredChallenges = challengeCatalog.slice(0, 4)

export const competitions = [
  { id: 'bupt-ctf-2026', name: 'BUPT-CTF 2026 asamu 杯', status: '进行中', mode: 'Jeopardy 团队赛', time: '07/10 09:00 - 07/13 09:00', teams: 1287, challenges: 36, prize: '¥ 50,000' },
  { id: 'weekend-45', name: '周末练习赛 #45', status: '报名中', mode: '个人练习赛', time: '07/18 20:00 - 22:00', teams: 3214, challenges: 12, prize: '限定徽章' },
  { id: 'reverse-5', name: 'Reverse 进阶赛 #5', status: '即将开始', mode: '个人赛', time: '07/22 10:00 - 18:00', teams: 1856, challenges: 18, prize: '2,000 积分' },
  { id: 'spring-campus', name: '春季高校邀请赛', status: '已结束', mode: '高校团队赛', time: '05/01 09:00 - 05/03 18:00', teams: 642, challenges: 32, prize: '荣誉证书' },
]

export const leaderboardRows = [
  { name: '0x3000', org: '清华大学', score: 28450, solves: 162, bloods: 24, delta: '+3' },
  { name: 'AAA_Wrapper', org: '浙江大学', score: 24190, solves: 138, bloods: 18, delta: '—' },
  { name: '0xCD1', org: '上海交通大学', score: 22680, solves: 129, bloods: 16, delta: '+1' },
  { name: 'LightHouse', org: '北京邮电大学', score: 20310, solves: 118, bloods: 12, delta: '-2' },
  { name: 'H3r0es', org: '西安电子科技大学', score: 18900, solves: 107, bloods: 9, delta: '+4' },
  { name: 'Mirage', org: '电子科技大学', score: 17740, solves: 98, bloods: 8, delta: '+1' },
]

export const writeups = [
  { id: 'ssti-to-rce', title: 'Lost in Mirage：从 SSTI 到 RCE', category: 'Web', author: 'asamu 官方', views: 3650, likes: 248, featured: true, summary: '从模板注入入口开始，逐步分析过滤规则、绕过策略与最终命令执行链。' },
  { id: 'tcache-strategy', title: 'HeapHouse 2.0：tcache 策略详解', category: 'Pwn', author: '0xBABY', views: 2180, likes: 176, featured: true, summary: '用清晰的内存布局图拆解堆利用中的关键操作。' },
  { id: 'flatten-control', title: '迷雾之下：控制流平坦化还原', category: 'Reverse', author: 'r0cky', views: 1890, likes: 143, featured: false, summary: '从识别调度器到自动化还原基本块，完整记录逆向过程。' },
  { id: 'rsa-low-e', title: 'RSA 低加密指数攻击实验笔记', category: 'Crypto', author: 'CryptoCat', views: 1640, likes: 119, featured: false, summary: '结合比赛题目复习低指数攻击的条件、推导与脚本实现。' },
  { id: 'router-firmware', title: 'Baby Router 固件分析复盘', category: 'IoT', author: 'IoT_Lab', views: 980, likes: 88, featured: false, summary: '从固件解包、凭据检索到模拟服务启动的完整路线。' },
]

export const teams = [
  { id: 'mirage', name: 'Mirage', slogan: '让安全被理解，让世界更安全。', members: 24, rank: 12, score: 20480, recruiting: true },
  { id: 'byte-rangers', name: 'Byte Rangers', slogan: '探索每一个未知字节。', members: 18, rank: 28, score: 17620, recruiting: true },
  { id: 'blue-lab', name: 'Blue Lab', slogan: '学习、分享、共同突破。', members: 31, rank: 41, score: 15300, recruiting: false },
  { id: 'zero-day-club', name: 'Zero Day Club', slogan: '保持好奇，持续验证。', members: 16, rank: 56, score: 12980, recruiting: true },
]

export const activityFeed = [
  { user: '0x3000', action: '解出', target: 'Kernel Door', time: '2 分钟前', score: '+600' },
  { user: 'AAA_Wrapper', action: '获得一血', target: 'Prompt Lab', time: '6 分钟前', score: '+450' },
  { user: 'Mirage', action: '完成比赛报名', target: 'asamu 杯', time: '12 分钟前', score: '' },
  { user: 'CryptoCat', action: '发布题解', target: 'RSA 低指数攻击', time: '20 分钟前', score: '' },
]

export const adminSections = {
  overview: { title: '管理概览', description: '平台运行、内容审核与资源使用情况。' },
  challenges: { title: '题目管理', description: '维护题目、附件、Hint 与动态环境配置。' },
  competitions: { title: '比赛管理', description: '创建比赛、配置题目池、赛制与封榜。' },
  instances: { title: '动态环境', description: '查看实例状态、资源占用与生命周期。' },
  users: { title: '用户与战队', description: '用户角色、战队成员和违规状态管理。' },
  submissions: { title: '提交记录', description: '检索判题结果、耗时与来源信息。' },
  antiCheat: { title: '反作弊中心', description: '审核异常提交、共享 Flag 与高风险行为。' },
  writeups: { title: 'WriteUp 审核', description: '审核投稿、设置精选并处理违规内容。' },
  announcements: { title: '公告管理', description: '发布平台公告、比赛通知和维护提醒。' },
  settings: { title: '系统设置', description: '站点、邮件、存储与容器平台配置。' },
} as const

export type AdminSection = keyof typeof adminSections

export const adminInstances = [
  { id: 'ins-9042', challenge: 'SQLi 是门艺术', owner: 'team-1024', status: '运行中', cpu: '12%', memory: '128 MB', ttl: '01:24:18' },
  { id: 'ins-9041', challenge: 'Baby Router', owner: 'Mirage', status: '启动中', cpu: '4%', memory: '64 MB', ttl: '—' },
  { id: 'ins-9039', challenge: 'Kernel Door', owner: '0x3000', status: '运行中', cpu: '38%', memory: '512 MB', ttl: '00:42:09' },
  { id: 'ins-9036', challenge: 'Prompt Lab', owner: 'BlueLab', status: '启动失败', cpu: '0%', memory: '0 MB', ttl: '—' },
]

export const adminUsers = [
  { name: '小鹿同学', role: '选手', team: 'Mirage', status: '正常', joined: '2026-03-18' },
  { name: '0x3000', role: '出题人', team: '0x3000', status: '正常', joined: '2025-11-02' },
  { name: 'AAA_Wrapper', role: '选手', team: 'Byte Rangers', status: '正常', joined: '2026-01-11' },
  { name: 'test_account_03', role: '选手', team: '—', status: '待审核', joined: '2026-07-11' },
]

export const adminSubmissions = [
  { time: '10:42:16', user: '0x3000', challenge: 'Kernel Door', result: '正确', score: 600, source: '10.2.4.18' },
  { time: '10:41:58', user: 'AAA_Wrapper', challenge: 'Prompt Lab', result: '正确', score: 450, source: '10.2.8.22' },
  { time: '10:41:32', user: 'BabyLogin', challenge: 'SQLi 是门艺术', result: '错误', score: 0, source: '10.2.7.31' },
  { time: '10:40:11', user: 'CryptoCat', challenge: 'RSA 小剧场', result: '正确', score: 350, source: '10.2.5.09' },
]

export const antiCheatCases = [
  { id: 'AC-184', level: '高风险', reason: '不同战队提交相同动态 Flag', subjects: '3 个账号', status: '待处理' },
  { id: 'AC-183', level: '中风险', reason: '一分钟内连续成功提交 7 题', subjects: '1 个账号', status: '调查中' },
  { id: 'AC-182', level: '低风险', reason: '登录地区短时间切换', subjects: '2 个账号', status: '已记录' },
]

export const announcements = [
  { title: 'asamu 杯动态环境资源扩容', type: '比赛公告', status: '已发布', date: '2026-07-11' },
  { title: '周末平台维护安排', type: '系统维护', status: '定时发布', date: '2026-07-12' },
  { title: 'WriteUp 创作激励计划', type: '社区活动', status: '草稿', date: '—' },
]
