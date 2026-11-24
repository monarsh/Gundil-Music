// Keeps the app shell available so the app opens instantly, and also with
// no connection to the server (then it plays downloaded songs, which the app
// keeps in IndexedDB). Browsers only run this on https or localhost; the
// Android app keeps its own copy of the shell.
// API calls, audio and artwork otherwise always go to the network.
const CACHE = 'gundil-shell-v16';
const SHELL = ['./', 'style.css', 'app.js', 'icon.svg', 'icon-192.png', 'manifest.webmanifest'];

self.addEventListener('install', e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(SHELL)).then(() => self.skipWaiting()));
});
self.addEventListener('activate', e => {
  e.waitUntil(caches.keys()
    .then(keys => Promise.all(keys.filter(k => k !== CACHE && !k.startsWith('gundil-downloads')).map(k => caches.delete(k))))
    .then(() => self.clients.claim()));
});

// A server that can't be reached can take long to fail; don't wait for it.
const within = (p, ms) => new Promise((ok, fail) => {
  const t = setTimeout(() => fail(new Error('timeout')), ms);
  p.then(v => { clearTimeout(t); ok(v); }, err => { clearTimeout(t); fail(err); });
});

self.addEventListener('fetch', e => {
  const url = new URL(e.request.url);
  if (e.request.method !== 'GET' || url.origin !== location.origin) return;
  if (url.pathname.includes('/api/') || url.pathname.startsWith('/_gundil')) return;
  const net = fetch(e.request).then(res => {
    if (res.ok) { const copy = res.clone(); caches.open(CACHE).then(c => c.put(e.request, copy)); }
    return res;
  });
  e.respondWith(within(net, 4000).catch(async () => (await caches.match(e.request, { ignoreSearch: true })) || net));
});
