'use strict';

/* ---------- helpers ---------- */
const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const enc = encodeURIComponent;

const ICONS = {
  play: 'M8 5v14l11-7z',
  pause: 'M6 19h4V5H6zm8-14v14h4V5z',
  next: 'M6 18l8.5-6L6 6zM16 6v12h2V6z',
  prev: 'M6 6h2v12H6zm3.5 6l8.5 6V6z',
  home: 'M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z',
  library: 'M4 6H2v14a2 2 0 002 2h14v-2H4zm16-4H8a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V4a2 2 0 00-2-2zm-2 5h-3v5.5a2.5 2.5 0 11-2.5-2.5c.57 0 1.08.19 1.5.51V5h4z',
  heart: 'M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54z',
  heartOutline: 'M16.5 3c-1.74 0-3.41.81-4.5 2.09C10.91 3.81 9.24 3 7.5 3 4.42 3 2 5.42 2 8.5c0 3.78 3.4 6.86 8.55 11.54L12 21.35l1.45-1.32C18.6 15.36 22 12.28 22 8.5 22 5.42 19.58 3 16.5 3zm-4.4 15.55l-.1.1-.1-.1C7.14 14.24 4 11.39 4 8.5 4 6.5 5.5 5 7.5 5c1.54 0 3.04.99 3.57 2.36h1.87C13.46 5.99 14.96 5 16.5 5c2 0 3.5 1.5 3.5 3.5 0 2.89-3.14 5.74-7.9 10.05z',
  search: 'M15.5 14h-.79l-.28-.27A6.47 6.47 0 0016 9.5 6.5 6.5 0 109.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z',
  more: 'M12 8a2 2 0 100-4 2 2 0 000 4zm0 2a2 2 0 100 4 2 2 0 000-4zm0 6a2 2 0 100 4 2 2 0 000-4z',
  shuffle: 'M10.59 9.17L5.41 4 4 5.41l5.17 5.17zM14.5 4l2.04 2.04L4 18.59 5.41 20 17.96 7.46 20 9.5V4zm.33 9.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04z',
  repeat: 'M7 7h10v3l4-4-4-4v3H5v6h2zm10 10H7v-3l-4 4 4 4v-3h12v-6h-2z',
  repeatOne: 'M7 7h10v3l4-4-4-4v3H5v6h2zm10 10H7v-3l-4 4 4 4v-3h12v-6h-2zm-4-2V9h-1l-2 1v1h1.5v4z',
  volume: 'M3 9v6h4l5 5V4L7 9zm13.5 3A4.5 4.5 0 0014 7.97v8.05c1.48-.73 2.5-2.25 2.5-4.02z',
  speaker: 'M17 2H7a2 2 0 00-2 2v16a2 2 0 002 2h10a2 2 0 002-2V4a2 2 0 00-2-2zm-5 2a2 2 0 110 4 2 2 0 010-4zm0 16a5 5 0 110-10 5 5 0 010 10zm0-8a3 3 0 100 6 3 3 0 000-6z',
  bluetooth: 'M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29zM13 5.83l1.88 1.88L13 9.59zm1.88 10.46L13 18.17v-3.76z',
  plus: 'M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6z',
  close: 'M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z',
  down: 'M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6z',
  queue: 'M15 6H3v2h12zm0 4H3v2h12zM3 16h8v-2H3zM17 6v8.18c-.31-.11-.65-.18-1-.18a3 3 0 103 3V8h3V6z',
  refresh: 'M17.65 6.35A7.95 7.95 0 0012 4a8 8 0 108 8h-2a6 6 0 11-6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4z',
};
const icon = name => `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="${ICONS[name]}"/></svg>`;

function fmt(s) {
  if (!s || !isFinite(s)) return '';
  s = Math.round(s);
  const m = Math.floor(s / 60), h = Math.floor(m / 60), ss = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m % 60).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}
const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

function shuffled(list) {
  const a = list.slice();
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

async function api(path, method = 'GET', body) {
  const res = await fetch('/api/' + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

let toastTimer;
function toast(msg) {
  const el = $('#toast');
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.hidden = true; }, 3500);
}

const store = {
  get(key, fallback) {
    try { const v = localStorage.getItem('gundil.' + key); return v === null ? fallback : JSON.parse(v); } catch { return fallback; }
  },
  set(key, value) {
    try { localStorage.setItem('gundil.' + key, JSON.stringify(value)); } catch { /* private mode */ }
  },
};

/* ---------- state ---------- */
const S = {
  info: {}, tracks: [], byId: new Map(), albums: [], albumByKey: new Map(), artists: [], artistByName: new Map(),
  scanning: false,
  user: { playlists: [], liked: [], plays: {} }, liked: new Set(),
  output: store.get('output', null), // 'local' | 'server'
  server: null,                      // last player state from the server
  queue: [], index: -1, original: null, shuffle: store.get('shuffle', false),
  repeat: store.get('repeat', 'off'),
  playing: false, paused: true, position: 0, duration: 0, volume: store.get('volume', 80),
  lists: {}, listOpts: {},
};
const audio = new Audio();
audio.preload = 'auto';

const isServer = () => S.output === 'server';
const current = () => S.byId.get(S.queue[S.index]);
const albumKey = t => (t.albumArtist || t.artist) + '\u0001' + t.album;

/* ---------- library ---------- */
function buildLibrary(tracks) {
  S.tracks = tracks;
  S.byId = new Map(tracks.map(t => [t.id, t]));
  S.albumByKey = new Map();
  S.artistByName = new Map();
  const artist = name => {
    let a = S.artistByName.get(name);
    if (!a) S.artistByName.set(name, a = { name, tracks: [], albums: [] });
    return a;
  };
  for (const t of tracks) {
    const key = albumKey(t);
    let al = S.albumByKey.get(key);
    if (!al) {
      al = { key, name: t.album, artist: t.albumArtist || t.artist, year: 0, tracks: [], artId: null, added: 0 };
      S.albumByKey.set(key, al);
      artist(al.artist).albums.push(al);
    }
    al.tracks.push(t);
    if (t.year && !al.year) al.year = t.year;
    if (t.hasArt && !al.artId) al.artId = t.id;
    if (t.added > al.added) al.added = t.added;
    artist(t.artist).tracks.push(t);
    if (t.albumArtist && t.albumArtist !== t.artist) artist(t.albumArtist).tracks.push(t);
  }
  const byName = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
  S.albums = [...S.albumByKey.values()].sort(byName);
  S.artists = [...S.artistByName.values()].sort(byName);
  for (const a of S.artists) a.artId = (a.albums.find(x => x.artId) || {}).artId || (a.tracks.find(x => x.hasArt) || {}).id || null;
}

async function loadLibrary() {
  const lib = await api('library');
  S.scanning = lib.scanning;
  buildLibrary(lib.tracks || []);
}

function setUser(u) {
  if (!u) return;
  S.user = u;
  S.liked = new Set(u.liked);
  renderSidebar();
}

/* ---------- artwork ---------- */
function hue(str) {
  let h = 0;
  for (let i = 0; i < str.length; i++) h = (h * 31 + str.charCodeAt(i)) % 360;
  return h;
}
function artInner(artId, label) {
  if (artId) return `<img src="/api/art/${artId}" alt="" loading="lazy">`;
  const text = String(label || '?').trim();
  return `<div class="ph" style="--h:${hue(text)}">${esc((text[0] || '?').toUpperCase())}</div>`;
}
const trackArt = t => artInner(t.hasArt ? t.id : null, t.album);

/* ---------- playback ---------- */
async function srv(action, body) {
  try {
    applyServer(await api('player/' + action, 'POST', body || {}));
    return true;
  } catch (e) {
    toast(e.message);
    return false;
  }
}

function applyServer(st) {
  S.server = st;
  if (!isServer()) { renderOutput(); return; }
  S.queue = st.queue || [];
  S.index = st.index;
  S.playing = st.playing;
  S.paused = st.paused || !st.playing;
  S.position = st.position || 0;
  S.duration = st.duration || 0;
  S.volume = st.volume;
  S.repeat = st.repeat;
  renderPlayer();
}

function saveLocalQueue() {
  if (!isServer()) store.set('queue', { ids: S.queue.slice(0, 2000), index: S.index });
}

function loadLocal(autoplay, at = 0) {
  const t = current();
  if (!t) return;
  audio.src = '/api/stream/' + t.id;
  if (at > 0) audio.addEventListener('loadedmetadata', () => { audio.currentTime = at; }, { once: true });
  S.playing = true;
  S.paused = !autoplay;
  S.position = at;
  S.duration = t.duration || 0;
  if (autoplay) {
    audio.play().catch(() => { S.paused = true; renderPlayer(); });
    api('played/' + t.id, 'POST').then(setUser).catch(() => {});
  }
  saveLocalQueue();
  renderPlayer();
}

async function playQueue(ids, index = 0) {
  ids = ids.filter(id => S.byId.has(id));
  if (!ids.length) return;
  S.original = null;
  if (S.shuffle) {
    S.original = ids.slice();
    const first = ids[index];
    ids = [first, ...shuffled(ids.filter((_, i) => i !== index))];
    index = 0;
  }
  S.queue = ids;
  S.index = index;
  if (isServer()) await srv('queue', { ids, index, play: true });
  else loadLocal(true);
}

function playShuffled(ids) {
  S.shuffle = true;
  store.set('shuffle', true);
  playQueue(ids, Math.floor(Math.random() * ids.length));
}

function syncQueue() {
  if (isServer()) return srv('queue', { ids: S.queue, index: S.index, play: false });
  saveLocalQueue();
  renderPlayer();
}

function toggle() {
  if (!S.queue.length) return;
  if (isServer()) return srv('toggle');
  if (!audio.src) return loadLocal(true);
  if (audio.paused) audio.play().catch(() => {}); else audio.pause();
}

function next() {
  if (!S.queue.length) return;
  if (isServer()) return srv('next');
  S.index = (S.index + 1) % S.queue.length;
  loadLocal(true);
}

function prev() {
  if (!S.queue.length) return;
  if (isServer()) return srv('prev');
  if (audio.currentTime > 3 || S.index <= 0) { audio.currentTime = 0; return; }
  S.index--;
  loadLocal(true);
}

function jump(i) {
  if (i < 0 || i >= S.queue.length) return;
  if (isServer()) return srv('jump', { index: i });
  S.index = i;
  loadLocal(true);
}

function seek(seconds) {
  S.position = seconds;
  if (isServer()) return srv('seek', { position: seconds });
  if (audio.src) audio.currentTime = seconds;
}

function setVolume(v) {
  S.volume = v;
  if (isServer()) return srv('volume', { volume: v });
  audio.volume = v / 100;
  store.set('volume', v);
}

function cycleRepeat() {
  S.repeat = { off: 'all', all: 'one', one: 'off' }[S.repeat] || 'off';
  store.set('repeat', S.repeat);
  if (isServer()) return srv('repeat', { mode: S.repeat });
  renderPlayer();
}

function toggleShuffle() {
  S.shuffle = !S.shuffle;
  store.set('shuffle', S.shuffle);
  const cur = S.queue[S.index];
  if (S.shuffle && S.queue.length) {
    S.original = S.queue.slice();
    S.queue = [cur, ...shuffled(S.queue.filter((_, i) => i !== S.index))];
    S.index = 0;
  } else if (!S.shuffle && S.original && S.original.includes(cur)) {
    S.queue = S.original;
    S.index = S.queue.indexOf(cur);
    S.original = null;
  }
  if (S.queue.length) syncQueue(); else renderPlayer();
}

function enqueue(ids, asNext) {
  ids = ids.filter(id => S.byId.has(id));
  if (!ids.length) return;
  if (!S.queue.length) return playQueue(ids, 0);
  if (asNext) S.queue.splice(S.index + 1, 0, ...ids); else S.queue.push(...ids);
  if (S.original) S.original.push(...ids);
  syncQueue();
  toast(asNext ? 'Playing next' : 'Added to queue');
}

function removeFromQueue(i) {
  if (i === S.index || i < 0 || i >= S.queue.length) return;
  S.queue.splice(i, 1);
  if (i < S.index) S.index--;
  syncQueue();
}

async function setOutput(out) {
  if (out === S.output) return;
  if (out === 'server' && !S.info.player) return;
  const ids = S.queue.slice(), index = S.index, pos = S.position;
  const wasPlaying = S.playing && !S.paused;
  S.output = out;
  store.set('output', out);
  if (out === 'server') {
    audio.pause();
    if (ids.length) {
      await srv('queue', { ids, index, play: wasPlaying });
      if (wasPlaying && pos > 2) {
        // mpv can only seek once the file is open.
        for (let i = 0; i < 30 && !(S.server && S.server.duration > 0); i++) await new Promise(r => setTimeout(r, 100));
        await srv('seek', { position: pos });
      }
      await srv('repeat', { mode: S.repeat });
    } else if (S.server) {
      applyServer(S.server);
    }
  } else {
    if (S.server && S.server.playing) await api('player/stop', 'POST', {}).catch(() => {});
    S.queue = ids; S.index = index;
    S.volume = store.get('volume', 80);
    audio.volume = S.volume / 100;
    S.playing = false; S.paused = true;
    if (ids.length) loadLocal(wasPlaying, pos);
  }
  renderOutput();
  renderPlayer();
}

audio.addEventListener('timeupdate', () => { if (!isServer()) { S.position = audio.currentTime; renderProgress(); } });
audio.addEventListener('play', () => { if (!isServer()) { S.paused = false; renderPlayer(); } });
audio.addEventListener('pause', () => { if (!isServer()) { S.paused = true; renderPlayer(); } });
audio.addEventListener('loadedmetadata', () => {
  if (isServer() || !isFinite(audio.duration)) return;
  S.duration = audio.duration;
  const t = current();
  if (t && !t.duration) {
    t.duration = audio.duration;
    api(`tracks/${t.id}/duration`, 'POST', { duration: audio.duration }).catch(() => {});
  }
  renderProgress();
});
audio.addEventListener('ended', () => {
  if (isServer()) return;
  if (S.repeat === 'one') { audio.currentTime = 0; audio.play(); return; }
  if (S.index + 1 < S.queue.length) { S.index++; loadLocal(true); }
  else if (S.repeat === 'all') { S.index = 0; loadLocal(true); }
  else { S.paused = true; renderPlayer(); }
});
audio.addEventListener('error', () => {
  if (isServer() || !audio.src) return;
  toast('Could not play this track');
});

/* Lock-screen / headset-button controls (this is what Bluetooth headphone buttons talk to). */
function updateMediaSession(t) {
  if (!('mediaSession' in navigator)) return;
  if (!t || isServer()) { navigator.mediaSession.metadata = null; return; }
  navigator.mediaSession.metadata = new MediaMetadata({
    title: t.title, artist: t.artist, album: t.album,
    artwork: t.hasArt ? [{ src: `/api/art/${t.id}`, sizes: '512x512' }] : [{ src: 'icon-512.png', sizes: '512x512', type: 'image/png' }],
  });
}
if ('mediaSession' in navigator) {
  const handlers = {
    play: toggle, pause: toggle, previoustrack: prev, nexttrack: next,
    seekto: e => seek(e.seekTime),
  };
  for (const [name, fn] of Object.entries(handlers)) {
    try { navigator.mediaSession.setActionHandler(name, fn); } catch { /* unsupported action */ }
  }
}

/* ---------- player rendering ---------- */
let shownTrack = null, seeking = false;

function renderProgress() {
  const dur = S.duration || (current() || {}).duration || 0;
  const value = dur ? Math.round(S.position / dur * 1000) : 0;
  if (!seeking) { $('#seek').value = value; $('#now-seek').value = value; }
  const pct = (dur ? Math.min(100, S.position / dur * 100) : 0) + '%';
  $('#seek').style.setProperty('--fill', pct);
  $('#now-seek').style.setProperty('--fill', pct);
  $('#p-time').textContent = dur ? `${fmt(S.position) || '0:00'} / ${fmt(dur)}` : '';
  $('#now-pos').textContent = fmt(S.position) || '0:00';
  $('#now-dur').textContent = fmt(dur) || '0:00';
}

function renderPlayer() {
  const t = current();
  const id = t ? t.id : null;
  $('#player').classList.toggle('empty', !t);
  if (id !== shownTrack) {
    shownTrack = id;
    $('#p-art').innerHTML = t ? trackArt(t) : '';
    $('#p-title').textContent = t ? t.title : 'Nothing playing';
    $('#p-sub').textContent = t ? `${t.artist} · ${t.album}` : 'Pick a song to start';
    $('#now-art').innerHTML = t ? trackArt(t) : '';
    $('#now-title').textContent = t ? t.title : '';
    $('#now-sub').innerHTML = t ? `<a href="#/artist/${enc(t.artist)}" data-act="close-now">${esc(t.artist)}</a> · <a href="#/album/${enc(albumKey(t))}" data-act="close-now">${esc(t.album)}</a>` : '';
    document.title = t ? `${t.title} · ${t.artist}` : 'Gundil Music';
    updateMediaSession(t);
    markPlaying();
    if (!t) closeNow();
  }
  const playing = t && S.playing && !S.paused;
  for (const el of [$('#p-play'), $('#now-play')]) {
    el.innerHTML = icon(playing ? 'pause' : 'play');
    el.setAttribute('aria-label', playing ? 'Pause' : 'Play');
  }
  if ('mediaSession' in navigator && !isServer()) navigator.mediaSession.playbackState = playing ? 'playing' : 'paused';
  for (const el of [$('#p-shuffle'), $('#now-shuffle')]) el.classList.toggle('on', S.shuffle);
  for (const el of [$('#p-repeat'), $('#now-repeat')]) {
    el.classList.toggle('on', S.repeat !== 'off');
    el.innerHTML = icon(S.repeat === 'one' ? 'repeatOne' : 'repeat');
    el.title = { off: 'Repeat is off', all: 'Repeating the queue', one: 'Repeating this song' }[S.repeat];
  }
  const liked = t && S.liked.has(t.id);
  for (const el of [$('#p-like'), $('#now-like')]) {
    el.classList.toggle('on', !!liked);
    el.innerHTML = icon(liked ? 'heart' : 'heartOutline');
  }
  if (document.activeElement !== $('#volume')) $('#volume').value = S.volume;
  renderProgress();
  if (!$('#now').hidden) renderQueue();
}

function markPlaying() {
  const id = shownTrack;
  for (const el of $$('.row.is-playing')) el.classList.remove('is-playing');
  if (id) for (const el of $$(`.row[data-id="${id}"]`)) el.classList.add('is-playing');
}

function renderQueue() {
  const el = $('#queue');
  const sig = S.queue.length + ':' + S.index + ':' + S.queue[S.index] + ':' + S.queue[S.queue.length - 1];
  if (el.dataset.sig === sig) return;
  el.dataset.sig = sig;
  const start = Math.max(0, S.index - 2), end = Math.min(S.queue.length, S.index + 100);
  let html = '';
  for (let i = start; i < end; i++) {
    const t = S.byId.get(S.queue[i]);
    if (!t) continue;
    html += `<div class="row q-row${i === S.index ? ' is-playing' : ''}${i < S.index ? ' past' : ''}" data-q="${i}">
      <div class="row-art art">${trackArt(t)}<span class="row-play">${icon('play')}</span></div>
      <div class="row-main"><div class="row-title">${esc(t.title)}</div><div class="row-sub">${esc(t.artist)}</div></div>
      <span class="row-dur">${fmt(t.duration)}</span>
      ${i === S.index ? '<span class="icon-btn"></span>' : `<button class="icon-btn" data-act="q-remove" aria-label="Remove from queue">${icon('close')}</button>`}
    </div>`;
  }
  if (end < S.queue.length) html += `<p class="muted pad">and ${S.queue.length - end} more</p>`;
  el.innerHTML = html || '<p class="muted pad">The queue is empty.</p>';
}

function openNow() {
  if (!current()) return;
  $('#now').hidden = false;
  $('#queue').dataset.sig = '';
  renderQueue();
  renderOutput();
}
function closeNow() { $('#now').hidden = true; }

function renderOutput() {
  const host = S.info.hostname || 'server';
  const label = isServer() ? host : 'This device';
  $('#output-label').textContent = label;
  $('#output-btn').classList.toggle('on', isServer());
  const elsewhere = !isServer() && S.server && S.server.playing && !S.server.paused;
  $('#output-btn').classList.toggle('dot', !!elsewhere);
  $('#now-output').textContent = isServer() ? `Playing on ${host}` : 'Playing on this device';
}

/* ---------- lists ---------- */
const CHUNK = 150;

function rowHTML(t, list, i, opts) {
  const liked = S.liked.has(t.id);
  const lead = opts.numbered
    ? `<div class="row-art num"><span class="n">${t.track || i + 1}</span><span class="row-play">${icon('play')}</span></div>`
    : `<div class="row-art art">${trackArt(t)}<span class="row-play">${icon('play')}</span></div>`;
  return `<div class="row${t.id === shownTrack ? ' is-playing' : ''}" data-id="${t.id}" data-list="${list}" data-idx="${i}">
    ${lead}
    <div class="row-main"><div class="row-title">${esc(t.title)}</div><div class="row-sub">${esc(t.artist)}${opts.numbered ? '' : ' · ' + esc(t.album)}</div></div>
    ${opts.numbered ? '' : `<a class="row-album" href="#/album/${enc(albumKey(t))}">${esc(t.album)}</a>`}
    <button class="icon-btn like${liked ? ' on' : ''}" data-act="like" aria-label="${liked ? 'Unlike' : 'Like'}">${icon(liked ? 'heart' : 'heartOutline')}</button>
    <span class="row-dur">${fmt(t.duration)}</span>
    <button class="icon-btn" data-act="menu" aria-label="More actions">${icon('more')}</button>
  </div>`;
}

function listHTML(name, tracks, opts = {}) {
  S.lists[name] = tracks.map(t => t.id);
  S.listOpts[name] = opts;
  if (!tracks.length) return `<p class="muted pad">${esc(opts.empty || 'Nothing here yet.')}</p>`;
  let html = `<div class="list${opts.grid ? ' grid-list' : ''}" data-name="${name}">`;
  const n = Math.min(tracks.length, CHUNK);
  for (let i = 0; i < n; i++) html += rowHTML(tracks[i], name, i, opts);
  if (tracks.length > n) html += `<div class="sentinel" data-from="${n}"></div>`;
  return html + '</div>';
}

const sentinelObserver = new IntersectionObserver(entries => {
  for (const e of entries) {
    if (!e.isIntersecting) continue;
    const s = e.target, listEl = s.parentElement, name = listEl.dataset.name;
    const ids = S.lists[name] || [], from = +s.dataset.from, to = Math.min(ids.length, from + CHUNK);
    let html = '';
    for (let i = from; i < to; i++) {
      const t = S.byId.get(ids[i]);
      if (t) html += rowHTML(t, name, i, S.listOpts[name] || {});
    }
    sentinelObserver.unobserve(s);
    s.insertAdjacentHTML('beforebegin', html);
    if (to < ids.length) { s.dataset.from = to; sentinelObserver.observe(s); } else s.remove();
  }
}, { rootMargin: '600px' });

function albumCard(al) {
  return `<a class="card" href="#/album/${enc(al.key)}">
    <div class="art">${artInner(al.artId, al.name)}<button class="card-play" data-act="play-album" data-key="${esc(al.key)}" aria-label="Play ${esc(al.name)}">${icon('play')}</button></div>
    <div class="card-title">${esc(al.name)}</div>
    <div class="card-sub">${esc(al.artist)}${al.year ? ' · ' + al.year : ''}</div>
  </a>`;
}
function artistCard(a) {
  return `<a class="card round" href="#/artist/${enc(a.name)}">
    <div class="art">${artInner(a.artId, a.name)}</div>
    <div class="card-title">${esc(a.name)}</div>
    <div class="card-sub">${plural(a.tracks.length, 'song')}</div>
  </a>`;
}
const shelf = (title, cards, wrap) => cards.length
  ? `<section><h2>${esc(title)}</h2><div class="${wrap ? 'cards' : 'shelf'}">${cards.join('')}</div></section>` : '';

/* ---------- views ---------- */
function emptyLibrary() {
  const folders = (S.info.folders || []).map(f => `<code>${esc(f)}</code>`).join(', ') || 'the music folder';
  return `<div class="empty-state">
    <img src="icon.svg" alt="" width="72" height="72">
    <h1>${S.scanning ? 'Scanning your music…' : 'No music yet'}</h1>
    <p class="muted">${S.scanning ? 'This page fills in as soon as the scan finishes.' : `Copy audio files into ${folders} on <b>${esc(S.info.hostname || 'the server')}</b>, then rescan.`}</p>
    ${S.scanning ? '' : `<button class="btn primary" data-act="rescan">${icon('refresh')}Rescan</button>`}
  </div>`;
}

function viewHome() {
  if (!S.tracks.length) return emptyLibrary();
  const plays = S.user.plays || {};
  const played = S.tracks.filter(t => plays[t.id]);
  let picks = played.slice().sort((a, b) => plays[b.id].count - plays[a.id].count).slice(0, 12);
  if (picks.length < 12) {
    const have = new Set(picks.map(t => t.id));
    picks = picks.concat(shuffled(S.tracks.filter(t => !have.has(t.id))).slice(0, 12 - picks.length));
  }
  const recentAlbums = [];
  for (const t of played.slice().sort((a, b) => plays[b.id].last - plays[a.id].last)) {
    const al = S.albumByKey.get(albumKey(t));
    if (al && !recentAlbums.includes(al)) recentAlbums.push(al);
    if (recentAlbums.length >= 12) break;
  }
  const added = S.albums.slice().sort((a, b) => b.added - a.added).slice(0, 12);
  const artists = S.artists.slice().sort((a, b) => b.tracks.length - a.tracks.length).slice(0, 12);
  const hour = new Date().getHours();
  const hello = hour < 5 ? 'Late night listening' : hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
  return `<div class="page-head"><h1>${hello}</h1>
      <button class="btn" data-act="shuffle-all">${icon('shuffle')}Shuffle all</button></div>
    <section><h2>${played.length ? 'Quick picks' : 'Start with these'}</h2>${listHTML('picks', picks, { grid: true })}</section>
    ${shelf('Recently played', recentAlbums.map(albumCard))}
    ${shelf('Recently added', added.map(albumCard))}
    ${shelf('Artists', artists.map(artistCard))}`;
}

function viewLibrary(tab) {
  if (!S.tracks.length) return emptyLibrary();
  const tabs = [['songs', 'Songs'], ['albums', 'Albums'], ['artists', 'Artists'], ['playlists', 'Playlists']];
  if (!tabs.some(x => x[0] === tab)) tab = 'songs';
  let body;
  if (tab === 'albums') body = `<div class="cards">${S.albums.map(albumCard).join('')}</div>`;
  else if (tab === 'artists') body = `<div class="cards">${S.artists.map(artistCard).join('')}</div>`;
  else if (tab === 'playlists') body = `<div class="cards">${playlistCard({ id: 'liked', name: 'Liked songs', tracks: S.user.liked }, '#/liked')}${S.user.playlists.map(p => playlistCard(p)).join('')}</div>
      <p><button class="btn" data-act="new-playlist">${icon('plus')}New playlist</button></p>`;
  else {
    const songs = S.tracks.slice().sort((a, b) => a.title.localeCompare(b.title, undefined, { sensitivity: 'base' }));
    body = listHTML('songs', songs);
  }
  return `<div class="page-head"><h1>Library</h1>
      <span class="muted">${plural(S.tracks.length, 'song')}</span>
      <button class="btn" data-act="rescan" ${S.scanning ? 'disabled' : ''}>${icon('refresh')}${S.scanning ? 'Scanning…' : 'Rescan'}</button></div>
    <div class="chips">${tabs.map(([k, label]) => `<a class="chip${k === tab ? ' on' : ''}" href="#/library/${k}">${label}</a>`).join('')}</div>
    ${body}`;
}

function playlistCard(p, href) {
  const first = p.tracks.map(id => S.byId.get(id)).find(t => t && t.hasArt);
  return `<a class="card" href="${href || '#/playlist/' + p.id}">
    <div class="art">${artInner(first ? first.id : null, p.name)}</div>
    <div class="card-title">${esc(p.name)}</div>
    <div class="card-sub">${plural(p.tracks.filter(id => S.byId.has(id)).length, 'song')}</div>
  </a>`;
}

function hero({ art, round, kind, title, sub, list, extra = '' }) {
  return `<div class="hero">
    <div class="art hero-art${round ? ' round' : ''}">${art}</div>
    <div class="hero-text">
      <div class="kind">${esc(kind)}</div>
      <h1>${esc(title)}</h1>
      <p class="muted">${sub}</p>
      <div class="hero-actions">
        <button class="btn primary" data-act="play-list" data-list="${list}">${icon('play')}Play</button>
        <button class="btn" data-act="shuffle-list" data-list="${list}">${icon('shuffle')}Shuffle</button>
        ${extra}
      </div>
    </div>
  </div>`;
}
const totalTime = tracks => {
  const s = tracks.reduce((n, t) => n + (t.duration || 0), 0);
  return s ? ' · ' + (s >= 3600 ? `${Math.floor(s / 3600)} hr ${Math.round(s % 3600 / 60)} min` : `${Math.max(1, Math.round(s / 60))} min`) : '';
};
const notFound = what => `<div class="empty-state"><h1>${what} not found</h1><p><a class="btn" href="#/">Back to home</a></p></div>`;

function viewAlbum(key) {
  const al = S.albumByKey.get(key);
  if (!al) return notFound('Album');
  const tracks = al.tracks.slice().sort((a, b) => (a.disc || 1) - (b.disc || 1) || (a.track || 0) - (b.track || 0));
  return hero({
    art: artInner(al.artId, al.name), kind: 'Album', title: al.name, list: 'album',
    sub: `<a href="#/artist/${enc(al.artist)}">${esc(al.artist)}</a>${al.year ? ' · ' + al.year : ''} · ${plural(tracks.length, 'song')}${totalTime(tracks)}`,
  }) + listHTML('album', tracks, { numbered: true });
}

function viewArtist(name) {
  const a = S.artistByName.get(name);
  if (!a) return notFound('Artist');
  return hero({
    art: artInner(a.artId, a.name), round: true, kind: 'Artist', title: a.name, list: 'artist',
    sub: `${plural(a.tracks.length, 'song')}${a.albums.length ? ' · ' + plural(a.albums.length, 'album') : ''}`,
  }) + shelf('Albums', a.albums.map(albumCard)) + `<section><h2>Songs</h2>${listHTML('artist', a.tracks)}</section>`;
}

function viewPlaylist(id) {
  const liked = id === 'liked';
  const p = liked ? { id, name: 'Liked songs', tracks: S.user.liked } : S.user.playlists.find(x => x.id === id);
  if (!p) return notFound('Playlist');
  const tracks = p.tracks.map(x => S.byId.get(x)).filter(Boolean);
  const first = tracks.find(t => t.hasArt);
  return hero({
    art: artInner(first ? first.id : null, p.name), kind: 'Playlist', title: p.name, list: 'playlist',
    sub: `${plural(tracks.length, 'song')}${totalTime(tracks)}`,
    extra: liked ? '' : `<button class="btn" data-act="rename-playlist" data-id="${p.id}">Rename</button>
      <button class="btn" data-act="delete-playlist" data-id="${p.id}">Delete</button>`,
  }) + listHTML('playlist', tracks, {
    playlist: liked ? null : p.id,
    empty: liked ? 'Songs you like show up here. Tap the heart on any song.' : 'This playlist is empty. Use the ⋯ menu on a song to add it.',
  });
}

function viewSearch(q) {
  const terms = q.toLowerCase().split(/\s+/).filter(Boolean);
  if (!terms.length) return `<div class="empty-state"><h1>Search your music</h1><p class="muted">Find songs, albums and artists.</p></div>`;
  const hit = text => { const s = text.toLowerCase(); return terms.every(w => s.includes(w)); };
  const artists = S.artists.filter(a => hit(a.name)).slice(0, 12);
  const albums = S.albums.filter(a => hit(a.name + ' ' + a.artist)).slice(0, 12);
  const songs = S.tracks.filter(t => hit(`${t.title} ${t.artist} ${t.album}`));
  // Title matches first.
  songs.sort((a, b) => hit(b.title) - hit(a.title));
  if (!artists.length && !albums.length && !songs.length) {
    return `<div class="empty-state"><h1>No results for “${esc(q)}”</h1><p class="muted">Check the spelling or try fewer words.</p></div>`;
  }
  return `<div class="page-head"><h1>Results for “${esc(q)}”</h1></div>
    ${shelf('Artists', artists.map(artistCard))}
    ${shelf('Albums', albums.map(albumCard))}
    ${songs.length ? `<section><h2>Songs</h2>${listHTML('search', songs.slice(0, 300))}</section>` : ''}`;
}

const NAV = [['#/', 'home', 'Home'], ['#/library/songs', 'library', 'Library'], ['#/liked', 'heart', 'Liked']];

function route() {
  const parts = location.hash.replace(/^#\/?/, '').split('/');
  const page = parts[0] || 'home';
  const arg = decodeURIComponent(parts.slice(1).join('/'));
  S.lists = {}; S.listOpts = {};
  let html;
  switch (page) {
    case 'library': html = viewLibrary(arg); break;
    case 'album': html = viewAlbum(arg); break;
    case 'artist': html = viewArtist(arg); break;
    case 'playlist': html = viewPlaylist(arg); break;
    case 'liked': html = viewPlaylist('liked'); break;
    case 'search': html = viewSearch(arg); break;
    default: html = viewHome();
  }
  const view = $('#view');
  view.innerHTML = html;
  for (const s of $$('.sentinel', view)) sentinelObserver.observe(s);
  if (S.routeKey !== location.hash) { view.scrollTop = 0; S.routeKey = location.hash; }
  if (page === 'search') { if (document.activeElement !== $('#search')) $('#search').value = arg; }
  else $('#search').value = '';
  const active = page === 'home' ? '#/' : page === 'library' ? '#/library/songs' : page === 'liked' ? '#/liked' : '';
  const links = NAV.map(([href, ic, label]) => `<a href="${href}" class="${href === active ? 'on' : ''}">${icon(ic)}<span>${label}</span></a>`).join('');
  $('#nav').innerHTML = links;
  $('#tabbar').innerHTML = links;
  renderSidebar();
}

function renderSidebar() {
  const here = location.hash;
  $('#side-playlists').innerHTML = S.user.playlists.map(p =>
    `<a href="#/playlist/${p.id}" class="${here === '#/playlist/' + p.id ? 'on' : ''}">${esc(p.name)}</a>`).join('')
    || '<p class="muted small">No playlists yet.</p>';
}

/* ---------- menu, dialogs ---------- */
let menuCtx = null;

function showMenu(items, anchor) {
  const m = $('#menu');
  m.innerHTML = items.map((it, i) => it === '-' ? '<hr>' : `<button data-i="${i}">${esc(it.label)}</button>`).join('');
  m.items = items;
  m.hidden = false;
  if (window.innerWidth > 700 && anchor) {
    const r = anchor.getBoundingClientRect();
    const w = m.offsetWidth, h = m.offsetHeight;
    m.style.left = Math.max(8, Math.min(r.right - w, window.innerWidth - w - 8)) + 'px';
    m.style.top = (r.bottom + h + 8 > window.innerHeight ? Math.max(8, r.top - h) : r.bottom) + 'px';
  } else {
    m.style.left = m.style.top = '';
  }
}
function hideMenu() { $('#menu').hidden = true; menuCtx = null; }

function trackMenu(t, anchor, playlistId, idx) {
  const items = [
    { label: 'Play next', run: () => enqueue([t.id], true) },
    { label: 'Add to queue', run: () => enqueue([t.id], false) },
    { label: 'Add to playlist…', keep: true, run: () => playlistMenu(t, anchor) },
    '-',
    { label: 'Go to album', run: () => { location.hash = '#/album/' + enc(albumKey(t)); } },
    { label: 'Go to artist', run: () => { location.hash = '#/artist/' + enc(t.artist); } },
  ];
  if (playlistId) {
    items.push('-', {
      label: 'Remove from this playlist', run: async () => {
        const p = S.user.playlists.find(x => x.id === playlistId);
        if (!p) return;
        // idx is the position among visible rows; map it back through missing tracks.
        const visible = p.tracks.map((id, i) => [id, i]).filter(([id]) => S.byId.has(id));
        const real = visible[idx] ? visible[idx][1] : p.tracks.indexOf(t.id);
        const tracks = p.tracks.filter((_, i) => i !== real);
        setUser(await api('playlists/' + p.id, 'PUT', { tracks }));
        route();
      },
    });
  }
  showMenu(items, anchor);
}

function playlistMenu(t, anchor) {
  const add = async p => {
    if (p.tracks.includes(t.id)) return toast(`Already in ${p.name}`);
    setUser(await api('playlists/' + p.id, 'PUT', { tracks: [...p.tracks, t.id] }));
    toast(`Added to ${p.name}`);
  };
  showMenu([
    { label: '+ New playlist', run: async () => {
      const name = await askText('New playlist', '', 'Create');
      if (!name) return;
      setUser(await api('playlists', 'POST', { name, tracks: [t.id] }));
      toast(`Added to ${name}`);
    } },
    ...(S.user.playlists.length ? ['-'] : []),
    ...S.user.playlists.map(p => ({ label: p.name, run: () => add(p) })),
  ], anchor);
}

function askText(title, value, okLabel) {
  const d = $('#prompt');
  d.innerHTML = `<form method="dialog"><h2>${esc(title)}</h2>
    <input name="text" value="${esc(value)}" maxlength="80" required autocomplete="off" aria-label="${esc(title)}">
    <div class="dialog-actions"><button class="btn" value="cancel" formnovalidate>Cancel</button><button class="btn primary" value="ok">${esc(okLabel)}</button></div></form>`;
  d.returnValue = '';
  d.showModal();
  $('input', d).select();
  return new Promise(resolve => {
    d.addEventListener('close', () => resolve(d.returnValue === 'ok' ? $('input', d).value.trim() : ''), { once: true });
  });
}

function confirmBox(title, text, okLabel) {
  const d = $('#prompt');
  d.innerHTML = `<form method="dialog"><h2>${esc(title)}</h2><p class="muted">${esc(text)}</p>
    <div class="dialog-actions"><button class="btn" value="cancel">Cancel</button><button class="btn primary" value="ok">${esc(okLabel)}</button></div></form>`;
  d.returnValue = '';
  d.showModal();
  return new Promise(resolve => d.addEventListener('close', () => resolve(d.returnValue === 'ok'), { once: true }));
}

async function newPlaylist() {
  const name = await askText('New playlist', '', 'Create');
  if (!name) return;
  setUser(await api('playlists', 'POST', { name }));
  const p = S.user.playlists[S.user.playlists.length - 1];
  if (p) location.hash = '#/playlist/' + p.id;
}

async function toggleLike(id) {
  setUser(await api('liked/' + id, 'POST', { liked: !S.liked.has(id) }));
  const liked = S.liked.has(id);
  for (const b of $$(`.row[data-id="${id}"] .like`)) {
    b.classList.toggle('on', liked);
    b.innerHTML = icon(liked ? 'heart' : 'heartOutline');
  }
  renderPlayer();
  if (location.hash === '#/liked') route();
}

/* ---------- speakers & Bluetooth panel ---------- */
const D = { bt: null, audio: null, busy: null, timer: null };

async function openDevices() {
  const d = $('#devices');
  renderDevices();
  if (!d.open) d.showModal();
  refreshDevices();
}

async function refreshDevices() {
  const jobs = [api('bluetooth').then(bt => { D.bt = bt; }).catch(() => {})];
  if (S.info.player && isServer()) jobs.push(api('player/devices').then(a => { D.audio = a; }).catch(() => { D.audio = null; }));
  await Promise.all(jobs);
  renderDevices();
  clearTimeout(D.timer);
  if ($('#devices').open && D.bt && D.bt.scanning) D.timer = setTimeout(refreshDevices, 2500);
}

function renderDevices() {
  const host = esc(S.info.hostname || 'server');
  const bt = D.bt;
  let html = `<div class="dlg-head"><h2>Speakers</h2><button class="icon-btn" data-act="close-devices" aria-label="Close">${icon('close')}</button></div>
  <h3>Play on</h3>
  <div class="outputs">
    <button class="output${!isServer() ? ' on' : ''}" data-act="output" data-out="local">
      <span class="output-title">This device</span>
      <span class="muted small">Plays in this browser. Bluetooth headphones paired with this phone or computer just work.</span>
    </button>
    <button class="output${isServer() ? ' on' : ''}" data-act="output" data-out="server" ${S.info.player ? '' : 'disabled'}>
      <span class="output-title">${host}</span>
      <span class="muted small">${S.info.player ? 'Plays from the machine running Gundil Music, through its speakers or a Bluetooth speaker.' : 'Install mpv on the server to play through its speakers.'}</span>
    </button>
  </div>`;

  if (isServer() && D.audio && D.audio.devices && D.audio.devices.length) {
    html += `<h3><label for="audio-device">Audio output on ${host}</label></h3>
      <select id="audio-device">${D.audio.devices.map(a =>
        `<option value="${esc(a.name)}"${a.name === D.audio.current ? ' selected' : ''}>${esc(a.description || a.name)}</option>`).join('')}</select>`;
  }

  html += `<div class="dlg-head sub"><h3>${icon('bluetooth')}Bluetooth on ${host}</h3>`;
  if (bt && bt.available) {
    html += `<button class="btn small" data-act="bt-scan" ${bt.scanning ? 'disabled' : ''}>${bt.scanning ? 'Scanning…' : 'Scan for devices'}</button>`;
  }
  html += '</div>';
  if (!bt) html += '<p class="muted small">Checking…</p>';
  else if (!bt.available) html += `<p class="muted small">${esc(bt.reason)}</p>`;
  else {
    const unnamed = d => /^([0-9A-F]{2}[-:]){5}[0-9A-F]{2}$/i.test(d.name);
    const shown = bt.devices.filter(d => d.paired || d.connected || !unnamed(d));
    if (!bt.powered) html += '<p class="muted small">Bluetooth is turned off. Scanning turns it on.</p>';
    if (!shown.length) html += '<p class="muted small">No devices yet. Put your speaker in pairing mode and scan.</p>';
    html += '<div class="bt-list">' + shown.map(d => {
      const busy = D.busy === d.mac;
      const status = busy ? 'Working…' : d.connected ? 'Connected' : d.paired ? 'Paired' : 'Nearby';
      return `<div class="bt${d.connected ? ' on' : ''}">
        <div class="bt-text"><div>${esc(d.name)}</div><div class="muted small">${status}${d.audio ? ' · Audio' : ''}</div></div>
        ${d.paired && !d.connected ? `<button class="btn small" data-act="bt-remove" data-mac="${d.mac}" ${busy ? 'disabled' : ''}>Forget</button>` : ''}
        <button class="btn small${d.connected ? '' : ' primary'}" data-act="${d.connected ? 'bt-disconnect' : 'bt-connect'}" data-mac="${d.mac}" ${busy ? 'disabled' : ''}>${d.connected ? 'Disconnect' : 'Connect'}</button>
      </div>`;
    }).join('') + '</div>';
  }
  $('#devices').innerHTML = html;
}

async function btAction(action, mac) {
  D.busy = mac || null;
  renderDevices();
  try {
    D.bt = await api('bluetooth/' + action, 'POST', mac ? { mac } : {});
    if (action === 'connect') {
      const dev = D.bt.devices.find(d => d.mac === mac);
      if (S.info.player && !isServer()) await setOutput('server');
      toast(`Connected to ${dev ? dev.name : 'device'}`);
    }
  } catch (e) {
    toast(e.message);
  }
  D.busy = null;
  refreshDevices();
}

/* ---------- events ---------- */
document.addEventListener('click', async e => {
  const menu = $('#menu');
  if (!menu.hidden) {
    const b = e.target.closest('#menu button');
    if (b) {
      const it = menu.items[+b.dataset.i];
      if (!it.keep) hideMenu();
      try { await it.run(); } catch (err) { toast(err.message); }
    } else hideMenu();
    e.preventDefault();
    return;
  }

  const actEl = e.target.closest('[data-act]');
  const row = e.target.closest('.row');
  const act = actEl && actEl.dataset.act;
  const listIds = name => S.lists[name] || [];

  try {
    switch (act) {
      case 'toggle': return void toggle();
      case 'next': return void next();
      case 'prev': return void prev();
      case 'shuffle': return void toggleShuffle();
      case 'repeat': return void cycleRepeat();
      case 'open-now': if (!e.target.closest('#p-like')) openNow(); return;
      case 'close-now': return void closeNow();
      case 'devices': return void openDevices();
      case 'close-devices': return void $('#devices').close();
      case 'output': await setOutput(actEl.dataset.out); return void refreshDevices();
      case 'bt-scan': return void btAction('scan');
      case 'bt-connect': return void btAction('connect', actEl.dataset.mac);
      case 'bt-disconnect': return void btAction('disconnect', actEl.dataset.mac);
      case 'bt-remove': return void btAction('remove', actEl.dataset.mac);
      case 'like': e.preventDefault(); return void await toggleLike(row.dataset.id);
      case 'like-current': if (current()) await toggleLike(current().id); return;
      case 'menu': {
        const t = S.byId.get(row.dataset.id);
        const opts = S.listOpts[row.dataset.list] || {};
        if (t) trackMenu(t, actEl, opts.playlist, +row.dataset.idx);
        return;
      }
      case 'q-remove': return void removeFromQueue(+row.dataset.q);
      case 'play-list': return void playQueue(listIds(actEl.dataset.list), 0);
      case 'shuffle-list': return void playShuffled(listIds(actEl.dataset.list));
      case 'shuffle-all': return void playShuffled(S.tracks.map(t => t.id));
      case 'play-album': {
        e.preventDefault();
        const al = S.albumByKey.get(actEl.dataset.key);
        if (al) playQueue(al.tracks.slice().sort((a, b) => (a.disc || 1) - (b.disc || 1) || (a.track || 0) - (b.track || 0)).map(t => t.id), 0);
        return;
      }
      case 'new-playlist': return void await newPlaylist();
      case 'rename-playlist': {
        const p = S.user.playlists.find(x => x.id === actEl.dataset.id);
        const name = p && await askText('Rename playlist', p.name, 'Save');
        if (name) { setUser(await api('playlists/' + p.id, 'PUT', { name })); route(); }
        return;
      }
      case 'delete-playlist': {
        const p = S.user.playlists.find(x => x.id === actEl.dataset.id);
        if (p && await confirmBox('Delete playlist?', `“${p.name}” will be removed. The songs stay in your library.`, 'Delete')) {
          setUser(await api('playlists/' + p.id, 'DELETE'));
          location.hash = '#/library/playlists';
        }
        return;
      }
      case 'rescan':
        await api('library/rescan', 'POST', {});
        S.scanning = true;
        toast('Scanning your music folders…');
        return void route();
    }
  } catch (err) {
    toast(err.message);
    return;
  }

  if (row && !e.target.closest('a')) {
    if (row.dataset.q !== undefined) jump(+row.dataset.q);
    else playQueue(listIds(row.dataset.list), +row.dataset.idx);
  }
});

$('#devices').addEventListener('click', e => { if (e.target === e.currentTarget) e.currentTarget.close(); });
$('#devices').addEventListener('change', async e => {
  if (e.target.id !== 'audio-device') return;
  try { await api('player/device', 'POST', { name: e.target.value }); toast('Audio output changed'); } catch (err) { toast(err.message); }
});

for (const el of [$('#seek'), $('#now-seek')]) {
  el.addEventListener('input', () => { seeking = true; });
  el.addEventListener('change', () => {
    seeking = false;
    const dur = S.duration || (current() || {}).duration || 0;
    if (dur) seek(el.value / 1000 * dur);
  });
}
let volTimer;
$('#volume').addEventListener('input', e => {
  const v = +e.target.value;
  clearTimeout(volTimer);
  volTimer = setTimeout(() => setVolume(v), isServer() ? 120 : 0);
});

let searchTimer;
$('#search').addEventListener('input', e => {
  clearTimeout(searchTimer);
  const q = e.target.value.trim();
  searchTimer = setTimeout(() => {
    const target = q ? '#/search/' + enc(q) : '#/';
    if (location.hash.startsWith('#/search/') && q) history.replaceState(null, '', target), route();
    else location.hash = target;
  }, 180);
});

document.addEventListener('keydown', e => {
  const typing = /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName) && e.target.type !== 'range';
  if (e.key === 'Escape') { if (!$('#menu').hidden) hideMenu(); else if (!$('#now').hidden && !$('dialog[open]')) closeNow(); return; }
  if (typing || $('dialog[open]') || e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key === ' ' && e.target.tagName !== 'BUTTON' && e.target.tagName !== 'A') { e.preventDefault(); toggle(); }
  else if (e.key === '/') { e.preventDefault(); $('#search').focus(); }
});

window.addEventListener('hashchange', () => { hideMenu(); route(); });
window.addEventListener('scroll', hideMenu, true);

function connectEvents() {
  const es = new EventSource('/api/events');
  es.addEventListener('player', e => applyServer(JSON.parse(e.data)));
  es.addEventListener('library', async () => { await loadLibrary(); route(); renderPlayer(); });
  es.addEventListener('bluetooth', () => { if ($('#devices').open) refreshDevices(); });
}

/* ---------- start ---------- */
function staticIcons() {
  const set = (sel, name) => { for (const el of $$(sel)) el.innerHTML = icon(name) + el.innerHTML; };
  set('[data-act="new-playlist"].icon-btn', 'plus');
  set('.search-icon', 'search');
  set('.pill-btn .ico', 'speaker');
  set('#p-prev, #now-prev', 'prev');
  set('#p-next, #now-next', 'next');
  set('#p-shuffle, #now-shuffle', 'shuffle');
  set('.vol-ico', 'volume');
  set('#p-queue', 'queue');
  set('#now-close', 'down');
}

async function init() {
  staticIcons();
  try {
    const [info, , user, server] = await Promise.all([api('info'), loadLibrary(), api('state'), api('player')]);
    S.info = info;
    S.server = server;
    setUser(user);
  } catch (e) {
    $('#view').innerHTML = `<div class="empty-state"><h1>Cannot reach the server</h1><p class="muted">${esc(e.message)}</p><p><button class="btn" onclick="location.reload()">Try again</button></p></div>`;
    return;
  }
  // First visit: if the server is already playing (e.g. a headless box), control that.
  if (S.output === null) S.output = S.info.player && S.server.playing ? 'server' : 'local';
  if (isServer() && !S.info.player) S.output = 'local';
  audio.volume = Math.min(1, store.get('volume', 80) / 100);

  if (isServer()) applyServer(S.server);
  else {
    const saved = store.get('queue', null);
    if (saved && Array.isArray(saved.ids)) {
      S.queue = saved.ids.filter(id => S.byId.has(id));
      S.index = Math.min(Math.max(0, saved.index | 0), S.queue.length - 1);
    }
  }
  if (!location.hash) history.replaceState(null, '', '#/');
  route();
  renderOutput();
  renderPlayer();
  connectEvents();
  if ('serviceWorker' in navigator) navigator.serviceWorker.register('sw.js').catch(() => {});
}
init();
