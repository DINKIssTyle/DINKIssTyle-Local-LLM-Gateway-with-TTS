/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

const CACHE_NAME = 'dkst-chat-v11';
const ASSETS = [
    '/',
    '/index.html',
    '/login.html',
    '/web.html',
    '/web.html?v=10',
    '/fonts.css?v=1',
    '/style.css?v=8',
    '/icons.css?v=2',
    '/app.js?v=9',
    '/icons.css',
    '/appicon.png',
    '/site.webmanifest',
    '/vendor/github-dark.min.css',
    '/vendor/marked.min.js',
    '/vendor/highlight.min.js',
    '/vendor/mathjax-tex-svg.js',
    '/vendor/katex.min.js',
    '/vendor/katex.min.css',
    '/vendor/auto-render.min.js',
    '/vendor/esm/unified@11.0.5/es2022/unified.bundle.mjs',
    '/vendor/esm/remark-parse@11.0.0/es2022/remark-parse.bundle.mjs',
    '/vendor/esm/remark-gfm@4.0.1/es2022/remark-gfm.bundle.mjs',
    '/vendor/esm/remark-breaks@4.0.0/es2022/remark-breaks.bundle.mjs',
    '/vendor/esm/remark-math@6.0.0/es2022/remark-math.bundle.mjs',
    '/vendor/esm/remark-rehype@11.1.2/es2022/remark-rehype.bundle.mjs',
    '/vendor/esm/rehype-raw@7.0.0/es2022/rehype-raw.bundle.mjs',
    '/vendor/esm/rehype-katex@7.0.1/es2022/rehype-katex.bundle.mjs',
    '/vendor/esm/rehype-stringify@10.0.1/es2022/rehype-stringify.bundle.mjs',
    '/vendor/fonts/inter-400-latin.woff2',
    '/vendor/fonts/inter-500-latin.woff2',
    '/vendor/fonts/inter-600-latin.woff2',
    '/vendor/fonts/jetbrains-mono-400-latin.woff2',
    '/vendor/fonts/material-icons-round.woff2',
    '/fonts/MaterialIconsRound-Regular.otf',
    '/favicon-32x32.png',
    '/favicon-16x16.png',
    '/favicon.ico',
    '/apple-touch-icon.png'
];

function normalizedPathname(input) {
    const url = typeof input === 'string' ? new URL(input, self.location.origin) : new URL(input.url);
    return url.pathname;
}

// Install Event
self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_NAME).then(cache => {
            console.log('[SW] Caching assets');
            return cache.addAll(ASSETS);
        })
    );
    self.skipWaiting();
});

// Activate Event
self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys => {
            return Promise.all(
                keys.filter(key => key !== CACHE_NAME)
                    .map(key => caches.delete(key))
            );
        })
    );
    self.clients.claim();
});

// Fetch Event (Network First Strategy)
self.addEventListener('fetch', event => {
    // Skip non-GET requests and API calls
    if (event.request.method !== 'GET' || event.request.url.includes('/api/')) {
        return;
    }

    event.respondWith(
        fetch(event.request).catch(() => {
            if (event.request.mode === 'navigate') {
                return caches.match('/web.html?v=10')
                    .then(response => response || caches.match('/web.html'))
                    .then(response => response || caches.match('/index.html'))
                    .then(response => {
                    if (response) {
                        return response;
                    }
                    return new Response('Offline login page is not cached.', {
                        status: 503,
                        headers: new Headers({ 'Content-Type': 'text/plain' })
                    });
                });
            }

            return caches.match(event.request).then(response => {
                if (response) {
                    return response;
                }
                const pathname = normalizedPathname(event.request);
                return caches.keys()
                    .then(keys => Promise.all(keys.map(key => caches.open(key))))
                    .then(cacheInstances => Promise.all(cacheInstances.map(cache => cache.keys())))
                    .then(cacheKeyLists => {
                        for (const keys of cacheKeyLists) {
                            const match = keys.find((key) => normalizedPathname(key) === pathname);
                            if (match) return caches.match(match);
                        }
                        return null;
                    });
            }).then(response => {
                if (response) {
                    return response;
                }
                // Safari throws if respondWith resolves to undefined/null.
                // Return a valid 503 response if network fails and no cache exists.
                return new Response('Network error and asset not cached.', {
                    status: 503,
                    headers: new Headers({ 'Content-Type': 'text/plain' })
                });
            });
        })
    );
});
