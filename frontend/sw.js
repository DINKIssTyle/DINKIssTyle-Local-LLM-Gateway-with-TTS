/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

const CACHE_NAME = 'dkst-chat-v24';
const ASSETS = [
    '/',
    '/index.html',
    '/login.html',
    '/web.html',
    '/web.html?v=19',
    '/fonts.css?v=2',
    '/style.css?v=8',
    '/icons.css?v=2',
    '/app-utils.js?v=1',
    '/app-i18n.js?v=1',
    '/app-saved-library.js?v=1',
    '/app-models.js?v=1',
    '/app-tts.js?v=3',
    '/app-session.js?v=1',
    '/app-chat-streaming.js?v=1',
    '/app-chat-ui.js?v=1',
    '/app-progress-ui.js?v=1',
    '/app-mic.js?v=2',
    '/app.js?v=19',
    '/icons.css',
    '/public/icon-512.png',
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
    '/vendor/fonts/noto-sans-kr-400.ttf',
    '/vendor/fonts/noto-sans-kr-500.ttf',
    '/vendor/fonts/noto-sans-kr-600.ttf',
    '/vendor/fonts/noto-sans-kr-700.ttf',
    '/vendor/fonts/jetbrains-mono-400-latin.woff2',
    '/vendor/fonts/material-icons-round.woff2',
    '/vendor/fonts/KaTeX_Main-Regular.woff2',
    '/vendor/fonts/KaTeX_Main-Regular.woff',
    '/vendor/fonts/KaTeX_Main-Regular.ttf',
    '/public/favicon-32.png',
    '/public/favicon-16.png',
    '/public/apple-touch-icon.png'
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
                return caches.match('/web.html?v=19')
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

// ============================================================================
// Push Notification Handling (Optimized for iOS 18.4+)
// ============================================================================

// Push Event: Handle incoming push messages
self.addEventListener('push', event => {
    if (!event.data) return;

    try {
        const data = event.data.json();
        
        // Support for Declarative Web Push (iOS 18.4+) 
        // and standard Web Push Notifications
        if (data.notification) {
            const options = {
                body: data.notification.body || '',
                icon: data.notification.icon || '/public/icon-512.png',
                badge: data.notification.badge || '/public/favicon-32.png',
                data: data.notification.data || {},
                actions: data.notification.actions || []
            };

            event.waitUntil(
                self.registration.showNotification(
                    data.notification.title || 'DKST Chat', 
                    options
                )
            );
        }
    } catch (e) {
        console.error('[SW] Push processing failed:', e);
        // Fallback for simple text push
        const text = event.data.text();
        event.waitUntil(
            self.registration.showNotification('DKST Chat', {
                body: text,
                icon: '/public/icon-512.png'
            })
        );
    }
});

// Notification Click: Handle interaction with notifications
self.addEventListener('notificationclick', event => {
    event.notification.close();

    // Primary action: focus or open the app
    event.waitUntil(
        clients.matchAll({ type: 'window', includeUncontrolled: true }).then(clientList => {
            if (clientList.length > 0) {
                let client = clientList[0];
                for (let i = 0; i < clientList.length; i++) {
                    if (clientList[i].focused) {
                        client = clientList[i];
                    }
                }
                return client.focus();
            }
            return clients.openWindow('/');
        })
    );
});
