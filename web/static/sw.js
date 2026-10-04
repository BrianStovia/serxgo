// SearXGo Service Worker for PWA & Offline Support
const CACHE_NAME = 'searxgo-v5';
const ASSETS_TO_CACHE = [
  '/',
  '/static/css/main.css',
  '/static/js/app.js',
  '/manifest.webmanifest'
];

self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME).then(cache => {
      return cache.addAll(ASSETS_TO_CACHE).catch(() => {});
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(keys => {
      return Promise.all(
        keys.filter(k => k !== CACHE_NAME).map(k => caches.delete(k))
      );
    })
  );
  self.clients.claim();
});

self.addEventListener('fetch', event => {
  if (event.request.method !== 'GET') return;
  const url = new URL(event.request.url);

  // 1. Do not intercept cross-origin requests (e.g. CDNs, avatars, external services)
  if (url.origin !== self.location.origin) return;

  // 2. Do not intercept API requests (live streams, SSE chat, instant answers, etc.)
  if (url.pathname.startsWith('/api/')) return;

  // 3. Network-first with cache fallback for static assets
  if (url.pathname.startsWith('/static/') || url.pathname === '/manifest.webmanifest') {
    event.respondWith(
      fetch(event.request).then(res => {
        if (res && res.status === 200) {
          const clone = res.clone();
          caches.open(CACHE_NAME).then(c => c.put(event.request, clone));
        }
        return res;
      }).catch(async () => {
        const cached = await caches.match(event.request);
        return cached || new Response('Offline asset unavailable', { status: 503, headers: { 'Content-Type': 'text/plain' } });
      })
    );
    return;
  }

  // 4. Network with cache fallback for HTML pages
  event.respondWith(
    fetch(event.request).catch(async () => {
      const cached = await caches.match(event.request);
      return cached || new Response('Offline page unavailable', { status: 503, headers: { 'Content-Type': 'text/plain' } });
    })
  );
});
