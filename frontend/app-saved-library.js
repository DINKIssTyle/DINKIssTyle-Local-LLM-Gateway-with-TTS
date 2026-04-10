/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTSavedLibrary(global) {
    function createSavedLibraryController(options = {}) {
        const { refs = {}, deps = {} } = options;
        const {
            savedLibraryView = null,
            savedLibraryList = null,
            savedLibrarySearchInput = null,
            savedTurnModal = null,
            savedTurnModalTitleView = null,
            savedTurnModalTitleEdit = null,
            savedTurnModalTitleInput = null,
            savedTurnModalTitleSaveBtn = null,
            savedTurnModalTitleCancelBtn = null
        } = refs;
        const {
            buildSavedTurnTitleRequestPayload,
            broadcastSavedTurnsChange,
            clearComposerBackgroundTask,
            escapeAttr,
            escapeHtml,
            fallbackCopyTextToClipboard,
            getCurrentUser,
            onOpenStateChange,
            renderMarkdownIntoHost,
            setComposerBackgroundTask,
            showToast,
            speakMessage,
            t,
            triggerHaptic
        } = deps;

        let savedTurns = [];
        let savedLibraryQuery = '';
        let savedTitleRefreshInFlight = false;
        let savedTitleRefreshTimer = null;
        let savedTitleRefreshAbortController = null;
        let isSavedLibraryOpen = false;
        let savedLibrarySwipeState = null;
        let savedLibraryCloseTimer = null;
        let savedTitleRefreshIds = new Set();

        function notifyOpenStateChange() {
            onOpenStateChange?.(isSavedLibraryOpen);
        }

        function summarizeSavedTurn(item) {
            const prompt = (item?.prompt_text || '').trim();
            const response = (item?.response_text || '').trim();
            const preview = [prompt, response].filter(Boolean).join(' ');
            return preview.length > 180 ? `${preview.slice(0, 180)}...` : preview;
        }

        function filterSavedTurns(query) {
            const needle = String(query || '').trim().toLowerCase();
            if (!needle) return savedTurns;
            return savedTurns.filter((item) => {
                const haystack = [item.title, item.prompt_text, item.response_text].join('\n').toLowerCase();
                return haystack.includes(needle);
            });
        }

        function renderSavedTurnInlineTitle(title) {
            if (!savedTurnModalTitleView) return;
            const trimmedTitle = (title || '').trim();
            savedTurnModalTitleView.textContent = trimmedTitle || t('library.modalTitle');
            savedTurnModalTitleView.classList.toggle('is-placeholder', !trimmedTitle);
        }

        function setSavedTurnTitleEditMode(isEditing) {
            if (!savedTurnModalTitleView || !savedTurnModalTitleEdit || !savedTurnModalTitleInput) return;
            savedTurnModalTitleView.hidden = !!isEditing;
            savedTurnModalTitleEdit.hidden = !isEditing;
            if (isEditing) {
                savedTurnModalTitleInput.value = savedTurnModal?.dataset?.title || '';
                global.requestAnimationFrame(() => {
                    savedTurnModalTitleInput.focus();
                    savedTurnModalTitleInput.select();
                });
            }
        }

        function renderSavedLibraryList() {
            if (!savedLibraryList) return;
            const items = filterSavedTurns(savedLibraryQuery);
            if (items.length === 0) {
                const emptyLabel = savedTurns.length === 0 ? t('library.empty') : t('library.emptyFiltered');
                savedLibraryList.innerHTML = `<div class="saved-library-empty">${escapeHtml(emptyLabel)}</div>`;
                return;
            }

            savedLibraryList.innerHTML = items.map((item) => `
                <article class="saved-library-item">
                    <div class="saved-library-item-main" onclick="openSavedTurnModal(${item.id})">
                        <div class="saved-library-item-title">${escapeHtml(item.title || '')}</div>
                        <div class="saved-library-item-preview">${escapeHtml(summarizeSavedTurn(item))}</div>
                        <div class="saved-library-item-meta">${escapeHtml(t('library.savedAt'))}: ${escapeHtml(new Date(item.created_at).toLocaleString())}</div>
                    </div>
                    ${item.processing ? `
                    <button class="icon-btn" title="${escapeAttr(t('background.savedTurnTitle'))}" disabled>
                        <span class="material-icons-round">hourglass_top</span>
                    </button>` : item.title_source === 'fallback' ? `
                    <button class="icon-btn" onclick="refreshSavedTurnTitleById(${item.id})" title="${escapeAttr(t('library.titleRefresh'))}" ${savedTitleRefreshIds.has(item.id) ? 'disabled' : ''}>
                        <span class="material-icons-round">refresh</span>
                    </button>` : ''}
                    <button class="icon-btn" onclick="deleteSavedTurn(${item.id})" title="Delete">
                        <span class="material-icons-round">delete</span>
                    </button>
                </article>
            `).join('');
        }

        function updateSavedLibrarySearchClearButton() {
            const clearBtn = global.document.getElementById('saved-library-search-clear');
            if (!clearBtn) return;
            const hasQuery = !!String(savedLibrarySearchInput?.value ?? savedLibraryQuery ?? '').trim();
            clearBtn.hidden = !hasQuery;
        }

        function hasPendingSavedTurnTitleRefresh() {
            return savedTurns.some((item) => item.title_source === 'fallback' && !item.processing);
        }

        function hasActiveSavedTurnTitleWork() {
            return savedTurns.some((item) => item.processing || item.title_source === 'fallback');
        }

        function updateSavedTurnEntry(updatedItem) {
            if (!updatedItem) return;
            savedTurns = savedTurns.map((item) => item.id === updatedItem.id ? updatedItem : item);
            renderSavedLibraryList();
            updateSavedLibrarySearchClearButton();
            reconcileSavedTitleRefreshState({ abortInFlightIfSettled: true });

            if (savedTurnModal?.classList.contains('active') && String(updatedItem.id) === savedTurnModal.dataset.turnId) {
                savedTurnModal.dataset.title = updatedItem.title || '';
                savedTurnModal.dataset.titleSource = updatedItem.title_source || '';
                renderSavedTurnInlineTitle(updatedItem.title || '');
            }
        }

        function scheduleSavedTitleRefresh(delay = 1200) {
            if (savedTitleRefreshTimer) {
                global.clearTimeout(savedTitleRefreshTimer);
            }
            if (!hasPendingSavedTurnTitleRefresh()) {
                clearComposerBackgroundTask('saved-turn-title-refresh');
                return;
            }

            setComposerBackgroundTask('saved-turn-title-refresh', {
                label: t('background.savedTurnTitle')
            });

            savedTitleRefreshTimer = global.setTimeout(() => {
                const runner = () => refreshSavedTurnTitle();
                if ('requestIdleCallback' in global) {
                    global.requestIdleCallback(runner, { timeout: 2000 });
                } else {
                    runner();
                }
            }, delay);
        }

        function reconcileSavedTitleRefreshState(options = {}) {
            const { abortInFlightIfSettled = false, delay = 1200 } = options;
            const hasActive = hasActiveSavedTurnTitleWork();
            const hasPending = hasPendingSavedTurnTitleRefresh();

            if (!hasActive) {
                if (savedTitleRefreshTimer) {
                    global.clearTimeout(savedTitleRefreshTimer);
                    savedTitleRefreshTimer = null;
                }
                if (abortInFlightIfSettled && savedTitleRefreshInFlight && savedTitleRefreshAbortController) {
                    savedTitleRefreshAbortController.abort();
                }
                clearComposerBackgroundTask('saved-turn-title-refresh');
                return;
            }

            if (!hasPending) {
                if (savedTitleRefreshTimer) {
                    global.clearTimeout(savedTitleRefreshTimer);
                }
                setComposerBackgroundTask('saved-turn-title-refresh', {
                    label: t('background.savedTurnTitle')
                });
                savedTitleRefreshTimer = global.setTimeout(() => {
                    loadSavedTurns();
                }, Math.max(1500, delay));
                return;
            }

            scheduleSavedTitleRefresh(delay);
        }

        async function refreshSavedTurnTitle() {
            if (savedTitleRefreshInFlight || !getCurrentUser?.() || global.document.hidden) return;
            if (!hasPendingSavedTurnTitleRefresh()) {
                reconcileSavedTitleRefreshState();
                return;
            }

            savedTitleRefreshInFlight = true;
            savedTitleRefreshAbortController = new AbortController();
            setComposerBackgroundTask('saved-turn-title-refresh', {
                label: t('background.savedTurnTitle'),
                abortController: savedTitleRefreshAbortController
            });
            try {
                const response = await global.fetch('/api/saved-turns/title-refresh', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify(buildSavedTurnTitleRequestPayload()),
                    signal: savedTitleRefreshAbortController.signal
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                if (data.updated && data.item) {
                    updateSavedTurnEntry(data.item);
                    broadcastSavedTurnsChange('title-updated');
                    reconcileSavedTitleRefreshState({ delay: 5000 });
                } else if (data.processing && data.item) {
                    updateSavedTurnEntry(data.item);
                    broadcastSavedTurnsChange('title-processing');
                    reconcileSavedTitleRefreshState({ delay: 2500 });
                } else if (!hasPendingSavedTurnTitleRefresh()) {
                    reconcileSavedTitleRefreshState();
                }
            } catch (error) {
                if (error.name !== 'AbortError') {
                    console.warn('Failed to refresh saved turn title:', error);
                }
            } finally {
                savedTitleRefreshInFlight = false;
                savedTitleRefreshAbortController = null;
                reconcileSavedTitleRefreshState();
            }
        }

        function cancelBackgroundTasks(reason = 'user-interrupt') {
            if (savedTitleRefreshTimer) {
                global.clearTimeout(savedTitleRefreshTimer);
                savedTitleRefreshTimer = null;
            }
            if (savedTitleRefreshInFlight && savedTitleRefreshAbortController) {
                savedTitleRefreshAbortController.abort(reason);
            }
        }

        async function loadSavedTurns() {
            if (!getCurrentUser?.()) return;
            try {
                const response = await global.fetch('/api/saved-turns', { credentials: 'include' });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                savedTurns = Array.isArray(data.items) ? data.items : [];
                renderSavedLibraryList();
                updateSavedLibrarySearchClearButton();
                reconcileSavedTitleRefreshState();
            } catch (error) {
                console.warn('Failed to load saved turns:', error);
            }
        }

        function openSavedLibrary() {
            if (!savedLibraryView) return;
            triggerHaptic('error');
            if (savedLibraryCloseTimer) {
                global.clearTimeout(savedLibraryCloseTimer);
                savedLibraryCloseTimer = null;
            }
            isSavedLibraryOpen = true;
            notifyOpenStateChange();
            savedLibraryView.hidden = false;
            savedLibraryView.classList.remove('is-closing');
            global.requestAnimationFrame(() => {
                savedLibraryView.classList.add('is-open');
            });
            renderSavedLibraryList();
            loadSavedTurns();
            if (savedLibrarySearchInput) {
                savedLibrarySearchInput.value = savedLibraryQuery;
                updateSavedLibrarySearchClearButton();
                const shouldAutoFocusSearch = global.matchMedia?.('(hover: hover) and (pointer: fine)')?.matches;
                if (shouldAutoFocusSearch) {
                    global.requestAnimationFrame(() => savedLibrarySearchInput.focus());
                } else {
                    savedLibrarySearchInput.blur();
                }
            }
        }

        function closeSavedLibrary() {
            if (!savedLibraryView) return;
            if (savedLibraryCloseTimer) {
                global.clearTimeout(savedLibraryCloseTimer);
            }
            isSavedLibraryOpen = false;
            notifyOpenStateChange();
            savedLibraryView.classList.remove('is-open');
            savedLibraryView.classList.add('is-closing');
            savedLibraryCloseTimer = global.setTimeout(() => {
                savedLibraryCloseTimer = null;
                if (!savedLibraryView || isSavedLibraryOpen) return;
                savedLibraryView.hidden = true;
                savedLibraryView.classList.remove('is-closing');
            }, 220);
        }

        function toggleSavedLibrary() {
            if (isSavedLibraryOpen) {
                closeSavedLibrary();
            } else {
                openSavedLibrary();
            }
        }

        function handleSavedLibrarySearch(value) {
            savedLibraryQuery = value || '';
            updateSavedLibrarySearchClearButton();
            renderSavedLibraryList();
        }

        function clearSavedLibrarySearch() {
            savedLibraryQuery = '';
            if (savedLibrarySearchInput) {
                savedLibrarySearchInput.value = '';
            }
            updateSavedLibrarySearchClearButton();
            renderSavedLibraryList();
            global.requestAnimationFrame(() => savedLibrarySearchInput?.focus());
        }

        function openSavedTurnModal(id) {
            const item = savedTurns.find((entry) => entry.id === id);
            if (!item || !savedTurnModal) return;

            savedTurnModal.dataset.turnId = String(item.id);
            savedTurnModal.dataset.title = item.title || '';
            savedTurnModal.dataset.titleSource = item.title_source || '';
            savedTurnModal.dataset.responseText = item.response_text || '';
            global.document.getElementById('saved-turn-modal-prompt').textContent = item.prompt_text || '';
            const responseHost = global.document.getElementById('saved-turn-modal-response');
            if (responseHost) {
                responseHost.innerHTML = '';
                renderMarkdownIntoHost(responseHost, item.response_text || '');
            }
            setSavedTurnTitleEditMode(false);
            renderSavedTurnInlineTitle(item.title || '');
            savedTurnModal.classList.add('active');
        }

        function closeSavedTurnModal() {
            if (savedTurnModal) {
                delete savedTurnModal.dataset.turnId;
                delete savedTurnModal.dataset.title;
                delete savedTurnModal.dataset.titleSource;
                delete savedTurnModal.dataset.responseText;
                delete savedTurnModal.dataset.titleSaving;
            }
            setSavedTurnTitleEditMode(false);
            savedTurnModal?.classList.remove('active');
        }

        async function copySavedTurnResponse() {
            const text = savedTurnModal?.dataset?.responseText || '';
            if (!text.trim()) return;
            try {
                await global.navigator.clipboard.writeText(text);
                triggerHaptic('success');
                showToast(t('clipboard.copied'));
            } catch (error) {
                console.warn('Clipboard API failed, trying fallback', error);
                fallbackCopyTextToClipboard(text);
            }
        }

        function speakSavedTurnResponse(btn) {
            const text = savedTurnModal?.dataset?.responseText || '';
            if (!text.trim()) return;
            speakMessage(text, btn);
        }

        function startEditSavedTurnTitle() {
            if (!savedTurnModal?.dataset?.turnId) return;
            if (savedTurnModal.dataset.titleSaving === 'true') return;
            setSavedTurnTitleEditMode(true);
        }

        function cancelEditSavedTurnTitle() {
            if (savedTurnModal?.dataset?.titleSaving === 'true') return;
            setSavedTurnTitleEditMode(false);
        }

        async function saveEditedSavedTurnTitle() {
            const turnId = parseInt(savedTurnModal?.dataset?.turnId || '', 10);
            const nextTitle = (savedTurnModalTitleInput?.value || '').trim();
            if (!turnId || !nextTitle) {
                showToast(t('library.titleUpdateFailed'), true);
                return;
            }
            if (savedTurnModal?.dataset?.titleSaving === 'true') return;

            savedTurnModal.dataset.titleSaving = 'true';
            if (savedTurnModalTitleSaveBtn) savedTurnModalTitleSaveBtn.disabled = true;
            if (savedTurnModalTitleCancelBtn) savedTurnModalTitleCancelBtn.disabled = true;
            if (savedTurnModalTitleInput) savedTurnModalTitleInput.disabled = true;

            try {
                const response = await global.fetch('/api/saved-turns', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify({ id: turnId, title: nextTitle })
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                if (!data.item) throw new Error('Missing item');
                updateSavedTurnEntry(data.item);
                setSavedTurnTitleEditMode(false);
                broadcastSavedTurnsChange('title-manual');
                showToast(t('library.titleUpdated'));
            } catch (error) {
                console.warn('Failed to update saved turn title:', error);
                showToast(t('library.titleUpdateFailed'), true);
            } finally {
                delete savedTurnModal.dataset.titleSaving;
                if (savedTurnModalTitleSaveBtn) savedTurnModalTitleSaveBtn.disabled = false;
                if (savedTurnModalTitleCancelBtn) savedTurnModalTitleCancelBtn.disabled = false;
                if (savedTurnModalTitleInput) savedTurnModalTitleInput.disabled = false;
            }
        }

        async function saveTurn(promptText, responseText) {
            try {
                const payload = buildSavedTurnTitleRequestPayload({
                    prompt_text: promptText,
                    response_text: responseText
                });
                const response = await global.fetch('/api/saved-turns', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify(payload)
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                const item = data.item;
                if (item) {
                    savedTurns = [item, ...savedTurns.filter((entry) => entry.id !== item.id)];
                    renderSavedLibraryList();
                    reconcileSavedTitleRefreshState();
                    broadcastSavedTurnsChange(item.processing ? 'title-processing' : 'saved');
                }
                showToast(t('library.saved'));
            } catch (error) {
                console.warn('Failed to save turn:', error);
                showToast(t('library.saveFailed'), true);
            }
        }

        async function deleteSavedTurn(id) {
            if (!global.confirm(t('library.deleteConfirm'))) return;
            try {
                const response = await global.fetch(`/api/saved-turns?id=${encodeURIComponent(String(id))}`, {
                    method: 'DELETE',
                    credentials: 'include'
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                savedTurns = savedTurns.filter((item) => item.id !== id);
                renderSavedLibraryList();
                closeSavedTurnModal();
                broadcastSavedTurnsChange('deleted');
                showToast(t('library.deleted'));
            } catch (error) {
                console.warn('Failed to delete saved turn:', error);
                showToast(t('library.deleteFailed'), true);
            }
        }

        async function refreshSavedTurnTitleById(id) {
            if (!id || savedTitleRefreshIds.has(id)) return;
            savedTitleRefreshIds.add(id);
            renderSavedLibraryList();

            try {
                const response = await global.fetch(`/api/saved-turns/title-refresh?id=${encodeURIComponent(String(id))}`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify(buildSavedTurnTitleRequestPayload())
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                if (data.item) {
                    updateSavedTurnEntry(data.item);
                }
                if (data.updated && data.item) {
                    broadcastSavedTurnsChange('title-updated');
                    showToast(t('library.titleRefreshed'));
                } else if (data.processing) {
                    broadcastSavedTurnsChange('title-processing');
                    reconcileSavedTitleRefreshState({ delay: 2500 });
                } else {
                    showToast(t('library.titleRefreshFailed'), true);
                }
            } catch (error) {
                console.warn('Failed to refresh saved turn title by id:', error);
                showToast(t('library.titleRefreshFailed'), true);
            } finally {
                savedTitleRefreshIds.delete(id);
                renderSavedLibraryList();
            }
        }

        return {
            cancelBackgroundTasks,
            cancelEditSavedTurnTitle,
            clearSavedLibrarySearch,
            closeSavedLibrary,
            closeSavedTurnModal,
            copySavedTurnResponse,
            deleteSavedTurn,
            handleSavedLibrarySearch,
            isOpen: () => isSavedLibraryOpen,
            loadSavedTurns,
            openSavedLibrary,
            openSavedTurnModal,
            refreshSavedTurnTitleById,
            resetSwipeState: () => {
                savedLibrarySwipeState = null;
            },
            saveEditedSavedTurnTitle,
            saveTurn,
            setupSavedLibrarySwipeGestures: () => {},
            speakSavedTurnResponse,
            startEditSavedTurnTitle,
            toggleSavedLibrary,
            updateSavedLibrarySearchClearButton
        };
    }

    global.DKSTSavedLibrary = {
        createSavedLibraryController
    };
})(window);
