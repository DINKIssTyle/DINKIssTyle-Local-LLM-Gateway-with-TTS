/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

import { unified } from './vendor/unified.bundle.mjs';
import remarkParse from './vendor/remark-parse.bundle.mjs';
import remarkGfm from './vendor/remark-gfm.bundle.mjs';
import remarkMath from './vendor/remark-math.bundle.mjs';
import remarkBreaks from './vendor/remark-breaks.bundle.mjs';
import remarkRehype from './vendor/remark-rehype.bundle.mjs';
import rehypeRaw from './vendor/rehype-raw.bundle.mjs';
import rehypeKatex from './vendor/rehype-katex.bundle.mjs';
import rehypeStringify from './vendor/rehype-stringify.bundle.mjs';
import { EventsOff, EventsOn } from './wailsjs/runtime/runtime.js';

let settings = {};
let rules = [];
let testCases = [];
let currentTestCaseId = null;
let currentRuleId = null;
let currentPreviewMode = 'rendered';
let lastNormalizedText = '';
let modalBusy = false;
let activeThinkingMessage = null;
let activeSelectedChatMessage = null;
let optimizerGenerating = false;
let optimizerResponseMessage = null;
let optimizerResponseBody = null;
let optimizerThinkingBody = null;
let optimizerResponseText = '';
let optimizerReasoningText = '';

const testCaseList = document.getElementById('testcase-list');
const rulesList = document.getElementById('rules-list');
const rawEditor = document.getElementById('raw-editor');
const previewContainer = document.getElementById('preview-container');
const chatInput = document.getElementById('chat-input');
const chatMessages = document.getElementById('chat-messages');
const btnSendChat = document.getElementById('btn-send-chat');
const settingsModal = document.getElementById('settings-modal');
const settingsStatus = document.getElementById('settings-status');
const llmStatus = document.getElementById('llm-status');
const previewButtons = Array.from(document.querySelectorAll('.preview-mode button'));

async function init() {
    try {
        settings = await window.go.main.App.GetSettings();
        rules = await window.go.main.App.GetRules();
        testCases = await window.go.main.App.GetTestCases();
    } catch (error) {
        addChatMessage('system', `Failed to initialize app data: ${error.message}`);
    }

    ensureSeedData();
    renderTestCaseList();
    renderRulesList();
    initEventListeners();
    updateLlmStatus();

    if (testCases.length > 0) {
        selectTestCase(testCases[0].id);
    } else {
        updateSummaryBar();
        renderPreview();
    }
}

function ensureSeedData() {
    if (!Array.isArray(testCases)) testCases = [];
    if (!Array.isArray(rules)) rules = [];
    if (!testCases.length) {
        testCases = [{
            id: `tc-${Date.now()}`,
            name: 'Sample Case',
            rawContent: '### Example1. Broken list\n1.Item one\n2.Item two\n\n- **bold marker'
        }];
        persistTestCases();
    }
}

function initEventListeners() {
    document.getElementById('btn-settings').addEventListener('click', openSettings);
    document.getElementById('btn-settings-save').addEventListener('click', saveSettings);
    document.getElementById('btn-settings-test').addEventListener('click', testSettingsConnection);
    document.getElementById('btn-settings-cancel').addEventListener('click', closeSettings);
    document.getElementById('btn-settings-close').addEventListener('click', closeSettings);
    settingsModal.addEventListener('click', (event) => {
        if (event.target === settingsModal && !modalBusy) {
            closeSettings();
        }
    });
    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape' && settingsModal.classList.contains('active') && !modalBusy) {
            closeSettings();
        }
    });

    document.getElementById('btn-add-testcase').addEventListener('click', addTestCase);
    document.getElementById('btn-rename-testcase').addEventListener('click', renameCurrentTestCase);
    document.getElementById('btn-delete-testcase').addEventListener('click', deleteCurrentTestCase);
    rawEditor.addEventListener('input', debounce(() => {
        saveCurrentTestCase();
        runNormalization();
    }, 250));

    document.getElementById('btn-add-rule').addEventListener('click', addRule);
    document.getElementById('btn-run-all').addEventListener('click', runNormalization);
    document.getElementById('btn-sync-rules').addEventListener('click', syncFromAppJs);

    btnSendChat.addEventListener('click', sendChat);
    chatInput.addEventListener('keydown', (event) => {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            sendChat();
        }
    });

    previewButtons.forEach((button) => {
        button.addEventListener('click', () => {
            currentPreviewMode = button.dataset.mode || 'rendered';
            previewButtons.forEach((item) => item.classList.toggle('active', item === button));
            renderPreview();
            updateSummaryBar();
        });
    });

    bindOptimizerEvents();
}

function renderTestCaseList() {
    if (!testCases.length) {
        testCaseList.innerHTML = '<div class="empty-state">No test cases yet. Add one and paste a raw model response.</div>';
        return;
    }

    testCaseList.innerHTML = testCases.map((testCase) => {
        const preview = compactText(testCase.rawContent || '', 64);
        const charCount = (testCase.rawContent || '').length;
        return `
            <div class="testcase-item ${testCase.id === currentTestCaseId ? 'active' : ''}" data-testcase-id="${escapeAttr(testCase.id)}">
                <div class="testcase-name">${escapeHtml(testCase.name)}</div>
                <div class="testcase-meta">${charCount} chars${preview ? ` • ${escapeHtml(preview)}` : ''}</div>
            </div>
        `;
    }).join('');

    Array.from(testCaseList.querySelectorAll('.testcase-item')).forEach((item) => {
        item.addEventListener('click', () => selectTestCase(item.dataset.testcaseId));
    });
}

function selectTestCase(id) {
    currentTestCaseId = id;
    const testCase = testCases.find((item) => item.id === id);
    rawEditor.value = testCase?.rawContent || '';
    renderTestCaseList();
    runNormalization();
}

async function addTestCase() {
    const id = `tc-${Date.now()}`;
    const nextIndex = testCases.length + 1;
    const entry = {
        id,
        name: `Case ${nextIndex}`,
        rawContent: ''
    };
    testCases.unshift(entry);
    await persistTestCases();
    renderTestCaseList();
    selectTestCase(id);
    rawEditor.focus();
}

window.addNormalizationTestCase = addTestCase;

async function renameCurrentTestCase() {
    if (!currentTestCaseId) return;
    const current = testCases.find((item) => item.id === currentTestCaseId);
    if (!current) return;
    const nextName = window.prompt('Rename test case', current.name);
    if (!nextName) return;
    current.name = nextName.trim() || current.name;
    await persistTestCases();
    renderTestCaseList();
    updateSummaryBar();
}

window.renameNormalizationTestCase = renameCurrentTestCase;

async function deleteCurrentTestCase() {
    if (!currentTestCaseId) return;
    const current = testCases.find((item) => item.id === currentTestCaseId);
    if (!current) return;
    const confirmed = window.confirm(`Delete "${current.name}"?`);
    if (!confirmed) return;
    testCases = testCases.filter((item) => item.id !== currentTestCaseId);
    await persistTestCases();
    currentTestCaseId = testCases[0]?.id || null;
    renderTestCaseList();
    if (currentTestCaseId) {
        selectTestCase(currentTestCaseId);
    } else {
        rawEditor.value = '';
        lastNormalizedText = '';
        renderPreview();
        updateSummaryBar();
    }
}

window.deleteNormalizationTestCase = deleteCurrentTestCase;

function saveCurrentTestCase() {
    if (!currentTestCaseId) return;
    const testCase = testCases.find((item) => item.id === currentTestCaseId);
    if (!testCase) return;
    testCase.rawContent = rawEditor.value;
    persistTestCases();
    renderTestCaseList();
    updateSummaryBar();
}

async function persistTestCases() {
    await window.go.main.App.SaveTestCases(testCases);
}

function renderRulesList() {
    if (!rules.length) {
        rulesList.innerHTML = '<div class="empty-state">No rules yet. Add one or import from the main app.</div>';
        return;
    }

    rulesList.innerHTML = rules.map((rule, index) => `
        <div class="rule-item ${rule.id === currentRuleId ? 'active' : ''}" data-rule-id="${escapeAttr(rule.id)}">
            <div class="rule-toggle-row">
                <input type="checkbox" ${rule.enabled ? 'checked' : ''} data-action="toggle">
                <div class="rule-main" data-action="select">
                    <div class="rule-title">${escapeHtml(rule.name || `Rule ${index + 1}`)}</div>
                    <div class="rule-meta">${rule.enabled ? 'Enabled' : 'Disabled'}${rule.pattern ? ` • /${escapeHtml(compactText(rule.pattern, 54))}/` : ''}</div>
                </div>
                <button class="icon-btn-small" data-action="delete" title="Delete rule">
                    <span class="material-icons-round">delete</span>
                </button>
            </div>
            ${rule.id === currentRuleId ? `
                <div class="rule-edit-detail">
                    <input type="text" data-field="name" placeholder="Rule name" value="${escapeAttr(rule.name || '')}">
                    <textarea data-field="pattern" placeholder="Regex pattern">${escapeHtml(rule.pattern || '')}</textarea>
                    <textarea data-field="replacement" placeholder="Replacement">${escapeHtml(rule.replacement || '')}</textarea>
                </div>
            ` : ''}
        </div>
    `).join('');

    Array.from(rulesList.querySelectorAll('.rule-item')).forEach((item) => {
        const id = item.dataset.ruleId;
        item.querySelector('[data-action="select"]')?.addEventListener('click', () => selectRule(id));
        item.querySelector('[data-action="toggle"]')?.addEventListener('change', () => toggleRule(id));
        item.querySelector('[data-action="delete"]')?.addEventListener('click', () => deleteRule(id));
        Array.from(item.querySelectorAll('[data-field]')).forEach((field) => {
            field.addEventListener('input', (event) => updateRuleDetail(id, event.target.dataset.field, event.target.value));
        });
    });

    updateSummaryBar();
}

function selectRule(id) {
    currentRuleId = id;
    renderRulesList();
}

async function toggleRule(id) {
    const rule = rules.find((item) => item.id === id);
    if (!rule) return;
    rule.enabled = !rule.enabled;
    await persistRules();
    renderRulesList();
    runNormalization();
}

const updateRuleDetail = debounce(async (id, field, value) => {
    const rule = rules.find((item) => item.id === id);
    if (!rule) return;
    rule[field] = value;
    await persistRules();
    renderRulesList();
    runNormalization();
}, 250);

async function deleteRule(id) {
    rules = rules.filter((rule) => rule.id !== id);
    if (currentRuleId === id) currentRuleId = null;
    await persistRules();
    renderRulesList();
    runNormalization();
}

async function addRule() {
    const id = `rule-${Date.now()}`;
    rules.unshift({
        id,
        name: `Rule ${rules.length + 1}`,
        pattern: '',
        replacement: '',
        enabled: true
    });
    currentRuleId = id;
    await persistRules();
    renderRulesList();
}

window.addNormalizationRule = addRule;

async function persistRules() {
    await window.go.main.App.SaveRules(rules);
}

async function runNormalization() {
    let text = rawEditor.value || '';
    let enabledRuleCount = 0;

    for (const rule of rules) {
        if (!rule.enabled || !rule.pattern) continue;
        enabledRuleCount += 1;
        try {
            const regex = new RegExp(rule.pattern, 'g');
            text = text.replace(regex, rule.replacement || '');
        } catch (error) {
            console.error('Regex Error:', error, rule.pattern);
            addChatMessage('system', `Regex error in "${rule.name || rule.id}": ${error.message}`);
        }
    }

    lastNormalizedText = text;
    renderPreview();
    updateSummaryBar(enabledRuleCount);
}

window.runNormalizationToolPreview = runNormalization;

function renderPreview() {
    previewContainer.classList.toggle('preview-source', currentPreviewMode === 'source');

    if (currentPreviewMode === 'source') {
        previewContainer.innerHTML = `<pre class="preview-source-block">${escapeHtml(lastNormalizedText || '')}</pre>`;
        return;
    }

    renderRemarkPreview(lastNormalizedText || '');
}

async function renderRemarkPreview(text) {
    try {
        const processor = unified()
            .use(remarkParse)
            .use(remarkGfm)
            .use(remarkMath)
            .use(remarkBreaks)
            .use(remarkRehype, { allowDangerousHtml: true })
            .use(rehypeKatex)
            .use(rehypeRaw)
            .use(rehypeStringify);

        const result = await processor.process(text);
        previewContainer.innerHTML = `<article class="markdown-body">${String(result)}</article>`;
    } catch (error) {
        console.error('Rendering Error:', error);
        previewContainer.innerHTML = `<div class="empty-state">Rendering error: ${escapeHtml(error.message)}</div>`;
    }
}

async function sendChat() {
    if (optimizerGenerating) {
        try {
            await window.go.main.App.StopOptimizerLLM();
        } catch (error) {
            addChatMessage('system', `Failed to stop LLM response: ${error.message}`);
        }
        return;
    }

    const message = chatInput.value.trim();
    if (!message) return;

    addChatMessage('user', message);
    chatInput.value = '';
    prepareOptimizerStreamUi();

    const systemPrompt = [
        'You are a Regex Normalization Expert.',
        'Improve the regex rules based on the rendering feedback.',
        `Current Rules: ${JSON.stringify(rules)}`,
        `Raw Input: ${rawEditor.value}`,
        'Respond with a short explanation and a JSON fenced code block containing the full updated rules array.'
    ].join('\n\n');

    try {
        setOptimizerBusy(true);
        await window.go.main.App.StartOptimizerLLM(systemPrompt, message);
    } catch (error) {
        finalizeOptimizerStream({ preserveThinking: true });
        setOptimizerBusy(false);
        addChatMessage('system', `Error calling LLM: ${error.message}`);
    }
}

window.__normalizationModuleSendChat = sendChat;
window.runNormalizationToolChat = sendChat;

function applyRulesFromOptimizerResponse(response) {
    const jsonMatch = response.match(/```json\s*([\s\S]*?)```/i);
    if (!jsonMatch) return;

    try {
        const nextRules = JSON.parse(jsonMatch[1]);
        if (!Array.isArray(nextRules)) return;
        rules = nextRules.map((rule, index) => ({
            id: rule.id || `rule-import-${Date.now()}-${index}`,
            name: rule.name || `Imported Rule ${index + 1}`,
            pattern: rule.pattern || '',
            replacement: rule.replacement || '',
            enabled: rule.enabled !== false
        }));
        persistRules();
        renderRulesList();
        runNormalization();
        addChatMessage('system', 'Updated rules applied automatically.');
    } catch (error) {
        console.error('Failed to parse optimizer JSON:', error);
        addChatMessage('system', `Failed to parse optimizer JSON: ${error.message}`);
    }
}

function addChatMessage(role, text) {
    const div = document.createElement('div');
    div.className = `message ${role}`;
    div.textContent = text;
    if (role === 'llm') {
        div.title = 'Click to load this response into the raw editor';
        div.addEventListener('click', () => loadChatMessageIntoRawEditor(div, text));
    }
    chatMessages.appendChild(div);
    chatMessages.scrollTop = chatMessages.scrollHeight;
    return div;
}

function createStructuredChatMessage(role, title) {
    const div = document.createElement('div');
    div.className = `message ${role}`;

    const titleNode = document.createElement('div');
    titleNode.className = 'message-title';
    titleNode.textContent = title;

    const bodyNode = document.createElement('div');
    bodyNode.className = 'message-body';

    div.appendChild(titleNode);
    div.appendChild(bodyNode);
    chatMessages.appendChild(div);
    chatMessages.scrollTop = chatMessages.scrollHeight;
    return { div, bodyNode };
}

function loadChatMessageIntoRawEditor(messageNode, text) {
    rawEditor.value = text || '';
    saveCurrentTestCase();
    runNormalization();
    rawEditor.focus();

    if (activeSelectedChatMessage && activeSelectedChatMessage !== messageNode) {
        activeSelectedChatMessage.classList.remove('selected-source');
    }
    messageNode.classList.add('selected-source');
    activeSelectedChatMessage = messageNode;

    addChatMessage('system', 'Loaded selected LLM response into the raw editor.');
}

function showThinkingMessage() {
    clearThinkingMessage();
    activeThinkingMessage = addChatMessage('thinking', 'Thinking about rule changes...');
    return activeThinkingMessage;
}

function clearThinkingMessage(target = activeThinkingMessage) {
    if (target && target.parentNode) {
        target.parentNode.removeChild(target);
    }
    if (!target || target === activeThinkingMessage) {
        activeThinkingMessage = null;
    }
}

function prepareOptimizerStreamUi() {
    optimizerResponseText = '';
    optimizerReasoningText = '';
    optimizerResponseMessage = null;
    optimizerResponseBody = null;
    optimizerThinkingBody = null;

    const thinking = createStructuredChatMessage('thinking', 'Thinking');
    activeThinkingMessage = thinking.div;
    optimizerThinkingBody = thinking.bodyNode;
    optimizerThinkingBody.textContent = 'Waiting for reasoning…';
}

function ensureOptimizerResponseMessage() {
    if (optimizerResponseMessage && optimizerResponseBody) {
        return optimizerResponseBody;
    }

    const response = createStructuredChatMessage('llm', 'LLM Response');
    optimizerResponseMessage = response.div;
    optimizerResponseBody = response.bodyNode;
    optimizerResponseMessage.title = 'Click to load this response into the raw editor';
    optimizerResponseMessage.addEventListener('click', () => {
        loadChatMessageIntoRawEditor(optimizerResponseMessage, optimizerResponseText);
    });
    return optimizerResponseBody;
}

function updateOptimizerReasoning(text) {
    if (!optimizerThinkingBody) return;
    optimizerThinkingBody.textContent = text || 'Waiting for reasoning…';
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

function updateOptimizerContent(text) {
    const body = ensureOptimizerResponseMessage();
    body.textContent = text || '';
    optimizerResponseMessage.dataset.messageText = text || '';
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

function setOptimizerBusy(isBusy) {
    optimizerGenerating = isBusy;
    const icon = btnSendChat?.querySelector('.material-icons-round');
    if (btnSendChat) {
        btnSendChat.classList.toggle('stop-mode', isBusy);
        btnSendChat.title = isBusy ? 'Stop generation' : 'Send';
        btnSendChat.setAttribute('aria-label', isBusy ? 'Stop generation' : 'Send');
    }
    if (icon) {
        icon.textContent = isBusy ? 'stop' : 'send';
    }
}

function finalizeOptimizerStream({ preserveThinking = true } = {}) {
    if (!preserveThinking || !optimizerReasoningText.trim()) {
        clearThinkingMessage();
        optimizerThinkingBody = null;
    } else {
        updateOptimizerReasoning(optimizerReasoningText);
    }

    if (optimizerResponseBody) {
        updateOptimizerContent(optimizerResponseText);
    }
}

function bindOptimizerEvents() {
    if (!window.runtime || typeof window.runtime.EventsOn !== 'function' || typeof window.runtime.EventsOff !== 'function') {
        console.warn('Wails runtime events are unavailable; optimizer streaming will use inline fallback only.');
        return;
    }

    EventsOff('optimizer:start', 'optimizer:reasoning', 'optimizer:content', 'optimizer:done', 'optimizer:error', 'optimizer:stopped');

    EventsOn('optimizer:start', () => {
        setOptimizerBusy(true);
    });

    EventsOn('optimizer:reasoning', (chunk) => {
        optimizerReasoningText += String(chunk || '');
        updateOptimizerReasoning(optimizerReasoningText);
    });

    EventsOn('optimizer:content', (chunk) => {
        optimizerResponseText += String(chunk || '');
        updateOptimizerContent(optimizerResponseText);
    });

    EventsOn('optimizer:done', (payload) => {
        const content = payload?.content ?? '';
        const reasoning = payload?.reasoning ?? '';
        if (content && !optimizerResponseText) {
            optimizerResponseText = String(content);
        }
        if (reasoning && !optimizerReasoningText) {
            optimizerReasoningText = String(reasoning);
        }
        finalizeOptimizerStream({ preserveThinking: true });
        setOptimizerBusy(false);
        if (optimizerResponseText.trim()) {
            applyRulesFromOptimizerResponse(optimizerResponseText);
        }
    });

    EventsOn('optimizer:error', (message) => {
        finalizeOptimizerStream({ preserveThinking: true });
        setOptimizerBusy(false);
        addChatMessage('system', `Error calling LLM: ${String(message || 'Unknown error')}`);
    });

    EventsOn('optimizer:stopped', () => {
        finalizeOptimizerStream({ preserveThinking: true });
        setOptimizerBusy(false);
        addChatMessage('system', 'LLM response stopped.');
    });
}

async function openSettings() {
    try {
        settings = await window.go.main.App.GetSettings();
    } catch (error) {
        console.warn('Failed to refresh settings before opening modal:', error);
    }
    document.getElementById('set-endpoint').value = settings.endpoint || '';
    document.getElementById('set-apikey').value = settings.apiKey || '';
    document.getElementById('set-model').value = settings.model || '';
    document.getElementById('set-temp').value = settings.temperature ?? 0.7;
    document.getElementById('set-tokens').value = settings.maxTokens ?? 2048;
    setSettingsStatus('', '');
    settingsModal.classList.add('active');
}

function closeSettings() {
    if (modalBusy) return;
    settingsModal.classList.remove('active');
}

window.openNormalizationSettings = openSettings;
window.closeNormalizationSettings = closeSettings;

function collectSettingsForm() {
    return {
        endpoint: document.getElementById('set-endpoint').value.trim(),
        apiKey: document.getElementById('set-apikey').value.trim(),
        model: document.getElementById('set-model').value.trim(),
        temperature: parseFloat(document.getElementById('set-temp').value),
        maxTokens: parseInt(document.getElementById('set-tokens').value, 10)
    };
}

async function saveSettings() {
    const nextSettings = collectSettingsForm();
    if (!nextSettings.endpoint || !nextSettings.model) {
        setSettingsStatus('Endpoint and model are required.', 'error');
        return;
    }

    modalBusy = true;
    updateSettingsButtons();
    setSettingsStatus('Saving settings...', '');

    try {
        await window.go.main.App.SaveSettings(nextSettings);
        settings = await window.go.main.App.GetSettings();
        setSettingsStatus('Settings saved.', 'success');
        updateLlmStatus();
        window.setTimeout(() => {
            closeSettings();
            setSettingsStatus('', '');
        }, 320);
    } catch (error) {
        setSettingsStatus(`Failed to save settings: ${error.message}`, 'error');
    } finally {
        modalBusy = false;
        updateSettingsButtons();
    }
}

async function testSettingsConnection() {
    const candidate = collectSettingsForm();
    if (!candidate.endpoint || !candidate.model) {
        setSettingsStatus('Endpoint and model are required before testing.', 'error');
        return;
    }

    modalBusy = true;
    updateSettingsButtons();
    setSettingsStatus('Testing connection...', '');

    try {
        const message = await window.go.main.App.ValidateSettings(candidate);
        setSettingsStatus(message, 'success');
        llmStatus.textContent = 'LLM connection verified';
        llmStatus.className = 'status-chip success';
    } catch (error) {
        setSettingsStatus(error.message || 'Connection test failed.', 'error');
        llmStatus.textContent = 'LLM connection failed';
        llmStatus.className = 'status-chip error';
    } finally {
        modalBusy = false;
        updateSettingsButtons();
    }
}

window.__normalizationModuleSave = saveSettings;
window.__normalizationModuleTest = testSettingsConnection;
window.saveNormalizationSettings = saveSettings;
window.testNormalizationSettingsConnection = testSettingsConnection;

function updateSettingsButtons() {
    const saveBtn = document.getElementById('btn-settings-save');
    const testBtn = document.getElementById('btn-settings-test');
    const cancelBtn = document.getElementById('btn-settings-cancel');
    saveBtn.disabled = modalBusy;
    testBtn.disabled = modalBusy;
    cancelBtn.disabled = modalBusy;
}

function setSettingsStatus(message, tone) {
    if (!message) {
        settingsStatus.hidden = true;
        settingsStatus.textContent = '';
        settingsStatus.className = 'settings-status';
        return;
    }
    settingsStatus.hidden = false;
    settingsStatus.textContent = message;
    settingsStatus.className = `settings-status${tone ? ` ${tone}` : ''}`;
}

function updateLlmStatus() {
    if (settings.endpoint && settings.model) {
        llmStatus.textContent = `${settings.model} @ ${compactText(settings.endpoint, 28)}`;
        llmStatus.className = 'status-chip';
    } else {
        llmStatus.textContent = 'LLM not configured';
        llmStatus.className = 'status-chip error';
    }
}

async function syncFromAppJs() {
    try {
        const importedRules = await window.go.main.App.SyncFromAppJs();
        if (!Array.isArray(importedRules) || !importedRules.length) {
            addChatMessage('system', 'No rules were imported from the main frontend/app.js.');
            return;
        }

        rules = [...rules, ...importedRules.map((rule, index) => ({
            id: rule.id || `sync-${Date.now()}-${index}`,
            name: rule.name || `Imported ${index + 1}`,
            pattern: rule.pattern || '',
            replacement: rule.replacement || '',
            enabled: rule.enabled !== false
        }))];
        await persistRules();
        renderRulesList();
        runNormalization();
        addChatMessage('system', `Imported ${importedRules.length} rules from the main app.`);
    } catch (error) {
        addChatMessage('system', `Sync error: ${error.message}`);
    }
}

window.syncNormalizationRules = syncFromAppJs;

function updateSummaryBar(enabledRuleCount = rules.filter((rule) => rule.enabled).length) {
    const currentCase = testCases.find((item) => item.id === currentTestCaseId);
    document.getElementById('summary-case').textContent = currentCase?.name || 'No case selected';
    document.getElementById('summary-length').textContent = `${lastNormalizedText.length} chars`;
    document.getElementById('summary-rules').textContent = `${enabledRuleCount} active`;
    document.getElementById('summary-preview').textContent = currentPreviewMode === 'source' ? 'Normalized' : 'Rendered';
}

function escapeHtml(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function escapeAttr(value) {
    return escapeHtml(value).replace(/`/g, '&#96;');
}

function compactText(value, limit = 48) {
    const text = String(value || '').replace(/\s+/g, ' ').trim();
    if (text.length <= limit) return text;
    return `${text.slice(0, Math.max(0, limit - 1))}…`;
}

function debounce(func, wait) {
    let timeout;
    return function debounced(...args) {
        window.clearTimeout(timeout);
        timeout = window.setTimeout(() => func.apply(this, args), wait);
    };
}

window.addEventListener('load', async () => {
    try {
        await init();
    } catch (error) {
        console.error('Initialization failed:', error);
        addChatMessage('system', `Initialization failed: ${error.message}`);
    }
});
