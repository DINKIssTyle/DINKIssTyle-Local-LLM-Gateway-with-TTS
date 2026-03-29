/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

// Import Unified/Remark stack from vendor
import { unified } from './vendor/unified.bundle.mjs';
import remarkParse from './vendor/remark-parse.bundle.mjs';
import remarkGfm from './vendor/remark-gfm.bundle.mjs';
import remarkMath from './vendor/remark-math.bundle.mjs';
import remarkBreaks from './vendor/remark-breaks.bundle.mjs';
import remarkRehype from './vendor/remark-rehype.bundle.mjs';
import rehypeRaw from './vendor/rehype-raw.bundle.mjs';
import rehypeKatex from './vendor/rehype-katex.bundle.mjs';
import rehypeStringify from './vendor/rehype-stringify.bundle.mjs';

// State
let settings = {};
let rules = [];
let testCases = [];
let currentTestCaseId = null;
let currentRuleId = null;

// DOM Elements
const testCaseList = document.getElementById('testcase-list');
const rulesList = document.getElementById('rules-list');
const rawEditor = document.getElementById('raw-editor');
const previewContainer = document.getElementById('preview-container');
const chatInput = document.getElementById('chat-input');
const chatMessages = document.getElementById('chat-messages');
const btnSendChat = document.getElementById('btn-send-chat');

// Initialize
async function init() {
    settings = await window.go.main.App.GetSettings();
    rules = await window.go.main.App.GetRules();
    testCases = await window.go.main.App.GetTestCases();

    renderTestCaseList();
    renderRulesList();
    initEventListeners();

    if (testCases.length > 0) {
        selectTestCase(testCases[0].id);
    }
}

function initEventListeners() {
    // Settings
    document.getElementById('btn-settings').onclick = openSettings;
    document.getElementById('btn-settings-save').onclick = saveSettings;
    document.getElementById('btn-settings-cancel').onclick = () => document.getElementById('settings-modal').classList.remove('active');

    // Test Cases
    document.getElementById('btn-add-testcase').onclick = addTestCase;
    rawEditor.oninput = debounce(() => {
        saveCurrentTestCase();
        runNormalization();
    }, 500);

    // Rules
    document.getElementById('btn-add-rule').onclick = addRule;
    document.getElementById('btn-run-all').onclick = runNormalization;
    document.getElementById('btn-sync-rules').onclick = syncFromAppJs;

    // Chat
    btnSendChat.onclick = sendChat;
    chatInput.onkeydown = (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendChat();
        }
    };
}

// --- Test Case Management ---
function renderTestCaseList() {
    testCaseList.innerHTML = testCases.map(tc => `
        <div class="testcase-item ${tc.id === currentTestCaseId ? 'active' : ''}" onclick="selectTestCase('${tc.id}')">
            ${tc.name}
        </div>
    `).join('');
}

function selectTestCase(id) {
    currentTestCaseId = id;
    const tc = testCases.find(t => t.id === id);
    if (tc) {
        rawEditor.value = tc.rawContent;
        renderTestCaseList();
        runNormalization();
    }
}

function addTestCase() {
    const id = 'tc-' + Date.now();
    const newCase = { id, name: 'New Case ' + (testCases.length + 1), rawContent: '' };
    testCases.push(newCase);
    window.go.main.App.SaveTestCases(testCases);
    renderTestCaseList();
    selectTestCase(id);
}

function saveCurrentTestCase() {
    if (!currentTestCaseId) return;
    const tc = testCases.find(t => t.id === currentTestCaseId);
    if (tc) {
        tc.rawContent = rawEditor.value;
        window.go.main.App.SaveTestCases(testCases);
    }
}

// --- Rule Management ---
function renderRulesList() {
    rulesList.innerHTML = rules.map((r, i) => `
        <div class="rule-item ${r.id === currentRuleId ? 'active' : ''}">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <input type="checkbox" ${r.enabled ? 'checked' : ''} onchange="toggleRule('${r.id}')">
                <span onclick="selectRule('${r.id}')" style="flex:1; margin-left:8px;">${r.name || 'Rule ' + (i+1)}</span>
                <button onclick="deleteRule('${r.id}')" class="icon-btn-small"><span class="material-icons-round">delete</span></button>
            </div>
            ${r.id === currentRuleId ? `
                <div class="rule-edit-detail">
                    <input type="text" placeholder="Rule Name" value="${r.name}" oninput="updateRuleDetail('${r.id}', 'name', this.value)">
                    <textarea placeholder="Regex Pattern" oninput="updateRuleDetail('${r.id}', 'pattern', this.value)">${r.pattern}</textarea>
                    <textarea placeholder="Replacement" oninput="updateRuleDetail('${r.id}', 'replacement', this.value)">${r.replacement}</textarea>
                </div>
            ` : ''}
        </div>
    `).join('');
}

window.selectRule = (id) => {
    currentRuleId = id;
    renderRulesList();
};

window.toggleRule = (id) => {
    const r = rules.find(rule => rule.id === id);
    if (r) {
        r.enabled = !r.enabled;
        window.go.main.App.SaveRules(rules);
        runNormalization();
    }
};

window.updateRuleDetail = debounce((id, field, value) => {
    const r = rules.find(rule => rule.id === id);
    if (r) {
        r[field] = value;
        window.go.main.App.SaveRules(rules);
        runNormalization();
    }
}, 500);

window.deleteRule = (id) => {
    rules = rules.filter(r => r.id !== id);
    window.go.main.App.SaveRules(rules);
    renderRulesList();
    runNormalization();
};

function addRule() {
    const id = 'rule-' + Date.now();
    rules.push({ id, name: 'New Regex', pattern: '', replacement: '', enabled: true });
    window.go.main.App.SaveRules(rules);
    currentRuleId = id;
    renderRulesList();
}

// --- Normalization & Rendering ---
async function runNormalization() {
    let text = rawEditor.value || '';

    // Apply rules
    rules.forEach(rule => {
        if (rule.enabled && rule.pattern) {
            try {
                const regex = new RegExp(rule.pattern, 'g');
                text = text.replace(regex, rule.replacement);
            } catch (e) {
                console.error('Regex Error:', e, rule.pattern);
            }
        }
    });

    // Render using Remark
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
        previewContainer.innerHTML = String(result);
    } catch (e) {
        console.error('Rendering Error:', e);
        previewContainer.innerHTML = `<div style="color:red">Error: ${e.message}</div>`;
    }
}

// --- Chat & Optimization ---
async function sendChat() {
    const message = chatInput.value.trim();
    if (!message) return;

    addChatMessage('user', message);
    chatInput.value = '';

    const systemPrompt = `You are a Regex Normalization Expert. 
Your goal is to provide updated JSON rules based on user feedback to fix Markdown rendering issues.
Current Rules: ${JSON.stringify(rules)}
Raw Input: ${rawEditor.value}

Respond with a brief explanation and a JSON block containing the updated rules array.
Example response: "I updated the table regex. \n \`\`\`json\n[...rules...]\n\`\`\`"`;

    try {
        const response = await window.go.main.App.CallOptimizerLLM(systemPrompt, message);
        addChatMessage('llm', response);
        
        // Try to parse JSON from response
        const jsonMatch = response.match(/```json\n([\s\S]*?)\n```/);
        if (jsonMatch) {
            try {
                const newRules = JSON.parse(jsonMatch[1]);
                if (Array.isArray(newRules)) {
                    rules = newRules;
                    window.go.main.App.SaveRules(rules);
                    renderRulesList();
                    runNormalization();
                    addChatMessage('system', 'Updated rules applied automatically.');
                }
            } catch (e) {
                console.error('Failed to parse model JSON:', e);
            }
        }
    } catch (e) {
        addChatMessage('system', 'Error calling LLM: ' + e.message);
    }
}

function addChatMessage(role, text) {
    const div = document.createElement('div');
    div.className = `message ${role}`;
    div.innerText = text;
    chatMessages.appendChild(div);
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

// --- Settings ---
function openSettings() {
    document.getElementById('set-endpoint').value = settings.endpoint || '';
    document.getElementById('set-apikey').value = settings.apiKey || '';
    document.getElementById('set-model').value = settings.model || '';
    document.getElementById('set-temp').value = settings.temperature || 0.7;
    document.getElementById('set-tokens').value = settings.maxTokens || 2048;
    document.getElementById('settings-modal').classList.add('active');
}

async function saveSettings() {
    settings = {
        endpoint: document.getElementById('set-endpoint').value,
        apiKey: document.getElementById('set-apikey').value,
        model: document.getElementById('set-model').value,
        temperature: parseFloat(document.getElementById('set-temp').value),
        maxTokens: parseInt(document.getElementById('set-tokens').value)
    };
    await window.go.main.App.SaveSettings(settings);
    document.getElementById('settings-modal').classList.remove('active');
}

// --- Utils ---
function debounce(func, wait) {
    let timeout;
    return function (...args) {
        clearTimeout(timeout);
        timeout = setTimeout(() => func.apply(this, args), wait);
    };
}

async function syncFromAppJs() {
    try {
        const importedRules = await window.go.main.App.SyncFromAppJs();
        if (importedRules && importedRules.length > 0) {
            rules = [...rules, ...importedRules];
            window.go.main.App.SaveRules(rules);
            renderRulesList();
            runNormalization();
            addChatMessage('system', `Imported ${importedRules.length} rules from app.js.`);
        } else {
            addChatMessage('system', 'No rules found or failed to parse app.js.');
        }
    } catch (e) {
        addChatMessage('system', 'Sync Error: ' + e.message);
    }
}

// Start
window.addEventListener('load', init);
