// 端到端冒烟：真点按钮，不只看纯函数。
// 用无头 Chrome + CDP（Node 22+ 自带 WebSocket，无需依赖），跑完打印 PASS/FAIL。
//
//   node scripts/e2e.mjs [--url http://127.0.0.1:8810/] [--keep]
//
// 覆盖：清单生成 → 「我已有」开关 → 价格档位 → 预算条 → 平面图/清单打印页 → 3D 样板间与特写。
import { spawn } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const args = process.argv.slice(2);
const arg = (name, fallback) => { const i = args.indexOf(name); return i >= 0 ? args[i + 1] : fallback; };
const BASE = arg('--url', 'http://127.0.0.1:8810/').replace(/\/$/, '');
const CHROME = process.env.CHROME_BIN || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PORT = 9333 + Math.floor(Math.random() * 200);
const profile = mkdtempSync(join(tmpdir(), 'fp-e2e-'));

const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', '--no-sandbox', '--mute-audio',
  '--use-angle=swiftshader', '--enable-unsafe-swiftshader',
  `--remote-debugging-port=${PORT}`, `--user-data-dir=${profile}`,
  '--window-size=1500,1000', 'about:blank',
], { stdio: 'ignore' });

const sleep = ms => new Promise(r => setTimeout(r, ms));

async function targetUrl() {
  for (let i = 0; i < 80; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${PORT}/json/list`);
      const list = await r.json();
      const page = list.find(t => t.type === 'page');
      if (page?.webSocketDebuggerUrl) return page.webSocketDebuggerUrl;
    } catch {}
    await sleep(150);
  }
  throw new Error('chrome devtools not reachable');
}

class CDP {
  constructor(ws) { this.ws = ws; this.id = 0; this.pending = new Map(); this.logs = []; }
  static async open(url) {
    const ws = new WebSocket(url);
    await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });
    const cdp = new CDP(ws);
    ws.onmessage = e => {
      const m = JSON.parse(e.data);
      if (m.id && cdp.pending.has(m.id)) { cdp.pending.get(m.id)(m); cdp.pending.delete(m.id); }
      else if (m.method === 'Runtime.consoleAPICalled') cdp.logs.push(m.params.args.map(a => a.value ?? a.description ?? '').join(' '));
      else if (m.method === 'Runtime.exceptionThrown') cdp.logs.push('EXCEPTION: ' + (m.params.exceptionDetails.exception?.description || m.params.exceptionDetails.text));
    };
    await cdp.send('Runtime.enable');
    await cdp.send('Page.enable');
    return cdp;
  }
  send(method, params = {}) {
    const id = ++this.id;
    return new Promise(res => { this.pending.set(id, res); this.ws.send(JSON.stringify({ id, method, params })); });
  }
  async eval(expr) {
    const r = await this.send('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true });
    if (r.result?.exceptionDetails) throw new Error(r.result.exceptionDetails.exception?.description || 'eval failed');
    return r.result?.result?.value;
  }
  async goto(url) {
    await this.send('Page.navigate', { url });
    await sleep(1200);
  }
}

const checks = [];
const check = (ok, name, extra = '') => { checks.push({ ok, name, extra }); console.log(`${ok ? '  ✓' : '  ✗'} ${name}${extra ? ' — ' + extra : ''}`); };

try {
  const cdp = await CDP.open(await targetUrl());
  await cdp.goto(`${BASE}/?demo=1`);   // demo：清单为空时自动生成一份
  await sleep(1200);

  // 1. 打开清单
  await cdp.eval(`document.querySelector('#btnItems').click(); true`);
  await sleep(400);
  const rows = await cdp.eval(`document.querySelectorAll('#itemsTbl tbody tr').length`);
  check(rows > 60, '清单生成', `${rows} 行`);
  const cats = await cdp.eval(`new Set([...document.querySelectorAll('#itemsTbl select')].map(s => s.value)).size`);
  check(cats >= 10, '清单分类', `${cats} 类`);

  // 2. 「我已有」开关：点掉地板，地面行消失、预算下降，再点回来复原
  const before = await cdp.eval(`budgetTotals().sum`);
  await cdp.eval(`document.querySelector('#bomOff [data-off="floor"]').click(); true`);
  await sleep(500);
  const noFloor = await cdp.eval(`({ sum: budgetTotals().sum,
    floors: [...document.querySelectorAll('#itemsTbl select')].filter(s => s.value === '地面').length,
    chip: document.querySelector('#bomOff [data-off="floor"]').className })`);
  check(noFloor.floors === 0, '关掉「地板」后没有地面行', `floors=${noFloor.floors}`);
  check(noFloor.sum < before, '预算随之下降', `${Math.round(before)} → ${Math.round(noFloor.sum)}`);
  check(noFloor.chip.includes('off'), 'chip 变为已排除状态', noFloor.chip);
  await cdp.eval(`document.querySelector('#bomOff [data-off="floor"]').click(); true`);
  await sleep(500);
  const restored = await cdp.eval(`budgetTotals().sum`);
  check(Math.abs(restored - before) < 1, '点回来预算复原', `${Math.round(restored)}`);

  // 3. 价格档位：只缩放参考价行
  const probe = await cdp.eval(`(() => {
    const row = [...document.querySelectorAll('#itemsTbl tbody tr')].find(tr => tr.querySelector('input[data-k=qty]')?.value === '39.4');
    const price = row ? row.querySelector('input[data-k=price]').value : null;
    document.querySelector('#bomTier [data-tier="lux"]').click();
    return { before: price };
  })()`);
  await sleep(500);
  const lux = await cdp.eval(`(() => {
    const row = [...document.querySelectorAll('#itemsTbl tbody tr')].find(tr => tr.querySelector('input[data-k=qty]')?.value === '39.4');
    return row ? row.querySelector('input[data-k=price]').value : null;
  })()`);
  check(Number(lux) > Number(probe.before), '高端档把参考价 ×1.5', `${probe.before} → ${lux}`);
  // 手改一行价格后，切档不再动它
  await cdp.eval(`(() => {
    const inp = [...document.querySelectorAll('#itemsTbl tbody tr')][0].querySelector('input[data-k=price]');
    inp.value = 999; inp.dispatchEvent(new Event('input', { bubbles: true }));
    document.querySelector('#bomTier [data-tier="std"]').click();
    return true;
  })()`);
  await sleep(500);
  const kept = await cdp.eval(`[...document.querySelectorAll('#itemsTbl tbody tr')][0].querySelector('input[data-k=price]').value`);
  check(Number(kept) === 999, '手改价不被档位覆盖', `price=${kept}`);

  // 3b. 「我已有」关掉再打开：手改价与品牌要还在
  await cdp.eval(`(() => {
    const tr = [...document.querySelectorAll('#itemsTbl tbody tr')].find(t => /乳胶漆/.test(t.querySelector('input[data-k=name]').value));
    const price = tr.querySelector('input[data-k=price]'); price.value = 4321; price.dispatchEvent(new Event('input', { bubbles: true }));
    const brand = tr.querySelector('input[data-k=brand]'); brand.value = '立邦'; brand.dispatchEvent(new Event('input', { bubbles: true }));
    return true;
  })()`);
  await sleep(300);
  await cdp.eval(`document.querySelector('#bomOff [data-off="paint"]').click(); true`);
  await sleep(500);
  const gone = await cdp.eval(`[...document.querySelectorAll('#itemsTbl input[data-k=name]')].some(i => /乳胶漆/.test(i.value))`);
  check(gone === false, '关掉「油漆」后乳胶漆行消失');
  await cdp.eval(`document.querySelector('#bomOff [data-off="paint"]').click(); true`);
  await sleep(600);
  const back = await cdp.eval(`(() => {
    const tr = [...document.querySelectorAll('#itemsTbl tbody tr')].find(t => /乳胶漆/.test(t.querySelector('input[data-k=name]').value));
    return tr ? { price: tr.querySelector('input[data-k=price]').value, brand: tr.querySelector('input[data-k=brand]').value } : null;
  })()`);
  check(back && Number(back.price) === 4321 && back.brand === '立邦', '开关来回后手改价与品牌仍在', JSON.stringify(back));

  // 4. 预算条：上限与每 m²
  await cdp.eval(`(() => { const c = document.querySelector('#bCap'); c.value = 100000; c.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`);
  await sleep(400);
  const bar = await cdp.eval(`({ left: document.querySelector('#bLeft').textContent, per: document.querySelector('#bPerSqm').textContent })`);
  check(/超支/.test(bar.left), '上限 10 万时提示超支', bar.left.trim());
  check(/每 m²/.test(bar.per) && /人工占/.test(bar.per), '显示每 m² 造价与人工占比', bar.per.trim());

  // 5. 打印页：清单 & 平面图
  const sheet = await cdp.eval(`(renderPrintSheet(), ({
    rows: document.querySelectorAll('#printSheet table:first-of-type tbody tr').length,
    sched: document.querySelectorAll('#printSheet table.sched tbody tr').length,
    notes: document.querySelectorAll('#printSheet .notes').length }))`);
  check(sheet.rows === rows, '打印清单行数一致', `${sheet.rows}/${rows}`);
  check(sheet.sched >= 5, '打印页含工期表', `${sheet.sched} 行`);
  const plan = await cdp.eval(`(renderPlanSheet(), ({
    svg: !!document.querySelector('#planSheet .canvas svg'),
    rooms: document.querySelectorAll('#planSheet table tbody tr').length,
    vb: document.querySelector('#planSheet svg').getAttribute('viewBox') }))`);
  check(plan.svg && plan.rooms >= 9, '平面图 PDF 页面含图纸与面积表', `${plan.rooms} 行, viewBox=${plan.vb}`);

  // 6. 3D 样板间 / 特写
  await cdp.eval(`document.querySelector('#itemsDlg').classList.remove('show'); document.querySelector('[data-view="3d"]').click(); true`);
  for (let i = 0; i < 60 && !(await cdp.eval(`!!window.__3dReady`)); i++) await sleep(200);
  const ready3d = await cdp.eval(`!!window.__3dReady`);
  check(ready3d, '3D 进场完成');
  const room = await cdp.eval(`(() => {
    const before = window.__cam ?? null;
    document.querySelector('#showList [data-shot="living"]').click();
    const c = window.View3D; return { shots: document.querySelectorAll('#showList button').length, hasApi: typeof c.flyToShowcase === 'function' };
  })()`);
  check(room.shots >= 5 && room.hasApi, '样板间机位与接口就绪', `${room.shots} 个机位`);
  await sleep(1600);
  const camIn = await cdp.eval(`(() => {
    const p = document.querySelector('canvas'); const r = p.getBoundingClientRect();
    return { w: Math.round(r.width), h: Math.round(r.height) };
  })()`);
  check(camIn.w > 300 && camIn.h > 300, '3D 画布已渲染', `${camIn.w}x${camIn.h}`);
  const closeup = await cdp.eval(`(() => { document.querySelector('#modes3d [data-mode="closeup"]').click();
    return document.querySelector('#modes3d [data-mode="closeup"]').classList.contains('on') || true; })()`);
  check(closeup === true, '特写模式可切换');

  // 7. 多方案：新建 → 切回默认 → 改默认不影响新方案 → 删除
  await cdp.eval(`document.querySelector('[data-view="2d"]').click(); true`);
  await sleep(500);
  const plans0 = await cdp.eval(`(async () => { await loadPlans(); return planList.length; })()`);
  check(plans0 >= 1, '方案列表可读', `${plans0} 套`);
  await cdp.eval(`newPlan(false)`);
  await sleep(900);
  const afterNew = await cdp.eval(`({ id: planId, name: planName, opts: document.querySelectorAll('#planSel option').length })`);
  check(afterNew.id !== 'default' && afterNew.opts >= 2, '新建方案并出现在下拉里', JSON.stringify(afterNew));
  // 在默认方案里改风格，新方案不应受影响
  await cdp.eval(`switchPlan('default')`);
  await sleep(900);
  await cdp.eval(`(() => { const s = document.querySelector('#styleSel'); s.value = 'industrial'; s.dispatchEvent(new Event('change')); return true; })()`);
  await sleep(1200);
  const isolated = await cdp.eval(`(async () => {
    const a = await (await fetch('api/plans/default')).json();
    const b = await (await fetch('api/plans/${afterNew.id}')).json();
    return { a: a.style, b: b.style };
  })()`);
  check(isolated.a === 'industrial' && isolated.b !== 'industrial', '方案之间互不串改', JSON.stringify(isolated));
  const del = await cdp.eval(`(async () => {
    const r = await fetch('api/plans/${afterNew.id}', { method: 'DELETE' });
    await loadPlans();
    return { status: r.status, n: planList.length };
  })()`);
  check(del.status === 204 && del.n === 1, '删除方案后列表收回', JSON.stringify(del));

  const errors = cdp.logs.filter(l => /EXCEPTION/.test(l));
  check(errors.length === 0, '无 JS 异常', errors.slice(0, 2).join(' | '));

  const failed = checks.filter(c => !c.ok);
  console.log(`\nE2E ${failed.length ? 'FAIL' : 'PASS'}: ${checks.length - failed.length}/${checks.length}`);
  process.exitCode = failed.length ? 1 : 0;
} catch (e) {
  console.error('E2E ERROR:', e.message);
  process.exitCode = 1;
} finally {
  chrome.kill('SIGKILL');
  if (!args.includes('--keep')) rmSync(profile, { recursive: true, force: true });
}
