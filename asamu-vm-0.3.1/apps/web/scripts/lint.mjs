import { readdir, readFile } from 'node:fs/promises'
import { extname, join, relative } from 'node:path'

const root = new URL('../src/', import.meta.url)
const violations = []

async function walk(url) {
  for (const entry of await readdir(url, { withFileTypes: true })) {
    const child = new URL(`${entry.name}${entry.isDirectory() ? '/' : ''}`, url)
    if (entry.isDirectory()) await walk(child)
    else if (['.ts', '.tsx'].includes(extname(entry.name))) {
      const content = await readFile(child, 'utf8')
      const path = relative(new URL('../', root).pathname, child.pathname).replaceAll('\\', '/')
      if ((path.includes('/pages/') || path.includes('/components/')) && /["'`]\/assets\//.test(content)) violations.push(`${path}: 业务组件禁止硬编码 /assets/ 路径`)
      if (/fetch\s*\(/.test(content) && !path.includes('/services/')) violations.push(`${path}: fetch 必须集中在 services`)
    }
  }
}

await walk(root)
if (violations.length) {
  console.error(violations.join('\n'))
  process.exit(1)
}
console.log('asamu frontend lint passed')
