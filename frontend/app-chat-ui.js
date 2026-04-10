/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTChatUI(global) {
    function createChatUIController(options = {}) {
        const { refs = {}, deps = {} } = options;
        const {
            chatMessages = null,
            inputArea = null,
            reasoningControlBar = null,
            scrollToBottomBtn = null
        } = refs;
        const {
            attachStreamingAudioButtonToMessage,
            commitChatScrollMetrics,
            getStreamingScrollMode,
            isAssistantMessageVisiblyEmpty,
            pulseMessageRender,
            refreshChatScrollMetrics,
            savedLibraryIsOpen,
            triggerHaptic,
            updateReasoningControlVisibility
        } = deps;

        let shouldAutoScroll = true;
        let autoScrollHoldTimeout = null;
        let autoScrollResizeObserver = null;
        let lockScrollToLatest = false;
        let suppressNextScrollEvent = false;
        let activeStreamingMessageId = null;
        let activeStreamingMessagePinnedToTop = false;
        let activeStreamingMessagePinPending = false;
        let pendingScrollToBottom = false;
        let chatScrollMetrics = {
            scrollTop: 0,
            scrollHeight: 0,
            clientHeight: 0,
            distanceFromBottom: 0,
            nearBottom: true,
            longScrollable: false
        };

        function getAssistantMessageParts(elementId) {
            const msgEl = global.document.getElementById(elementId);
            if (!msgEl) return {};
            return {
                msgEl,
                reasoningHost: msgEl.querySelector('.assistant-reasoning'),
                toolsHost: msgEl.querySelector('.assistant-tools'),
                bubble: msgEl.querySelector('.assistant-response-card .message-bubble'),
                markdownBody: msgEl.querySelector('.assistant-response-card .markdown-body'),
                committedHost: msgEl.querySelector('.assistant-response-card .markdown-committed'),
                pendingHost: msgEl.querySelector('.assistant-response-card .markdown-pending')
            };
        }

        function setAssistantActionBarReady(elementId) {
            const { msgEl } = getAssistantMessageParts(elementId);
            if (!msgEl) return;
            const actionBar = msgEl.querySelector('.message-actions');
            if (!actionBar) return;
            if (isAssistantMessageVisiblyEmpty?.(msgEl)) return;
            if (actionBar.classList.contains('is-ready') && !actionBar.hidden) {
                attachStreamingAudioButtonToMessage?.(msgEl);
                return;
            }

            actionBar.classList.add('is-pending');

            const finishReadyState = () => {
                if (!actionBar.isConnected) return;
                actionBar.classList.add('is-ready');
                actionBar.classList.remove('is-pending');
                delete actionBar.dataset.readyScheduled;
                attachStreamingAudioButtonToMessage?.(msgEl);
            };

            if (actionBar.dataset.readyScheduled === 'true') {
                finishReadyState();
                return;
            }

            actionBar.dataset.readyScheduled = 'true';
            global.requestAnimationFrame(() => {
                finishReadyState();
            });
            global.setTimeout(() => {
                finishReadyState();
            }, 120);
        }

        function reconcileAssistantActionBarForMessage(msgEl) {
            if (!msgEl || !msgEl.classList?.contains('assistant') || !msgEl.id) return;
            const actionBar = msgEl.querySelector('.message-actions');
            if (!actionBar || isAssistantMessageVisiblyEmpty?.(msgEl)) return;
            setAssistantActionBarReady(msgEl.id);
        }

        function reconcileVisibleAssistantActionBars() {
            if (!chatMessages) return;
            chatMessages.querySelectorAll('.message.assistant').forEach((msgEl) => {
                reconcileAssistantActionBarForMessage(msgEl);
            });
        }

        function isChatNearBottom() {
            if (!chatMessages) return true;
            return !!chatScrollMetrics.nearBottom;
        }

        function hasLongScrollableChat() {
            if (!chatMessages) return false;
            return !!chatScrollMetrics.longScrollable;
        }

        function updateScrollToBottomButton() {
            if (!scrollToBottomBtn) return;
            const shouldShow = !!chatMessages
                && hasLongScrollableChat()
                && !isChatNearBottom()
                && !savedLibraryIsOpen?.();

            if (!shouldShow && global.document.activeElement === scrollToBottomBtn) {
                scrollToBottomBtn.blur();
            }

            scrollToBottomBtn.classList.toggle('is-visible', shouldShow);
            scrollToBottomBtn.hidden = !shouldShow;
            if (shouldShow) {
                scrollToBottomBtn.removeAttribute('aria-hidden');
                scrollToBottomBtn.removeAttribute('inert');
            } else {
                scrollToBottomBtn.setAttribute('inert', '');
            }
            updateReasoningControlVisibility?.();
        }

        function scrollActiveMessageIntoView() {
            if (!chatMessages || !activeStreamingMessageId) return;
            const activeMessage = global.document.getElementById(activeStreamingMessageId);
            if (!activeMessage) return;
            const responseCard = activeMessage.querySelector('.assistant-response-card');
            const target = responseCard || activeMessage;
            const containerRect = chatMessages.getBoundingClientRect();
            const targetRect = target.getBoundingClientRect();
            const inputRect = inputArea ? inputArea.getBoundingClientRect() : null;
            const reasoningHeight = (!reasoningControlBar || reasoningControlBar.hidden)
                ? 0
                : reasoningControlBar.getBoundingClientRect().height;
            const occlusion = inputRect ? Math.max(0, containerRect.bottom - inputRect.top) : 0;
            const desiredBottom = containerRect.bottom - occlusion - Math.max(16, reasoningHeight > 0 ? 28 : 16);
            const delta = targetRect.bottom - desiredBottom;

            if (delta > 0) {
                suppressNextScrollEvent = true;
                chatMessages.scrollTop += delta;
                updateScrollToBottomButton();
            }
        }

        function scheduleChatScrollToBottom() {
            if (!chatMessages || pendingScrollToBottom) return;
            pendingScrollToBottom = true;
            global.requestAnimationFrame(() => {
                pendingScrollToBottom = false;
                if (!chatMessages) return;
                const metrics = refreshChatScrollMetrics?.() || chatScrollMetrics;
                suppressNextScrollEvent = true;
                chatMessages.scrollTop = metrics.scrollHeight;
                chatScrollMetrics = {
                    scrollTop: Math.max(0, metrics.scrollHeight - metrics.clientHeight),
                    scrollHeight: metrics.scrollHeight,
                    clientHeight: metrics.clientHeight,
                    distanceFromBottom: 0,
                    nearBottom: true,
                    longScrollable: metrics.longScrollable
                };
                commitChatScrollMetrics?.(chatScrollMetrics);
                scrollActiveMessageIntoView();
                updateScrollToBottomButton();
            });
        }

        function scrollToBottom(force = false) {
            if (!chatMessages) return;
            if (!lockScrollToLatest && getStreamingScrollMode?.() === 'label-top' && activeStreamingMessagePinnedToTop) {
                refreshChatScrollMetrics?.();
                return;
            }

            if (!force && !shouldAutoScroll && !lockScrollToLatest) return;
            scheduleChatScrollToBottom();
            shouldAutoScroll = true;
        }

        function holdAutoScrollAtBottom(durationMs = 700) {
            if (!chatMessages) return;
            if (autoScrollHoldTimeout) {
                global.clearTimeout(autoScrollHoldTimeout);
                autoScrollHoldTimeout = null;
            }
            scheduleChatScrollToBottom();
            autoScrollHoldTimeout = global.setTimeout(() => {
                if (chatMessages && (shouldAutoScroll || lockScrollToLatest)) {
                    scheduleChatScrollToBottom();
                }
                autoScrollHoldTimeout = null;
            }, durationMs);
        }

        function observeAutoScrollResizes(elements) {
            if (!chatMessages || typeof global.ResizeObserver === 'undefined') return;
            if (autoScrollResizeObserver) {
                autoScrollResizeObserver.disconnect();
                autoScrollResizeObserver = null;
            }

            const targets = (elements || []).filter(Boolean);
            if (targets.length === 0) return;
            refreshChatScrollMetrics?.();

            autoScrollResizeObserver = new global.ResizeObserver(() => {
                if (!chatMessages) return;
                const previousMetrics = chatScrollMetrics;
                const currentMetrics = refreshChatScrollMetrics?.() || previousMetrics;
                const delta = currentMetrics.scrollHeight - previousMetrics.scrollHeight;

                if (shouldAutoScroll || lockScrollToLatest) {
                    scheduleChatScrollToBottom();
                } else if (Math.abs(delta) > 1) {
                    if (getStreamingScrollMode?.() === 'label-top' && activeStreamingMessagePinnedToTop) {
                        chatScrollMetrics = currentMetrics;
                        commitChatScrollMetrics?.(currentMetrics);
                        updateScrollToBottomButton();
                        return;
                    }

                    suppressNextScrollEvent = true;
                    chatMessages.scrollTop += delta;
                    chatScrollMetrics = {
                        ...currentMetrics,
                        scrollTop: currentMetrics.scrollTop + delta,
                        distanceFromBottom: Math.max(0, currentMetrics.distanceFromBottom - delta)
                    };
                    commitChatScrollMetrics?.(chatScrollMetrics);
                    updateScrollToBottomButton();
                }
            });

            targets.forEach((target) => autoScrollResizeObserver.observe(target));
        }

        function pinActiveMessageLabelToTop() {
            if (!chatMessages || !activeStreamingMessageId) return;
            const activeMessage = global.document.getElementById(activeStreamingMessageId);
            if (!activeMessage) return;
            const label = activeMessage.querySelector('.message-label');
            const target = label || activeMessage;
            const containerRect = chatMessages.getBoundingClientRect();
            const targetRect = target.getBoundingClientRect();
            const topPadding = 12;
            const delta = targetRect.top - containerRect.top - topPadding;
            if (Math.abs(delta) > 1) {
                suppressNextScrollEvent = true;
                chatMessages.scrollTop += delta;
                chatScrollMetrics = refreshChatScrollMetrics?.() || chatScrollMetrics;
                updateScrollToBottomButton();
            }
        }

        function checkAndTriggerLabelPin() {
            if (!activeStreamingMessagePinPending || getStreamingScrollMode?.() !== 'label-top' || !activeStreamingMessageId) {
                return;
            }

            const activeMessage = global.document.getElementById(activeStreamingMessageId);
            if (!activeMessage) return;

            const responseCard = activeMessage.querySelector('.assistant-response-card');
            const hasResponseContent = responseCard && !responseCard.hidden && !!responseCard.querySelector('.markdown-body')?.textContent.trim();
            const toolsHost = activeMessage.querySelector('.assistant-tools');
            const hasToolsContent = toolsHost && toolsHost.children.length > 0;
            const reasoningBody = activeMessage.querySelector('.reasoning-body');
            const reasoningText = reasoningBody ? reasoningBody.textContent.trim() : '';
            const hasReasoningContent = reasoningText.length > 0 && reasoningText !== 'Thinking...';

            if (hasResponseContent || hasToolsContent || hasReasoningContent) {
                global.requestAnimationFrame(() => {
                    global.requestAnimationFrame(() => {
                        if (activeStreamingMessagePinPending) {
                            pinActiveMessageLabelToTop();
                            activeStreamingMessagePinnedToTop = true;
                            activeStreamingMessagePinPending = false;
                        }
                    });
                });
            }
        }

        function pinTurnToTop(turnId) {
            if (!chatMessages || !turnId) return;
            const el = global.document.querySelector(`.message[data-turn-id="${turnId}"]`);
            if (!el) return;
            const label = el.querySelector('.message-label');
            const target = label || el;
            const containerRect = chatMessages.getBoundingClientRect();
            const targetRect = target.getBoundingClientRect();
            const topPadding = 12;
            const delta = targetRect.top - containerRect.top - topPadding;
            if (Math.abs(delta) > 1) {
                suppressNextScrollEvent = true;
                chatMessages.scrollTop += delta;
                chatScrollMetrics = refreshChatScrollMetrics?.() || chatScrollMetrics;
                updateScrollToBottomButton();
            }
        }

        function startStreamingMessageAutoScroll(messageId) {
            activeStreamingMessageId = messageId;
            activeStreamingMessagePinnedToTop = false;
            const activeMessage = global.document.getElementById(messageId);
            if (!activeMessage || !chatMessages) return;

            if (getStreamingScrollMode?.() === 'label-top') {
                chatMessages.classList.add('is-label-top-streaming');
                activeStreamingMessagePinPending = true;
                activeStreamingMessagePinnedToTop = true;
                shouldAutoScroll = false;
                lockScrollToLatest = false;
                if (autoScrollResizeObserver) {
                    autoScrollResizeObserver.disconnect();
                    autoScrollResizeObserver = null;
                }
                return;
            }
            const responseCard = activeMessage.querySelector('.assistant-response-card');
            const markdownBody = activeMessage.querySelector('.markdown-body');
            const codeBlocks = activeMessage.querySelectorAll('pre');
            observeAutoScrollResizes([activeMessage, responseCard, markdownBody, ...codeBlocks]);
        }

        function stopStreamingMessageAutoScroll() {
            activeStreamingMessagePinnedToTop = false;
            activeStreamingMessagePinPending = false;
            if (chatMessages) {
                chatMessages.classList.remove('is-label-top-streaming');
            }
            if (autoScrollResizeObserver) {
                autoScrollResizeObserver.disconnect();
                autoScrollResizeObserver = null;
            }
        }

        function jumpToLatestMessages() {
            triggerHaptic?.('success');
            lockScrollToLatest = true;
            holdAutoScrollAtBottom(900);
            scrollToBottom(true);
            updateScrollToBottomButton();
        }

        return {
            getAssistantMessageParts,
            setAssistantActionBarReady,
            reconcileAssistantActionBarForMessage,
            reconcileVisibleAssistantActionBars,
            isChatNearBottom,
            hasLongScrollableChat,
            updateScrollToBottomButton,
            jumpToLatestMessages,
            scrollToBottom,
            holdAutoScrollAtBottom,
            scheduleChatScrollToBottom,
            observeAutoScrollResizes,
            scrollActiveMessageIntoView,
            pinActiveMessageLabelToTop,
            checkAndTriggerLabelPin,
            pinTurnToTop,
            startStreamingMessageAutoScroll,
            stopStreamingMessageAutoScroll
        };
    }

    global.DKSTChatUI = {
        createChatUIController
    };
})(window);
