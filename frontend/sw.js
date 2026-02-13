/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

const CACHE_NAME = 'dkst-chat-v1';
const ASSETS = [
    '/',
    '/index.html',
    '/web.html',
    '/style.css',
    '/app.js',
    '/favicon-32x32.png',
    '/apple-touch-icon.png'
];

// Install Event
self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_NAME).then(cache => {
            console.log('[SW] Caching assets');
            return cache.addAll(ASSETS);
        })
    );
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
});

// Fetch Event (Network First Strategy)
self.addEventListener('fetch', event => {
    // Skip non-GET requests and API calls
    if (event.request.method !== 'GET' || event.request.url.includes('/api/')) {
        return;
    }

    event.respondWith(
        fetch(event.request).catch(() => {
            return caches.match(event.request);
        })
    );
});
