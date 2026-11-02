// 前端语法闸：index.html 里的 <script> 能否解析、<style> 括号是否配平。
// 没有构建步骤，就在 CI / make check 里兜一层，避免推到线上才发现白屏。
import { readFileSync } from 'node:fs';

const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8');
let failed = 0;

// importmap 之类非 JS 的 <script> 跳过，只校验真正的脚本
const scripts = [...html.matchAll(/<script([^>]*)>([\s\S]*?)<\/script>/g)]
  .filter(m => !/type\s*=\s*"(importmap|application\/json)/.test(m[1]))
  .map(m => m[2]);
if (!scripts.length) { console.error('no <script> found'); process.exit(1); }
scripts.forEach((src, i) => {
  // 模块脚本里的 import 语句交给浏览器，这里只校验函数体语法
  const body = src.replace(/^\s*import\s.+$/gm, '');
  try { new Function(body); } catch (e) { failed++; console.error(`script #${i} syntax error: ${e.message}`); }
});
console.log(`scripts checked: ${scripts.length}`);

const styles = [...html.matchAll(/<style>([\s\S]*?)<\/style>/g)].map(m => m[1]).join('\n');
let depth = 0, bad = false;
for (const ch of styles) { if (ch === '{') depth++; else if (ch === '}') { depth--; if (depth < 0) bad = true; } }
if (depth !== 0 || bad) { failed++; console.error(`css braces unbalanced (depth ${depth})`); }
console.log('css braces balanced');

// 关键钩子存在性（被 selftest 依赖的 id）
['selftestResult-hook', 'itemsDlg', 'printSheet', 'styleSel', 'furnAll'].forEach(id => {
  const probe = id === 'selftestResult-hook' ? /has\('selftest'\)/ : new RegExp(`id="${id}"`);
  if (!probe.test(html)) { failed++; console.error(`missing hook: ${id}`); }
});

process.exit(failed ? 1 : 0);
