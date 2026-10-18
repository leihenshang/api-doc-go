// 极简静态服务器 + /api 反向代理，用于本地验证 web 端生产构建产物。
//   node serve-dist.mjs  → http://localhost:4173
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'

const ROOT = process.argv[2] || 'dist-review'
const BACKEND = process.env.BACKEND || 'http://127.0.0.1:2028'
const PORT = Number(process.env.PORT || 4173)

const mime = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.svg': 'image/svg+xml', '.json': 'application/json',
  '.png': 'image/png', '.ico': 'image/x-icon', '.woff2': 'font/woff2',
}

http.createServer((req, res) => {
  if (req.url.startsWith('/api/') || req.url.startsWith('/uploads/')) {
    const target = new URL(req.url, BACKEND)
    const proxy = http.request(target, {
      method: req.method,
      headers: { ...req.headers, host: target.host },
    }, (r) => {
      res.writeHead(r.statusCode || 502, r.headers)
      r.pipe(res)
    })
    proxy.on('error', (e) => { res.writeHead(502); res.end('proxy error: ' + e.message) })
    req.pipe(proxy)
    return
  }
  let file = path.join(ROOT, decodeURIComponent(req.url.split('?')[0]))
  if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(ROOT, 'index.html')
  fs.readFile(file, (err, buf) => {
    if (err) { res.writeHead(404); res.end('not found'); return }
    res.writeHead(200, { 'Content-Type': mime[path.extname(file)] || 'application/octet-stream' })
    res.end(buf)
  })
}).listen(PORT, () => console.log(`serve ${ROOT} on http://localhost:${PORT}, proxy -> ${BACKEND}`))
