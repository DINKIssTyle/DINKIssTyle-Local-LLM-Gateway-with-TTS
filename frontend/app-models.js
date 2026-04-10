/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTModels(global) {
    function createModelController(options = {}) {
        const { refs = {}, deps = {} } = options;
        const {
            composerReasoningSelect = null,
            reasoningControlBar = null,
            scrollToBottomBtn = null
        } = refs;
        const {
            DEFAULT_REASONING_OPTIONS = [],
            config,
            enforceMCPPolicyForMode,
            escapeAttr,
            escapeHtml,
            isGenerating,
            normalizeContextStrategyForMode,
            normalizeReasoningValue,
            persistClientConfig,
            saveConfig,
            showToast,
            t,
            triggerHaptic,
            updateComposerLayoutMetrics,
            updateReasoningControlVisibility,
            updateScrollToBottomButton
        } = deps;

        let availableModels = [];
        let availableModelInfoById = new Map();

        function parseReasoningCapability(rawCapability) {
            if (rawCapability === null || rawCapability === undefined || rawCapability === false) {
                return { supportsReasoning: false, reasoningOptions: [] };
            }

            if (rawCapability === true) {
                return {
                    supportsReasoning: true,
                    reasoningOptions: [...DEFAULT_REASONING_OPTIONS]
                };
            }

            if (typeof rawCapability === 'string') {
                const normalized = normalizeReasoningValue(rawCapability);
                if (normalized) {
                    return {
                        supportsReasoning: true,
                        reasoningOptions: normalized === 'on' || normalized === 'off'
                            ? [...DEFAULT_REASONING_OPTIONS]
                            : [normalized]
                    };
                }
                const truthy = ['true', 'supported', 'enabled', 'yes', 'available'];
                if (truthy.includes(rawCapability.trim().toLowerCase())) {
                    return {
                        supportsReasoning: true,
                        reasoningOptions: [...DEFAULT_REASONING_OPTIONS]
                    };
                }
                return { supportsReasoning: false, reasoningOptions: [] };
            }

            if (Array.isArray(rawCapability)) {
                const options = [...new Set(
                    rawCapability
                        .map((value) => normalizeReasoningValue(value))
                        .filter(Boolean)
                )];
                return {
                    supportsReasoning: options.length > 0,
                    reasoningOptions: options
                };
            }

            if (typeof rawCapability === 'object') {
                const options = [...new Set(
                    (rawCapability.options
                        || rawCapability.values
                        || rawCapability.levels
                        || rawCapability.supported_values
                        || rawCapability.allowed
                        || [])
                        .map((value) => normalizeReasoningValue(value))
                        .filter(Boolean)
                )];
                const supportedFlag = rawCapability.supported
                    ?? rawCapability.enabled
                    ?? rawCapability.available
                    ?? rawCapability.reasoning;
                const supportsReasoning = supportedFlag === true || options.length > 0;
                return {
                    supportsReasoning,
                    reasoningOptions: supportsReasoning
                        ? (options.length > 0 ? options : [...DEFAULT_REASONING_OPTIONS])
                        : []
                };
            }

            return { supportsReasoning: false, reasoningOptions: [] };
        }

        function normalizeModelInfo(model) {
            if (!model || typeof model !== 'object') return null;
            const id = String(model.id || model.key || model.name || '').trim();
            if (!id) return null;

            const reasoningCapability = parseReasoningCapability(
                model.reasoning
                ?? model.capabilities?.reasoning
                ?? model.metadata?.reasoning
                ?? model.model_info?.reasoning
            );

            const loadedInstances = Array.isArray(model.loaded_instances) ? model.loaded_instances : [];
            const primaryLoadedInstance = loadedInstances.find((instance) => {
                if (!instance) return false;
                if (typeof instance === 'string') return instance.trim() !== '';
                if (typeof instance === 'object') {
                    return String(instance.instance_id || instance.id || instance.key || '').trim() !== '';
                }
                return false;
            }) || null;
            const primaryLoadedInstanceId = typeof primaryLoadedInstance === 'string'
                ? primaryLoadedInstance.trim()
                : String(
                    primaryLoadedInstance?.instance_id
                    || primaryLoadedInstance?.id
                    || primaryLoadedInstance?.key
                    || ''
                ).trim();
            const rawLoaded = model.loaded
                ?? model.is_loaded
                ?? model.isLoaded
                ?? model.active
                ?? model.currently_loaded
                ?? model.metadata?.loaded
                ?? model.model_info?.loaded;
            const rawState = String(
                model.state
                ?? model.status
                ?? model.load_state
                ?? model.metadata?.state
                ?? model.model_info?.state
                ?? ''
            ).trim().toLowerCase();
            const isLoaded = loadedInstances.length > 0 || rawLoaded === true || ['loaded', 'active', 'ready', 'resident'].includes(rawState);

            return {
                id,
                displayName: String(model.display_name || model.displayName || model.name || id).trim() || id,
                isLoaded,
                loadedInstances,
                primaryLoadedInstanceId,
                stateLabel: rawState,
                supportsReasoning: reasoningCapability.supportsReasoning,
                reasoningOptions: reasoningCapability.reasoningOptions
            };
        }

        function setAvailableModels(models) {
            availableModels = Array.isArray(models)
                ? models.map((model) => normalizeModelInfo(model)).filter(Boolean)
                : [];
            availableModelInfoById = new Map(availableModels.map((model) => [model.id, model]));
        }

        function getHeaderModelTrigger() {
            return global.document.getElementById('header-model-trigger');
        }

        function updateHeaderModelDisplay() {
            const headerModelName = global.document.getElementById('header-model-name');
            if (!headerModelName) return;
            const selectedModel = availableModelInfoById.get(String(config.model || '').trim());
            headerModelName.textContent = selectedModel?.displayName || config.model || 'No Model Set';
        }

        function closeModelPickerModal() {
            const trigger = getHeaderModelTrigger();
            if (trigger) trigger.setAttribute('aria-expanded', 'false');
            global.document.getElementById('model-picker-modal')?.classList.remove('active');
        }

        function renderModelPickerModal() {
            const list = global.document.getElementById('model-picker-list');
            if (!list) return;

            if (!availableModels.length) {
                list.innerHTML = `<div class="model-picker-empty">${escapeHtml(t('models.empty'))}</div>`;
                return;
            }

            const isStatefulMode = String(config.llmMode || '').trim().toLowerCase() === 'stateful';
            list.innerHTML = availableModels.map((model) => {
                const selected = model.id === String(config.model || '').trim();
                const loadedBadge = isStatefulMode && model.isLoaded
                    ? `<span class="model-picker-badge is-loaded">${escapeHtml(t('models.loaded'))}</span>`
                    : '';
                const currentLabel = selected ? `<span class="model-picker-current">${escapeHtml(model.id)}</span>` : '';
                const unloadButton = isStatefulMode && model.isLoaded && model.primaryLoadedInstanceId
                    ? `<button type="button" class="icon-btn model-picker-unload" title="${escapeAttr(t('models.unload'))}" onclick="unloadHeaderModel(event, '${escapeAttr(model.primaryLoadedInstanceId)}')"><span class="material-icons-round">eject</span></button>`
                    : '';

                return `
                    <div class="model-picker-item${selected ? ' is-selected' : ''}">
                        <button
                            class="model-picker-main"
                            type="button"
                            onclick="selectHeaderModel('${escapeAttr(model.id)}')">
                            <div class="model-picker-copy">
                                <div class="model-picker-name">${escapeHtml(model.displayName || model.id)}</div>
                                <div class="model-picker-meta">
                                    ${loadedBadge}
                                    ${currentLabel}
                                </div>
                            </div>
                        </button>
                        ${unloadButton}
                    </div>
                `;
            }).join('');
        }

        function getSelectedModelInfo() {
            return availableModelInfoById.get(String(config.model || '').trim()) || null;
        }

        function shouldShowReasoningControl() {
            if (!config.showReasoningControl) return false;
            const selectedModelInfo = getSelectedModelInfo();
            return !!(selectedModelInfo?.supportsReasoning || config.forceShowReasoningControl);
        }

        function getReasoningOptionsForCurrentModel() {
            const selectedModelInfo = getSelectedModelInfo();
            if (selectedModelInfo?.supportsReasoning && selectedModelInfo.reasoningOptions.length > 0) {
                return selectedModelInfo.reasoningOptions;
            }
            if (config.forceShowReasoningControl) {
                return [...DEFAULT_REASONING_OPTIONS];
            }
            return [];
        }

        function validateReasoningSelection() {
            const normalizedValue = normalizeReasoningValue(config.reasoning);
            if (!shouldShowReasoningControl()) {
                config.reasoning = '';
                return;
            }

            const options = getReasoningOptionsForCurrentModel();
            config.reasoning = options.includes(normalizedValue) ? normalizedValue : '';
        }

        async function fetchModels() {
            const select = global.document.getElementById('cfg-model');
            const secondarySelect = global.document.getElementById('cfg-secondary-model');
            if (!select) return;

            try {
                const mode = encodeURIComponent(String(config.llmMode || 'standard').trim().toLowerCase() || 'standard');
                const response = await global.fetch(`/api/models?mode=${mode}`, { credentials: 'include' });
                if (!response.ok) {
                    const errText = await response.text();
                    throw new Error(errText || `HTTP ${response.status}`);
                }

                const data = await response.json();

                let models = [];
                if (Array.isArray(data)) {
                    models = data;
                } else if (data.data && Array.isArray(data.data)) {
                    models = data.data;
                } else if (data.object === 'list' && Array.isArray(data.data)) {
                    models = data.data;
                } else if (data.models && Array.isArray(data.models)) {
                    models = data.models.map((model) => ({
                        id: model.key,
                        ...model
                    }));
                }

                const normalizedModels = models.map((model) => normalizeModelInfo(model)).filter(Boolean);
                setAvailableModels(models);

                select.innerHTML = '';
                if (secondarySelect) {
                    secondarySelect.innerHTML = '<option value="">Use primary model</option>';
                }

                if (normalizedModels.length === 0) {
                    select.innerHTML = '<option value="">No models available</option>';
                    renderReasoningControl();
                    return;
                }

                normalizedModels.forEach((model) => {
                    const option = global.document.createElement('option');
                    option.value = model.id;
                    option.textContent = model.displayName;
                    select.appendChild(option);
                    if (secondarySelect) {
                        const secondaryOption = global.document.createElement('option');
                        secondaryOption.value = model.id;
                        secondaryOption.textContent = model.displayName;
                        secondarySelect.appendChild(secondaryOption);
                    }
                });

                if (config.model && Array.from(select.options).some((opt) => opt.value === config.model)) {
                    select.value = config.model;
                } else if (normalizedModels.length > 0) {
                    select.value = normalizedModels[0].id;
                    config.model = normalizedModels[0].id;
                }
                if (secondarySelect) {
                    if (config.secondaryModel && Array.from(secondarySelect.options).some((opt) => opt.value === config.secondaryModel)) {
                        secondarySelect.value = config.secondaryModel;
                    } else {
                        secondarySelect.value = '';
                    }
                }

                updateHeaderModelDisplay();
                renderModelPickerModal();
                renderReasoningControl();
            } catch (error) {
                console.error('[Models] Failed to fetch:', error);
                select.innerHTML = `<option value="">Error: ${error.message}</option>`;
                setAvailableModels([]);
                renderReasoningControl();

                const manualOption = global.document.createElement('option');
                manualOption.value = config.model || '';
                manualOption.textContent = config.model || 'Enter model manually';
                select.appendChild(manualOption);
                if (secondarySelect && config.secondaryModel) {
                    const manualSecondary = global.document.createElement('option');
                    manualSecondary.value = config.secondaryModel;
                    manualSecondary.textContent = config.secondaryModel;
                    secondarySelect.appendChild(manualSecondary);
                    secondarySelect.value = config.secondaryModel;
                }
                updateHeaderModelDisplay();
                renderModelPickerModal();
            }
        }

        function openSettingsModal() {
            triggerHaptic('error');
            global.document.getElementById('settings-modal')?.classList.add('active');
            fetchModels();
        }

        function closeSettingsModal() {
            global.document.getElementById('settings-modal')?.classList.remove('active');
        }

        function renderContextStrategyOptions() {
            const select = global.document.getElementById('cfg-context-strategy');
            if (!select) return;
            const mode = global.document.getElementById('cfg-llm-mode')?.value || config.llmMode || 'standard';
            const normalizedMode = mode === 'stateful' ? 'stateful' : 'standard';
            const options = normalizedMode === 'stateful'
                ? [
                    { value: 'retrieval', label: t('setting.contextStrategy.option.retrieval') },
                    { value: 'stateful', label: t('setting.contextStrategy.option.stateful') },
                    { value: 'none', label: t('setting.contextStrategy.option.none') }
                ]
                : [
                    { value: 'retrieval', label: t('setting.contextStrategy.option.retrieval') },
                    { value: 'history', label: t('setting.contextStrategy.option.history') },
                    { value: 'none', label: t('setting.contextStrategy.option.none') }
                ];
            const currentValue = normalizeContextStrategyForMode(normalizedMode, select.value || config.contextStrategy);
            select.innerHTML = options.map((option) => `<option value="${option.value}">${option.label}</option>`).join('');
            select.value = currentValue;
        }

        async function openModelPickerModal(event) {
            if (event) {
                event.preventDefault();
                event.stopPropagation();
            }
            const trigger = getHeaderModelTrigger();
            if (!trigger) return;
            await fetchModels();
            renderModelPickerModal();
            global.document.getElementById('model-picker-modal')?.classList.add('active');
            trigger.setAttribute('aria-expanded', 'true');
        }

        async function selectHeaderModel(modelId) {
            const nextModel = String(modelId || '').trim();
            if (!nextModel) return;
            config.model = nextModel;
            const cfgModel = global.document.getElementById('cfg-model');
            if (cfgModel) {
                const matchingOption = Array.from(cfgModel.options).find((option) => option.value === nextModel);
                if (matchingOption) {
                    cfgModel.value = nextModel;
                } else {
                    const option = global.document.createElement('option');
                    option.value = nextModel;
                    option.textContent = nextModel;
                    cfgModel.appendChild(option);
                    cfgModel.value = nextModel;
                }
            }
            updateHeaderModelDisplay();
            renderModelPickerModal();
            closeModelPickerModal();
            saveConfig(false);
        }

        async function unloadHeaderModel(event, instanceId) {
            if (event) {
                event.preventDefault();
                event.stopPropagation();
            }
            const nextInstanceId = String(instanceId || '').trim();
            if (!nextInstanceId) return;

            try {
                const response = await global.fetch('/api/models/unload', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include',
                    body: JSON.stringify({
                        instance_id: nextInstanceId,
                        mode: config.llmMode || 'standard'
                    })
                });
                if (!response.ok) {
                    throw new Error(await response.text().catch(() => `HTTP ${response.status}`));
                }
                await fetchModels();
                renderModelPickerModal();
                showToast(`${t('models.unload')} ✓`);
            } catch (error) {
                console.warn('[Models] Unload failed:', error);
                showToast(String(error?.message || 'Unload failed'), true);
            }
        }

        function renderReasoningControl() {
            if (!reasoningControlBar || !composerReasoningSelect) return;

            const previousReasoningValue = config.reasoning;
            validateReasoningSelection();
            if (previousReasoningValue !== config.reasoning) {
                persistClientConfig();
            }
            const shouldShow = shouldShowReasoningControl();
            reasoningControlBar.hidden = !shouldShow;

            if (shouldShow) {
                const options = getReasoningOptionsForCurrentModel();
                composerReasoningSelect.innerHTML = '';

                const autoOption = global.document.createElement('option');
                autoOption.value = '';
                autoOption.textContent = t('reasoning.auto');
                composerReasoningSelect.appendChild(autoOption);

                options.forEach((value) => {
                    const option = global.document.createElement('option');
                    option.value = value;
                    option.textContent = `${value.charAt(0).toUpperCase()}${value.slice(1)}`;
                    composerReasoningSelect.appendChild(option);
                });

                composerReasoningSelect.value = config.reasoning || '';
            } else {
                composerReasoningSelect.innerHTML = `<option value="">${t('reasoning.auto')}</option>`;
                composerReasoningSelect.value = '';
            }

            updateComposerLayoutMetrics();
            updateReasoningControlVisibility();
            updateScrollToBottomButton();
        }

        function getEffectiveReasoningSelection() {
            if (!shouldShowReasoningControl()) return '';
            const normalized = normalizeReasoningValue(config.reasoning);
            const options = getReasoningOptionsForCurrentModel();
            return options.includes(normalized) ? normalized : '';
        }

        return {
            closeModelPickerModal,
            closeSettingsModal,
            fetchModels,
            getEffectiveReasoningSelection,
            openModelPickerModal,
            openSettingsModal,
            renderContextStrategyOptions,
            renderModelPickerModal,
            renderReasoningControl,
            selectHeaderModel,
            unloadHeaderModel,
            updateHeaderModelDisplay
        };
    }

    global.DKSTModels = {
        createModelController
    };
})(window);
