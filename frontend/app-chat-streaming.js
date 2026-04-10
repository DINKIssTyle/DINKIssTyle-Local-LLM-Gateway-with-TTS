/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTChatStreaming(global) {
    function createChatStreamingController(options = {}) {
        const { deps = {} } = options;
        const {
            attachStreamingAudioButtonToMessage,
            checkAndTriggerLabelPin,
            ensureAssistantMessageElement,
            ensureStreamingMarkdownHosts,
            getMarkdownRenderMode,
            getPassiveSyncWaitingText,
            getStreamingScrollMode,
            holdAutoScrollAtBottom,
            isChatNearBottom,
            observeAutoScrollResizes,
            pulseMessageRender,
            reconcileAssistantActionBarForMessage,
            renderMarkdownIntoHost,
            renderStreamingPreviewIntoHost,
            sanitizeAssistantRenderText,
            scrollToBottom,
            syncAssistantMessageShellState
        } = deps;

        let pendingStreamRenderFrame = 0;
        const pendingStreamRenderUpdates = new Map();

        function appendStreamChunkDedup(existingText, nextChunk) {
            const prev = String(existingText || '');
            const chunk = String(nextChunk || '');
            if (!chunk) return prev;
            if (!prev) return chunk;
            if (prev === chunk) return prev;
            if (prev.endsWith(chunk)) return prev;
            if (chunk.startsWith(prev)) return chunk;
            if (chunk.length > prev.length && chunk.includes(prev)) return chunk;

            const maxOverlap = Math.min(prev.length, chunk.length);
            const minSafeOverlap = 8;
            for (let overlap = maxOverlap; overlap >= minSafeOverlap; overlap -= 1) {
                if (prev.slice(-overlap) === chunk.slice(0, overlap)) {
                    const prevTail = prev.slice(-overlap);
                    const chunkRemainderFirst = chunk.charAt(overlap);
                    const prevTailLast = prevTail.charAt(prevTail.length - 1);
                    if (/\d/.test(prevTailLast) && /\d/.test(chunkRemainderFirst)) {
                        continue;
                    }
                    return prev + chunk.slice(overlap);
                }
            }
            return prev + chunk;
        }

        function deduplicateTrailingParagraph(text) {
            const source = String(text || '');
            if (!source) return source;

            const blocks = source.split(/\n{2,}/);
            if (blocks.length >= 2) {
                const last = blocks[blocks.length - 1].trim();
                const prev = blocks[blocks.length - 2].trim();
                if (last && prev && last === prev) {
                    return blocks.slice(0, -1).join('\n\n');
                }
            }

            const lines = source.split('\n');
            if (lines.length >= 2) {
                const lastLine = lines[lines.length - 1].trim();
                const prevLine = lines[lines.length - 2].trim();
                if (lastLine && prevLine && lastLine === prevLine) {
                    return lines.slice(0, -1).join('\n');
                }
            }

            return source;
        }

        function normalizeComparableTail(text) {
            return String(text || '')
                .replace(/\s+/g, ' ')
                .trim();
        }

        function deduplicateCommittedPending(committedText, pendingText) {
            const committed = String(committedText || '');
            const pending = String(pendingText || '');
            if (!pending) {
                return { committedText: committed, pendingText: '' };
            }

            const normalizedCommitted = normalizeComparableTail(committed);
            const normalizedPending = normalizeComparableTail(pending);
            if (!normalizedPending) {
                return { committedText: committed, pendingText: '' };
            }

            if (normalizedCommitted.endsWith(normalizedPending)) {
                return { committedText: committed, pendingText: '' };
            }

            const committedBlocks = committed.split(/\n{2,}/).map(normalizeComparableTail).filter(Boolean);
            const pendingBlocks = pending.split(/\n{2,}/).map(normalizeComparableTail).filter(Boolean);
            const lastCommittedBlock = committedBlocks[committedBlocks.length - 1] || '';
            const firstPendingBlock = pendingBlocks[0] || '';
            if (lastCommittedBlock && firstPendingBlock && lastCommittedBlock === firstPendingBlock) {
                return { committedText: committed, pendingText: '' };
            }

            return { committedText: committed, pendingText: pending };
        }

        function schedulePendingMarkdownRender(el, pendingHost, pendingText) {
            if (!el || !pendingHost) return;
            if (!el._pendingRenderState) {
                el._pendingRenderState = { scheduled: false, text: '', timerId: null };
            }

            el._pendingRenderState.text = pendingText;
            const renderMode = getMarkdownRenderMode();
            const throttleMs = renderMode === 'balanced' ? 96 : 0;

            if (el._pendingRenderState.scheduled) return;

            const runRender = () => {
                if (!el._pendingRenderState) return;
                el._pendingRenderState.scheduled = false;
                el._pendingRenderState.timerId = null;
                renderStreamingPreviewIntoHost(pendingHost, el._pendingRenderState.text || '');
            };

            el._pendingRenderState.scheduled = true;
            if (throttleMs > 0) {
                el._pendingRenderState.timerId = global.setTimeout(() => {
                    global.requestAnimationFrame(runRender);
                }, throttleMs);
                return;
            }

            global.requestAnimationFrame(runRender);
        }

        function updateMessageContent(id, text) {
            const el = ensureAssistantMessageElement(id);
            if (!el) return;
            const wasNearBottom = isChatNearBottom();

            const bubble = el.querySelector('.message-bubble');
            const { markdownBody: mdBody, committedHost, pendingHost } = ensureStreamingMarkdownHosts(bubble);

            let cleanText = sanitizeAssistantRenderText(text);
            const previousCommittedText = String(el._streamRenderState?.committedText || '');
            const renderMode = getMarkdownRenderMode();
            if (renderMode === 'final') {
                el._streamRenderState = {
                    committedText: cleanText,
                    pendingText: ''
                };
                const responseCard = el.querySelector('.assistant-response-card');
                const actionBar = el.querySelector('.message-actions');
                if (responseCard) responseCard.hidden = !cleanText.trim();
                if (actionBar) {
                    if (el.id && !actionBar.classList.contains('is-ready')) {
                        actionBar.hidden = true;
                    } else {
                        actionBar.hidden = !cleanText.trim();
                    }
                }
                renderStreamingPreviewIntoHost(committedHost, cleanText);
                renderStreamingPreviewIntoHost(pendingHost, '');
                syncAssistantMessageShellState(el);
                if (cleanText.trim()) checkAndTriggerLabelPin();
                scrollToBottom(wasNearBottom);
                return;
            }
            let committedText = '';
            let pendingText = '';

            if (renderMode === 'fast') {
                committedText = cleanText;
                if (committedText !== previousCommittedText) {
                    renderMarkdownIntoHost(committedHost, committedText, { allowLooseFallback: false });
                }
                renderStreamingPreviewIntoHost(pendingHost, '');
            } else {
                pendingText = cleanText;
                renderStreamingPreviewIntoHost(committedHost, '');
                schedulePendingMarkdownRender(el, pendingHost, pendingText);
            }
            el._streamRenderState = { committedText, pendingText };

            const responseCard = el.querySelector('.assistant-response-card');
            const actionBar = el.querySelector('.message-actions');
            const hasVisibleContent = !!(committedText.trim() || pendingText.trim());
            if (responseCard) responseCard.hidden = !hasVisibleContent;
            if (actionBar) {
                if (el.id && !actionBar.classList.contains('is-ready')) {
                    actionBar.hidden = true;
                } else {
                    actionBar.hidden = !hasVisibleContent;
                }
            }
            syncAssistantMessageShellState(el);
            if (hasVisibleContent) checkAndTriggerLabelPin();

            if (!previousCommittedText.trim() && hasVisibleContent) {
                pulseMessageRender(el.querySelector('.assistant-response-card'));
            }

            scrollToBottom(wasNearBottom);
            const codeBlocks = mdBody.querySelectorAll('pre code');
            if (wasNearBottom && codeBlocks.length > 0 && getStreamingScrollMode() !== 'label-top') {
                holdAutoScrollAtBottom(900);
                observeAutoScrollResizes([el, bubble, mdBody, ...mdBody.querySelectorAll('pre')]);
            }
        }

        function flushStreamMessageRender(id) {
            const key = String(id || '');
            if (!key) return;
            const pendingText = pendingStreamRenderUpdates.get(key);
            if (typeof pendingText !== 'string') return;
            pendingStreamRenderUpdates.delete(key);
            updateMessageContent(key, pendingText);
        }

        function scheduleStreamMessageRender(id, text) {
            const key = String(id || '');
            if (!key) return;
            pendingStreamRenderUpdates.set(key, String(text || ''));
            if (pendingStreamRenderFrame) return;

            pendingStreamRenderFrame = global.requestAnimationFrame(() => {
                pendingStreamRenderFrame = 0;
                const updates = Array.from(pendingStreamRenderUpdates.entries());
                pendingStreamRenderUpdates.clear();
                updates.forEach(([messageId, latestText]) => {
                    updateMessageContent(messageId, latestText);
                });
            });
        }

        function finalizeMessageContent(id, text) {
            flushStreamMessageRender(id);
            const el = ensureAssistantMessageElement(id);
            if (!el) return;
            const bubble = el.querySelector('.message-bubble');
            const { committedHost, pendingHost } = ensureStreamingMarkdownHosts(bubble);
            if (!committedHost || !pendingHost) return;

            if (el._pendingRenderState?.timerId) {
                global.clearTimeout(el._pendingRenderState.timerId);
            }
            el._pendingRenderState = null;

            const cleanText = sanitizeAssistantRenderText(text);

            renderMarkdownIntoHost(committedHost, cleanText);
            pendingHost.innerHTML = '';
            pendingHost.textContent = '';
            pendingHost.dataset.markdownSource = '';
            pendingHost.classList.remove('is-stream-preview');
            el._streamRenderState = {
                committedText: cleanText,
                pendingText: ''
            };

            const responseCard = el.querySelector('.assistant-response-card');
            const actionBar = el.querySelector('.message-actions');
            const hasVisibleContent = !!cleanText.trim();
            if (responseCard) responseCard.hidden = !hasVisibleContent;

            if (actionBar) {
                actionBar.hidden = !hasVisibleContent;
                const copyBtn = actionBar.querySelector('.copy-btn');
                const speakBtn = actionBar.querySelector('.speak-btn');
                if (copyBtn) copyBtn.hidden = !hasVisibleContent;
                if (speakBtn) speakBtn.hidden = !hasVisibleContent;
                if (hasVisibleContent) {
                    actionBar.classList.add('is-ready');
                    actionBar.classList.remove('is-pending');
                    attachStreamingAudioButtonToMessage(el);
                }
            }
            syncAssistantMessageShellState(el);
            if (hasVisibleContent) checkAndTriggerLabelPin();
        }

        function updateSyncedMessageContent(id, text, options = {}) {
            const el = ensureAssistantMessageElement(id);
            if (!el) return;
            const animate = options.animate !== false;
            const wasNearBottom = isChatNearBottom();
            const bubble = el.querySelector('.message-bubble');
            const { markdownBody: mdBody, committedHost, pendingHost } = ensureStreamingMarkdownHosts(bubble);
            if (!committedHost || !pendingHost) return;

            const previousCommittedText = String(el._streamRenderState?.committedText || '');
            const cleanText = sanitizeAssistantRenderText(text);
            const renderMode = getMarkdownRenderMode();
            if (renderMode === 'final') {
                renderStreamingPreviewIntoHost(committedHost, cleanText);
                renderStreamingPreviewIntoHost(pendingHost, '');

                if (cleanText !== getPassiveSyncWaitingText()) {
                    el._streamRenderState = {
                        committedText: cleanText,
                        pendingText: ''
                    };
                }

                const responseCard = el.querySelector('.assistant-response-card');
                const hasVisibleContent = !!cleanText.trim();
                if (responseCard) responseCard.hidden = !hasVisibleContent;
                syncAssistantMessageShellState(el);
                if (cleanText.trim()) checkAndTriggerLabelPin();
                scrollToBottom(wasNearBottom);
                return;
            }

            let committedText = '';
            let pendingText = '';

            if (renderMode === 'fast') {
                committedText = cleanText;
                if (committedText !== previousCommittedText) {
                    renderMarkdownIntoHost(committedHost, committedText);
                }
                renderStreamingPreviewIntoHost(pendingHost, '');
            } else {
                pendingText = cleanText;
                renderStreamingPreviewIntoHost(committedHost, '');
                schedulePendingMarkdownRender(el, pendingHost, pendingText);
            }
            el._streamRenderState = { committedText, pendingText };

            const responseCard = el.querySelector('.assistant-response-card');
            const actionBar = el.querySelector('.message-actions');
            const hasVisibleContent = !!(committedText.trim() || pendingText.trim());
            if (responseCard) responseCard.hidden = !hasVisibleContent;
            if (actionBar && actionBar.classList.contains('is-ready')) {
                actionBar.hidden = !hasVisibleContent;
            }
            syncAssistantMessageShellState(el);
            if (hasVisibleContent) {
                checkAndTriggerLabelPin();
                reconcileAssistantActionBarForMessage(el);
            }

            const shouldPulse = animate && !previousCommittedText.trim() && hasVisibleContent;
            if (shouldPulse) {
                pulseMessageRender(el.querySelector('.assistant-response-card'));
            }

            scrollToBottom(wasNearBottom);
            const codeBlocks = mdBody.querySelectorAll('pre code');
            if (wasNearBottom && codeBlocks.length > 0 && getStreamingScrollMode() !== 'label-top') {
                holdAutoScrollAtBottom(900);
                observeAutoScrollResizes([el, bubble, mdBody, ...mdBody.querySelectorAll('pre')]);
            }
        }

        function getSpeakableTextFromMarkdownHost(host) {
            if (!host) return '';
            const clone = host.cloneNode(true);
            clone.querySelectorAll('pre, code').forEach((node) => node.remove());
            return clone.innerText || clone.textContent || '';
        }

        return {
            appendStreamChunkDedup,
            deduplicateCommittedPending,
            deduplicateTrailingParagraph,
            finalizeMessageContent,
            flushStreamMessageRender,
            getSpeakableTextFromMarkdownHost,
            normalizeComparableTail,
            schedulePendingMarkdownRender,
            scheduleStreamMessageRender,
            updateMessageContent,
            updateSyncedMessageContent
        };
    }

    global.DKSTChatStreaming = {
        createChatStreamingController
    };
})(window);
