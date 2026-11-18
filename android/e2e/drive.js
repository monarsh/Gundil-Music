// Drives the app's WebView over the Chrome DevTools protocol (adb-forwarded
// to localhost:9222): plays a song, then reports playback state on demand.
//   node drive.js play | node drive.js state | node drive.js check (exit 2 unless playing)
const mode = process.argv[2] || 'state';
(async () => {
  // The WebView's debugging target can take a while to appear after launch.
  let page, pages = [];
  for (let i = 0; i < 45 && !page; i++) {
    try { pages = await (await fetch('http://127.0.0.1:9222/json')).json(); } catch { pages = []; }
    page = pages.find(p => p.url.startsWith('http://10.0.2.2'));
    if (!page) await new Promise(r => setTimeout(r, 2000));
  }
  if (!page) throw new Error('no WebView page found; targets: ' + JSON.stringify(pages.map(p => p.url)));
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  let n = 0;
  const pending = new Map();
  ws.onmessage = e => { const m = JSON.parse(e.data); if (pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); } };
  await new Promise((ok, fail) => { ws.onopen = ok; ws.onerror = fail; });
  const ev = expr => new Promise(ok => {
    const id = ++n;
    pending.set(id, m => ok(m.result && m.result.result ? m.result.result.value : m));
    ws.send(JSON.stringify({ id, method: 'Runtime.evaluate', params: { expression: expr, awaitPromise: true, returnByValue: true } }));
  });
  const state = () => ev(`JSON.stringify({ title: (current() || {}).title, t: Math.round(audio.currentTime * 10) / 10, paused: audio.paused, index: S.index, queue: S.queue.length, visible: document.visibilityState, error: audio.error && audio.error.code })`);
  if (mode === 'play') {
    for (let i = 0; i < 30; i++) {
      if (await ev(`!!document.querySelector('.qp-grid .row')`)) break;
      await new Promise(r => setTimeout(r, 1000));
    }
    console.log('page:', page.url);
    await ev(`document.querySelector('.qp-grid .row').click(); true`);
    await new Promise(r => setTimeout(r, 8000));
  }
  if (mode === 'check') {
    // Is music still playing? The position moves on (or the next song started).
    const a = JSON.parse(await state());
    await new Promise(r => setTimeout(r, 5000));
    const b = JSON.parse(await state());
    console.log('check', JSON.stringify(a), '->', JSON.stringify(b));
    ws.close();
    if (b.paused || !(b.t > a.t || b.index !== a.index)) { console.error('not playing'); process.exit(2); }
    return;
  }
  console.log(mode, await state());
  ws.close();
})().catch(e => { console.error('drive failed:', e.message); process.exit(1); });
