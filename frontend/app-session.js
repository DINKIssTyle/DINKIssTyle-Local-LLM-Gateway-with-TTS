/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTSession(global) {
    function createSessionController(options = {}) {
        const { deps = {} } = options;
        const {
            buildSessionFetchOptions,
            getCurrentUser,
            onExternalConfigSync,
            onLLMActivitySyncState,
            onSavedTurnsExternalSync
        } = deps;

        const savedTurnsSyncChannel = typeof global.BroadcastChannel !== 'undefined'
            ? new global.BroadcastChannel('dkst-saved-turns-sync')
            : null;
        const configSyncChannel = typeof global.BroadcastChannel !== 'undefined'
            ? new global.BroadcastChannel('dkst-config-sync')
            : null;
        const llmActivitySyncChannel = typeof global.BroadcastChannel !== 'undefined'
            ? new global.BroadcastChannel('dkst-llm-activity-sync')
            : null;
        const localLLMActivitySourceId = `chat-${Math.random().toString(36).slice(2, 10)}`;

        let syncListenersAttached = false;
        let llmActivityListenerAttached = false;

        function broadcastConfigSync() {
            const payload = {
                type: 'config-updated',
                timestamp: Date.now()
            };
            try {
                configSyncChannel?.postMessage(payload);
            } catch (err) {
                console.warn('Failed to broadcast config sync channel event:', err);
            }
            try {
                global.localStorage.setItem('dkst-config-sync', JSON.stringify(payload));
            } catch (err) {
                console.warn('Failed to broadcast config via storage:', err);
            }
        }

        async function handleExternalConfigSync() {
            try {
                await onExternalConfigSync?.();
            } catch (err) {
                console.warn('Failed to sync config from external event:', err);
            }
        }

        function handleSavedTurnsExternalSync() {
            if (!getCurrentUser?.()) return;
            onSavedTurnsExternalSync?.();
        }

        function broadcastSavedTurnsChange(reason = 'updated') {
            const payload = {
                type: 'saved-turns-sync',
                reason,
                userId: getCurrentUser?.()?.id || '',
                timestamp: Date.now()
            };
            try {
                savedTurnsSyncChannel?.postMessage(payload);
            } catch (err) {
                console.warn('Failed to broadcast saved turns via channel:', err);
            }
            try {
                global.localStorage.setItem('savedTurnsSyncEvent', JSON.stringify(payload));
            } catch (err) {
                console.warn('Failed to broadcast saved turns via storage:', err);
            }
        }

        function setupSyncListeners() {
            if (syncListenersAttached) return;
            syncListenersAttached = true;

            if (savedTurnsSyncChannel) {
                savedTurnsSyncChannel.onmessage = (event) => {
                    const payload = event?.data;
                    if (!payload || payload.type !== 'saved-turns-sync') return;
                    const currentUserId = getCurrentUser?.()?.id || '';
                    if (payload.userId && currentUserId && payload.userId !== currentUserId) return;
                    handleSavedTurnsExternalSync();
                };
            }

            if (configSyncChannel) {
                configSyncChannel.onmessage = (event) => {
                    const payload = event?.data;
                    if (!payload || payload.type !== 'config-updated') return;
                    handleExternalConfigSync();
                };
            }

            global.addEventListener('storage', (event) => {
                if (event.key !== 'savedTurnsSyncEvent' || !event.newValue) return;
                try {
                    const payload = JSON.parse(event.newValue);
                    if (!payload || payload.type !== 'saved-turns-sync') return;
                    const currentUserId = getCurrentUser?.()?.id || '';
                    if (payload.userId && currentUserId && payload.userId !== currentUserId) return;
                    handleSavedTurnsExternalSync();
                } catch (err) {
                    console.warn('Failed to parse saved turn sync payload:', err);
                }
            });

            global.addEventListener('storage', (event) => {
                if (event.key !== 'dkst-config-sync' || !event.newValue) return;
                try {
                    const payload = JSON.parse(event.newValue);
                    if (!payload || payload.type !== 'config-updated') return;
                    handleExternalConfigSync();
                } catch (err) {
                    console.warn('Failed to parse config sync payload:', err);
                }
            });
        }

        function broadcastLLMActivityState(busy, phase = 'answering') {
            if (!llmActivitySyncChannel) return;
            try {
                llmActivitySyncChannel.postMessage({
                    sourceId: localLLMActivitySourceId,
                    busy: !!busy,
                    phase: String(phase || '').trim().toLowerCase() || 'answering',
                    at: Date.now()
                });
            } catch (error) {
                console.warn('[LLMActivitySync] broadcast failed:', error);
            }
        }

        function setupLLMActivitySyncListener() {
            if (llmActivityListenerAttached || !llmActivitySyncChannel) return;
            llmActivityListenerAttached = true;

            llmActivitySyncChannel.addEventListener('message', (event) => {
                const state = event?.data || {};
                if (state?.sourceId === localLLMActivitySourceId) {
                    return;
                }
                onLLMActivitySyncState?.(state);
            });
        }

        async function fetchLastSession() {
            try {
                const response = await global.fetch('/api/last-session', buildSessionFetchOptions());
                if (!response.ok) {
                    return null;
                }
                const data = await response.json();
                if (!data.has_session) {
                    return null;
                }
                return data;
            } catch (e) {
                console.warn('Failed to fetch last session:', e);
                return null;
            }
        }

        async function fetchCurrentChatSession() {
            try {
                const response = await global.fetch('/api/chat-session/current', buildSessionFetchOptions());
                if (!response.ok) return null;
                const data = await response.json();
                return data?.has_session ? data.item : null;
            } catch (e) {
                console.warn('Failed to fetch current chat session:', e);
                return null;
            }
        }

        async function fetchCurrentChatSessionEvents(afterSeq = 0, limit = 400) {
            try {
                const response = await global.fetch(`/api/chat-session/events?after_seq=${afterSeq}&limit=${limit}`, buildSessionFetchOptions());
                if (!response.ok) return { session: null, items: [], totalCount: 0 };
                const data = await response.json();
                return {
                    session: data?.has_session ? data.session : null,
                    items: Array.isArray(data?.items) ? data.items : [],
                    totalCount: Number(data?.total_count || 0)
                };
            } catch (e) {
                console.warn('Failed to fetch current chat session events:', e);
                return { session: null, items: [], totalCount: 0 };
            }
        }

        return {
            broadcastConfigSync,
            broadcastLLMActivityState,
            broadcastSavedTurnsChange,
            fetchCurrentChatSession,
            fetchCurrentChatSessionEvents,
            fetchLastSession,
            setupLLMActivitySyncListener,
            setupSyncListeners
        };
    }

    global.DKSTSession = {
        createSessionController
    };
})(window);
