'use strict';

/* =========================================================================
   helpers
   ========================================================================= */
const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const enc = encodeURIComponent;
const mobile = () => matchMedia('(max-width: 760px)').matches;

const ICONS = {
  menu: 'M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z',
  home: 'M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z',
  explore: 'M12 10.9c-.61 0-1.1.49-1.1 1.1s.49 1.1 1.1 1.1c.61 0 1.1-.49 1.1-1.1s-.49-1.1-1.1-1.1zM12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm2.19 12.19L6 18l3.81-8.19L18 6l-3.81 8.19z',
  library: 'M4 6H2v14a2 2 0 002 2h14v-2H4zm16-4H8a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V4a2 2 0 00-2-2zm-2 5h-3v5.5a2.5 2.5 0 11-2.5-2.5c.57 0 1.08.19 1.5.51V5h4z',
  search: 'M15.5 14h-.79l-.28-.27A6.47 6.47 0 0016 9.5 6.5 6.5 0 109.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z',
  cast: 'M21 3H3c-1.1 0-2 .9-2 2v3h2V5h18v14h-7v2h7c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zM1 18v3h3c0-1.66-1.34-3-3-3zm0-4v2c2.76 0 5 2.24 5 5h2c0-3.87-3.13-7-7-7zm0-4v2c4.97 0 9 4.03 9 9h2c0-6.08-4.93-11-11-11z',
  castOn: 'M1 18v3h3c0-1.66-1.34-3-3-3zm0-4v2c2.76 0 5 2.24 5 5h2c0-3.87-3.13-7-7-7zm18-7H5v1.63c3.96 1.28 7.09 4.41 8.37 8.37H19V7zM1 10v2c4.97 0 9 4.03 9 9h2c0-6.08-4.93-11-11-11zm20-7H3c-1.1 0-2 .9-2 2v3h2V5h18v14h-7v2h7c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z',
  more: 'M12 8a2 2 0 100-4 2 2 0 000 4zm0 2a2 2 0 100 4 2 2 0 000-4zm0 6a2 2 0 100 4 2 2 0 000-4z',
  like: 'M1 21h4V9H1v12zm22-11c0-1.1-.9-2-2-2h-6.31l.95-4.57.03-.32c0-.41-.17-.79-.44-1.06L14.17 1 7.59 7.59C7.22 7.95 7 8.45 7 9v10c0 1.1.9 2 2 2h9c.83 0 1.54-.5 1.84-1.22l3.02-7.05c.09-.23.14-.47.14-.73v-2z',
  likeOff: 'M9 21h9c.83 0 1.54-.5 1.84-1.22l3.02-7.05c.09-.23.14-.47.14-.73v-2c0-1.1-.9-2-2-2h-6.31l.95-4.57.03-.32c0-.41-.17-.79-.44-1.06L14.17 1 7.58 7.59C7.22 7.95 7 8.45 7 9v10c0 1.1.9 2 2 2zM9 9l4.34-4.34L12 10h9v2l-3 7H9V9zM1 9h4v12H1z',
  dislike: 'M15 3H6c-.83 0-1.54.5-1.84 1.22l-3.02 7.05c-.09.23-.14.47-.14.73v2c0 1.1.9 2 2 2h6.31l-.95 4.57-.03.32c0 .41.17.79.44 1.06L9.83 23l6.59-6.59c.36-.36.58-.86.58-1.41V5c0-1.1-.9-2-2-2zm4 0v12h4V3h-4z',
  dislikeOff: 'M15 3H6c-.83 0-1.54.5-1.84 1.22l-3.02 7.05c-.09.23-.14.47-.14.73v2c0 1.1.9 2 2 2h6.31l-.95 4.57-.03.32c0 .41.17.79.44 1.06L9.83 23l6.59-6.59c.36-.36.58-.86.58-1.41V5c0-1.1-.9-2-2-2zm0 12l-4.34 4.34L12 14H3v-2l3-7h9v10zm4-12h4v12h-4z',
  play: 'M8 5v14l11-7z',
  pause: 'M6 19h4V5H6zm8-14v14h4V5z',
  next: 'M6 18l8.5-6L6 6zM16 6v12h2V6z',
  prev: 'M6 6h2v12H6zm3.5 6l8.5 6V6z',
  shuffle: 'M10.59 9.17L5.41 4 4 5.41l5.17 5.17zM14.5 4l2.04 2.04L4 18.59 5.41 20 17.96 7.46 20 9.5V4zm.33 9.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04z',
  repeat: 'M7 7h10v3l4-4-4-4v3H5v6h2zm10 10H7v-3l-4 4 4 4v-3h12v-6h-2z',
  repeatOne: 'M7 7h10v3l4-4-4-4v3H5v6h2zm10 10H7v-3l-4 4 4 4v-3h12v-6h-2zm-4-2V9h-1l-2 1v1h1.5v4z',
  volume: 'M3 9v6h4l5 5V4L7 9zm13.5 3A4.5 4.5 0 0014 7.97v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z',
  mute: 'M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z',
  up: 'M12 8l-6 6 1.41 1.41L12 10.83l4.59 4.58L18 14z',
  down: 'M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6z',
  left: 'M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z',
  right: 'M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z',
  add: 'M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6z',
  account: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 4c1.93 0 3.5 1.57 3.5 3.5S13.93 13 12 13s-3.5-1.57-3.5-3.5S10.07 6 12 6zm0 14c-2.03 0-4.43-.82-6.14-2.88a9.947 9.947 0 0112.28 0C16.43 19.18 14.03 20 12 20z',
  power: 'M13 3h-2v10h2V3zm4.83 2.17l-1.42 1.42A6.92 6.92 0 0119 12c0 3.87-3.13 7-7 7A6.995 6.995 0 016.58 6.58L5.17 5.17A8.932 8.932 0 003 12a9 9 0 0018 0c0-2.74-1.23-5.18-3.17-6.83z',
  logout: 'M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.58L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z',
  import: 'M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z',
  check: 'M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z',
  download: 'M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z',
  downloadDone: 'M5 18h14v2H5v-2zm4.6-2.7L5 10.7l2-1.9 2.6 2.6L17 4l2 2-9.4 9.3z',
  offline: 'M23.64 7c-.45-.34-4.93-4-11.64-4-1.5 0-2.89.19-4.15.48L18.18 13.8 23.64 7zm-6.6 8.22L3.27 1.44 2 2.72l2.05 2.06C1.91 5.76.59 6.82.36 7l11.63 14.49.01.01.01-.01 3.9-4.86 3.32 3.32 1.27-1.27-3.46-3.46z',
  playlistAdd: 'M14 10H2v2h12v-2zm0-4H2v2h12V6zm4 8v-4h-2v4h-4v2h4v4h2v-4h4v-2h-4zM2 16h8v-2H2v2z',
  playNext: 'M19 9H2v2h17V9zm0-4H2v2h17V5zM2 15h13v-2H2v2zm15-2v6l5-3-5-3z',
  queue: 'M15 6H3v2h12zm0 4H3v2h12zM3 16h8v-2H3zM17 6v8.18c-.31-.11-.65-.18-1-.18a3 3 0 103 3V8h3V6z',
  radio: 'M3.24 6.15C2.51 6.43 2 7.17 2 8v12c0 1.1.89 2 2 2h16c1.11 0 2-.9 2-2V8c0-1.11-.89-2-2-2H8.3l8.26-3.34L15.88 1 3.24 6.15zM7 20c-1.66 0-3-1.34-3-3s1.34-3 3-3 3 1.34 3 3-1.34 3-3 3zm13-8h-2v-2h-2v2H4V8h16v4z',
  close: 'M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z',
  back: 'M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z',
  history: 'M13 3a9 9 0 00-9 9H1l3.89 3.89.07.14L9 12H6c0-3.87 3.13-7 7-7s7 3.13 7 7-3.13 7-7 7c-1.93 0-3.68-.79-4.94-2.06l-1.42 1.42A8.954 8.954 0 0013 21a9 9 0 000-18zm-1 5v5l4.28 2.54.72-1.21-3.5-2.08V8H12z',
  newReleases: 'M23 12l-2.44-2.78.34-3.68-3.61-.82-1.89-3.18L12 3 8.6 1.54 6.71 4.72l-3.61.81.34 3.68L1 12l2.44 2.78-.34 3.69 3.61.82 1.89 3.18L12 21l3.4 1.46 1.89-3.18 3.61-.82-.34-3.68L23 12zm-10 5h-2v-2h2v2zm0-4h-2V7h2v6z',
  charts: 'M16 6l2.29 2.29-4.88 4.88-4-4L2 16.59 3.41 18l6-6 4 4 6.3-6.29L22 12V6z',
  mood: 'M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm3.5-9c.83 0 1.5-.67 1.5-1.5S16.33 8 15.5 8 14 8.67 14 9.5s.67 1.5 1.5 1.5zm-7 0c.83 0 1.5-.67 1.5-1.5S9.33 8 8.5 8 7 8.67 7 9.5 7.67 11 8.5 11zm3.5 6.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z',
  edit: 'M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04a.996.996 0 000-1.41l-2.34-2.34a.996.996 0 00-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z',
  delete: 'M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z',
  share: 'M18 16.08c-.76 0-1.44.3-1.96.77L8.91 12.7c.05-.23.09-.46.09-.7s-.04-.47-.09-.7l7.05-4.11c.54.5 1.25.81 2.04.81 1.66 0 3-1.34 3-3s-1.34-3-3-3-3 1.34-3 3c0 .24.04.47.09.7L8.04 9.81C7.5 9.31 6.79 9 6 9c-1.66 0-3 1.34-3 3s1.34 3 3 3c.79 0 1.5-.31 2.04-.81l7.12 4.16c-.05.21-.08.43-.08.65 0 1.61 1.31 2.92 2.92 2.92 1.61 0 2.92-1.31 2.92-2.92s-1.31-2.92-2.92-2.92z',
  libAdd: 'M4 6H2v14c0 1.1.9 2 2 2h14v-2H4V6zm16-4H8c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-1 9h-4v4h-2v-4H9V9h4V5h2v4h4v2z',
  libDone: 'M20 2H8c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-7.53 12L9 10.5l1.4-1.41 2.07 2.08L17.6 6 19 7.41 12.47 14zM4 6H2v14c0 1.1.9 2 2 2h14v-2H4V6z',
  album: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 14.5c-2.49 0-4.5-2.01-4.5-4.5S9.51 7.5 12 7.5s4.5 2.01 4.5 4.5-2.01 4.5-4.5 4.5zm0-5.5c-.55 0-1 .45-1 1s.45 1 1 1 1-.45 1-1-.45-1-1-1z',
  artist: 'M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z',
  remove: 'M7 11v2h10v-2H7zm5-9C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z',
  bluetooth: 'M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29zM13 5.83l1.88 1.88L13 9.59zm1.88 10.46L13 18.17v-3.76z',
  speaker: 'M17 2H7a2 2 0 00-2 2v16a2 2 0 002 2h10a2 2 0 002-2V4a2 2 0 00-2-2zm-5 2a2 2 0 110 4 2 2 0 010-4zm0 16a5 5 0 110-10 5 5 0 010 10zm0-8a3 3 0 100 6 3 3 0 000-6z',
  phone: 'M17 1.01L7 1c-1.1 0-2 .9-2 2v18c0 1.1.9 2 2 2h10c1.1 0 2-.9 2-2V3c0-1.1-.9-1.99-2-1.99zM17 19H7V5h10v14z',
  info: 'M11 7h2v2h-2zm0 4h2v6h-2zm1-9C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z',
  lyrics: 'M14 17H4v2h10v-2zm6-8H4v2h16V9zM4 15h16v-2H4v2zM4 5v2h16V5H4z',
  drag: 'M20 9H4v2h16V9zM4 15h16v-2H4v2z',
  timer: 'M15 1H9v2h6V1zm-4 13h2V8h-2v6zm8.03-6.61l1.42-1.42c-.43-.51-.9-.99-1.41-1.41l-1.42 1.42A8.96 8.96 0 0012 4a9 9 0 109 9c0-2.12-.74-4.07-1.97-5.61zM12 20a7 7 0 110-14 7 7 0 010 14z',
  settings: 'M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 00.12-.61l-1.92-3.32a.49.49 0 00-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.48.48 0 00-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96a.49.49 0 00-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58a.49.49 0 00-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6a3.6 3.6 0 110-7.2 3.6 3.6 0 010 7.2z',
  fullscreen: 'M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z',
  lyricsFs: 'M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z',
  fullscreenExit: 'M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z',
  refresh: 'M17.65 6.35A7.95 7.95 0 0012 4a8 8 0 108 8h-2a6 6 0 11-6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4z',
};
const icon = name => `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="${ICONS[name]}"/></svg>`;

function fmt(s) {
  if (!s || !isFinite(s)) return '';
  s = Math.floor(s);
  const m = Math.floor(s / 60), h = Math.floor(m / 60), ss = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m % 60).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}
const plural = (n, w) => `${n} ${w}${n === 1 ? '' : 's'}`;

function shuffled(list) {
  const a = list.slice();
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

/* Ask YouTube's image servers for the size we draw at. */
function sized(url, px) {
  if (!url) return '';
  px = Math.round(px * Math.min(2, window.devicePixelRatio || 1));
  if (/googleusercontent\.com|ggpht\.com/.test(url)) {
    if (/=w\d+-h\d+/.test(url)) return url.replace(/=w\d+-h\d+/, `=w${px}-h${px}`);
    if (/=s\d+/.test(url)) return url.replace(/=s\d+/, `=s${px}`);
  }
  return url;
}
const img = (url, px, alt = '') => url ? `<img src="${esc(dlThumbs.get(url) || sized(url, px))}" alt="${esc(alt)}" loading="lazy" referrerpolicy="no-referrer">` : '';

async function api(path, method = 'GET', body) {
  const res = await fetch('/api/' + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (res.status === 401 && data.signin) showSignIn();
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

/* YouTube Music calls, cached for the session so going back is instant. */
const ytCache = new Map();
function yt(method, args = {}, { fresh = false } = {}) {
  const qs = Object.entries(args).filter(([, v]) => v !== undefined && v !== null && v !== '').map(([k, v]) => `${k}=${enc(v)}`).join('&');
  const key = method + '?' + qs;
  const hit = ytCache.get(key);
  if (!fresh && hit && Date.now() - hit.at < 10 * 60 * 1000) return hit.p;
  const p = api('yt/' + key);
  ytCache.set(key, { at: Date.now(), p });
  p.catch(() => ytCache.delete(key));
  if (ytCache.size > 300) ytCache.delete(ytCache.keys().next().value);
  return p;
}

let toastTimer;
function toast(msg, action) {
  const el = $('#toast');
  el.innerHTML = `<span>${esc(msg)}</span>${action ? `<button>${esc(action.label)}</button>` : ''}`;
  if (action) $('button', el).onclick = () => { el.hidden = true; action.run(); };
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.hidden = true; }, action ? 6000 : 3500);
}

const local = {
  get(k, d) { try { const v = localStorage.getItem('gundil.' + k); return v === null ? d : JSON.parse(v); } catch { return d; } },
  set(k, v) { try { localStorage.setItem('gundil.' + k, JSON.stringify(v)); } catch { /* private mode */ } },
};

/* =========================================================================
   state
   ========================================================================= */
const S = {
  info: { youtube: {} },
  session: { accounts: false, user: null, admin: true },
  google: null,
  user: { playlists: [], liked: [], disliked: [], saved: [], history: [], searches: [] },
  liked: new Set(), disliked: new Set(), saved: new Set(),
  output: local.get('output', null), // 'local' | 'server'
  server: null,
  queue: [], index: -1, source: '', queueVer: 0, original: null,
  shuffle: false, repeat: local.get('repeat', 'off'), autoplay: local.get('autoplay', true),
  playing: false, paused: true, loading: false, position: 0, duration: 0,
  volume: local.get('volume', 80), muted: false,
  ppOpen: false, ppTab: 'upnext',
  mode: local.get('mode', 'song'), // Song / Video switch
  sleep: { at: 0, end: false },     // sleep timer: pause at (ms), or at the end of the song
  visible: true,                    // the app is on screen (the Android app tells us)
  settings: null,
};
const reg = new Map(); // key -> registered item or list, referenced from the DOM
let regN = 0;
function register(v) {
  const k = 'r' + (++regN);
  reg.set(k, v);
  if (reg.size > 6000) for (const key of [...reg.keys()].slice(0, 2000)) reg.delete(key);
  return k;
}

/* Two players, so songs can crossfade: `audio` is the one playing now, the
   other fades the next song in (see crossfade below) and then takes over. */
const decks = [new Audio(), new Audio()];
let audio = decks[0];
for (const d of decks) d.preload = 'auto';
/* Listens to whichever player is current; the other one's events are ignored. */
function onAudio(type, fn) {
  for (const d of decks) d.addEventListener(type, e => { if (d === audio) fn(e); });
}
const isServer = () => S.output === 'server';
const audioQuality = () => local.get('quality', 'normal');
const streamSrc = id => `/api/stream/${id}?q=${audioQuality()}`;
const current = () => S.queue[S.index];

/* A compact copy of a track, as stored in queues, history and playlists. */
const slim = t => ({ type: t.type, videoId: t.videoId, title: t.title, artists: t.artists || [], album: t.album || undefined, duration: t.duration || 0, thumb: t.thumb || '', explicit: !!t.explicit });
const isTrack = it => it && it.videoId && (it.type === 'song' || it.type === 'video' || !it.type);
const names = t => (t.artists || []).map(a => a.name).join(', ');
const artistLinks = arts => (arts || []).map(a => a.id ? `<a href="#/artist/${enc(a.id)}">${esc(a.name)}</a>` : esc(a.name)).join(', ');
const albumLink = al => al ? (al.id ? `<a href="#/album/${enc(al.id)}">${esc(al.name)}</a>` : esc(al.name)) : '';
const explicit = t => t.explicit ? '<span class="explicit" title="Explicit">E</span>' : '';

function trackSub(t, { links = true, album = true, views = true } = {}) {
  const parts = [];
  if (t.type === 'video') parts.push('Video');
  else if (t.type === 'song' && !links) parts.push('Song');
  if (t.artists && t.artists.length) parts.push(links ? artistLinks(t.artists) : esc(names(t)));
  if (album && t.album && t.type !== 'video') parts.push(links ? albumLink(t.album) : esc(t.album.name));
  if (views && t.views) parts.push(esc(t.views) + (/view|play/i.test(t.views) ? '' : ' views'));
  return parts.join(' • ');
}

function itemHref(it) {
  if (!it) return '';
  if (it.type === 'album') return '#/album/' + enc(it.id);
  if (it.type === 'artist') return '#/artist/' + enc(it.id);
  if (it.type === 'playlist') return '#/playlist/' + enc(it.id);
  return '';
}

/* =========================================================================
   user library
   ========================================================================= */
function setUser(u) {
  if (!u) return;
  S.user = u;
  S.liked = new Set(u.liked.map(t => t.videoId));
  S.disliked = new Set(u.disliked);
  S.saved = new Set(u.saved.map(x => x.type + ':' + x.id));
  renderGuide();
  refreshRatings();
}
const rating = id => S.liked.has(id) ? 'like' : S.disliked.has(id) ? 'dislike' : 'none';

async function rate(t, want) {
  const now = rating(t.videoId);
  const r = now === want ? 'none' : want;
  setUser(await api('rating', 'POST', { track: slim(t), rating: r }));
  if (r === 'like') toast('Saved to liked music');
  else if (r === 'dislike') {
    toast('Got it. You won\'t hear this song as much.');
    if (current() && current().videoId === t.videoId) next();
  } else if (now === 'like') toast('Removed from liked music');
  if (location.hash === '#/playlist/LM' || location.hash.startsWith('#/library/songs')) route(true);
}

function rateButtons(t) {
  const r = rating(t.videoId);
  return `<button class="icon-btn rate${r === 'dislike' ? ' rated' : ''}" data-act="dislike" aria-label="Dislike" title="Dislike">${icon(r === 'dislike' ? 'dislike' : 'dislikeOff')}</button>
    <button class="icon-btn rate${r === 'like' ? ' rated like-on' : ''}" data-act="like" aria-label="Like" title="Like">${icon(r === 'like' ? 'like' : 'likeOff')}</button>`;
}

function refreshRatings() {
  for (const row of $$('.row[data-vid]')) {
    const box = $('.rate-box', row);
    const t = rowTrack(row);
    if (box && t) box.innerHTML = rateButtons(t);
  }
  renderRating();
}

async function setSaved(item, saved) {
  setUser(await api('saved', 'POST', { item: { type: item.type, id: item.id, title: item.title, subtitle: item.subtitle || '', thumb: item.thumb || '' }, saved }));
  if (item.type === 'artist') toast(saved ? `Subscribed to ${item.title}` : `Unsubscribed from ${item.title}`);
  else toast(saved ? 'Added to library' : 'Removed from library');
}
const isSaved = it => S.saved.has(it.type + ':' + it.id);

/* =========================================================================
   playback
   ========================================================================= */
async function srv(action, body) {
  try {
    applyServer(await api('player/' + action, 'POST', body || {}));
    return true;
  } catch (e) {
    toast(e.message);
    return false;
  }
}

let serverQVer = -1, resyncing = false;
function applyServer(st) {
  const known = S.server ? S.server.queue : [];
  // Replies and live events can arrive out of order: never let an older queue replace a newer one.
  const fresh = st.queue && st.queueVersion >= serverQVer;
  if (fresh) serverQVer = st.queueVersion;
  else if (!st.queue && st.queueVersion > serverQVer && !resyncing) {
    resyncing = true; // we missed a queue update
    api('player').then(applyServer).catch(() => {}).finally(() => { resyncing = false; });
  }
  S.server = { ...st, queue: fresh ? st.queue : known || [] };
  if (!isServer()) { renderCast(); return; }
  if (fresh) { S.queue = st.queue; S.queueVer++; }
  S.index = st.index;
  S.source = st.source || '';
  S.playing = st.playing;
  S.paused = st.paused || !st.playing;
  S.loading = st.loading && st.playing;
  S.position = st.position || 0;
  S.duration = st.duration || (current() || {}).duration || 0;
  S.volume = st.volume;
  S.repeat = st.repeat;
  S.autoplay = st.autoplay;
  S.sleep = { at: st.sleepAt || 0, end: !!st.sleepEnd };
  if (st.error && st.error !== S.lastError) toast(st.error);
  if (st.playing) S.stuck = '';
  S.lastError = st.error;
  renderPlayer();
}

function saveLocalQueue() {
  if (!isServer()) local.set('queue', { tracks: S.queue.slice(0, 500), index: S.index, source: S.source });
}

let errorsInRow = 0, loadToken = 0;
function loadLocal(play, at = 0, ready = false) {
  const t = current();
  if (!t) return;
  const tok = ++loadToken;
  if (!ready && S.mode === 'video' && t.type !== 'video' && !t.noVideo) {
    // Video mode: play the song's music video when it has one, like YouTube Music.
    S.playing = true; S.paused = !play; S.loading = play;
    renderPlayer();
    findVideo(t).then(alt => {
      if (tok !== loadToken || current() !== t) return;
      if (alt) { swapBack.set(alt.videoId, t); S.queue[S.index] = alt; S.queueVer++; }
      loadLocal(play, at, true);
    });
    return;
  }
  if (!fadeKeep) endFade();
  fadeKeep = false;
  const id = t.videoId;
  const start = src => {
    if (audio.getAttribute('src') !== src) audio.src = src;
    if (at > 0) audio.addEventListener('loadedmetadata', () => { audio.currentTime = at; }, { once: true });
    if (play) audio.play().catch(e => { if (e.name === 'NotAllowedError') { S.paused = true; S.loading = false; renderPlayer(); } });
  };
  if (dl.list[id] && !dl.urls.has(id)) {
    // Downloaded: open the copy on this device first (a moment), else stream.
    dlUrl(id).then(u => { if (tok === loadToken) start(u || streamSrc(id)); }, () => { if (tok === loadToken) start(streamSrc(id)); });
  } else start(srcFor(id));
  S.playing = true;
  S.paused = !play;
  S.loading = play;
  S.position = at;
  S.duration = t.duration || 0;
  if (play && !S.offline) api('history', 'POST', { track: slim(t) }).then(u => { S.user = u; }).catch(() => {});
  const nxt = S.queue[S.index + 1];
  if (nxt && dl.list[nxt.videoId]) dlUrl(nxt.videoId).catch(() => {}); // ready for a crossfade
  else if (nxt && !S.offline) api(`prefetch/${nxt.videoId}?q=${audioQuality()}`, 'POST').catch(() => {});
  if (S.autoplay && S.index >= S.queue.length - 2) extendLocal();
  saveLocalQueue();
  renderPlayer();
}

let extending = false;
async function extendLocal() {
  if (extending || !S.queue.length || isServer()) return false;
  extending = true;
  try {
    const last = S.queue[S.queue.length - 1];
    const w = await yt('watch', { videoId: last.videoId, radio: 1 }, { fresh: true });
    if (isServer()) return false;
    const have = new Set(S.queue.map(t => t.videoId));
    const add = w.tracks.filter(t => !have.has(t.videoId) && !S.disliked.has(t.videoId)).map(slim);
    if (!add.length) return false;
    S.queue.push(...add);
    S.queueVer++;
    saveLocalQueue();
    renderPlayer();
    return true;
  } catch {
    return false;
  } finally {
    extending = false;
  }
}

let playToken = 0;

/* Replace the queue and start playing. */
async function setQueue(tracks, index, source, { shuffle = false } = {}) {
  tracks = tracks.filter(isTrack).map(slim);
  if (!tracks.length) return;
  index = Math.max(0, Math.min(index, tracks.length - 1));
  S.shuffle = shuffle;
  S.original = null;
  if (shuffle && tracks.length > 1) {
    S.original = tracks.slice();
    const first = tracks[index];
    tracks = [first, ...shuffled(tracks.filter((_, i) => i !== index))];
    index = 0;
  }
  if (isServer()) return srv('queue', { tracks, index, play: true, source });
  S.queue = tracks;
  S.index = index;
  S.source = source || '';
  S.queueVer++;
  errorsInRow = 0;
  loadLocal(true);
}

/* Play one song and fill "Up next" with its radio, like tapping a song in YouTube Music. */
async function playRadio(track, source) {
  const tok = ++playToken;
  await setQueue([track], 0, source || `${track.title} radio`);
  try {
    const w = await yt('watch', { videoId: track.videoId });
    if (tok !== playToken) return;
    const rest = w.tracks.filter(t => t.videoId !== track.videoId && !S.disliked.has(t.videoId));
    const cur = current();
    if (rest.length && cur && cur.videoId === track.videoId && S.queue.length <= 2) await insert(rest, false, true);
  } catch { /* the song still plays */ }
}

async function playList(tracks, index, source, opts) {
  ++playToken;
  const keep = tracks[index];
  const list = tracks.filter(t => t === keep || !S.disliked.has(t.videoId));
  await setQueue(list, list.indexOf(keep), source, opts);
}

async function insert(tracks, asNext, silent) {
  tracks = tracks.filter(isTrack).map(slim);
  if (!tracks.length) return;
  if (!S.queue.length) return setQueue(tracks, 0, 'Queue');
  if (isServer()) await srv('insert', { tracks, next: asNext });
  else {
    if (asNext) S.queue.splice(S.index + 1, 0, ...tracks); else S.queue.push(...tracks);
    if (S.original) S.original.push(...tracks);
    S.queueVer++;
    saveLocalQueue();
    renderPlayer();
  }
  if (!silent) toast(asNext ? (tracks.length > 1 ? `${tracks.length} songs will play next` : 'Song will play next') : (tracks.length > 1 ? `${tracks.length} songs added to queue` : 'Song added to queue'));
}

function removeFromQueue(i) {
  if (i === S.index || i < 0 || i >= S.queue.length) return;
  if (isServer()) return srv('remove', { index: i });
  S.queue.splice(i, 1);
  if (i < S.index) S.index--;
  S.queueVer++;
  saveLocalQueue();
  renderPlayer();
}

/* Move a queue entry (drag to reorder), keeping the current song playing. */
function moveInQueue(from, to) {
  if (from === to || from < 0 || to < 0 || from >= S.queue.length || to >= S.queue.length) return;
  if (isServer()) return srv('move', { from, to });
  const [t] = S.queue.splice(from, 1);
  S.queue.splice(to, 0, t);
  if (S.index === from) S.index = to;
  else if (from < S.index && to >= S.index) S.index--;
  else if (from > S.index && to <= S.index) S.index++;
  S.queueVer++;
  saveLocalQueue();
  renderPlayer();
}

function toggle() {
  if (!S.queue.length) return;
  if (isServer()) return srv('toggle');
  if (!audio.src) return loadLocal(true);
  endFade();
  if (audio.paused) audio.play().catch(() => {}); else audio.pause();
}

async function next() {
  if (!S.queue.length) return;
  if (isServer()) return srv('next');
  if (S.index + 1 >= S.queue.length && !(await extendLocal())) { S.index = 0; loadLocal(false); return; }
  S.index++;
  loadLocal(true);
}

function prev() {
  if (!S.queue.length) return;
  if (isServer()) return srv('prev');
  endFade();
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

function seek(sec) {
  S.position = sec;
  renderProgress();
  if (isServer()) return srv('seek', { position: sec });
  endFade();
  if (audio.src) audio.currentTime = sec;
  if (video.getAttribute('src') && video.readyState > 0) video.currentTime = sec;
}

function setVolume(v) {
  S.volume = v;
  S.muted = false;
  if (isServer()) return srv('volume', { volume: v });
  audio.volume = fade ? audio.volume : v / 100; // a crossfade sets it as it goes
  for (const d of decks) d.muted = false;
  local.set('volume', v);
  renderVolume();
}

function toggleMute() {
  if (isServer()) {
    if (S.volume > 0) { S.unmuteTo = S.volume; setVolume(0); } else setVolume(S.unmuteTo || 60);
    return;
  }
  S.muted = !S.muted;
  for (const d of decks) d.muted = S.muted;
  renderVolume();
}

function cycleRepeat() {
  S.repeat = { off: 'all', all: 'one', one: 'off' }[S.repeat] || 'off';
  local.set('repeat', S.repeat);
  toast({ off: 'Repeat off', all: 'Repeat all', one: 'Repeat one' }[S.repeat]);
  if (isServer()) return srv('repeat', { mode: S.repeat });
  renderPlayer();
}

function setAutoplay(on) {
  S.autoplay = on;
  local.set('autoplay', on);
  if (isServer()) return srv('autoplay', { on });
  if (on && S.index >= S.queue.length - 2) extendLocal();
  renderPlayer();
}

function toggleShuffle() {
  if (!S.queue.length) return;
  S.shuffle = !S.shuffle;
  const cur = S.queue[S.index];
  if (S.shuffle) {
    S.original = S.queue.slice();
    S.queue = [...S.queue.slice(0, S.index + 1), ...shuffled(S.queue.slice(S.index + 1))];
  } else if (S.original) {
    const at = S.original.findIndex(t => t.videoId === cur.videoId);
    if (at >= 0) { S.queue = S.original; S.index = at; }
    S.original = null;
  }
  S.queueVer++;
  toast(S.shuffle ? 'Shuffle on' : 'Shuffle off');
  if (isServer()) return srv('queue', { tracks: S.queue, index: S.index, play: false, source: S.source });
  saveLocalQueue();
  renderPlayer();
}

async function setOutput(out) {
  if (out === S.output) return;
  if (out === 'server' && !S.info.player) return;
  const tracks = S.queue.slice(), index = S.index, pos = S.position, source = S.source;
  const wasPlaying = S.playing && !S.paused;
  const sleep = { ...S.sleep };
  clearTimeout(sleepTimer);
  if (out === 'local' && S.server && (S.server.sleepAt || S.server.sleepEnd)) api('player/sleep', 'POST', { minutes: 0 }).catch(() => {});
  S.output = out;
  local.set('output', out);
  if (out === 'server') {
    endFade();
    audio.pause();
    audio.removeAttribute('src');
    if (tracks.length && index >= 0) {
      await srv('queue', { tracks, index, play: wasPlaying, source });
      await srv('repeat', { mode: S.repeat });
      await srv('autoplay', { on: S.autoplay });
      if (wasPlaying && pos > 2) {
        for (let i = 0; i < 80 && !(S.server && S.server.duration > 0 && !S.server.loading); i++) await new Promise(r => setTimeout(r, 150));
        await srv('seek', { position: pos });
      }
    } else if (S.server) {
      applyServer(S.server);
    }
  } else {
    if (S.server && S.server.playing) await api('player/stop', 'POST', {}).catch(() => {});
    S.queue = tracks; S.index = index; S.source = source;
    S.volume = local.get('volume', 80);
    audio.volume = S.volume / 100;
    S.playing = false; S.paused = true;
    S.queueVer++;
    if (tracks.length && index >= 0) loadLocal(wasPlaying, pos);
  }
  if (sleep.at > Date.now() || sleep.end) await setSleep(sleep.end ? 0 : (sleep.at - Date.now()) / 60000, sleep.end).catch(() => {});
  else S.sleep = isServer() ? S.sleep : { at: 0, end: false };
  renderCast();
  renderPlayer();
}

onAudio('timeupdate', () => {
  if (isServer()) return;
  S.position = audio.currentTime;
  maybeCrossfade();
  renderProgress();
  if (videoShown) fixDrift(false);
  if (S.sleep.at && Date.now() >= S.sleep.at) sleepNow(); // timers can be slowed in the background
});
onAudio('seeked', () => { if (video.getAttribute('src')) fixDrift(true); });
onAudio('playing', () => { if (!isServer()) { S.loading = false; S.paused = false; errorsInRow = 0; S.stuck = ''; renderPlayer(); } });
onAudio('waiting', () => { if (!isServer()) { S.loading = true; renderPlayer(); } });
onAudio('pause', () => { if (!isServer()) { S.paused = true; renderPlayer(); } });
onAudio('loadedmetadata', () => {
  if (isServer() || !isFinite(audio.duration)) return;
  S.duration = audio.duration;
  renderProgress();
});
onAudio('ended', async () => {
  if (isServer()) return;
  if (S.sleep.end) {
    // Sleep timer: stop at the end of this song, ready with the next one.
    S.sleep = { at: 0, end: false };
    if (S.index + 1 < S.queue.length) { S.index++; loadLocal(false); } else { S.paused = true; renderPlayer(); }
    return;
  }
  if (S.repeat === 'one') { audio.currentTime = 0; audio.play(); return; }
  if (S.index + 1 < S.queue.length) { S.index++; loadLocal(true); return; }
  if (S.repeat === 'all') { S.index = 0; loadLocal(true); return; }
  if (S.autoplay && await extendLocal()) { S.index++; loadLocal(true); return; }
  S.paused = true;
  renderPlayer();
});
onAudio('error', async () => {
  if (isServer() || !audio.getAttribute('src')) return;
  S.loading = false;
  errorsInRow++;
  const t = current();
  // Ask the server why, so problems like YouTube's sign-in check are explained.
  let why = '';
  try {
    const r = await fetch(audio.getAttribute('src'), { headers: { Range: 'bytes=0-1' } });
    if (!r.ok) why = ((await r.json().catch(() => ({}))).error || '');
  } catch { /* offline */ }
  toast(why && /sign in/i.test(why) ? why : `Couldn't play “${t ? t.title : 'this song'}”`,
    why && !/sign in/i.test(why) ? { label: 'Update', run: () => aboutDialog() } : undefined);
  if (errorsInRow < 4 && S.index + 1 < S.queue.length) setTimeout(() => { if (!isServer()) { S.index++; loadLocal(true); } }, 1200);
  else { S.paused = true; S.stuck = t ? t.videoId : ''; renderPlayer(); }
});

/* Lock screen, notification and Bluetooth headset buttons. */
function updateMediaSession(t) {
  if (!('mediaSession' in navigator)) return;
  if (!t || isServer()) { navigator.mediaSession.metadata = null; return; }
  navigator.mediaSession.metadata = new MediaMetadata({
    title: t.title, artist: names(t), album: t.album ? t.album.name : '',
    artwork: t.thumb ? [{ src: sized(t.thumb, 512), sizes: '512x512' }] : [{ src: 'icon-512.png', sizes: '512x512', type: 'image/png' }],
  });
}
if ('mediaSession' in navigator) {
  const handlers = { play: toggle, pause: toggle, previoustrack: prev, nexttrack: next, seekto: e => seek(e.seekTime) };
  for (const [name, fn] of Object.entries(handlers)) {
    try { navigator.mediaSession.setActionHandler(name, fn); } catch { /* unsupported */ }
  }
}

/* =========================================================================
   player rendering
   ========================================================================= */
let shown = null, seeking = false, shownQueue = -1;

function renderProgress() {
  const dur = S.duration || (current() || {}).duration || 0;
  const pct = dur ? Math.min(100, S.position / dur * 100) : 0;
  const v = Math.round(pct * 10);
  for (const el of [$('#seek'), $('#pp-seek')]) {
    if (!seeking) el.value = v;
    el.style.setProperty('--fill', pct + '%');
  }
  $('#mini-fill').style.width = pct + '%';
  $('#b-time').textContent = dur ? `${fmt(S.position) || '0:00'} / ${fmt(dur)}` : '';
  $('#pp-pos').textContent = fmt(S.position) || '0:00';
  $('#pp-dur').textContent = fmt(dur) || '0:00';
  if (S.ppOpen && S.ppTab === 'lyrics') syncLyrics();
}

function renderVolume() {
  const v = S.muted ? 0 : S.volume;
  if (document.activeElement !== $('#volume')) $('#volume').value = v;
  $('#b-vol').innerHTML = icon(v === 0 ? 'mute' : 'volume');
}

function renderRating() {
  const t = current();
  const r = t ? rating(t.videoId) : 'none';
  $('#b-like').innerHTML = icon(r === 'like' ? 'like' : 'likeOff');
  $('#b-dislike').innerHTML = icon(r === 'dislike' ? 'dislike' : 'dislikeOff');
  $('#b-like').classList.toggle('on', r === 'like');
  $('#b-dislike').classList.toggle('on', r === 'dislike');
  $('#pp-like').innerHTML = icon(r === 'like' ? 'like' : 'likeOff');
  $('#pp-dislike').innerHTML = icon(r === 'dislike' ? 'dislike' : 'dislikeOff');
}

function renderPlayer() {
  const t = current();
  const id = t ? t.videoId : null;
  $('#bar').classList.toggle('empty', !t);
  $('#app').classList.toggle('bar-hidden', !t);
  if (id !== shown) {
    shown = id;
    if (t) {
      $('#b-title').textContent = t.title;
      $('#b-sub').innerHTML = trackSub(t, { views: false }) + (t.year ? ' • ' + esc(t.year) : '');
      $('#pp-img').innerHTML = img(t.thumb, 720);
      $('#pp-art').classList.toggle('video', t.type === 'video' && !/googleusercontent/.test(t.thumb));
      $('#pp-title').textContent = t.title;
      $('#pp-sub').innerHTML = artistLinks(t.artists);
      document.title = `${t.title} - Gundil Music`;
    } else {
      document.title = 'Gundil Music';
      closePlayer();
    }
    updateMediaSession(t);
    markPlaying();
    if (S.ppOpen && S.ppTab !== 'upnext') renderPPTab();
  }
  const thumb = $('#b-thumb');
  const want = t ? (S.loading ? 'loading:' : '') + t.thumb : '';
  if (thumb.dataset.v !== want) {
    thumb.dataset.v = want;
    thumb.innerHTML = t ? img(t.thumb, 60) + (S.loading ? '<div class="spinner"></div>' : '') : '';
  }
  const playing = t && S.playing && !S.paused;
  for (const el of [$('#b-play'), $('#pp-play')]) {
    el.innerHTML = icon(playing ? 'pause' : 'play');
    el.setAttribute('aria-label', playing ? 'Pause' : 'Play');
  }
  document.body.classList.toggle('paused', !playing);
  if ('mediaSession' in navigator && !isServer()) navigator.mediaSession.playbackState = playing ? 'playing' : 'paused';
  for (const el of [$('#b-shuffle'), $('#pp-shuffle')]) { el.classList.toggle('on', S.shuffle); el.classList.toggle('dim', !S.shuffle); }
  for (const el of [$('#b-repeat'), $('#pp-repeat')]) {
    el.classList.toggle('on', S.repeat !== 'off');
    el.classList.toggle('dim', S.repeat === 'off');
    el.innerHTML = icon(S.repeat === 'one' ? 'repeatOne' : 'repeat');
    el.title = { off: 'Repeat off', all: 'Repeat all', one: 'Repeat one' }[S.repeat];
  }
  $('#b-expand').innerHTML = icon(S.ppOpen ? 'down' : 'up');
  $('#pp-where').textContent = S.source ? `Playing from ${S.source}` : '';
  for (const b of $$('#pp-mode button')) b.classList.toggle('on', b.dataset.mode === (isServer() ? 'song' : S.mode));
  $('#pp-mode').classList.toggle('off', isServer());
  renderRating();
  renderVolume();
  renderProgress();
  renderSleep();
  if (S.ppOpen && S.ppTab === 'upnext' && shownQueue !== S.queueVer + ':' + S.index) renderPPTab();
  syncVideo();
  reportToAndroid();
}

/* =========================================================================
   Song / Video switch
   ========================================================================= */
const video = $('#pp-video');
const swapBack = new Map(); // music video id -> the song it stands in for
let videoShown = false, lastFix = 0;
const videoHeight = () => { const q = local.get('videoQuality', 'auto'); return q !== 'auto' ? q : mobile() ? '480' : '720'; };

function videoActive() {
  const t = current();
  return !!(t && t.type === 'video' && S.mode === 'video' && !isServer() && S.ppOpen && S.visible && !document.hidden);
}

function showVideo(on) {
  if (videoShown === on) return;
  videoShown = on;
  $('#pp-art').classList.toggle('show-video', on);
  if (!on) setFullscreen(false);
}

/* The picture is a muted video stream kept in step with the song, so sound
   never stops when the app is minimised or the screen goes off. */
function syncVideo() {
  const t = current();
  if (!videoActive()) {
    if (!video.paused) video.pause();
    showVideo(false);
    const keep = t && t.type === 'video' && S.mode === 'video' && !isServer() && video.dataset.vid === t.videoId;
    if (!keep && video.getAttribute('src')) { video.removeAttribute('src'); video.dataset.key = video.dataset.vid = ''; video.load(); }
    return;
  }
  const key = t.videoId + '@' + videoHeight();
  if (video.dataset.key !== key) {
    if (video.dataset.failed === t.videoId) return;
    video.dataset.key = key;
    video.dataset.vid = t.videoId;
    showVideo(false);
    video.src = `/api/stream/${t.videoId}?kind=video&h=${videoHeight()}`;
  }
  if (S.playing && !S.paused && !S.loading) {
    fixDrift(true);
    if (video.paused) video.play().catch(() => {});
  } else if (!video.paused) video.pause();
}

function fixDrift(force) {
  if (video.readyState < 1 || video.seeking) return;
  const d = video.currentTime - audio.currentTime;
  if (Math.abs(d) > 0.4) {
    if (!force && Date.now() - lastFix < 2000) return;
    lastFix = Date.now();
    video.playbackRate = 1;
    video.currentTime = audio.currentTime + (video.paused ? 0 : 0.1);
  } else video.playbackRate = Math.abs(d) > 0.06 ? (d > 0 ? 0.95 : 1.05) : 1;
}

video.addEventListener('loadedmetadata', () => { video.currentTime = audio.currentTime || 0; });
video.addEventListener('playing', () => { if (videoActive()) showVideo(true); });
video.addEventListener('error', () => {
  if (!video.getAttribute('src')) return;
  video.dataset.failed = video.dataset.vid;
  video.removeAttribute('src');
  showVideo(false);
  toast('This video can\'t be shown here. The song keeps playing.');
});
document.addEventListener('visibilitychange', syncVideo);
window.gundilVisible = v => { S.visible = !!v; syncVideo(); };

/* A song's music video, from YouTube Music's song/video pairs. */
async function findVideo(t) {
  if (t.noVideo) return null;
  try {
    const w = await Promise.race([yt('watch', { videoId: t.videoId }), new Promise((_, no) => setTimeout(() => no(new Error('timeout')), 5000))]);
    const me = w.tracks.find(x => x.videoId === t.videoId) || w.tracks[0];
    if (me && me.counterpart && me.counterpart.type === 'video') return slim(me.counterpart);
    t.noVideo = true;
  } catch { /* slow or offline: play the song */ }
  return null;
}

async function setMode(mode) {
  if (isServer()) {
    toast('Video plays on this device only. Choose “This device” under Speakers to watch.');
    return;
  }
  if (mode === S.mode) return;
  S.mode = mode;
  local.set('mode', mode);
  renderPlayer();
  const t = current();
  if (!t) return;
  const at = audio.currentTime || S.position, play = S.playing && !S.paused;
  if (mode === 'video' && t.type !== 'video') {
    const alt = await findVideo(t);
    if (current() !== t || S.mode !== 'video') return;
    if (!alt) { toast('This song doesn\'t have a video'); return; }
    swapBack.set(alt.videoId, t);
    S.queue[S.index] = alt;
    S.queueVer++;
    loadLocal(play, at, true);
  } else if (mode === 'song' && swapBack.has(t.videoId)) {
    S.queue[S.index] = swapBack.get(t.videoId);
    S.queueVer++;
    loadLocal(play, at, true);
  }
}

function setFullscreen(on) {
  const art = $('#pp-art');
  if (on === art.classList.contains('fs')) return;
  art.classList.toggle('fs', on);
  $('#pp-fs').innerHTML = icon(on ? 'fullscreenExit' : 'fullscreen');
  $('#pp-fs').setAttribute('aria-label', on ? 'Exit full screen' : 'Full screen');
  try {
    if (on && art.requestFullscreen && document.fullscreenEnabled) art.requestFullscreen().catch(() => {});
    else if (!on && document.fullscreenElement) document.exitFullscreen().catch(() => {});
  } catch { /* not supported: the page-filling view is enough */ }
}
document.addEventListener('fullscreenchange', () => { if (!document.fullscreenElement) setFullscreen(false); });

/* =========================================================================
   sleep timer
   ========================================================================= */
let sleepTimer = null;
function sleepToast(minutes, end) {
  toast(end ? 'Music will stop at the end of this song' : minutes > 0 ? `Sleep timer set for ${minutes >= 60 ? plural(minutes / 60, 'hour') : plural(minutes, 'minute')}` : 'Sleep timer off');
}
async function setSleep(minutes, end) {
  clearTimeout(sleepTimer);
  sleepTimer = null;
  if (isServer()) {
    if (await srv('sleep', { minutes, end })) sleepToast(minutes, end);
    return;
  }
  S.sleep = { at: !end && minutes > 0 ? Date.now() + minutes * 60000 : 0, end: !!end };
  if (S.sleep.at) sleepTimer = setTimeout(sleepNow, minutes * 60000);
  sleepToast(minutes, end);
  renderPlayer();
}
/* ---------- crossfade (on this device) ---------- */
let fade = null, fadeKeep = false, fadeTimer = null;
const crossfadeSecs = () => +local.get('crossfade', 0) || 0;

/* The song after this one, if one follows by itself. */
function nextIndex() {
  if (S.index + 1 < S.queue.length) return S.index + 1;
  return S.repeat === 'all' && S.queue.length > 1 ? 0 : -1;
}

function maybeCrossfade() {
  if (fade) return stepFade();
  const secs = crossfadeSecs();
  if (!secs || isServer() || audio.paused || S.mode === 'video' || S.repeat === 'one' || S.sleep.end) return;
  const dur = audio.duration;
  if (!isFinite(dur) || dur < secs * 3) return; // too short to fade over
  const left = dur - audio.currentTime;
  const ni = nextIndex();
  if (ni < 0) return;
  const other = decks[0] === audio ? decks[1] : decks[0];
  const nsrc = srcFor(S.queue[ni].videoId);
  // Start loading the next song a little early, so it's ready to fade in.
  if (left <= secs + 12 && other.getAttribute('src') !== nsrc) { other.src = nsrc; other.volume = 0; other.load(); }
  if (left > secs) return;
  // Start the fade: the other player becomes the current one.
  const out = audio;
  fade = { out, start: Date.now(), ms: Math.max(1000, left * 1000) };
  audio = other;
  audio.volume = 0;
  out.addEventListener('ended', endFade, { once: true });
  S.index = ni;
  fadeKeep = true;
  loadLocal(true);
  fadeTimer = setInterval(stepFade, 100); // smooth while visible; timeupdate keeps it going in the background
  stepFade();
}

function stepFade() {
  if (!fade) return;
  const p = Math.min(1, (Date.now() - fade.start) / fade.ms);
  const v = S.muted ? 0 : S.volume / 100;
  // Equal-power curves: the loudness stays even through the fade.
  audio.volume = v * Math.sin(p * Math.PI / 2);
  fade.out.volume = v * Math.cos(p * Math.PI / 2);
  if (p >= 1) endFade();
}

/* Finishes a crossfade at once: the old song stops, the new one is at full volume. */
function endFade() {
  clearInterval(fadeTimer);
  fadeTimer = null;
  if (!fade) return;
  const out = fade.out;
  fade = null;
  out.removeEventListener('ended', endFade);
  out.pause();
  out.removeAttribute('src');
  out.load();
  audio.volume = S.volume / 100;
}

function sleepNow() {
  clearTimeout(sleepTimer);
  sleepTimer = null;
  S.sleep = { at: 0, end: false };
  renderSleep();
  endFade();
  if (isServer() || audio.paused) return;
  const vol = audio.volume;
  let i = 0;
  const fade = setInterval(() => { // fade out over three seconds, like falling asleep
    audio.volume = Math.max(0, vol * (1 - ++i / 20));
    if (i >= 20) { clearInterval(fade); audio.pause(); audio.volume = vol; }
  }, 150);
}
function sleepLeft() {
  if (S.sleep.end) return 'End of song';
  if (!S.sleep.at) return '';
  const s = Math.max(0, Math.round((S.sleep.at - Date.now()) / 1000));
  return fmt(s) || '0:00';
}
let shownSleep = null;
function renderSleep() {
  const left = sleepLeft();
  if (left === shownSleep) return;
  shownSleep = left;
  const b = $('#pp-sleep');
  b.classList.toggle('on', !!left);
  b.innerHTML = icon('timer') + (left ? `<span>${esc(left)}</span>` : '');
  b.title = left ? `Sleep timer: ${left}` : 'Sleep timer';
}
setInterval(() => { if (S.sleep.at) renderSleep(); }, 1000);
function sleepMenu(anchor) {
  const on = S.sleep.at || S.sleep.end;
  showMenu([
    ...[5, 10, 15, 30, 45, 60].map(m => ({ icon: 'timer', label: m === 60 ? '1 hour' : `${m} minutes`, run: () => setSleep(m, false) })),
    current() ? { icon: 'timer', label: 'End of song', run: () => setSleep(0, true) } : null,
    on ? '-' : null,
    on ? { icon: 'close', label: 'Turn off timer', run: () => setSleep(0, false) } : null,
  ], anchor, `<div class="menu-title">Sleep timer${on ? ` · ${esc(sleepLeft())}` : ''}</div>`);
}

/* The Android app shows a media notification and keeps playing in the background. */
const android = window.GundilAndroid;
let lastReport = '';
function reportToAndroid() {
  if (!android) return;
  const t = current();
  const msg = JSON.stringify(t ? {
    hasTrack: true, title: t.title, artist: names(t), thumb: sized(t.thumb, 512),
    playing: !!(S.playing && !S.paused), local: !isServer(), where: isServer() ? `Playing on ${S.info.hostname || 'server'}` : '',
  } : { hasTrack: false });
  if (msg === lastReport) return;
  lastReport = msg;
  try { android.update(msg); } catch { /* old app version */ }
}
window.gundilControl = cmd => {
  const playingNow = S.playing && !S.paused;
  if (cmd === 'toggle' || (cmd === 'play' && !playingNow) || (cmd === 'pause' && playingNow)) toggle();
  else if (cmd === 'next') next();
  else if (cmd === 'prev') prev();
};
window.gundilBack = () => {
  if ($('#pp-art').classList.contains('fs')) { setFullscreen(false); return true; }
  if ($('#pp-body').classList.contains('lyrics-fs')) { setLyricsFullscreen(false); return true; }
  if (!$('#menu').hidden) { hideMenu(); return true; }
  if ($('dialog[open]')) { $('dialog[open]').close(); return true; }
  if ($('#app').classList.contains('searching')) { $('#app').classList.remove('searching'); closeSuggest(); return true; }
  if (S.ppOpen) { closePlayer(); return true; }
  return false;
};

function markPlaying() {
  for (const el of $$('.row.playing')) el.classList.remove('playing');
  if (!shown) return;
  for (const el of $$(`.row[data-vid="${shown}"]:not(.qrow)`)) el.classList.add('playing');
}

/* =========================================================================
   now playing page (Up next / Lyrics / Related)
   ========================================================================= */
function openPlayer(tab) {
  if (!current()) return;
  S.ppOpen = true;
  if (tab) S.ppTab = tab;
  $('#pp').hidden = false;
  $('#pp').classList.toggle('sheet', !!tab && mobile());
  shownQueue = -1;
  renderPPTab();
  renderPlayer();
}
function closePlayer() {
  if (!S.ppOpen) return;
  setLyricsFullscreen(false);
  S.ppOpen = false;
  $('#pp').hidden = true;
  $('#pp').classList.remove('sheet');
  renderPlayer();
}

function setPPTab(tab) {
  if (mobile()) {
    const pp = $('#pp');
    if (pp.classList.contains('sheet') && S.ppTab === tab) { pp.classList.remove('sheet'); return; }
    pp.classList.add('sheet');
  }
  if (tab !== 'lyrics') setLyricsFullscreen(false);
  S.ppTab = tab;
  renderPPTab();
}

async function renderPPTab() {
  for (const b of $$('.pp-tabs button')) b.classList.toggle('on', b.dataset.tab === S.ppTab);
  const body = $('#pp-body');
  const t = current();
  if (!t) return;
  if (S.ppTab === 'upnext') {
    if (dragging) return; // redrawn when the drag ends
    shownQueue = S.queueVer + ':' + S.index;
    const scroll = body.scrollTop;
    body.innerHTML = upNextHTML();
    body.scrollTop = scroll;
    if (!body.dataset.scrolled) {
      const row = $('.qrow.playing', body);
      if (row) row.scrollIntoView({ block: 'center' });
      body.dataset.scrolled = '1';
    }
    return;
  }
  body.dataset.scrolled = '';
  body.innerHTML = '<div class="spinner"></div>';
  const vid = t.videoId, tab = S.ppTab;
  try {
    const w = await yt('watch', { videoId: vid });
    if (!current() || current().videoId !== vid || S.ppTab !== tab) return;
    if (tab === 'lyrics') {
      const ly = await lyricsOf(t, w.lyrics);
      if (!current() || current().videoId !== vid || S.ppTab !== tab) return;
      const tools = `<div class="ly-tools"><button class="icon-btn" data-act="lyrics-fs" aria-label="Full-screen lyrics" title="Full-screen lyrics">${icon('lyricsFs')}</button></div>`;
      if (ly.lines && ly.lines.length) {
        body.innerHTML = `${tools}<div class="lyrics timed">${lyricItems(ly.lines).map(l => l.gap
          ? `<div class="ln gap" data-start="${l.start}" data-end="${l.end}" data-act="lyric-seek"><i></i><i></i><i></i></div>`
          : `<div class="ln" data-start="${l.start}" data-act="lyric-seek">${esc(l.text)}</div>`).join('')}</div>
          ${ly.source ? `<div class="lyrics-src">${esc(ly.source)}</div>` : ''}
          <button class="btn white ly-back" data-act="lyrics-back" hidden>${icon('down')}Back to current line</button>`;
        body.dataset.line = '';
        lyricHold = 0;
        if (body.classList.contains('lyrics-fs')) body.style.setProperty('--ly-art', t.thumb ? `url("${sized(t.thumb, 120)}")` : 'none');
        syncLyrics(true);
      } else if (ly.text) {
        body.innerHTML = `${tools}<div class="lyrics plain">${esc(ly.text)}</div>${ly.source ? `<div class="lyrics-src">${esc(ly.source)}</div>` : ''}`;
      } else body.innerHTML = emptyHTML('lyrics', 'Lyrics not available', '');
    } else {
      if (!w.related) { body.innerHTML = emptyHTML('info', 'Nothing related found', ''); return; }
      const rel = await yt('related', { id: w.related });
      if (!current() || current().videoId !== vid || S.ppTab !== tab) return;
      body.innerHTML = rel.sections.map(sec => sec.text
        ? `<section class="shelf"><div class="shelf-head"><div class="shelf-titles"><h2>${esc(sec.title)}</h2></div></div><p class="about">${esc(sec.text)}</p></section>`
        : shelf(sec, { source: sec.title, grid: false, nav: false })).join('') || emptyHTML('info', 'Nothing related found', '');
    }
  } catch (e) {
    body.innerHTML = emptyHTML('info', 'Couldn\'t load this', esc(e.message));
  }
}

/* Lyrics for a track: YouTube Music's, or timed ones from LRCLIB (looked up by the server). */
const lyricsMemo = new Map();
function lyricsOf(t, id) {
  if (lyricsMemo.has(t.videoId)) return lyricsMemo.get(t.videoId);
  const q = new URLSearchParams({ id: id || '', title: t.title || '', artist: (t.artists && t.artists[0] && t.artists[0].name) || '', album: (t.album && t.album.name) || '', duration: t.duration || '' });
  const p = api('lyrics?' + q);
  lyricsMemo.set(t.videoId, p);
  p.catch(() => lyricsMemo.delete(t.videoId));
  if (lyricsMemo.size > 50) lyricsMemo.delete(lyricsMemo.keys().next().value);
  return p;
}

/* Timed lines, with "♪ ♪ ♪" gaps for the intro, instrumental breaks and empty lines. */
function lyricItems(lines) {
  const out = [];
  if (lines[0].start > 5) out.push({ gap: true, start: 0, end: lines[0].start });
  lines.forEach((l, i) => {
    const next = lines[i + 1];
    const text = (l.text || '').trim();
    const end = next ? next.start : (l.end || l.start + 5);
    if (!text || /^[♪♫\s]+$/.test(text)) {
      if (end - l.start > 2.5) out.push({ gap: true, start: l.start, end });
      return;
    }
    out.push({ start: l.start, text });
    // A line with a known end, followed by a long silence before the next one.
    if (next && l.end && next.start - l.end > 6) out.push({ gap: true, start: l.end, end: next.start });
  });
  return out;
}

const LYRIC_LEAD = 0.35; // light a line up as it's sung, not just after
let lyricHold = 0; // following pauses until then, after scrolling by hand

function syncLyrics(force) {
  const body = $('#pp-body');
  const lines = $$('.lyrics.timed .ln', body);
  if (!lines.length) return;
  const pos = S.position + LYRIC_LEAD;
  let on = -1;
  for (let i = 0; i < lines.length; i++) if (+lines[i].dataset.start <= pos) on = i;
  const cur = lines[on];
  if (cur && cur.classList.contains('gap')) {
    const st = +cur.dataset.start, en = +cur.dataset.end;
    cur.dataset.n = Math.min(3, Math.floor((pos - st) / Math.max(0.1, en - st) * 3) + 1);
  }
  const held = Date.now() < lyricHold;
  const back = $('.ly-back', body);
  if (back && back.hidden === held) back.hidden = !held;
  if (!held && lyricHold) { lyricHold = 0; force = true; } // the pause just ended: catch up
  if (!force && body.dataset.line === String(on)) return;
  body.dataset.line = on;
  lines.forEach((l, i) => { l.classList.toggle('on', i === on); l.classList.toggle('past', i < on); if (i !== on) delete l.dataset.n; });
  if (on >= 0 && !held) lines[on].scrollIntoView({ block: 'center', behavior: force ? 'auto' : 'smooth' });
}

/* Scrolling the lyrics by hand pauses following for a few seconds. */
function holdLyrics() {
  if (S.ppTab !== 'lyrics' || !$('.lyrics.timed', $('#pp-body'))) return;
  lyricHold = Date.now() + 6000;
  syncLyrics();
}
for (const ev of ['wheel', 'touchmove']) $('#pp-body').addEventListener(ev, holdLyrics, { passive: true });
$('#pp-body').addEventListener('keydown', e => { if (/^(Arrow|Page|Home|End)/.test(e.key)) holdLyrics(); });

function setLyricsFullscreen(on) {
  const body = $('#pp-body');
  if (on === body.classList.contains('lyrics-fs')) return;
  body.classList.toggle('lyrics-fs', on);
  const b = $('[data-act="lyrics-fs"]', body);
  if (b) { b.innerHTML = icon(on ? 'fullscreenExit' : 'lyricsFs'); b.setAttribute('aria-label', on ? 'Exit full screen' : 'Full-screen lyrics'); }
  if (on) { const t = current(); body.style.setProperty('--ly-art', t && t.thumb ? `url("${sized(t.thumb, 120)}")` : 'none'); }
  try {
    if (on && body.requestFullscreen && document.fullscreenEnabled) body.requestFullscreen().catch(() => {});
    else if (!on && document.fullscreenElement === body) document.exitFullscreen().catch(() => {});
  } catch { /* the page-filling view is enough */ }
  lyricHold = 0;
  syncLyrics(true);
}
document.addEventListener('fullscreenchange', () => { if (!document.fullscreenElement && $('#pp-body').classList.contains('lyrics-fs')) setLyricsFullscreen(false); });

function upNextHTML() {
  const start = Math.max(0, S.index - 50), end = Math.min(S.queue.length, S.index + 250);
  let rows = '';
  for (let i = start; i < end; i++) {
    const t = S.queue[i];
    const on = i === S.index;
    rows += `<div class="row qrow${on ? ' playing' : ''}" data-q="${i}" data-vid="${t.videoId}">
      <div class="thumb">${img(t.thumb, 40)}${on ? '<div class="eq-wrap"><div class="eq"><i></i><i></i><i></i></div></div>' : ''}</div>
      <div class="row-text"><div class="row-title">${esc(t.title)}${explicit(t)}</div><div class="row-sub">${esc(names(t))}</div></div>
      <div class="row-actions">${on ? '' : `<button class="icon-btn" data-act="q-remove" aria-label="Remove from queue" title="Remove from queue">${icon('close')}</button>`}</div>
      <div class="row-dur">${fmt(t.duration)}</div>
      <button class="icon-btn drag-handle" aria-label="Drag to reorder" title="Drag to reorder">${icon('drag')}</button>
    </div>`;
  }
  return `<div class="pp-source"><div><div class="muted small">Playing from</div><div class="pp-source-title">${esc(S.source || 'Queue')}</div></div>
      <button class="btn small" data-act="save-queue">${icon('playlistAdd')}Save</button></div>
    <div class="list">${rows}</div>
    <div class="autoplay"><div><b>Autoplay</b><span class="muted small">Add similar content to the end of the queue</span></div>
      <button class="switch${S.autoplay ? ' on' : ''}" data-act="autoplay" role="switch" aria-checked="${S.autoplay}" aria-label="Autoplay"></button></div>`;
}

/* =========================================================================
   building blocks
   ========================================================================= */
const emptyHTML = (ic, title, text, extra = '') => `<div class="empty">${icon(ic)}<h2>${esc(title)}</h2>${text ? `<p class="muted">${text}</p>` : ''}${extra}</div>`;

function rowTrack(row) {
  const l = reg.get(row.dataset.list);
  return l ? l.tracks[+row.dataset.idx] : null;
}

/* A song row. ctx: list key, idx, and display options. */
function songRow(t, list, idx, o = {}) {
  const lead = o.number !== undefined
    ? `<div class="row-num"><span class="n">${o.number || ''}</span><span class="thumb-play">${icon('play')}</span></div>`
    : `${o.rank ? `<div class="row-rank">${o.rank}</div>` : ''}<div class="thumb${t.type === 'video' ? ' video' : ''}">${img(t.thumb, 60)}<span class="thumb-play">${icon('play')}</span></div>`;
  const sub = o.sub !== undefined ? o.sub : trackSub(t, { album: !o.albumCol, views: !!o.views });
  return `<div class="row${t.videoId === shown ? ' playing' : ''}${t.available === false ? ' disabled' : ''}" data-list="${list}" data-idx="${idx}" data-vid="${t.videoId}">
    ${lead}
    <div class="row-text"><div class="row-title">${esc(t.title)}${explicit(t)}${dlMark(t)}</div>${sub ? `<div class="row-sub">${sub}</div>` : ''}</div>
    ${o.albumCol ? `<div class="row-col">${albumLink(t.album)}</div>` : ''}
    ${o.plays ? `<div class="row-plays">${esc(t.plays || '')}</div>` : ''}
    <div class="row-actions"><span class="rate-box">${rateButtons(t)}</span><button class="icon-btn" data-act="menu" aria-label="More actions">${icon('more')}</button></div>
    ${o.noDur ? '' : `<div class="row-dur">${fmt(t.duration)}</div>`}
    ${o.drag ? `<button class="icon-btn drag-handle" aria-label="Drag to reorder" title="Drag to reorder">${icon('drag')}</button>` : ''}
  </div>`;
}

/* A row for an album, artist or playlist (search results, library). */
function itemRow(it, o = {}) {
  const k = register(it);
  return `<a class="row" href="${itemHref(it)}" data-k="${k}">
    ${o.rank ? `<div class="row-rank">${o.rank}</div>` : ''}
    <div class="thumb${it.type === 'artist' ? ' round' : ''}">${img(it.thumb, 60)}</div>
    <div class="row-text"><div class="row-title">${esc(it.title)}${explicit(it)}</div><div class="row-sub">${esc(it.subtitle || '')}</div></div>
    <div class="row-actions"><button class="icon-btn" data-act="item-menu" aria-label="More actions">${icon('more')}</button></div>
  </a>`;
}

function card(it, list, idx) {
  const k = register(it);
  const href = itemHref(it);
  const tag = href ? 'a' : 'div';
  const round = it.type === 'artist';
  const wide = it.type === 'video' && !/googleusercontent/.test(it.thumb || '');
  const sub = isTrack(it) ? trackSub(it, { links: false, album: false, views: true }) : esc(it.subtitle || '');
  const attrs = href ? `href="${href}"` : `data-list="${list}" data-idx="${idx}" data-vid="${it.videoId}"`;
  return `<${tag} class="card${round ? ' round' : ''}${wide ? ' wide' : ''}" ${attrs} data-k="${k}">
    <div class="card-thumb">${it.liked ? `<div class="liked-art">${icon('like')}</div>` : img(it.thumb, wide ? 400 : 260)}
      <div class="card-hover">${round ? '' : `<button class="card-play" data-act="card-play" aria-label="Play ${esc(it.title)}">${icon('play')}</button>`}
      ${it.own ? '' : `<button class="icon-btn card-more" data-act="item-menu" aria-label="More actions">${icon('more')}</button>`}</div>
    </div>
    <div class="card-title">${esc(it.title)}</div>
    <div class="card-sub">${sub}</div>
  </${tag}>`;
}

const MOOD_COLORS = ['#e3a42f', '#4c8bf5', '#e4518c', '#2fb37b', '#9b6cf2', '#f2683f', '#47c1d1', '#d8d843', '#ff7a7a', '#6fd06f'];
function moodColor(s) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return MOOD_COLORS[h % MOOD_COLORS.length];
}
const moodBtn = m => `<a class="mood" style="--c:${moodColor(m.title)}" href="#/mood/${enc(m.params)}/${enc(m.title)}">${esc(m.title)}</a>`;

/* A titled row of cards, or a "quick picks" grid when it is all songs. */
function shelf(sec, o = {}) {
  const items = (sec.items || []).filter(Boolean);
  if (!items.length) return '';
  const tracks = items.filter(isTrack);
  const list = register({ tracks, source: o.source || sec.title, mode: 'radio' });
  const grid = o.grid !== undefined ? o.grid : tracks.length === items.length && items.length >= 8;
  let body;
  if (grid) body = `<div class="qp-grid">${tracks.map((t, i) => songRow(t, list, i, { noDur: true })).join('')}</div>`;
  else {
    const wide = items.every(it => it.type === 'video' && !/googleusercontent/.test(it.thumb || ''));
    body = `<div class="carousel${wide ? ' wide' : ''}">${items.map(it => card(it, list, tracks.indexOf(it))).join('')}</div>`;
  }
  const nav = o.nav === false ? '' : `<div class="carousel-nav">
      ${o.more ? `<a class="btn small" href="${o.more}">More</a>` : ''}
      <button class="circle-btn" data-act="scroll" data-dir="-1" aria-label="Scroll left">${icon('left')}</button>
      <button class="circle-btn" data-act="scroll" data-dir="1" aria-label="Scroll right">${icon('right')}</button></div>`;
  return `<section class="shelf"><div class="shelf-head"><div class="shelf-titles">${o.strap ? `<div class="strap">${esc(o.strap)}</div>` : ''}<h2>${o.href ? `<a href="${o.href}">${esc(sec.title)}</a>` : esc(sec.title)}</h2></div>${nav}</div>${body}</section>`;
}

function statusBanner() {
  const y = S.info.youtube || {};
  if (y.ok === false || y.error) {
    return `<div class="banner">${icon('info')}<div><b>YouTube Music isn't connected on ${esc(S.info.hostname || 'the server')}.</b>
      <p class="muted">${esc(y.error || 'The helper did not start.')}</p></div></div>`;
  }
  if (y.warning) return `<div class="banner warn">${icon('info')}<div><p>${esc(y.warning)}</p></div></div>`;
  return '';
}

/* =========================================================================
   pages
   ========================================================================= */
async function viewHome() {
  const [home, moods] = await Promise.all([yt('home'), yt('moods').catch(() => null)]);
  const chips = [];
  if (moods) for (const sec of moods.sections) for (const m of sec.items) if (chips.length < 16) chips.push(m);
  const history = S.user.history.slice(0, 20);
  let html = statusBanner();
  if (chips.length) html += `<div class="chips">${chips.map(m => `<a class="chip" href="#/mood/${enc(m.params)}/${enc(m.title)}">${esc(m.title)}</a>`).join('')}</div>`;
  if (history.length >= 4) html += shelf({ title: 'Listen again', items: history }, { strap: 'Your history', grid: false, more: '#/library/history' });
  html += home.sections.map((sec, i) => shelf(sec, { grid: /quick picks/i.test(sec.title) || undefined, strap: i === 0 && history.length < 4 ? 'Start radio from a song' : '' })).join('');
  if (!home.sections.length) html += emptyHTML('home', 'Nothing here yet', 'YouTube Music didn\'t return a home page. Try searching for something.');
  return html;
}

async function viewExplore() {
  const ex = await yt('explore');
  let html = statusBanner() + `<div class="explore-tiles">
      <a class="explore-tile" href="#/explore/new">${icon('newReleases')}New releases</a>
      <a class="explore-tile" href="#/explore/charts">${icon('charts')}Charts</a>
      <a class="explore-tile" href="#/explore/moods">${icon('mood')}Moods & genres</a></div>`;
  ex.sections.forEach((sec, i) => {
    if (i === 1 && ex.moods.length) html += moodShelf(ex.moods);
    const songs = /song|trending/i.test(sec.title);
    html += shelf(sec, {
      grid: songs ? true : false,
      more: /new albums/i.test(sec.title) ? '#/explore/new' : sec.playlistId ? '#/playlist/' + enc(sec.playlistId) : '',
    });
  });
  if (ex.sections.length < 2 && ex.moods.length) html += moodShelf(ex.moods);
  return html;
}

const moodShelf = moods => `<section class="shelf"><div class="shelf-head"><div class="shelf-titles"><h2>Moods & genres</h2></div>
  <div class="carousel-nav"><a class="btn small" href="#/explore/moods">More</a></div></div>
  <div class="mood-grid">${moods.map(moodBtn).join('')}</div></section>`;

async function viewNewReleases() {
  const ex = await yt('explore');
  const sec = ex.sections.find(s => /new albums/i.test(s.title)) || { items: [] };
  const vids = ex.sections.find(s => /video/i.test(s.title));
  const list = register({ tracks: sec.items.filter(isTrack), source: 'New releases', mode: 'radio' });
  return `<h1 class="page-title">New releases</h1>
    <div class="card-grid">${sec.items.map(it => card(it, list, -1)).join('')}</div>
    ${vids ? shelf(vids) : ''}`;
}

async function viewCharts(country) {
  const ch = await yt('charts', { country });
  const dn = (() => { try { return new Intl.DisplayNames(['en'], { type: 'region' }); } catch { return null; } })();
  const label = c => c === 'ZZ' ? 'Global' : (dn ? dn.of(c) : c);
  const opts = (ch.countries || []).map(c => `<option value="${esc(c)}"${(country || 'ZZ') === c ? ' selected' : ''}>${esc(label(c))}</option>`).join('');
  let html = `<div class="section-head"><h1 class="page-title" style="margin:0">Charts</h1>${opts ? `<select id="chart-country" class="btn" aria-label="Country">${opts}</select>` : ''}</div>`;
  for (const sec of ch.sections) {
    if (sec.ranked) {
      html += `<div class="section-head"><h2>${esc(sec.title)}</h2></div><div class="list search-list">${sec.items.map((it, i) => itemRow(it, { rank: i + 1 })).join('')}</div>`;
    } else html += shelf(sec, { grid: sec.items.every(isTrack) });
  }
  return html || emptyHTML('charts', 'No charts here', '');
}

async function viewMoods() {
  const m = await yt('moods');
  return `<h1 class="page-title">Moods & genres</h1>` + m.sections.map(sec =>
    `<div class="section-head"><h2>${esc(sec.title)}</h2></div><div class="mood-grid wrap">${sec.items.map(moodBtn).join('')}</div>`).join('');
}

async function viewMood(params, title) {
  const m = await yt('mood', { params });
  return `<h1 class="page-title">${esc(title || 'Playlists')}</h1><div class="card-grid">${m.items.map(it => card(it, '', -1)).join('')}</div>`;
}

const FILTERS = [['songs', 'Songs'], ['videos', 'Videos'], ['albums', 'Albums'], ['featured_playlists', 'Featured playlists'], ['community_playlists', 'Community playlists'], ['artists', 'Artists']];
const FILTER_OF = { Songs: 'songs', Videos: 'videos', Albums: 'albums', Artists: 'artists', 'Community playlists': 'community_playlists', 'Featured playlists': 'featured_playlists' };

async function viewSearch(q, filter) {
  if (!q) return emptyHTML('search', 'Search YouTube Music', 'Find songs, albums, artists and playlists.');
  const chips = `<div class="chips">${FILTERS.map(([f, label]) => f === filter
    ? `<a class="chip on" href="#/search/${enc(q)}">${icon('check')}${label}</a>`
    : `<a class="chip" href="#/search/${enc(q)}/${f}">${label}</a>`).join('')}</div>`;
  if (filter) {
    const res = await yt('search', { q, filter });
    const tracks = res.items.filter(isTrack);
    const list = register({ tracks, source: `Search: ${q}`, mode: 'radio' });
    const rows = res.items.map(it => isTrack(it) ? songRow(it, list, tracks.indexOf(it), { views: true, albumCol: !mobile() && filter === 'songs' }) : itemRow(it)).join('');
    return statusBanner() + chips + (rows ? `<div class="list search-list">${rows}</div>` : emptyHTML('search', 'No results', `Try different keywords for “${esc(q)}”.`));
  }
  const res = await yt('search', { q });
  if (!res.sections.length) return statusBanner() + chips + emptyHTML('search', 'No results', `Try different keywords for “${esc(q)}”.`);
  let html = statusBanner() + chips;
  const secs = res.sections.slice();
  if (/top result/i.test(secs[0].title) && secs[0].items.length) {
    const top = secs.shift().items[0];
    const songs = (secs.find(s => s.title === 'Songs') || { items: [] }).items.filter(t => !top.videoId || t.videoId !== top.videoId).slice(0, 4);
    const list = register({ tracks: songs, source: `Search: ${q}`, mode: 'radio' });
    html += `<div class="top-result">${topResult(top, q)}${songs.length ? `<div class="list search-list">${songs.map((t, i) => songRow(t, list, i, { views: true })).join('')}</div>` : ''}</div>`;
  }
  for (const sec of secs) {
    const tracks = sec.items.filter(isTrack);
    const list = register({ tracks, source: `Search: ${q}`, mode: 'radio' });
    const f = FILTER_OF[sec.title];
    html += `<div class="section-head"><h2>${esc(sec.title)}</h2>${f ? `<a class="btn small" href="#/search/${enc(q)}/${f}">Show all</a>` : ''}</div>
      <div class="list search-list">${sec.items.map(it => isTrack(it) ? songRow(it, list, tracks.indexOf(it), { views: true }) : itemRow(it)).join('')}</div>`;
  }
  return html;
}

function topResult(it, q) {
  const k = register(it);
  if (isTrack(it)) {
    const list = register({ tracks: [it], source: `Search: ${q}`, mode: 'radio' });
    return `<div class="tr-card" data-k="${k}">
      <div class="tr-main"><div class="tr-art">${img(it.thumb, 120)}</div><div><div class="tr-title">${esc(it.title)}${explicit(it)}</div><div class="muted">${trackSub(it, { views: true })}</div></div></div>
      <div class="tr-actions"><button class="btn white" data-act="list-play" data-list="${list}">${icon('play')}Play</button>
        <button class="btn" data-act="save-track" data-k="${k}">${icon('playlistAdd')}Save</button></div></div>`;
  }
  const href = itemHref(it);
  const round = it.type === 'artist';
  const saved = isSaved(it);
  return `<div class="tr-card" data-k="${k}">
    <div class="tr-main"><a class="tr-art${round ? ' round' : ''}" href="${href}">${img(it.thumb, 120)}</a><div><div class="tr-title"><a href="${href}">${esc(it.title)}</a></div><div class="muted">${esc(it.subtitle || '')}</div></div></div>
    <div class="tr-actions">${round
      ? `<button class="btn white" data-act="artist-shuffle" data-k="${k}">${icon('shuffle')}Shuffle</button><button class="btn" data-act="artist-radio" data-k="${k}">${icon('radio')}Radio</button>`
      : `<button class="btn white" data-act="card-play" data-k="${k}">${icon('play')}Play</button><button class="btn" data-act="toggle-save" data-k="${k}">${icon(saved ? 'libDone' : 'libAdd')}${saved ? 'Saved' : 'Save'}</button>`}</div></div>`;
}

function detailPage({ art, title, meta, meta2, desc, actions, rows, after = '' }) {
  return `<div class="detail">
    <div class="detail-head">
      <div class="detail-art">${art}</div>
      <h1>${esc(title)}</h1>
      ${meta ? `<div class="detail-meta">${meta}</div>` : ''}
      ${meta2 ? `<div class="detail-meta">${meta2}</div>` : ''}
      ${desc ? `<div class="detail-desc" data-act="expand">${esc(desc)}</div>` : ''}
      <div class="detail-actions">${actions}</div>
    </div>
    <div class="list">${rows}</div>
  </div>${after}`;
}

async function viewAlbum(id) {
  const a = await yt('album', { id });
  const item = { type: 'album', id, title: a.title, subtitle: `${a.kind} • ${a.artists.map(x => x.name).join(', ')}`, thumb: a.thumb, playlistId: a.playlistId };
  const k = register(item);
  const list = register({ tracks: a.tracks, source: a.title, mode: 'list' });
  const saved = isSaved(item);
  const multi = a.tracks.some(t => names(t) !== a.artists.map(x => x.name).join(', '));
  const rows = a.tracks.map((t, i) => songRow(t, list, i, { number: t.number || i + 1, sub: multi ? artistLinks(t.artists) : '', plays: !mobile() })).join('');
  return detailPage({
    art: img(a.thumb, 400), title: a.title,
    meta: [esc(a.kind), artistLinks(a.artists), esc(a.year)].filter(Boolean).join(' • '),
    meta2: [a.trackCount ? plural(+a.trackCount, 'song') : '', esc(a.duration)].filter(Boolean).join(' • '),
    desc: a.description,
    actions: `<button class="circle-btn lg" data-act="toggle-save" data-k="${k}" title="${saved ? 'Remove from library' : 'Save to library'}" aria-label="${saved ? 'Remove from library' : 'Save to library'}">${icon(saved ? 'libDone' : 'libAdd')}</button>
      <button class="play-circle" data-act="list-play" data-list="${list}" aria-label="Play">${icon('play')}</button>${dlButton(list)}
      <button class="circle-btn lg" data-act="item-menu" data-k="${k}" aria-label="More actions">${icon('more')}</button>`,
    rows,
    after: a.other && a.other.length ? shelf({ title: 'Other versions', items: a.other }) : '',
  });
}

function ownPlaylist(id) {
  if (id === 'LM') return { id: 'LM', name: 'Liked Music', description: '', tracks: S.user.liked, auto: true };
  return S.user.playlists.find(p => p.id === id);
}

async function viewPlaylist(id) {
  const own = ownPlaylist(id);
  if (own) {
    const list = register({ tracks: own.tracks, source: own.name, mode: 'list', own: own.auto ? 'LM' : own.id });
    const k = register({ type: 'playlist', id: own.id, title: own.name, own: true });
    const total = own.tracks.reduce((n, t) => n + (t.duration || 0), 0);
    return detailPage({
      art: own.auto ? `<div class="liked-art">${icon('like')}</div>` : (own.tracks[0] ? img(own.tracks[0].thumb, 400) : `<div class="liked-art" style="background:#333">${icon('queue')}</div>`),
      title: own.name,
      meta: own.auto ? 'Auto playlist' : 'Playlist • You',
      meta2: [plural(own.tracks.length, 'song'), total ? Math.round(total / 60) + ' minutes' : ''].filter(Boolean).join(' • '),
      desc: own.description,
      actions: own.auto
        ? `<button class="circle-btn lg" data-act="list-shuffle" data-list="${list}" aria-label="Shuffle">${icon('shuffle')}</button>
           <button class="play-circle" data-act="list-play" data-list="${list}" aria-label="Play">${icon('play')}</button>${dlButton(list)}`
        : `<button class="circle-btn lg" data-act="edit-playlist" data-id="${own.id}" aria-label="Edit playlist">${icon('edit')}</button>
           <button class="play-circle" data-act="list-play" data-list="${list}" aria-label="Play">${icon('play')}</button>${dlButton(list)}
           <button class="circle-btn lg" data-act="own-menu" data-k="${k}" data-list="${list}" aria-label="More actions">${icon('more')}</button>`,
      rows: own.tracks.length ? own.tracks.map((t, i) => songRow(t, list, i, { albumCol: !mobile(), drag: !own.auto && own.tracks.length > 1 })).join('')
        : emptyHTML(own.auto ? 'like' : 'playlistAdd', own.auto ? 'No liked songs yet' : 'This playlist is empty', own.auto ? 'Tap the thumbs up on a song to add it here.' : 'Use “Save to playlist” on any song to add it.'),
    });
  }
  const p = await yt('playlist', { id });
  const item = { type: 'playlist', id: p.id, title: p.title, subtitle: `Playlist • ${p.author}`, thumb: p.thumb };
  const k = register(item);
  const list = register({ tracks: p.tracks, source: p.title, mode: 'list' });
  const saved = isSaved(item);
  return detailPage({
    art: img(p.thumb, 400), title: p.title,
    meta: ['Playlist', esc(p.author), esc(p.year)].filter(Boolean).join(' • '),
    meta2: [esc(p.views), p.trackCount ? plural(+p.trackCount, 'song') : '', esc(p.duration)].filter(Boolean).join(' • '),
    desc: p.description,
    actions: `<button class="circle-btn lg" data-act="toggle-save" data-k="${k}" aria-label="${saved ? 'Remove from library' : 'Save to library'}">${icon(saved ? 'libDone' : 'libAdd')}</button>
      <button class="play-circle" data-act="list-play" data-list="${list}" aria-label="Play">${icon('play')}</button>${dlButton(list)}
      <button class="circle-btn lg" data-act="item-menu" data-k="${k}" aria-label="More actions">${icon('more')}</button>`,
    rows: p.tracks.map((t, i) => songRow(t, list, i, { albumCol: !mobile() })).join('') || emptyHTML('queue', 'This playlist is empty', ''),
  });
}

async function viewArtist(id) {
  const a = await yt('artist', { id });
  const item = { type: 'artist', id, title: a.name, subtitle: a.subscribers ? `${a.subscribers} subscribers` : 'Artist', thumb: a.thumb, shuffleId: a.shuffleId, radioId: a.radioId, songs: a.songs };
  const k = register(item);
  const subbed = isSaved(item);
  const list = register({ tracks: a.songs, source: a.name, mode: 'list' });
  let html = `<div class="artist-hero">${a.thumb ? `<img class="hero-img" src="${esc(sized(a.thumb, 1200))}" alt="" referrerpolicy="no-referrer">` : ''}<div>
      <h1>${esc(a.name)}</h1>
      ${a.monthly || a.views ? `<div class="detail-meta">${esc(a.monthly ? a.monthly + ' monthly audience' : a.views)}</div>` : ''}
      ${a.description ? `<div class="detail-desc" data-act="expand">${esc(a.description)}</div>` : ''}
      <div class="detail-actions">
        <button class="btn white" data-act="artist-shuffle" data-k="${k}">${icon('shuffle')}Shuffle</button>
        <button class="btn" data-act="artist-radio" data-k="${k}">${icon('radio')}Mix</button>
        <button class="btn" data-act="toggle-save" data-k="${k}">${subbed ? 'Subscribed' : 'Subscribe'}${a.subscribers ? ' ' + esc(a.subscribers) : ''}</button>
      </div></div></div>`;
  if (a.songs.length) {
    html += `<div class="section-head"><h2>Top songs</h2>${a.songsPlaylist ? `<a class="btn small" href="#/playlist/${enc(a.songsPlaylist)}">Show all</a>` : ''}</div>
      <div class="list">${a.songs.map((t, i) => songRow(t, list, i, { albumCol: !mobile(), views: true })).join('')}</div>`;
  }
  html += a.sections.map(sec => shelf(sec, { grid: false, more: sec.playlistId ? '#/playlist/' + enc(sec.playlistId) : '' })).join('');
  return html;
}

async function viewLibrary(tab) {
  let tabs = [['playlists', 'Playlists'], ['songs', 'Songs'], ['albums', 'Albums'], ['artists', 'Artists'], ['downloads', 'Downloads'], ['history', 'History']];
  if (S.offline) { tabs = [['downloads', 'Downloads']]; tab = 'downloads'; }
  if (!tabs.some(t => t[0] === tab)) tab = 'playlists';
  const u = S.user;
  let body;
  if (tab === 'downloads') {
    body = downloadsView();
  } else if (tab === 'songs') {
    const list = register({ tracks: u.liked, source: 'Liked Music', mode: 'list', own: 'LM' });
    body = u.liked.length ? `<div class="list">${u.liked.map((t, i) => songRow(t, list, i, { albumCol: !mobile() })).join('')}</div>`
      : emptyHTML('like', 'No songs yet', 'Songs you like show up here.');
  } else if (tab === 'albums') {
    const items = u.saved.filter(x => x.type === 'album');
    body = items.length ? `<div class="card-grid">${items.map(x => card(x, '', -1)).join('')}</div>` : emptyHTML('album', 'No albums yet', 'Albums you save show up here.');
  } else if (tab === 'artists') {
    const items = u.saved.filter(x => x.type === 'artist');
    body = items.length ? `<div class="list search-list">${items.map(x => itemRow(x)).join('')}</div>` : emptyHTML('artist', 'No artists yet', 'Artists you subscribe to show up here.');
  } else if (tab === 'history') {
    const list = register({ tracks: u.history, source: 'History', mode: 'radio' });
    body = u.history.length ? `<div class="section-head"><h2>Recently played</h2><button class="btn small" data-act="clear-history">Clear</button></div>
      <div class="list">${u.history.map((t, i) => songRow(t, list, i, { albumCol: !mobile() })).join('')}</div>`
      : emptyHTML('history', 'No history yet', 'Songs you play show up here.');
  } else {
    const items = [
      { type: 'playlist', id: 'LM', title: 'Liked Music', subtitle: 'Auto playlist', liked: true, own: true },
      ...u.playlists.map(p => ({ type: 'playlist', id: p.id, title: p.name, subtitle: `Playlist • ${plural(p.tracks.length, 'song')}`, thumb: p.tracks[0] ? p.tracks[0].thumb : '', own: true })),
      ...u.saved.filter(x => x.type === 'playlist'),
    ];
    body = `<div class="card-grid"><button class="card card-new" data-act="new-playlist"><div class="card-thumb">${icon('add')}</div><div class="card-title">New playlist</div></button>
      <button class="card card-new" data-act="import-playlist"><div class="card-thumb">${icon('import')}</div><div class="card-title">Import playlist</div></button>
      ${items.map(x => card(x, '', -1)).join('')}</div>`;
  }
  return `<h1 class="page-title">Library</h1><div class="chips">${tabs.map(([t, label]) => `<a class="chip${t === tab ? ' on' : ''}" href="#/library/${t}">${label}</a>`).join('')}</div><div style="margin-top:24px">${body}</div>`;
}

/* =========================================================================
   router
   ========================================================================= */
const NAV = [['#/', 'home', 'Home', 'home'], ['#/explore', 'explore', 'Explore', 'explore'], ['#/library', 'library', 'Library', 'library']];
let routeN = 0;
const scrolls = new Map();

async function route(keepScroll) {
  const n = ++routeN;
  const hash = location.hash || '#/';
  const parts = hash.replace(/^#\/?/, '').split('/').map(p => { try { return decodeURIComponent(p); } catch { return p; } });
  const page = parts[0] || 'home';
  hideMenu();
  closeSuggest();
  if (!keepScroll) closePlayer();
  const section = { explore: 'explore', mood: 'explore', library: 'library', playlist: 'library' }[page] || (page === 'home' ? 'home' : '');
  for (const a of $$('#guide .g-item, #bottomnav a')) a.classList.toggle('on', a.dataset.sec === section);
  for (const a of $$('#guide .g-pl')) a.classList.toggle('on', a.getAttribute('href') === hash);
  if (page === 'search') { if (document.activeElement !== $('#search')) setSearch(parts[1] || ''); } else if (!$('#app').classList.contains('searching')) setSearch('');

  const view = $('#view');
  const main = $('#main');
  const spin = setTimeout(() => { if (n === routeN) view.innerHTML = '<div class="spinner"></div>'; }, 120);
  let html, failed = false;
  try {
    if (S.offline && page !== 'library') {
      html = `<div class="banner warn">${icon('offline')}<div><b>You're offline.</b> Only downloaded songs play until the server can be reached.</div></div>` + emptyHTML('offline', 'Not available offline', '', '<a class="btn white" href="#/library/downloads">Go to downloads</a>');
    } else switch (page) {
      case 'explore':
        html = parts[1] === 'new' ? await viewNewReleases() : parts[1] === 'charts' ? await viewCharts(parts[2]) : parts[1] === 'moods' ? await viewMoods() : await viewExplore();
        break;
      case 'mood': html = await viewMood(parts[1], parts[2]); break;
      case 'search': html = await viewSearch(parts[1] || '', parts[2] || ''); break;
      case 'album': html = await viewAlbum(parts[1]); break;
      case 'artist': html = await viewArtist(parts[1]); break;
      case 'playlist': html = await viewPlaylist(parts[1]); break;
      case 'library': html = await viewLibrary(parts[1]); break;
      default: html = await viewHome();
    }
  } catch (e) {
    failed = true;
    html = statusBanner() + emptyHTML('info', 'Something went wrong', esc(e.message), `<button class="btn" data-act="retry">${icon('refresh')}Retry</button>`);
  }
  clearTimeout(spin);
  if (n !== routeN) return;
  S.pageFailed = failed || !!(S.info.youtube && !S.info.youtube.ok);
  const top = main.scrollTop;
  view.innerHTML = html;
  main.scrollTop = keepScroll ? top : (scrolls.get(hash) || 0);
  markPlaying();
}

function renderGuide() {
  const items = NAV.map(([href, ic, label, sec]) => `<a class="g-item" href="${href}" data-sec="${sec}">${icon(ic)}<span>${label}</span></a>`).join('');
  const pls = [{ id: 'LM', name: 'Liked Music', sub: 'Auto playlist' },
    ...S.user.playlists.map(p => ({ id: p.id, name: p.name, sub: 'You' })),
    ...S.user.saved.filter(x => x.type === 'playlist').map(x => ({ id: x.id, name: x.title, sub: (x.subtitle || '').replace(/^Playlist • /, '') }))];
  $('#guide').innerHTML = items + `<hr class="g-sep"><button class="g-new" data-act="new-playlist">${icon('add')}New playlist</button>
    <div class="g-pls">${pls.map(p => `<a class="g-pl" href="#/playlist/${enc(p.id)}"><div class="g-pl-name">${esc(p.name)}</div><div class="g-pl-sub">${esc(p.sub)}</div></a>`).join('')}</div>`;
  $('#bottomnav').innerHTML = NAV.map(([href, ic, label, sec]) => `<a href="${href}" data-sec="${sec}">${icon(ic)}<span>${label}</span></a>`).join('');
  const hash = location.hash || '#/';
  const page = hash.replace(/^#\/?/, '').split('/')[0] || 'home';
  const section = { explore: 'explore', mood: 'explore', library: 'library', playlist: 'library' }[page] || (page === 'home' ? 'home' : '');
  for (const a of $$('#guide .g-item, #bottomnav a')) a.classList.toggle('on', a.dataset.sec === section);
  for (const a of $$('#guide .g-pl')) a.classList.toggle('on', a.getAttribute('href') === hash);
}

/* =========================================================================
   search box
   ========================================================================= */
let suggestTimer, suggestN = 0, suggestSel = -1;

function setSearch(q) {
  $('#search').value = q;
  $('.search-clear').hidden = !q;
}

function openSuggest() {
  $('#suggest').hidden = false;
  $('#app').classList.add('suggest-open');
}
function closeSuggest() {
  $('#suggest').hidden = true;
  $('#app').classList.remove('suggest-open');
  suggestSel = -1;
}

async function updateSuggest() {
  const q = $('#search').value.trim();
  const n = ++suggestN;
  let items;
  if (!q) items = S.user.searches.slice(0, 10).map(s => ({ text: s, recent: true }));
  else {
    const recent = S.user.searches.filter(s => s.toLowerCase().startsWith(q.toLowerCase()) && s.toLowerCase() !== q.toLowerCase()).slice(0, 3).map(s => ({ text: s, recent: true }));
    let sug = [];
    try { sug = (await yt('suggest', { q })).items; } catch { /* offline */ }
    if (n !== suggestN) return;
    const seen = new Set(recent.map(r => r.text.toLowerCase()));
    items = recent.concat(sug.filter(s => !seen.has(s.toLowerCase())).slice(0, 8).map(s => ({ text: s })));
  }
  if (n !== suggestN) return;
  if (!items.length || document.activeElement !== $('#search')) { closeSuggest(); return; }
  const lower = q.toLowerCase();
  $('#suggest').innerHTML = items.map((it, i) => {
    const t = it.text;
    const label = q && t.toLowerCase().startsWith(lower) ? `${esc(t.slice(0, q.length))}<b>${esc(t.slice(q.length))}</b>` : `<b>${esc(t)}</b>`;
    return `<div class="sg" data-i="${i}" data-q="${esc(t)}">${icon(it.recent ? 'history' : 'search')}<span class="sg-text">${label}</span>
      ${it.recent ? `<button class="icon-btn" data-act="forget-search" data-q="${esc(t)}" aria-label="Remove from history">${icon('close')}</button>` : ''}</div>`;
  }).join('');
  suggestSel = -1;
  openSuggest();
}

function submitSearch(q) {
  q = q.trim();
  if (!q) return;
  if (/^(https?:\/\/)?([a-z]+\.)?(youtube\.com|youtu\.be)\//i.test(q) && /[?&]list=|\/browse\//.test(q)) {
    $('#search').blur();
    return void importDialog(q);
  }
  clearTimeout(suggestTimer);
  suggestN++;
  closeSuggest();
  $('#search').blur();
  $('#app').classList.remove('searching');
  setSearch(q);
  api('searches', 'POST', { q }).then(u => { S.user = u; }).catch(() => {});
  const target = '#/search/' + enc(q);
  if (location.hash === target) route(); else location.hash = target;
}

$('#search-form').addEventListener('submit', e => {
  e.preventDefault();
  const sel = $(`.sg[data-i="${suggestSel}"]`);
  submitSearch(sel ? sel.dataset.q : $('#search').value);
});
$('#search').addEventListener('input', () => {
  $('.search-clear').hidden = !$('#search').value;
  clearTimeout(suggestTimer);
  suggestTimer = setTimeout(updateSuggest, 150);
});
$('#search').addEventListener('focus', updateSuggest);
$('#search').addEventListener('keydown', e => {
  const items = $$('.sg');
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    if (!items.length) return;
    e.preventDefault();
    suggestSel = (suggestSel + (e.key === 'ArrowDown' ? 1 : -1) + items.length + 1) % (items.length + 1) - 0;
    if (suggestSel >= items.length) suggestSel = -1;
    items.forEach((el, i) => el.classList.toggle('sel', i === suggestSel));
  } else if (e.key === 'Escape') {
    closeSuggest();
    $('#search').blur();
    $('#app').classList.remove('searching');
  }
});

/* =========================================================================
   menus and dialogs
   ========================================================================= */
function showMenu(items, anchor, head) {
  const m = $('#menu');
  m.items = items.filter(Boolean);
  m.innerHTML = (head || '') + m.items.map((it, i) => it === '-' ? '<hr>' : `<button class="mi" role="menuitem" data-i="${i}">${icon(it.icon)}<span>${esc(it.label)}</span></button>`).join('');
  m.hidden = false;
  if (mobile() || !anchor) {
    $('#scrim').hidden = !mobile();
    m.style.left = m.style.top = '';
    return;
  }
  const r = anchor.getBoundingClientRect();
  const w = m.offsetWidth, h = m.offsetHeight;
  m.style.left = Math.max(8, Math.min(r.right - w, innerWidth - w - 8)) + 'px';
  m.style.top = (r.bottom + h + 8 > innerHeight ? Math.max(8, r.top - h) : r.bottom) + 'px';
}
function hideMenu() { $('#menu').hidden = true; $('#scrim').hidden = true; }

const menuHead = (thumb, title, sub, round) => mobile() ? `<div class="menu-head"><div class="thumb${round ? ' round' : ''}">${img(thumb, 60)}</div><div><b>${esc(title)}</b><span>${esc(sub)}</span></div></div>` : '';

function trackMenu(t, anchor, ctx = {}) {
  const r = rating(t.videoId);
  const art = (t.artists || []).find(a => a.id);
  showMenu([
    { icon: 'radio', label: 'Start radio', run: () => playRadio(t, `${t.title} radio`) },
    { icon: 'playNext', label: 'Play next', run: () => insert([t], true) },
    { icon: 'queue', label: 'Add to queue', run: () => insert([t], false) },
    { icon: r === 'like' ? 'like' : 'likeOff', label: r === 'like' ? 'Remove from liked songs' : 'Add to liked songs', run: () => rate(t, 'like') },
    { icon: 'playlistAdd', label: 'Save to playlist', run: () => saveToPlaylist([t]) },
    isDownloaded(t.videoId) ? { icon: 'downloadDone', label: 'Remove download', run: () => removeDownloads([t.videoId]).then(() => { toast('Download removed'); if (location.hash.startsWith('#/library/downloads')) route(true); }) }
      : { icon: 'download', label: 'Download', run: () => download([t]) },
    ctx.queueIndex !== undefined && ctx.queueIndex !== S.index ? { icon: 'remove', label: 'Remove from queue', run: () => removeFromQueue(ctx.queueIndex) } : null,
    ctx.own && ctx.own !== 'LM' ? { icon: 'remove', label: 'Remove from playlist', run: () => removeFromPlaylist(ctx.own, ctx.idx) } : null,
    '-',
    t.album && t.album.id ? { icon: 'album', label: 'Go to album', run: () => { location.hash = '#/album/' + enc(t.album.id); } } : null,
    art ? { icon: 'artist', label: 'Go to artist', run: () => { location.hash = '#/artist/' + enc(art.id); } } : null,
    { icon: 'share', label: 'Share', run: () => share(`https://music.youtube.com/watch?v=${t.videoId}`) },
    ctx.current ? { icon: 'timer', label: 'Sleep timer', run: () => sleepMenu(anchor) } : null,
  ], anchor, menuHead(t.thumb, t.title, names(t)));
}

async function tracksOf(it) {
  if (isTrack(it)) return [it];
  if (it.type === 'album') return (await yt('album', { id: it.id })).tracks.filter(t => t.available !== false);
  if (it.type === 'playlist') { const own = ownPlaylist(it.id); return own ? own.tracks : (await yt('playlist', { id: it.id })).tracks; }
  if (it.type === 'artist') return (await yt('artist', { id: it.id })).songs;
  return [];
}

async function playItem(it, opts = {}) {
  if (isTrack(it)) return playRadio(it);
  if (it.type === 'artist') return artistPlay(it, 'shuffle');
  const tracks = await tracksOf(it);
  if (!tracks.length) return toast('Nothing to play here');
  playList(tracks, opts.shuffle ? Math.floor(Math.random() * tracks.length) : 0, it.title, opts);
}

async function artistPlay(it, how) {
  const pid = how === 'shuffle' ? it.shuffleId : it.radioId;
  try {
    if (pid) {
      const w = await yt('watch', { playlistId: pid }, { fresh: true });
      if (w.tracks.length) return playList(w.tracks, 0, it.title);
    }
    const songs = it.songs || (await yt('artist', { id: it.id })).songs;
    if (!songs.length) return toast('Nothing to play here');
    if (how === 'shuffle') playList(songs, Math.floor(Math.random() * songs.length), it.title, { shuffle: true });
    else playRadio(songs[0], `${it.title} mix`);
  } catch (e) { toast(e.message); }
}

function itemMenu(it, anchor) {
  if (isTrack(it)) return trackMenu(it, anchor);
  const saved = isSaved(it);
  const items = it.type === 'artist' ? [
    { icon: 'shuffle', label: 'Shuffle play', run: () => artistPlay(it, 'shuffle') },
    { icon: 'radio', label: 'Start radio', run: () => artistPlay(it, 'radio') },
    { icon: saved ? 'remove' : 'add', label: saved ? 'Unsubscribe' : 'Subscribe', run: () => setSaved(it, !saved) },
    { icon: 'share', label: 'Share', run: () => share(`https://music.youtube.com/channel/${it.id}`) },
  ] : [
    { icon: 'shuffle', label: 'Shuffle play', run: () => playItem(it, { shuffle: true }) },
    { icon: 'radio', label: 'Start radio', run: async () => { const t = await tracksOf(it); if (t.length) playRadio(t[0], `${it.title} radio`); } },
    { icon: 'playNext', label: 'Play next', run: async () => insert(await tracksOf(it), true) },
    { icon: 'queue', label: 'Add to queue', run: async () => insert(await tracksOf(it), false) },
    it.own ? null : { icon: saved ? 'libDone' : 'libAdd', label: saved ? 'Remove from library' : 'Save to library', run: () => setSaved(it, !saved) },
    { icon: 'playlistAdd', label: 'Save to playlist', run: async () => saveToPlaylist(await tracksOf(it)) },
    it.type === 'artist' ? null : { icon: 'download', label: 'Download', run: async () => download(await tracksOf(it)) },
    it.own ? null : { icon: 'import', label: 'Import as my playlist', run: () => importLink(it.type === 'album' ? `https://music.youtube.com/browse/${it.id}` : `https://music.youtube.com/playlist?list=${it.id}`) },
    it.own ? null : { icon: 'share', label: 'Share', run: () => share(it.type === 'album' && it.playlistId ? `https://music.youtube.com/playlist?list=${it.playlistId}` : it.type === 'album' ? `https://music.youtube.com/browse/${it.id}` : `https://music.youtube.com/playlist?list=${it.id}`) },
  ];
  showMenu(items, anchor, menuHead(it.thumb, it.title, it.subtitle || '', it.type === 'artist'));
}

async function share(url) {
  try {
    if (navigator.share && mobile()) { await navigator.share({ url }); return; }
    await navigator.clipboard.writeText(url);
    toast('Link copied to clipboard');
  } catch {
    const ta = document.createElement('textarea');
    ta.value = url;
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand && document.execCommand('copy');
    ta.remove();
    toast(ok ? 'Link copied to clipboard' : url);
  }
}

function dialog(html) {
  const d = $('#dlg');
  if (d.open) d.close();
  d.onclick = d.onchange = null;
  d.innerHTML = html;
  d.returnValue = '';
  d.showModal();
  return d;
}

/* Resolves with the form when its OK button is used, or null when dismissed. */
function formResult(d) {
  return new Promise(resolve => {
    const form = $('form', d);
    form.addEventListener('submit', e => resolve(e.submitter && e.submitter.value === 'ok' ? form : null), { once: true });
    // Added a tick later so a close event still queued from an earlier dialog can't answer this one.
    setTimeout(() => d.addEventListener('close', () => resolve(null), { once: true }));
  });
}

function newPlaylistDialog(tracks = []) {
  const d = dialog(`<form method="dialog"><h2>New playlist</h2>
    <div class="field"><label for="pl-name">Title</label><input id="pl-name" name="name" maxlength="150" required autocomplete="off"></div>
    <div class="field"><label for="pl-desc">Description</label><textarea id="pl-desc" name="desc" rows="2" maxlength="5000"></textarea></div>
    <div class="dlg-actions"><button class="btn" value="cancel" formnovalidate>Cancel</button><button class="btn white" value="ok">Create</button></div></form>`);
  $('#pl-name').focus();
  return formResult(d).then(async form => {
    if (!form) return null;
    const name = $('#pl-name', form).value.trim();
    try {
      const r = await api('playlists', 'POST', { name, description: $('#pl-desc', form).value, tracks: tracks.map(slim) });
      setUser(r.state);
      toast(tracks.length ? `Saved to ${name}` : 'Playlist created', { label: 'View', run: () => { location.hash = '#/playlist/' + r.id; } });
      return r.id;
    } catch (e) { toast(e.message); return null; }
  });
}

function saveToPlaylist(tracks) {
  tracks = tracks.filter(isTrack);
  if (!tracks.length) return;
  const d = dialog(`<div class="dlg-head"><h2>Save to playlist</h2><button class="icon-btn" data-act="dlg-close" aria-label="Close">${icon('close')}</button></div>
    <div class="dlg-list">${S.user.playlists.map(p => `<button data-pl="${p.id}"><div class="thumb">${img(p.tracks[0] && p.tracks[0].thumb, 60)}</div><div><b>${esc(p.name)}</b><span class="muted small">${plural(p.tracks.length, 'song')}</span></div></button>`).join('')
      || '<p class="muted" style="padding:8px 24px">You don\'t have any playlists yet.</p>'}</div>
    <div class="dlg-actions" style="margin-top:16px"><button class="btn white" data-act="dlg-new">${icon('add')}New playlist</button></div>`);
  d.onclick = async e => {
    const b = e.target.closest('[data-pl]');
    if (b) {
      d.close();
      try {
        const r = await api(`playlists/${b.dataset.pl}/add`, 'POST', { tracks: tracks.map(slim) });
        setUser(r.state);
        const p = S.user.playlists.find(x => x.id === b.dataset.pl);
        toast(r.added ? `Saved to ${p ? p.name : 'playlist'}` : `Already in ${p ? p.name : 'playlist'}`);
      } catch (err) { toast(err.message); }
    } else if (e.target.closest('[data-act="dlg-new"]')) {
      d.close();
      newPlaylistDialog(tracks);
    } else if (e.target.closest('[data-act="dlg-close"]') || e.target === d) d.close();
  };
}

async function removeFromPlaylist(id, idx) {
  const p = S.user.playlists.find(x => x.id === id);
  if (!p) return;
  const tracks = p.tracks.filter((_, i) => i !== idx);
  setUser(await api('playlists/' + id, 'PUT', { tracks }));
  toast(`Removed from ${p.name}`);
  route(true);
}

function editPlaylist(id) {
  const p = S.user.playlists.find(x => x.id === id);
  if (!p) return;
  const d = dialog(`<form method="dialog"><h2>Edit playlist</h2>
    <div class="field"><label for="pl-name">Title</label><input id="pl-name" maxlength="150" required value="${esc(p.name)}" autocomplete="off"></div>
    <div class="field"><label for="pl-desc">Description</label><textarea id="pl-desc" rows="3" maxlength="5000">${esc(p.description || '')}</textarea></div>
    <div class="dlg-actions"><button class="btn" value="cancel" formnovalidate>Cancel</button><button class="btn white" value="ok">Save</button></div></form>`);
  formResult(d).then(async form => {
    if (!form) return;
    try {
      setUser(await api('playlists/' + id, 'PUT', { name: $('#pl-name', form).value, description: $('#pl-desc', form).value }));
      route(true);
    } catch (e) { toast(e.message); }
  });
}

function confirmDialog(title, text, ok) {
  const d = dialog(`<form method="dialog"><h2>${esc(title)}</h2><p class="muted">${esc(text)}</p>
    <div class="dlg-actions"><button class="btn" value="cancel">Cancel</button><button class="btn white" value="ok">${esc(ok)}</button></div></form>`);
  return formResult(d).then(form => !!form);
}

/* New yt-dlp / ytmusicapi are running on the server: no reload needed, but
   whatever failed before gets another go, and open screens show the new versions. */
function youtubeUpdated() {
  ytCache.clear();
  const y = S.info.youtube || {};
  if (!S.ytUpdating) toast(`YouTube support was updated${y.yt_dlp ? ` (yt-dlp ${y.yt_dlp})` : ''}`);
  const d = $('#dlg');
  if (d.open && $('.settings', d)) renderSettings(d);
  else if (d.open && $('[data-act="yt-update"]', d) && !S.ytUpdating) aboutDialog();
  if (S.pageFailed) route(true);
  const t = current();
  if (S.stuck && t && t.videoId === S.stuck) {
    // The song that couldn't be played: try it again.
    S.stuck = '';
    errorsInRow = 0;
    if (isServer()) srv('jump', { index: S.index }); else loadLocal(true);
  } else if (isServer() && S.lastError && !S.playing && t) {
    srv('jump', { index: S.index });
  }
}

function aboutDialog() {
  const y = S.info.youtube || {};
  const last = S.info.lastUpdate;
  const when = last && last.time ? new Date(last.time * 1000).toLocaleString() : '';
  dialog(`<div class="dlg-head"><h2>Gundil Music</h2><button class="icon-btn" data-act="dlg-close" aria-label="Close">${icon('close')}</button></div>
    <p class="muted">Server: <b>${esc(S.info.hostname)}</b> · version ${esc(S.info.version)}</p>
    <p class="muted" style="margin-top:8px">YouTube Music: ${y.ok ? `connected (ytmusicapi ${esc(y.ytmusicapi)}, yt-dlp ${esc(y.yt_dlp)}, JavaScript: ${esc((y.js || []).join(', ') || 'none')})` : esc(y.error || 'not connected')}</p>
    ${y.warning ? `<p style="margin-top:8px">${esc(y.warning)}</p>` : ''}
    <p class="muted small" style="margin-top:12px">YouTube changes often. The server updates yt-dlp and ytmusicapi daily${when ? `; last updated ${esc(when)}` : ''}.</p>
    <div class="dlg-actions" style="justify-content:flex-start;margin-top:12px"><button class="btn white" data-act="yt-update">${icon('refresh')}Update YouTube support</button></div>
    <p class="muted small" style="margin-top:16px">Not affiliated with YouTube or Google. Your library is stored on the server.</p>`);
  $('#dlg').onclick = async e => {
    if (e.target.closest('[data-act="dlg-close"]') || e.target === $('#dlg')) return $('#dlg').close();
    const b = e.target.closest('[data-act="yt-update"]');
    if (b) await runYtUpdate(b, () => { if ($('#dlg').open) aboutDialog(); });
  };
}

/* =========================================================================
   drag to reorder (Up next and your playlists)
   ========================================================================= */
let dragging = null;
document.addEventListener('pointerdown', e => {
  const h = e.target.closest('.drag-handle');
  if (!h || e.button > 0) return;
  const row = h.closest('.row');
  const box = row && row.parentElement;
  if (!box) return;
  e.preventDefault();
  const rows = [...box.children].filter(r => r.classList.contains('row'));
  dragging = {
    row, box, from: rows.indexOf(row),
    keys: rows.map(r => +(r.dataset.q !== undefined ? r.dataset.q : r.dataset.idx)),
    scroller: row.closest('.pp-body') || $('#main'), y: e.clientY,
  };
  row.classList.add('dragging');
  try { h.setPointerCapture(e.pointerId); } catch { /* fine without */ }
  dragging.timer = setInterval(dragScroll, 30);
});
function dragScroll() {
  if (!dragging) return;
  const r = dragging.scroller.getBoundingClientRect();
  const edge = 56;
  const dy = dragging.y < r.top + edge ? -(r.top + edge - dragging.y) / 4 : dragging.y > r.bottom - edge ? (dragging.y - (r.bottom - edge)) / 4 : 0;
  if (dy) { dragging.scroller.scrollTop += dy; dragOver(); }
}
function dragOver() {
  const { row, box, y } = dragging;
  const others = [...box.children].filter(r => r.classList.contains('row') && r !== row);
  const before = others.find(r => { const b = r.getBoundingClientRect(); return y < b.top + b.height / 2; });
  if (before) { if (row.nextElementSibling !== before) box.insertBefore(row, before); }
  else if (others.length && others[others.length - 1].nextElementSibling !== row) others[others.length - 1].after(row);
}
document.addEventListener('pointermove', e => {
  if (!dragging) return;
  e.preventDefault();
  dragging.y = e.clientY;
  dragOver();
}, { passive: false });
function endDrag() {
  if (!dragging) return;
  const { row, box, from, keys, timer } = dragging;
  clearInterval(timer);
  dragging = null;
  row.classList.remove('dragging');
  const at = [...box.children].filter(r => r.classList.contains('row')).indexOf(row);
  shownQueue = -1;
  if (at === from || at < 0) { if (S.ppOpen) renderPPTab(); return; }
  const a = keys[from], b = keys[at];
  if (row.dataset.q !== undefined) return void moveInQueue(a, b);
  const l = reg.get(row.dataset.list);
  if (l && l.own && l.own !== 'LM') movePlaylistTrack(l.own, a, b);
}
document.addEventListener('pointerup', endDrag);
document.addEventListener('pointercancel', endDrag);

async function movePlaylistTrack(id, from, to) {
  const p = S.user.playlists.find(x => x.id === id);
  if (!p) return;
  const tracks = p.tracks.slice();
  const [t] = tracks.splice(from, 1);
  tracks.splice(to, 0, t);
  try { setUser(await api('playlists/' + id, 'PUT', { tracks })); } catch (e) { toast(e.message); }
  route(true);
}

/* =========================================================================
   downloads: songs kept on this device, to play without the server
   ========================================================================= */
/* Kept in IndexedDB: unlike the Cache API it also works when the server is
   reached over plain http on the local network (as the Android app does). */
const dlStore = (() => {
  let opening = null;
  const db = () => opening || (opening = new Promise((ok, fail) => {
    const r = indexedDB.open('gundil-downloads', 1);
    r.onupgradeneeded = () => r.result.createObjectStore('files');
    r.onsuccess = () => ok(r.result);
    r.onerror = () => { opening = null; fail(r.error); };
  }));
  const run = async (mode, fn) => {
    const d = await db();
    return new Promise((ok, fail) => {
      const tx = d.transaction('files', mode);
      const req = fn(tx.objectStore('files'));
      tx.oncomplete = () => ok(req.result);
      tx.onerror = tx.onabort = () => fail(tx.error || new Error('storage failed'));
    });
  };
  return {
    get: key => run('readonly', s => s.get(key)),
    put: (key, blob) => run('readwrite', s => s.put(blob, key)),
    del: key => run('readwrite', s => s.delete(key)),
  };
})();
const dl = {
  list: local.get('downloads', {}), // videoId -> { track, size, img, at }
  urls: new Map(), // videoId -> blob: URL of the downloaded copy
  queue: [], running: false, done: 0, total: 0,
};
const dlThumbs = new Map(); // a downloaded song's art URL -> its copy on this device
const dlThumbsReady = !window.indexedDB ? Promise.resolve() : Promise.all(Object.entries(dl.list).filter(([, d]) => d.img && d.track.thumb).map(([id, d]) =>
  dlStore.get('img/' + id).then(b => { if (b) dlThumbs.set(d.track.thumb, URL.createObjectURL(b)); }, () => {})));
const isDownloaded = id => !!dl.list[id];
const dlCount = () => Object.keys(dl.list).length;
const srcFor = id => dl.urls.get(id) || streamSrc(id);
function saveDl() { local.set('downloads', dl.list); }
function dlBytes() { return Object.values(dl.list).reduce((n, d) => n + (d.size || 0), 0); }
const mb = n => n >= 1e9 ? (n / 1e9).toFixed(1) + ' GB' : Math.max(0.1, n / 1e6).toFixed(n >= 1e7 ? 0 : 1) + ' MB';

/* The downloaded copy as a URL the player can open. */
async function dlUrl(id) {
  if (dl.urls.has(id)) return dl.urls.get(id);
  const blob = await dlStore.get('audio/' + id);
  if (!blob) { delete dl.list[id]; saveDl(); return null; } // cleared by the browser
  const u = URL.createObjectURL(blob);
  dl.urls.set(id, u);
  return u;
}

/* Downloads songs one after another, with a running count. */
function download(tracks) {
  if (!window.indexedDB) return toast('This browser can\'t keep songs for offline');
  const add = tracks.filter(t => isTrack(t) && t.available !== false && !isDownloaded(t.videoId) && !dl.queue.some(q => q.videoId === t.videoId)).map(slim);
  if (!add.length) return toast(tracks.length > 1 ? 'Already downloaded' : 'Already downloaded');
  if (navigator.storage && navigator.storage.persist) navigator.storage.persist().catch(() => {});
  dl.queue.push(...add);
  dl.total += add.length;
  toast(add.length > 1 ? `Downloading ${add.length} songs…` : `Downloading “${add[0].title}”…`);
  if (!dl.running) runDownloads();
}

async function runDownloads() {
  dl.running = true;
  let failed = 0;
  while (dl.queue.length) {
    const t = dl.queue[0];
    try { await fetchDownload(t); dl.done++; } catch (e) { failed++; console.warn('download', t.title, e); }
    dl.queue.shift();
    renderDownloadMarks(t.videoId);
    if (dl.total > 1 && dl.queue.length) toast(`Downloaded ${dl.done} of ${dl.total}…`);
  }
  const done = dl.done;
  dl.running = false; dl.done = 0; dl.total = 0;
  if (failed) toast(`${plural(done, 'song')} downloaded; ${failed} couldn't be`, { label: 'Downloads', run: () => { location.hash = '#/library/downloads'; } });
  else toast(done > 1 ? `${plural(done, 'song')} downloaded` : 'Downloaded', { label: 'Downloads', run: () => { location.hash = '#/library/downloads'; } });
  if (location.hash.startsWith('#/library/downloads')) route(true);
}

async function fetchDownload(t) {
  const res = await fetch(`/api/stream/${t.videoId}?q=${audioQuality()}`);
  if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
  const blob = await res.blob();
  if (blob.size < 10000) throw new Error('not audio');
  await dlStore.put('audio/' + t.videoId, blob.type ? blob : new Blob([blob], { type: res.headers.get('Content-Type') || 'audio/mp4' }));
  let pic = null;
  if (t.thumb) {
    try { // the cover, for when there is no connection (some image hosts don't allow reading it)
      const r = await fetch(sized(t.thumb, 226), { mode: 'cors', referrerPolicy: 'no-referrer' });
      if (r.ok) { pic = await r.blob(); await dlStore.put('img/' + t.videoId, pic); }
    } catch { /* keep the online cover */ }
  }
  dl.list[t.videoId] = { track: t, size: blob.size, img: !!pic, at: Date.now() };
  if (pic) dlThumbs.set(t.thumb, URL.createObjectURL(pic));
  saveDl();
}

async function removeDownloads(ids) {
  for (const id of ids) {
    const d = dl.list[id];
    if (!d) continue;
    await dlStore.del('audio/' + id);
    await dlStore.del('img/' + id);
    if (d.track.thumb && dlThumbs.has(d.track.thumb)) { URL.revokeObjectURL(dlThumbs.get(d.track.thumb)); dlThumbs.delete(d.track.thumb); }
    const u = dl.urls.get(id);
    if (u && audio.getAttribute('src') !== u) { URL.revokeObjectURL(u); dl.urls.delete(id); }
    delete dl.list[id];
  }
  saveDl();
  for (const id of ids) renderDownloadMarks(id);
}

/* Updates the ✓ marks on rows of a song after its download state changed. */
function renderDownloadMarks(id) {
  for (const row of $$(`.row[data-vid="${id}"] .row-title`)) {
    const m = $('.dl-mark', row);
    if (isDownloaded(id) && !m) row.insertAdjacentHTML('beforeend', `<span class="dl-mark" title="Downloaded">${icon('downloadDone')}</span>`);
    else if (!isDownloaded(id) && m) m.remove();
  }
  for (const b of $$('[data-act="list-download"]')) {
    const l = reg.get(b.dataset.list);
    if (!l) continue;
    const all = l.tracks.length && l.tracks.every(t => isDownloaded(t.videoId));
    b.innerHTML = icon(all ? 'downloadDone' : 'download');
    b.title = b.ariaLabel = all ? 'Downloaded' : 'Download';
    b.setAttribute('aria-label', b.title);
  }
}
const dlMark = t => isDownloaded(t.videoId) ? `<span class="dl-mark" title="Downloaded">${icon('downloadDone')}</span>` : '';
const dlButton = list => {
  const l = reg.get(list);
  const all = l && l.tracks.length && l.tracks.every(t => isDownloaded(t.videoId));
  return `<button class="circle-btn lg" data-act="list-download" data-list="${list}" aria-label="${all ? 'Downloaded' : 'Download'}" title="${all ? 'Downloaded' : 'Download'}">${icon(all ? 'downloadDone' : 'download')}</button>`;
};

/* The Downloads tab (and all of the app when offline). */
function downloadsView() {
  const items = Object.values(dl.list).sort((a, b) => b.at - a.at).map(d => d.track);
  const list = register({ tracks: items, source: 'Downloads', mode: 'list' });
  const head = S.offline
    ? `<div class="banner warn">${icon('offline')}<div><b>You're offline.</b> Can't reach the Gundil Music server, so only downloaded songs play. <button class="btn small" onclick="location.reload()">Try again</button></div></div>`
    : '';
  if (!items.length) return head + emptyHTML('download', 'No downloads yet', 'Download songs, albums and playlists to play them without a connection: use ⋮ → Download, or the download button on an album or playlist.');
  return `${head}<div class="section-head"><h2>${plural(items.length, 'song')} · ${mb(dlBytes())}</h2>
      <div class="dl-actions"><button class="btn small" data-act="list-shuffle" data-list="${list}">${icon('shuffle')}Shuffle</button><button class="btn white small" data-act="list-play" data-list="${list}">${icon('play')}Play</button>${S.offline ? '' : '<button class="btn small" data-act="dl-clear">Remove all</button>'}</div></div>
    <div class="list">${items.map((t, i) => songRow(t, list, i, { albumCol: !mobile() })).join('')}</div>`;
}

/* No server: play what's downloaded. */
async function startOffline() {
  await dlThumbsReady;
  S.offline = true;
  S.info = { youtube: {}, hostname: '' };
  S.server = null;
  S.output = 'local';
  setUser({ playlists: [], liked: [], disliked: [], saved: [], history: [], searches: [] });
  const saved = local.get('queue', null);
  if (saved && Array.isArray(saved.tracks)) {
    S.queue = saved.tracks.filter(t => isTrack(t) && isDownloaded(t.videoId));
    S.index = 0;
    S.playing = false; S.paused = true;
  }
  audio.volume = Math.min(1, S.volume / 100);
  renderGuide();
  renderPlayer();
  if (location.hash !== '#/library/downloads') location.hash = '#/library/downloads'; else route();
  // Back online? Offer to reload.
  const ping = setInterval(async () => {
    try {
      const r = await fetch('/api/info', { cache: 'no-store' });
      if (r.ok) { clearInterval(ping); toast('Connected to the server again', { label: 'Reload', run: () => location.reload() }); }
    } catch { /* still offline */ }
  }, 30000);
}

/* =========================================================================
   accounts
   ========================================================================= */
function renderAvatar() {
  const u = S.session.user;
  const a = $('.avatar');
  a.textContent = u ? (u.name.trim()[0] || 'G').toUpperCase() : 'G';
  a.title = u ? `Signed in as ${u.name}` : '';
}

/* The whole-page sign-in form, shown once accounts exist and nobody is signed in. */
function showSignIn(msg) {
  if ($('#signin')) return;
  audio.pause();
  const el = document.createElement('div');
  el.id = 'signin';
  const host = esc((S.info && S.info.hostname) || location.hostname);
  const canSignUp = !!S.session.signUp, needsCode = !!S.session.needsCode;
  el.innerHTML = `<form class="signin-box" autocomplete="on">
      <div class="logo-row">${$('.logo') ? $('.logo').innerHTML : 'Gundil Music'}</div>
      <h1 class="si-title">Sign in</h1>
      <p class="muted si-intro"></p>
      <div class="signin-err" hidden></div>
      <label>Name<input name="name" autocomplete="username" autocapitalize="off" spellcheck="false" maxlength="32" required></label>
      <label>Password<input name="password" type="password" autocomplete="current-password" required></label>
      <label class="si-new" hidden>Password again<input name="again" type="password" autocomplete="new-password"></label>
      ${needsCode ? '<label class="si-new" hidden>Invite code<input name="code" autocapitalize="off" autocomplete="off" spellcheck="false"></label>' : ''}
      <button class="btn white si-go" type="submit">Sign in</button>
      ${canSignUp ? '<p class="si-switch muted">New here? <a href="#" data-si="new">Create an account</a></p>' : ''}
    </form>`;
  document.body.appendChild(el);
  const err = $('.signin-err', el);
  if (msg) { err.textContent = msg; err.hidden = false; }
  const form = $('form', el);
  let creating = false;
  const showMode = make => {
    creating = make;
    $('.si-title', el).textContent = make ? 'Create an account' : 'Sign in';
    $('.si-intro', el).innerHTML = make
      ? `Make your own Gundil Music account on ${host}: your own playlists, likes and history.${needsCode ? ' Ask whoever runs the server for the invite code.' : ''}`
      : `Sign in to your Gundil Music account on ${host}.${canSignUp ? '' : ' Ask whoever runs the server if you don\'t have one.'}`;
    for (const l of $$('.si-new', el)) { l.hidden = !make; const i = $('input', l); i.required = make; }
    form.password.autocomplete = make ? 'new-password' : 'current-password';
    $('.si-go', el).textContent = make ? 'Create account' : 'Sign in';
    const sw = $('.si-switch', el);
    if (sw) sw.innerHTML = make ? 'Already have an account? <a href="#" data-si="old">Sign in</a>' : 'New here? <a href="#" data-si="new">Create an account</a>';
    err.hidden = true;
    form.name.focus();
  };
  el.addEventListener('click', e => { const a = e.target.closest('[data-si]'); if (a) { e.preventDefault(); showMode(a.dataset.si === 'new'); } });
  showMode(false);
  form.addEventListener('submit', async e => {
    e.preventDefault();
    const b = $('.si-go', form);
    if (creating && form.password.value !== form.again.value) {
      err.textContent = 'The two passwords are different';
      err.hidden = false;
      return form.again.select();
    }
    b.disabled = true;
    try {
      if (creating) await api('signup', 'POST', { name: form.name.value, password: form.password.value, code: form.code ? form.code.value : '' });
      else await api('login', 'POST', { name: form.name.value, password: form.password.value });
      try { localStorage.removeItem('gundil.queue'); } catch { /* ignore */ }
      location.reload();
    } catch (x) {
      err.textContent = x.message;
      err.hidden = false;
      b.disabled = false;
      form.password.select();
    }
  });
}

async function signOut() {
  try { await api('logout', 'POST', {}); } catch { /* ignore */ }
  try { localStorage.removeItem('gundil.queue'); } catch { /* ignore */ }
  location.reload();
}

/* Accounts were set up, changed or removed (here or on another device). */
async function accountsChanged() {
  let now;
  try { now = await api('session'); } catch { return; }
  const was = S.session;
  S.session = now;
  if (now.accounts && !now.user) return showSignIn(was.user ? 'You were signed out.' : 'Accounts were set up on this server. Sign in to continue.');
  if (!was.accounts && now.accounts) return location.reload();
  renderAvatar();
  if ($('#dlg').open && $('.settings', $('#dlg'))) settingsDialog();
}

/* A small form in the shared dialog. fields: [name, label, type, value]. Resolves with the values or null. */
function askForm(title, intro, fields, okLabel) {
  const d = dialog(`<form method="dialog" class="ask"><h2>${esc(title)}</h2>${intro ? `<p class="muted">${intro}</p>` : ''}
    ${fields.map(([n, label, type = 'text', v = '']) => type === 'checkbox'
      ? `<label class="check"><input type="checkbox" name="${n}"${v ? ' checked' : ''}> ${esc(label)}</label>`
      : `<label>${esc(label)}<input name="${n}" type="${type}" value="${esc(v)}" ${type === 'password' ? 'autocomplete="new-password"' : 'autocomplete="off"'}></label>`).join('')}
    <div class="ask-err" hidden></div>
    <div class="dlg-actions"><button class="btn" value="cancel">Cancel</button><button class="btn white" value="ok">${esc(okLabel)}</button></div></form>`);
  const first = $('input', d);
  if (first) first.focus();
  return formResult(d).then(form => form && Object.fromEntries(fields.map(([n, , type]) => [n, type === 'checkbox' ? form[n].checked : form[n].value])));
}

async function accountAction(k, el) {
  const name = el.dataset.user;
  try {
    if (k === 'setup-accounts') {
      const v = await askForm('Set up accounts', 'Everyone will sign in with their own name and password, and get their own library, playlists and history. <b>This first account is the admin</b>, and gets the library used so far.', [['name', 'Your name'], ['password', 'Password', 'password'], ['again', 'Password again', 'password']], 'Set up');
      if (!v) return settingsDialog();
      if (v.password !== v.again) { toast('The passwords are different'); return accountAction(k, el); }
      await api('accounts/setup', 'POST', { name: v.name, password: v.password });
      location.reload();
    } else if (k === 'add-user') {
      const v = await askForm('Add an account', 'They sign in with this name and password, and can change the password later.', [['name', 'Name'], ['password', 'Password', 'password'], ['admin', 'Admin: can manage accounts and server settings', 'checkbox']], 'Add');
      if (v) { await api('users', 'POST', v); toast(`Account ${v.name} added`); }
      settingsDialog();
    } else if (k === 'my-password') {
      const v = await askForm('Change your password', 'Your other devices will be signed out.', [['current', 'Current password', 'password'], ['password', 'New password', 'password']], 'Change');
      if (v) { await api('users/' + enc(S.session.user.name), 'PUT', v); toast('Password changed'); }
      settingsDialog();
    } else if (k === 'reset-password') {
      const v = await askForm(`New password for ${name}`, `${esc(name)} will be signed out on every device.`, [['password', 'New password', 'password']], 'Set password');
      if (v) { await api('users/' + enc(name), 'PUT', v); toast(`Password for ${name} changed`); }
      settingsDialog();
    } else if (k === 'toggle-admin') {
      await api('users/' + enc(name), 'PUT', { admin: el.dataset.admin !== '1' });
      settingsDialog();
    } else if (k === 'delete-user') {
      if (!await confirmInline(el, 'Tap again to delete')) return;
      await api('users/' + enc(name), 'DELETE');
      toast(`Account ${name} and its library deleted`);
      settingsDialog();
    } else if (k === 'sign-out') signOut();
  } catch (e) { toast(e.message); settingsDialog(); }
}

/* ---------- quitting (on the computer running Gundil Music) ---------- */
async function quitDialog() {
  const serving = S.conn && S.conn.mode !== 'client';
  const d = dialog(`<form method="dialog" class="ask"><h2>Quit Gundil Music?</h2>
    <p class="muted">This stops Gundil Music on this computer. Closing the browser window doesn't: it keeps running for your phone and other devices.${serving ? ' <b>Phones and other devices using this computer as their server will stop playing too.</b>' : ''}</p>
    <div class="dlg-actions"><button class="btn" value="cancel">Cancel</button><button class="btn white" value="ok">Quit</button></div></form>`);
  if (!await formResult(d)) return;
  try {
    const r = await fetch('/_gundil/quit', { method: 'POST', headers: { 'X-Gundil': '1' } });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || r.statusText);
  } catch (e) { return toast(e.message); }
  audio.pause();
  document.body.innerHTML = `<div id="signin"><div class="signin-box"><div class="logo-row">${icon('power')}Gundil Music</div>
    <h1>Gundil Music has stopped</h1><p class="muted">You can close this window. To use it again, start Gundil Music from your apps menu.</p></div></div>`;
}

/* ---------- linking a Google (YouTube Music) account ---------- */
function googleDialog() {
  const d = dialog(`<form method="dialog" class="ask"><h2>Link your YouTube Music account</h2>
    <p class="muted">Gundil Music signs in to YouTube Music with your browser's cookies. Then you can import your private playlists and Liked Music, and the server uses your account when YouTube asks it to "confirm you're not a bot".</p>
    <ol class="steps">
      <li>On a computer, open a <b>private (incognito) window</b> and sign in at <b>music.youtube.com</b>.</li>
      <li>Export its cookies with a "cookies.txt" browser extension (for example <i>Get cookies.txt LOCALLY</i>), in Netscape format.</li>
      <li>Close the private window without signing out, so the cookies keep working.</li>
      <li>Choose the file here, or paste its text.</li>
    </ol>
    <label class="filepick">${icon('import')}<span>Choose cookies.txt</span><input type="file" name="file" accept=".txt,text/plain"></label>
    <textarea name="text" rows="4" placeholder="…or paste the cookies.txt text here" spellcheck="false"></textarea>
    <p class="muted small">The cookies are stored only on this server, for your account, and give access to your Google account: link only on a server you trust. Unlink any time in Settings.</p>
    <div class="ask-err" hidden></div>
    <div class="dlg-actions"><button class="btn" value="cancel">Cancel</button><button class="btn white" value="ok" data-go>Link account</button></div></form>`);
  const form = $('form', d);
  form.file.addEventListener('change', async () => {
    const f = form.file.files[0];
    if (f) { form.text.value = await f.text(); $('.filepick span', d).textContent = f.name; }
  });
  form.addEventListener('submit', async e => {
    if (!e.submitter || e.submitter.value !== 'ok') return;
    e.preventDefault();
    const b = e.submitter, err = $('.ask-err', d);
    if (!form.text.value.trim()) { err.textContent = 'Choose the cookies.txt file first'; err.hidden = false; return; }
    b.disabled = true;
    b.innerHTML = '<span class="spinner small"></span>Checking…';
    try {
      const r = await fetch('/api/google', { method: 'PUT', headers: { 'Content-Type': 'text/plain' }, body: form.text.value });
      const data = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(data.error || r.statusText);
      S.google = data;
      toast(`Linked ${data.account && data.account.name ? data.account.name : 'your YouTube Music account'}`);
      settingsDialog();
    } catch (x) {
      err.textContent = x.message;
      err.hidden = false;
      b.disabled = false;
      b.textContent = 'Link account';
    }
  });
}

/* ---------- importing playlists ---------- */
async function importLink(url, name = '', btn) {
  if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spinner small"></span>Importing…'; }
  else toast('Importing…');
  try {
    const r = await api('import', 'POST', { url, name });
    setUser(r.state);
    toast(`Imported ${plural(r.count, 'song')} into “${r.name}”`);
    if ($('#dlg').open) $('#dlg').close();
    location.hash = '#/playlist/' + enc(r.id);
    return true;
  } catch (e) {
    toast(e.message);
    if (btn) { btn.disabled = false; btn.textContent = 'Import'; }
    return false;
  }
}

async function importDialog(prefill = '') {
  const d = dialog(`<form method="dialog" class="ask"><div class="dlg-head"><h2>Import playlist</h2><button class="icon-btn" value="cancel" aria-label="Close">${icon('close')}</button></div>
    <p class="muted">Paste a link to a YouTube Music or YouTube playlist or album. Every song is copied into a playlist of your own, which you can reorder and edit.</p>
    <label>Link<input name="url" value="${esc(prefill)}" placeholder="https://music.youtube.com/playlist?list=…" autocomplete="off" spellcheck="false"></label>
    <label><span>Name <span class="muted small">(optional)</span></span><input name="name" placeholder="Same as on YouTube Music" autocomplete="off"></label>
    <div class="dlg-actions"><button class="btn white" value="ok" data-go>Import</button></div>
    <div class="import-lib"></div></form>`);
  const form = $('form', d);
  form.url.focus();
  form.addEventListener('submit', e => {
    if (!e.submitter || e.submitter.value !== 'ok') return;
    e.preventDefault();
    if (!form.url.value.trim()) return form.url.focus();
    importLink(form.url.value, form.name.value, e.submitter);
  });
  const box = $('.import-lib', d);
  box.innerHTML = '<div class="spinner"></div>';
  try { if (!S.google) S.google = await api('google'); } catch { S.google = { linked: false }; }
  if (!d.open) return;
  if (!S.google.linked) {
    box.innerHTML = `<div class="set-row"><div class="set-text"><b>Your private playlists</b><span>Link your YouTube Music account to import playlists only you can see, and your Liked Music</span></div><button class="btn small" type="button" data-google-link>Link account</button></div>`;
  } else {
    box.innerHTML = `<h3>From your YouTube Music library</h3><div class="spinner"></div>`;
    try {
      const list = await api('google/playlists');
      if (!d.open) return;
      box.innerHTML = `<h3>From your YouTube Music library</h3>` + (list.length ? list.map(p => `<div class="set-row imp-row">
          <div class="imp-thumb">${p.thumb ? img(p.thumb, 96) : icon(p.id === 'LM' ? 'like' : 'queue')}</div>
          <div class="set-text"><b>${esc(p.title)}</b>${p.count ? `<span>${esc(p.count)} songs</span>` : ''}</div>
          <button class="btn small" type="button" data-import="${esc(p.id)}" data-title="${esc(p.title)}">Import</button></div>`).join('')
        : '<p class="muted">No playlists in your YouTube Music library.</p>');
    } catch (e) { if (d.open) box.innerHTML = `<h3>From your YouTube Music library</h3><p class="muted">${esc(e.message)}</p>`; }
  }
  box.addEventListener('click', e => {
    const b = e.target.closest('button');
    if (!b) return;
    if (b.hasAttribute('data-google-link')) googleDialog();
    else if (b.dataset.import) importLink(b.dataset.import === 'LM' ? 'LM' : `https://music.youtube.com/playlist?list=${b.dataset.import}`, b.dataset.import === 'LM' ? 'Liked Music (YouTube)' : '', b);
  });
}

/* =========================================================================
   settings
   ========================================================================= */
const QUALITY = [['low', 'Low'], ['normal', 'Normal'], ['high', 'High']];
const CROSSFADE = [['0', 'Off'], ['2', '2 seconds'], ['4', '4 seconds'], ['6', '6 seconds'], ['8', '8 seconds'], ['10', '10 seconds'], ['12', '12 seconds']];
const VIDEO_QUALITY = [['auto', 'Auto'], ['360', '360p'], ['480', '480p'], ['720', '720p'], ['1080', '1080p']];
function displayName(type, code) {
  try { return new Intl.DisplayNames([navigator.language || 'en', 'en'], { type }).of(code.replace('_', '-')) || code; } catch { return code; }
}
const options = (list, value) => list.map(([v, label]) => `<option value="${esc(v)}"${v === value ? ' selected' : ''}>${esc(label)}</option>`).join('');
const setRow = (title, sub, control) => `<div class="set-row"><div class="set-text"><b>${esc(title)}</b>${sub ? `<span>${esc(sub)}</span>` : ''}</div>${control}</div>`;
const switchBtn = (key, on) => `<button class="switch${on ? ' on' : ''}" data-set="${key}" role="switch" aria-checked="${on}"></button>`;

async function settingsDialog() {
  const d = dialog(`<div class="dlg-head"><h2>Settings</h2><button class="icon-btn" data-act="dlg-close" aria-label="Close">${icon('close')}</button></div><div class="spinner"></div>`);
  d.classList.add('wide');
  d.addEventListener('close', () => d.classList.remove('wide'), { once: true });
  bindSettings(d);
  let data = {};
  const mode = fetch('/_gundil/mode').then(r => r.ok ? r.json() : null).catch(() => null);
  const google = api('google').catch(() => null);
  const people = S.session.accounts && S.session.admin ? api('users').catch(() => []) : Promise.resolve([]);
  try { data = await api('settings'); } catch (e) { toast(e.message); }
  S.conn = await mode;
  S.people = await people;
  if (!d.open) return;
  S.settings = data.settings || S.settings;
  S.options = data.options || S.options;
  renderSettings(d);
  S.google = (await google) || { linked: false };
  if (d.open && $('.settings', d)) renderSettings(d);
}

function renderSettings(d) {
  const st = S.settings || {};
  const op = S.options || { languages: [], locations: [] };
  const regions = [['', 'Default (chosen by YouTube)'], ...op.locations.map(c => [c, displayName('region', c)]).sort((a, b) => a[1].localeCompare(b[1]))];
  if (st.region && !op.locations.includes(st.region)) regions.push([st.region, st.region]);
  const langs = op.languages.map(c => [c, displayName('language', c)]).sort((a, b) => a[1].localeCompare(b[1]));
  if (!langs.length) langs.push([st.language || 'en', displayName('language', st.language || 'en')]);
  const last = S.info.lastUpdate;
  $('.spinner', d) && $('.spinner', d).remove();
  const body = $('.settings', d) || d.appendChild(Object.assign(document.createElement('div'), { className: 'settings' }));
  const m = S.conn;
  const conn = m && m.canChange ? `
    <h3>This computer</h3>
    ${setRow(m.mode === 'client' ? `Client of ${m.server.replace(/^https?:\/\//, '')}` : 'This computer is the server',
      m.mode === 'client' ? 'Your library and YouTube Music come from that server' : `Other devices connect to ${(m.addresses || [])[0] || `port ${location.port || 80}`}`,
      `<a class="btn small" href="/_gundil/">Change</a>`)}` : '';
  const me = S.session.user;
  const g = S.google;
  const acct = me ? `
    <h3>Your account</h3>
    ${setRow(`Signed in as ${me.name}`, me.admin ? 'Admin: manages accounts and server settings' : 'Your own library, playlists and history', `<span class="row-btns"><button class="btn small" data-set="my-password">Change password</button><button class="btn small" data-set="sign-out">Sign out</button></span>`)}` : '';
  const people = !S.session.accounts ? (S.session.admin ? `
    <h3>Accounts</h3>
    ${setRow('Accounts are off', 'Everyone using this server shares one library. Set up accounts to give each person their own playlists, likes and history, with a password.', `<button class="btn small" data-set="setup-accounts">Set up accounts</button>`)}` : '')
    : me && me.admin ? `
    <h3>Accounts <span class="muted small">· on this server</span></h3>
    ${(S.people || []).map(u => setRow(u.name + (u.admin ? ' · admin' : ''), u.name === me.name ? 'You' : '',
      u.name === me.name ? '' : `<span class="row-btns"><button class="btn small" data-set="toggle-admin" data-user="${esc(u.name)}" data-admin="${u.admin ? 1 : 0}">${u.admin ? 'Remove admin' : 'Make admin'}</button><button class="btn small" data-set="reset-password" data-user="${esc(u.name)}">Password</button><button class="btn small" data-set="delete-user" data-user="${esc(u.name)}">Delete</button></span>`)).join('')}
    <div class="set-row"><div class="set-text"><b>Add an account</b><span>For each person using this server</span></div><button class="btn small" data-set="add-user">${icon('add')}Add</button></div>
    ${setRow('Let people create their own account', 'A "Create an account" link on the sign-in screen. New accounts are never admins.', switchBtn('signUp', !!st.signUp))}
    ${st.signUp ? setRow('Invite code', st.inviteCode ? 'Only people who know it can create an account' : 'Empty: anyone who can reach this server can create an account', `<input data-set="inviteCode" class="set-input" value="${esc(st.inviteCode || '')}" placeholder="No code" maxlength="64" autocomplete="off" spellcheck="false">`) : ''}` : '';
  const google = `
    <h3>YouTube Music account</h3>
    ${g === null || g === undefined ? '<div class="spinner small"></div>' : !g.linked
      ? setRow('Not linked', 'Link it to import your private playlists and Liked Music, and to get past YouTube\'s "confirm you\'re not a bot" check', `<button class="btn small" data-set="google-link">Link</button>`)
      : setRow(g.account ? `${g.account.name}${g.account.handle ? ' · ' + g.account.handle : ''}` : 'Linked', g.error ? `Not working: ${g.error}. Link again with fresh cookies.` : 'Used to import your playlists, and when YouTube asks to sign in',
        `<span class="row-btns"><button class="btn small" data-set="import">Import playlists</button>${g.error ? '<button class="btn small" data-set="google-link">Link again</button>' : ''}<button class="btn small" data-set="google-unlink">Unlink</button></span>`)}`;
  const admin = S.session.admin;
  body.innerHTML = `${acct}${conn}
    <h3>Playback on this device</h3>
    ${setRow('Audio quality', 'Lower quality uses less mobile data', `<select data-set="quality">${options(QUALITY, audioQuality())}</select>`)}
    ${setRow('Video quality', 'For the Video side of the Song / Video switch', `<select data-set="videoQuality">${options(VIDEO_QUALITY, local.get('videoQuality', 'auto'))}</select>`)}
    ${setRow('Autoplay', 'Keep playing similar songs when the queue ends', switchBtn('autoplay', S.autoplay))}
    ${setRow('Downloads', dlCount() ? `${plural(dlCount(), 'song')} on this device, ${mb(dlBytes())}` : 'Songs you download play without a connection', `<a class="btn small" href="#/library/downloads">Open</a>`)}
    ${setRow('Crossfade', 'Fade each song into the next. Not in Video mode or on the server\'s speaker', `<select data-set="crossfade">${options(CROSSFADE, String(crossfadeSecs()))}</select>`)}
    ${google}
    ${people}
    <h3>Server <span class="muted small">· ${esc(S.info.hostname || '')}, for everyone using it${admin ? '' : '; only an admin can change these'}</span></h3>
    ${setRow('Region', 'Which country Home, Explore and charts are for', `<select data-set="region">${options(regions, st.region || '')}</select>`)}
    ${setRow('Language', 'Language of YouTube Music titles and sections', `<select data-set="language">${options(langs, st.language || 'en')}</select>`)}
    ${setRow('Speaker audio quality', 'When the server plays to its speaker or Bluetooth', `<select data-set="audioQuality">${options(QUALITY, st.audioQuality || 'normal')}</select>`)}
    ${setRow('Update YouTube support daily', last && last.time ? `Last updated ${new Date(last.time * 1000).toLocaleString()}` : 'Installs new yt-dlp and ytmusicapi versions when YouTube changes', switchBtn('autoUpdate', !!st.autoUpdate))}
    <div class="set-row"><div class="set-text"><b>Update YouTube support now</b></div><button class="btn small" data-set="yt-update">${icon('refresh')}Update</button></div>
    <h3>History</h3>
    <div class="set-row"><div class="set-text"><b>Clear watch history</b><span>Songs you played${me ? '' : ', on this server'}</span></div><button class="btn small" data-set="clear-history">Clear</button></div>
    <div class="set-row"><div class="set-text"><b>Clear search history</b></div><button class="btn small" data-set="clear-searches">Clear</button></div>`;
  if (!admin) for (const el of body.querySelectorAll('[data-set=region], [data-set=language], [data-set=audioQuality], [data-set=autoUpdate], [data-set=yt-update]')) el.disabled = true;
}

async function saveServerSetting(change) {
  try {
    const r = await api('settings', 'PUT', change);
    settingsChanged(r.settings);
    return true;
  } catch (e) { toast(e.message); return false; }
}

/* The server's settings changed (here or on another device). */
function settingsChanged(now, fromNotice) {
  const old = S.settings;
  if (fromNotice && old) now = { ...now, inviteCode: old.inviteCode };
  S.settings = now;
  if (old && (old.region !== now.region || old.language !== now.language)) {
    ytCache.clear();
    toast(now.region ? `YouTube Music results now for ${displayName('region', now.region)}` : 'YouTube Music region set to default');
    setTimeout(() => route(true), 1500); // the helper restarts with the new region
  }
  if ($('#dlg').open && $('.settings', $('#dlg'))) renderSettings($('#dlg'));
}

function bindSettings(d) {
  d.onchange = async e => {
    const el = e.target.closest('[data-set]');
    if (!el) return;
    const k = el.dataset.set, v = el.value;
    if (k === 'quality') { local.set('quality', v); toast(S.playing && !isServer() ? 'Audio quality changes from the next song' : 'Audio quality saved'); }
    else if (k === 'videoQuality') { local.set('videoQuality', v); syncVideo(); }
    else if (k === 'crossfade') { local.set('crossfade', +v); if (!+v) endFade(); toast(+v ? `Songs now crossfade over ${v} seconds` : 'Crossfade off'); }
    else if (k === 'inviteCode') { if (await saveServerSetting({ inviteCode: v })) toast(v.trim() ? 'Invite code saved' : 'Invite code removed: anyone can create an account'); }
    else await saveServerSetting({ [k]: v });
  };
  d.onclick = async e => {
    if (e.target.closest('[data-act="dlg-close"]') || e.target === d) return d.close();
    const el = e.target.closest('button[data-set]');
    if (!el) return;
    const k = el.dataset.set;
    if (['setup-accounts', 'add-user', 'my-password', 'reset-password', 'toggle-admin', 'delete-user', 'sign-out'].includes(k)) return void accountAction(k, el);
    if (k === 'google-link') return void googleDialog();
    if (k === 'import') return void importDialog();
    if (k === 'google-unlink') {
      if (!await confirmInline(el, 'Tap again to unlink')) return;
      S.google = await api('google', 'DELETE');
      toast('YouTube Music account unlinked');
      return void renderSettings(d);
    }
    if (k === 'autoplay') { setAutoplay(!S.autoplay); renderSettings(d); }
    else if (k === 'autoUpdate') await saveServerSetting({ autoUpdate: !(S.settings || {}).autoUpdate });
    else if (k === 'signUp') {
      const on = !(S.settings || {}).signUp;
      if (await saveServerSetting({ signUp: on })) toast(on ? 'People can now create their own account' : 'Only admins can add accounts now');
    }
    else if (k === 'yt-update') await runYtUpdate(el, () => { if (d.open) renderSettings(d); });
    else if (k === 'clear-history') {
      if (await confirmInline(el)) { setUser(await api('history', 'DELETE')); toast('Watch history cleared'); }
    } else if (k === 'clear-searches') {
      if (await confirmInline(el)) { setUser(await api('searches', 'DELETE')); toast('Search history cleared'); }
    }
  };
}

/* Second tap confirms, so a dialog doesn't have to open over this one. */
function confirmInline(btn, ask = 'Tap again to clear') {
  if (btn.dataset.sure) return Promise.resolve(true);
  const was = btn.textContent;
  btn.dataset.sure = '1';
  btn.textContent = ask;
  setTimeout(() => { if (btn.isConnected) { delete btn.dataset.sure; btn.textContent = was; } }, 4000);
  return Promise.resolve(false);
}

async function runYtUpdate(b, done) {
  S.ytUpdating = true; // this device shows its own result
  b.disabled = true;
  const was = b.innerHTML;
  b.innerHTML = '<span class="spinner small"></span>Updating… this can take a minute';
  try {
    const r = await api('youtube/update', 'POST', {});
    S.info = await api('info');
    toast(r.message);
    if (done) done();
  } catch (err) {
    toast(err.message.split('\n')[0]);
    b.innerHTML = was;
  }
  b.disabled = false;
  setTimeout(() => { S.ytUpdating = false; }, 3000); // after the server's own notice arrives
}

/* =========================================================================
   speakers & Bluetooth
   ========================================================================= */
const D = { bt: null, audio: null, busy: null, timer: null };

function renderCast() {
  const elsewhere = isServer();
  for (const b of $$('.cast-btn')) {
    b.innerHTML = icon(elsewhere ? 'castOn' : 'cast');
    b.title = elsewhere ? `Playing on ${S.info.hostname || 'server'}` : 'Play on a speaker';
  }
}

async function openCast() {
  hideMenu();
  renderCastDialog();
  if (!$('#cast-dlg').open) $('#cast-dlg').showModal();
  refreshCast();
}

async function refreshCast() {
  const jobs = [api('bluetooth').then(bt => { D.bt = bt; }).catch(() => {})];
  if (S.info.player && isServer()) jobs.push(api('player/devices').then(a => { D.audio = a; }).catch(() => { D.audio = null; }));
  await Promise.all(jobs);
  if (!$('#cast-dlg').open) return;
  renderCastDialog();
  clearTimeout(D.timer);
  if (D.bt && D.bt.scanning) D.timer = setTimeout(refreshCast, 2500);
}

function renderCastDialog() {
  const host = esc(S.info.hostname || 'server');
  const bt = D.bt;
  let html = `<div class="dlg-head"><h2>Play on</h2><button class="icon-btn" data-act="cast-close" aria-label="Close">${icon('close')}</button></div>
  <div class="outputs">
    <button class="output${!isServer() ? ' on' : ''}" data-act="output" data-out="local">${icon('phone')}
      <div><div class="output-title">This device</div><div class="muted small">Plays here. Bluetooth headphones paired with this phone or computer work as usual.</div></div></button>
    <button class="output${isServer() ? ' on' : ''}" data-act="output" data-out="server" ${S.info.player ? '' : 'disabled'}>${icon('speaker')}
      <div><div class="output-title">${host}</div><div class="muted small">${S.info.player ? 'Plays from the music server, through its speaker or a Bluetooth speaker connected to it.' : 'Install mpv on the server to play through its speakers.'}</div></div></button>
  </div>`;
  if (isServer() && D.audio && D.audio.devices && D.audio.devices.length) {
    html += `<div class="dlg-sub"><label for="audio-device">Sound output on ${host}</label></div>
      <select id="audio-device">${D.audio.devices.map(a => `<option value="${esc(a.name)}"${a.name === D.audio.current ? ' selected' : ''}>${esc(a.description || a.name)}</option>`).join('')}</select>`;
  }
  html += `<div class="dlg-sub"><span>${icon('bluetooth')}Bluetooth on ${host}</span>`;
  if (bt && bt.available) html += `<button class="btn small" data-act="bt-scan" ${bt.scanning ? 'disabled' : ''}>${bt.scanning ? 'Scanning…' : 'Scan'}</button>`;
  html += '</div>';
  if (!bt) html += '<p class="muted small">Checking…</p>';
  else if (!bt.available) html += `<p class="muted small">${esc(bt.reason)}</p>`;
  else {
    const unnamed = d => /^([0-9A-F]{2}[-:]){5}[0-9A-F]{2}$/i.test(d.name);
    const list = bt.devices.filter(d => d.paired || d.connected || !unnamed(d));
    if (!bt.powered) html += '<p class="muted small">Bluetooth is off. Scanning turns it on.</p>';
    if (!list.length) html += '<p class="muted small">No devices yet. Put your speaker in pairing mode, then scan.</p>';
    html += '<div class="bt-list">' + list.map(d => {
      const busy = D.busy === d.mac;
      const status = busy ? 'Connecting…' : d.connected ? 'Connected' : d.paired ? 'Paired' : 'Available';
      return `<div class="bt${d.connected ? ' on' : ''}">
        <div class="bt-text"><div>${esc(d.name)}</div><div class="muted small">${status}${d.audio ? ' · Audio' : ''}</div></div>
        ${d.paired && !d.connected ? `<button class="btn small" data-act="bt-remove" data-mac="${d.mac}" ${busy ? 'disabled' : ''}>Forget</button>` : ''}
        <button class="btn small${d.connected ? '' : ' white'}" data-act="${d.connected ? 'bt-disconnect' : 'bt-connect'}" data-mac="${d.mac}" ${busy ? 'disabled' : ''}>${d.connected ? 'Disconnect' : 'Connect'}</button>
      </div>`;
    }).join('') + '</div>';
  }
  $('#cast-dlg').innerHTML = html;
}

async function btAction(action, mac) {
  D.busy = mac || null;
  renderCastDialog();
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
  refreshCast();
}

/* =========================================================================
   events
   ========================================================================= */
document.addEventListener('click', async e => {
  const menu = $('#menu');
  if (!menu.hidden) {
    const b = e.target.closest('#menu .mi');
    hideMenu();
    e.preventDefault();
    e.stopPropagation();
    if (b) { try { await menu.items[+b.dataset.i].run(); } catch (err) { toast(err.message); } }
    return;
  }
  if (!$('#suggest').hidden && !e.target.closest('.tb-center')) closeSuggest();

  const actEl = e.target.closest('[data-act]');
  const act = actEl && actEl.dataset.act;
  const row = e.target.closest('.row');
  const keyed = e.target.closest('[data-k]');
  const item = keyed ? reg.get(keyed.dataset.k) : null;

  try {
    switch (act) {
      case 'toggle': return void toggle();
      case 'next': return void next();
      case 'prev': return void prev();
      case 'shuffle': return void toggleShuffle();
      case 'repeat': return void cycleRepeat();
      case 'mute': return void toggleMute();
      case 'autoplay': return void setAutoplay(!S.autoplay);
      case 'pp-open': if (!e.target.closest('button, a')) openPlayer(); return;
      case 'pp-toggle': return void (S.ppOpen ? closePlayer() : openPlayer());
      case 'pp-close': return void closePlayer();
      case 'pp-tab': return void setPPTab(actEl.dataset.tab);
      case 'like-current': if (current()) await rate(current(), 'like'); return;
      case 'dislike-current': if (current()) await rate(current(), 'dislike'); return;
      case 'save-current': if (current()) saveToPlaylist([current()]); return;
      case 'share-current': if (current()) share(`https://music.youtube.com/watch?v=${current().videoId}`); return;
      case 'current-menu': if (current()) trackMenu(current(), actEl, { queueIndex: S.index, current: true }); return;
      case 'mode': return void setMode(actEl.dataset.mode);
      case 'sleep': return void sleepMenu(actEl);
      case 'fullscreen': return void setFullscreen(!$('#pp-art').classList.contains('fs'));
      case 'video-tap': return void ($('#pp-art').classList.contains('fs') ? toggle() : null);
      case 'save-queue': return void newPlaylistDialog(S.queue);
      case 'q-remove': return void removeFromQueue(+row.dataset.q);
      case 'lyric-seek': lyricHold = 0; return void seek(+actEl.dataset.start);
      case 'lyrics-fs': return void setLyricsFullscreen(!$('#pp-body').classList.contains('lyrics-fs'));
      case 'lyrics-back': lyricHold = 0; return void syncLyrics(true);
      case 'cast': return void openCast();
      case 'cast-close': return void $('#cast-dlg').close();
      case 'output': await setOutput(actEl.dataset.out); return void refreshCast();
      case 'bt-scan': return void btAction('scan');
      case 'bt-connect': return void btAction('connect', actEl.dataset.mac);
      case 'bt-disconnect': return void btAction('disconnect', actEl.dataset.mac);
      case 'bt-remove': return void btAction('remove', actEl.dataset.mac);
      case 'guide':
        if (innerWidth >= 1150) { $('#app').classList.toggle('mini'); } else { $('#app').classList.toggle('guide-forced'); }
        return;
      case 'import-playlist': return void importDialog();
      case 'list-download': {
        const l = reg.get(actEl.dataset.list);
        if (!l || !l.tracks.length) return;
        if (l.tracks.every(t => isDownloaded(t.videoId))) return void toast('All downloaded. To remove them, use Library → Downloads');
        return void download(l.tracks);
      }
      case 'dl-clear': {
        if (!actEl.dataset.sure) { actEl.dataset.sure = '1'; actEl.textContent = 'Tap again to remove all'; return; }
        return void removeDownloads(Object.keys(dl.list)).then(() => { toast('Downloads removed'); route(true); });
      }
      case 'avatar-menu':
        return void showMenu([
          S.session.user ? { icon: 'account', label: S.session.user.name + (S.session.user.admin ? ' (admin)' : ''), run: settingsDialog } : null,
          S.session.user ? '-' : null,
          { icon: 'library', label: 'Library', run: () => { location.hash = '#/library'; } },
          { icon: 'history', label: 'History', run: () => { location.hash = '#/library/history'; } },
          { icon: 'cast', label: 'Speakers & Bluetooth', run: openCast },
          { icon: 'import', label: 'Import playlist', run: () => importDialog() },
          { icon: 'settings', label: 'Settings', run: settingsDialog },
          { icon: 'info', label: 'About', run: aboutDialog },
          S.session.user ? { icon: 'logout', label: 'Sign out', run: signOut } : null,
          !android && S.conn && S.conn.canQuit ? '-' : null,
          !android && S.conn && S.conn.canQuit ? { icon: 'power', label: 'Quit Gundil Music', run: quitDialog } : null,
          android ? { icon: 'speaker', label: 'Change server', run: () => android.changeServer() } : null,
        ], actEl);
      case 'search-open':
        $('#app').classList.add('searching');
        $('#search').focus();
        return;
      case 'search-close':
        $('#app').classList.remove('searching');
        closeSuggest();
        return;
      case 'search-clear':
        setSearch('');
        $('#search').focus();
        return void updateSuggest();
      case 'forget-search': {
        e.stopPropagation();
        S.user = await api('searches?q=' + enc(actEl.dataset.q), 'DELETE');
        return void updateSuggest();
      }
      case 'like': case 'dislike': {
        e.preventDefault();
        const t = rowTrack(row);
        if (t) await rate(t, act);
        return;
      }
      case 'menu': {
        e.preventDefault();
        const t = rowTrack(row);
        const l = reg.get(row.dataset.list) || {};
        if (t) trackMenu(t, actEl, { own: l.own, idx: +row.dataset.idx });
        return;
      }
      case 'item-menu': {
        e.preventDefault();
        if (item) itemMenu(item, actEl);
        return;
      }
      case 'card-play': e.preventDefault(); if (item) await playItem(item); return;
      case 'artist-shuffle': return void artistPlay(item, 'shuffle');
      case 'artist-radio': return void artistPlay(item, 'radio');
      case 'list-play': case 'list-shuffle': {
        const l = reg.get(actEl.dataset.list);
        if (!l || !l.tracks.length) return;
        const playable = l.tracks.filter(t => t.available !== false);
        if (l.mode === 'radio' && playable.length === 1) return void playRadio(playable[0], l.source);
        return void playList(playable, act === 'list-shuffle' ? Math.floor(Math.random() * playable.length) : 0, l.source, { shuffle: act === 'list-shuffle' });
      }
      case 'toggle-save':
        if (item) { await setSaved(item, !isSaved(item)); route(true); }
        return;
      case 'save-track': if (item) saveToPlaylist([item]); return;
      case 'own-menu': {
        const l = reg.get(actEl.dataset.list);
        const id = item.id;
        return void showMenu([
          { icon: 'shuffle', label: 'Shuffle play', run: () => l.tracks.length && playList(l.tracks, Math.floor(Math.random() * l.tracks.length), l.source, { shuffle: true }) },
          { icon: 'playNext', label: 'Play next', run: () => insert(l.tracks, true) },
          { icon: 'queue', label: 'Add to queue', run: () => insert(l.tracks, false) },
          { icon: 'edit', label: 'Edit playlist', run: () => editPlaylist(id) },
          { icon: 'delete', label: 'Delete playlist', run: async () => {
            if (!await confirmDialog('Delete playlist?', `“${item.title}” will be deleted. This can't be undone.`, 'Delete')) return;
            setUser(await api('playlists/' + id, 'DELETE'));
            location.hash = '#/library';
          } },
        ], actEl);
      }
      case 'edit-playlist': return void editPlaylist(actEl.dataset.id);
      case 'new-playlist': {
        const id = await newPlaylistDialog();
        if (id) location.hash = '#/playlist/' + id;
        return;
      }
      case 'clear-history':
        if (await confirmDialog('Clear history?', 'Your listening history on this server will be erased.', 'Clear')) { setUser(await api('history', 'DELETE')); route(true); }
        return;
      case 'scroll': {
        const sc = actEl.closest('.shelf').querySelector('.carousel, .qp-grid, .mood-grid');
        if (sc) sc.scrollBy({ left: +actEl.dataset.dir * sc.clientWidth * 0.9, behavior: 'smooth' });
        return;
      }
      case 'expand': return void actEl.classList.toggle('open');
      case 'retry': ytCache.clear(); return void route();
    }
  } catch (err) {
    toast(err.message);
    return;
  }

  // Clicking a song row or song card plays it.
  if (e.target.closest('a, button')) {
    const sg = e.target.closest('.sg');
    if (sg && !e.target.closest('button')) submitSearch(sg.dataset.q);
    return;
  }
  const sg = e.target.closest('.sg');
  if (sg) return submitSearch(sg.dataset.q);
  const playable = e.target.closest('.row[data-list], .card[data-list]');
  if (playable) {
    if (playable.dataset.q !== undefined) return jump(+playable.dataset.q);
    const l = reg.get(playable.dataset.list);
    const i = +playable.dataset.idx;
    if (!l || !l.tracks[i]) return;
    if (l.mode === 'radio') playRadio(l.tracks[i], l.source);
    else {
      const ok = l.tracks.filter(t => t.available !== false);
      playList(ok, ok.indexOf(l.tracks[i]), l.source);
    }
    return;
  }
  const q = e.target.closest('.qrow');
  if (q) jump(+q.dataset.q);
});

$('#scrim').addEventListener('click', hideMenu);
$('#cast-dlg').addEventListener('click', e => { if (e.target === e.currentTarget) e.currentTarget.close(); });
$('#dlg').addEventListener('click', e => { if (e.target === e.currentTarget) e.currentTarget.close(); });
document.addEventListener('change', async e => {
  if (e.target.id === 'audio-device') {
    try { await api('player/device', 'POST', { name: e.target.value }); toast('Sound output changed'); } catch (err) { toast(err.message); }
  } else if (e.target.id === 'chart-country') {
    location.hash = '#/explore/charts/' + enc(e.target.value);
  }
});
document.addEventListener('contextmenu', e => {
  const row = e.target.closest('.row[data-list]');
  if (!row || mobile()) return;
  const t = rowTrack(row);
  if (!t) return;
  e.preventDefault();
  const l = reg.get(row.dataset.list) || {};
  trackMenu(t, { getBoundingClientRect: () => ({ right: e.clientX + 240, bottom: e.clientY, top: e.clientY }) }, { own: l.own, idx: +row.dataset.idx });
});

for (const el of [$('#seek'), $('#pp-seek')]) {
  el.addEventListener('input', () => {
    seeking = true;
    el.style.setProperty('--fill', el.value / 10 + '%');
  });
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

document.addEventListener('keydown', e => {
  const typing = /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName) && e.target.type !== 'range';
  if (e.key === 'Escape') {
    if ($('#pp-art').classList.contains('fs')) setFullscreen(false);
    else if ($('#pp-body').classList.contains('lyrics-fs')) setLyricsFullscreen(false);
    else if (!$('#menu').hidden) hideMenu();
    else if (S.ppOpen && !$('dialog[open]')) closePlayer();
    return;
  }
  if (typing || $('dialog[open]') || e.ctrlKey || e.metaKey || e.altKey) return;
  const k = e.key;
  if ((k === ' ' && !/^(BUTTON|A)$/.test(e.target.tagName)) || k === 'k') { e.preventDefault(); toggle(); }
  else if (k === '/') { e.preventDefault(); if (mobile()) $('#app').classList.add('searching'); $('#search').focus(); }
  else if (k === 'N' || k === 'j' && e.shiftKey) next();
  else if (k === 'P') prev();
  else if (k === 'ArrowRight' && current()) seek(Math.min(S.position + 10, (S.duration || 1e9) - 1));
  else if (k === 'ArrowLeft' && current()) seek(Math.max(0, S.position - 10));
  else if (k === '+' || k === '=') setVolume(Math.min(100, S.volume + 5));
  else if (k === '-') setVolume(Math.max(0, S.volume - 5));
  else if (k === 'm') toggleMute();
  else if (k === 'r') cycleRepeat();
  else if (k === 's') toggleShuffle();
  else if (k === 'f' || k === 'p') S.ppOpen ? closePlayer() : openPlayer();
});

$('#main').addEventListener('scroll', () => { scrolls.set(location.hash || '#/', $('#main').scrollTop); }, { passive: true });
window.addEventListener('hashchange', () => route());
addEventListener('resize', () => { if (!mobile()) $('#pp').classList.remove('sheet'); });

function connectEvents() {
  const es = new EventSource('/api/events');
  es.addEventListener('open', () => { serverQVer = -1; }); // the server may have restarted
  es.addEventListener('player', e => applyServer(JSON.parse(e.data)));
  es.addEventListener('state', async () => { try { setUser(await api('state')); } catch { /* ignore */ } });
  es.addEventListener('info', async e => {
    let d = null;
    try { d = JSON.parse(e.data); } catch { /* ignore */ }
    try { S.info = await api('info'); } catch { /* ignore */ }
    if (d && d.youtubeUpdated) youtubeUpdated();
  });
  es.addEventListener('settings', e => settingsChanged(JSON.parse(e.data), true));
  es.addEventListener('accounts', accountsChanged);
  es.addEventListener('bluetooth', () => { if ($('#cast-dlg').open) refreshCast(); });
}

/* =========================================================================
   start
   ========================================================================= */
function staticIcons() {
  const set = (sel, name) => { for (const el of $$(sel)) el.innerHTML = icon(name) + el.innerHTML; };
  set('#guide-btn', 'menu');
  set('.search-ico', 'search');
  set('.search-back', 'back');
  set('.search-clear', 'close');
  set('#m-search', 'search');
  set('.cast-btn', 'cast');
  set('#b-prev, #pp-prev', 'prev');
  set('#b-next, #pp-next', 'next');
  set('#b-shuffle, #pp-shuffle', 'shuffle');
  set('#b-more', 'more');
  set('#pp-close', 'down');
  set('#pp-fs', 'fullscreen');
  set('.ico-playlist-add', 'playlistAdd');
  set('.ico-share', 'share');
}

async function init() {
  staticIcons();
  try {
    S.session = await api('session');
  } catch { /* an older server: no accounts */ }
  if (S.session.accounts && !S.session.user) return showSignIn();
  renderAvatar();
  fetch('/_gundil/mode').then(r => r.ok ? r.json() : null).then(m => { if (m) S.conn = m; }).catch(() => {});
  try {
    const [info, user, server] = await Promise.all([api('info'), api('state'), api('player')]);
    S.info = info;
    S.server = server;
    setUser(user);
  } catch (e) {
    if (dlCount()) return startOffline(); // no server: play the downloads
    $('#view').innerHTML = emptyHTML('info', 'Can\'t reach the music server', esc(e.message), '<button class="btn" onclick="location.reload()">Try again</button>');
    return;
  }
  if (S.output === null) S.output = S.info.player && S.server.playing ? 'server' : 'local';
  if (isServer() && !S.info.player) S.output = 'local';
  audio.volume = Math.min(1, S.volume / 100);

  if (isServer()) applyServer(S.server);
  else {
    const saved = local.get('queue', null);
    if (saved && Array.isArray(saved.tracks) && saved.tracks.length) {
      S.queue = saved.tracks.filter(isTrack);
      S.index = Math.min(Math.max(0, saved.index | 0), S.queue.length - 1);
      S.source = saved.source || '';
      S.playing = false;
      S.paused = true;
    }
  }
  if (!location.hash) history.replaceState(null, '', '#/');
  renderGuide();
  renderCast();
  renderPlayer();
  route();
  connectEvents();
  if ('serviceWorker' in navigator) navigator.serviceWorker.register('sw.js').catch(() => {});
}
init();
