// Isolated acceptance fixture. This is not a production moderation classifier.
const http = require('node:http');
http.createServer((req, res) => {
  if (req.url === '/health') { res.end('ok'); return; }
  if (req.url !== '/v1/chat/completions' || req.method !== 'POST' || req.headers.authorization !== 'Bearer fixture-only') { res.writeHead(404).end(); return; }
  let data = '', size = 0;
  req.on('data', chunk => { size += chunk.length; if (size > 7 * 1024 * 1024) req.destroy(); else data += chunk; });
  req.on('end', () => {
    try {
      const body = JSON.parse(data), content = body.messages[1].content;
      const parts = Array.isArray(content) ? content : [];
      const recipe = JSON.parse(parts.length ? parts.find(p => p.type === 'text').text : content);
      const withCover = parts.some(p => p.type === 'image_url' && p.image_url.url.startsWith('data:image/jpeg;base64,'));
      if (recipe.title.includes('初审服务异常')) { res.writeHead(503).end(); return; }
      const issues = recipe.title.includes('初审拒绝封面') && withCover
        ? [{ field: 'cover', reason: '封面包含广告信息，请更换为与配方相关的食物照片。' }]
        : recipe.title.includes('初审拒绝文本')
          ? [{ field: 'steps', reason: '请补充安全、完整的烹饪步骤，并删除危险做法。' }] : [];
      const verdict = { decision: issues.length ? 'rejected' : 'approved', image_checked: withCover, issues };
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({ choices: [{ message: { content: JSON.stringify(verdict) } }] }));
    } catch { res.writeHead(400).end(); }
  });
}).listen(8080, '0.0.0.0');
