import { challengeCatalog } from './platform'

export type EnvironmentStatus = 'idle' | 'starting' | 'running' | 'expired' | 'failed'

export type SubmissionRecord = {
  value: string
  result: 'success' | 'error'
  time: string
}

type ChallengeDetailOverride = {
  author: string
  description: string
  story: string
  attachments: Array<{ name: string; size: string; downloadUrl?: string }>
  hints: string[]
  knowledgePoints: string[]
  similarChallenges: Array<{ title: string; category: string; score: number }>
  discussionPrompt: string
  firstBloods: Array<{ label: '一血' | '二血' | '三血'; user: string; time: string }>
  submissions: SubmissionRecord[]
}

const categoryIllustrations: Record<string, string> = {
  Web: '🌐', Pwn: '🚀', Reverse: '💻', Crypto: '🔐', Misc: '🧩', Forensics: '🔎', IoT: '📡', Mobile: '📱', Cloud: '☁️', 'AI Security': '🤖',
}

const detailOverrides: Record<string, ChallengeDetailOverride> = {
  'baby-router': {
    author: 'IoT_Lab',
    description: '你收到了一台朋友送来的智能路由器。它已接入互联网，并提供了 Web 管理界面与若干服务。请从固件和服务端口中找出隐藏的 Flag。',
    story: '一台被遗忘在实验室角落的 Baby Router，正向外广播着不寻常的信号。',
    attachments: [{ name: 'firmware.bin', size: '3.24 MB' }, { name: 'router-notes.txt', size: '1.25 KB' }],
    hints: ['从设备对外开放的服务开始观察。', '固件中也许隐藏着管理凭据。'],
    knowledgePoints: ['固件提取与分析', 'Web 漏洞挖掘', 'IoT 协议分析', 'MIPS 逆向工程'],
    similarChallenges: [{ title: 'Smart Light', category: 'IoT', score: 150 }, { title: 'Home Camera', category: 'IoT', score: 200 }, { title: 'WiFi-Printer', category: 'IoT', score: 200 }],
    discussionPrompt: '分享你的分析思路，注意不要直接公开 Flag。',
    firstBloods: [{ label: '一血', user: '0x3000', time: '09:41:23' }, { label: '二血', user: 'AAA_Wrapper', time: '10:12:07' }, { label: '三血', user: 'Re1ic', time: '10:58:33' }],
    submissions: [{ value: 'flag{...router...}', result: 'success', time: '2 分钟前' }, { value: 'flag{...test...}', result: 'error', time: '8 分钟前' }, { value: 'flag{...demo...}', result: 'error', time: '18 分钟前' }],
  },
  'sqli-art': {
    author: 'WebKing',
    description: '欢迎来到一间看似普通的在线书店。认真阅读每一个请求参数，也许你会发现数据库正在偷偷向你讲述故事。',
    story: '图书管理员留下了一条奇怪的检索语句，只有真正的查询大师才能读懂。',
    attachments: [{ name: 'request-sample.txt', size: '1.8 KB' }],
    hints: ['试着比较不同检索条件返回的内容。', '不要忽略报错信息中的细节。'],
    knowledgePoints: ['SQL 注入基础', '联合查询', '报错注入', 'Web 请求分析'],
    similarChallenges: [{ title: 'Lost in Mirage', category: 'Web', score: 200 }, { title: 'JWT 安全指南', category: 'Web', score: 250 }, { title: 'Header Puzzle', category: 'Web', score: 150 }],
    discussionPrompt: '你从哪一个输入点发现了异常？留下你的思路吧。',
    firstBloods: [{ label: '一血', user: 'PwnStar', time: '08:32:15' }, { label: '二血', user: 'byte_b0y', time: '08:45:28' }, { label: '三血', user: 'BabyLogin', time: '09:02:41' }],
    submissions: [{ value: 'flag{...sqli...}', result: 'success', time: '5 分钟前' }, { value: 'flag{union_test}', result: 'error', time: '12 分钟前' }],
  },
}

function createFallbackDetail(id: string) {
  const challenge = challengeCatalog.find((item) => item.id === id) ?? challengeCatalog[0]
  const override = detailOverrides[challenge.id]

  return {
    ...challenge,
    illustration: categoryIllustrations[challenge.category] ?? '🎯',
    author: override?.author ?? 'asamu Lab',
    description: override?.description ?? `${challenge.title} 是一道 ${challenge.category} 方向的训练题。请结合题目附件、提示和动态环境进行分析。`,
    story: override?.story ?? '一段等待你解开的安全挑战故事。',
    attachments: override?.attachments ?? [{ name: `${challenge.id}.zip`, size: '2.40 MB' }],
    hints: override?.hints ?? ['关注题目描述中的关键词。', '尝试从基础功能开始逐层分析。'],
    knowledgePoints: override?.knowledgePoints ?? [challenge.category, ...challenge.tags],
    similarChallenges: override?.similarChallenges ?? [{ title: '安全小练习', category: challenge.category, score: 150 }, { title: '进阶挑战', category: challenge.category, score: 250 }, { title: '综合实验室', category: challenge.category, score: 300 }],
    discussionPrompt: override?.discussionPrompt ?? '留下你的分析思路，与其他 CTFer 一起交流。',
    firstBloods: override?.firstBloods ?? [{ label: '一血' as const, user: '0x3000', time: '10:12:08' }, { label: '二血' as const, user: 'AAA_Wrapper', time: '10:26:11' }, { label: '三血' as const, user: 'LightHouse', time: '10:43:29' }],
    submissions: override?.submissions ?? [{ value: 'flag{...sample...}', result: 'error' as const, time: '刚刚' }],
  }
}

export function getChallengeDetail(id?: string) {
  return createFallbackDetail(id ?? '')
}

export const environmentStatusMeta: Record<EnvironmentStatus, { label: string; tone: 'blue' | 'yellow' | 'green' | 'red'; description: string }> = {
  idle: { label: '未启动', tone: 'blue', description: '点击按钮创建专属靶机环境。' },
  starting: { label: '启动中', tone: 'yellow', description: '正在分配容器和端口，请稍候。' },
  running: { label: '运行中', tone: 'green', description: '靶机已准备就绪，可以开始挑战。' },
  expired: { label: '已过期', tone: 'blue', description: '环境已自动回收，请重新启动。' },
  failed: { label: '启动失败', tone: 'red', description: '环境暂时不可用，请重新尝试。' },
}
