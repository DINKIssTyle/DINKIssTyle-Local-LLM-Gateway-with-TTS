/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

// Configuration State
let config = {
    apiEndpoint: 'http://127.0.0.1:1234',
    model: 'qwen/qwen3-vl-30b',
    hideThink: true,
    temperature: 0.7,
    maxTokens: 4096,
    historyCount: 10,
    enableTTS: true,
    ttsLang: 'ko',
    chunkSize: 300,
    systemPrompt: 'You are a helpful AI assistant.',
    ttsVoice: '',
    ttsSpeed: 1.3,
    autoTTS: true
};

// Chat State
let messages = [];
let pendingImage = null;
let isGenerating = false;
let abortController = null;

// Audio State
let currentAudio = null;
let currentAudioBtn = null;
let audioWarmup = null; // Used to bypass auto-play blocks
let ttsQueue = [];
let isPlayingQueue = false;


// DOM Elements
const chatMessages = document.getElementById('chat-messages');
const messageInput = document.getElementById('message-input');
const sendBtn = document.getElementById('send-btn');
const imagePreviewVal = document.getElementById('image-preview');
const previewContainer = document.getElementById('preview-container');

// Audio Context for Auto-play
let audioContextUnlocked = false;

function unlockAudioContext() {
    if (audioContextUnlocked) return;

    // Create and resume an AudioContext or play a silent buffer
    const AudioContext = window.AudioContext || window.webkitAudioContext;
    if (AudioContext) {
        const ctx = new AudioContext();
        ctx.resume().then(() => {
            const buffer = ctx.createBuffer(1, 1, 22050);
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(0);
            audioContextUnlocked = true;
            console.log("AudioContext unlocked");

            // Warm up a silent audio element
            if (!audioWarmup) {
                audioWarmup = new Audio();
                audioWarmup.play().catch(() => { });
            }
        });
    }
}

// Initialization
document.addEventListener('DOMContentLoaded', async () => {
    // Check authentication first
    await checkAuth();

    await checkAuth();

    loadConfig();
    await loadVoiceStyles(); // Fetch voice styles
    await syncServerConfig(); // Sync with server
    setupEventListeners();
    initServerControl();

    // Setup Markdown
    marked.setOptions({
        highlight: function (code, lang) {
            const language = highlight.getLanguage(lang) ? lang : 'plaintext';
            return highlight.highlight(code, { language }).value;
        },
        langPrefix: 'hljs language-'
    });
});

// Current user state
let currentUser = null;

// Check authentication status
async function checkAuth() {
    try {
        const response = await fetch('/api/auth/check');
        const data = await response.json();

        if (!data.authenticated) {
            window.location.href = '/login.html';
            return;
        }

        currentUser = {
            id: data.user_id,
            role: data.role
        };

        // Show admin features if admin
        if (currentUser.role === 'admin') {
            const adminSection = document.getElementById('admin-section');
            if (adminSection) adminSection.style.display = 'block';
            loadUserList();
        }
    } catch (e) {
        console.error('Auth check failed:', e);
        // Don't redirect on network error (might be running in Wails)
    }
}

// Load user list for admin
async function loadUserList() {
    try {
        const response = await fetch('/api/users');
        if (!response.ok) return;

        const users = await response.json();
        const listEl = document.getElementById('user-list');
        if (!listEl) return;

        listEl.innerHTML = users.map(u => `
            <div class="user-item">
                <span>${u.id} (${u.role})</span>
                ${u.id !== currentUser.id ? `<button class="icon-btn" onclick="deleteUser('${u.id}')" title="Delete"><span class="material-icons-round">delete</span></button>` : ''}
            </div>
        `).join('');
    } catch (e) {
        console.error('Failed to load users:', e);
    }
}

// Add user
async function addUser() {
    const id = prompt('Enter username:');
    if (!id) return;

    const password = prompt('Enter password:');
    if (!password) return;

    const role = confirm('Grant admin privileges?') ? 'admin' : 'user';

    try {
        const response = await fetch('/api/users/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, password, role })
        });

        if (response.ok) {
            loadUserList();
            alert('User added successfully');
        } else {
            alert('Failed to add user');
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

// Delete user
async function deleteUser(id) {
    if (!confirm(`Delete user "${id}"?`)) return;

    try {
        const response = await fetch('/api/users/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id })
        });

        if (response.ok) {
            loadUserList();
        } else {
            alert('Failed to delete user');
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

// Logout
async function logout() {
    try {
        await fetch('/api/logout', { method: 'POST' });
        window.location.href = '/login.html';
    } catch (e) {
        console.error('Logout failed:', e);
    }
}

// Server state
let serverRunning = false;

// Initialize server control and check status
async function initServerControl() {
    // Check if Wails runtime is available
    if (typeof window.go === 'undefined') {
        console.log('Wails runtime not detected. Running in web mode.');
        document.querySelector('.server-control').style.display = 'none';
        // Web mode: do not add is-desktop class
        return;
    }

    // Desktop mode: add class to show desktop-only elements
    document.body.classList.add('is-desktop');

    // Get initial server status
    try {
        const status = await window.go.main.App.GetServerStatus();
        updateServerUI(status.running, status.port);
    } catch (e) {
        console.error('Failed to get server status:', e);
    }
}

// Toggle server start/stop
async function toggleServer() {
    if (typeof window.go === 'undefined') {
        alert('Wails runtime not available.');
        return;
    }

    const port = document.getElementById('server-port').value;
    const btn = document.getElementById('server-toggle-btn');
    btn.disabled = true;

    try {
        if (serverRunning) {
            await window.go.main.App.StopServer();
            updateServerUI(false, port);
        } else {
            // Also update LLM endpoint
            const llmEndpoint = document.getElementById('cfg-api').value;
            await window.go.main.App.SetLLMEndpoint(llmEndpoint);
            await window.go.main.App.StartServer(port);
            updateServerUI(true, port);
        }
    } catch (e) {
        alert('Server error: ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

// Update server status UI
function updateServerUI(running, port) {
    serverRunning = running;
    const statusEl = document.getElementById('server-status');
    const dot = statusEl.querySelector('.status-dot');
    const text = statusEl.querySelector('span:last-child');
    const btn = document.getElementById('server-toggle-btn');

    if (running) {
        dot.className = 'status-dot running';
        text.textContent = `Server: Running on :${port}`;
        btn.innerHTML = '<span class="material-icons-round">stop</span> Stop Server';
    } else {
        dot.className = 'status-dot stopped';
        text.textContent = 'Server: Stopped';
        btn.innerHTML = '<span class="material-icons-round">play_arrow</span> Start Server';
    }
}

function loadConfig() {
    const saved = localStorage.getItem('appConfig');
    if (saved) {
        config = { ...config, ...JSON.parse(saved) };
    }

    // Update UI
    document.getElementById('cfg-api').value = config.apiEndpoint;
    document.getElementById('cfg-model').value = config.model;
    document.getElementById('cfg-hide-think').checked = config.hideThink;
    document.getElementById('cfg-temp').value = config.temperature;
    document.getElementById('cfg-max-tokens').value = config.maxTokens;
    document.getElementById('cfg-history').value = config.historyCount;
    document.getElementById('cfg-enable-tts').checked = config.enableTTS;
    document.getElementById('cfg-enable-tts').checked = config.enableTTS;
    document.getElementById('cfg-auto-tts').checked = config.autoTTS || false;
    document.getElementById('cfg-tts-lang').value = config.ttsLang;
    document.getElementById('cfg-chunk-size').value = config.chunkSize || 300;
    document.getElementById('cfg-system-prompt').value = config.systemPrompt || 'You are a helpful AI assistant.';
    if (config.ttsVoice) document.getElementById('cfg-tts-voice').value = config.ttsVoice;
    document.getElementById('cfg-tts-speed').value = config.ttsSpeed || 1.0;
    document.getElementById('speed-val').textContent = config.ttsSpeed || 1.0;
}

function saveConfig() {
    config.apiEndpoint = document.getElementById('cfg-api').value.trim();
    config.model = document.getElementById('cfg-model').value.trim();
    config.hideThink = document.getElementById('cfg-hide-think').checked;
    config.temperature = parseFloat(document.getElementById('cfg-temp').value);
    config.maxTokens = parseInt(document.getElementById('cfg-max-tokens').value);
    config.historyCount = parseInt(document.getElementById('cfg-history').value);
    config.enableTTS = document.getElementById('cfg-enable-tts').checked;
    config.enableTTS = document.getElementById('cfg-enable-tts').checked;
    config.autoTTS = document.getElementById('cfg-auto-tts').checked;
    config.ttsLang = document.getElementById('cfg-tts-lang').value;
    config.chunkSize = parseInt(document.getElementById('cfg-chunk-size').value) || 300;
    config.systemPrompt = document.getElementById('cfg-system-prompt').value.trim() || 'You are a helpful AI assistant.';
    config.ttsVoice = document.getElementById('cfg-tts-voice').value;
    config.ttsSpeed = parseFloat(document.getElementById('cfg-tts-speed').value);

    localStorage.setItem('appConfig', JSON.stringify(config));
    alert('Settings saved!');
}

async function syncServerConfig() {
    try {
        if (response.ok) {
            const serverCfg = await response.json();
            if (serverCfg.enable_tts !== undefined) {
                config.enableTTS = serverCfg.enable_tts;
                document.getElementById('cfg-enable-tts').checked = config.enableTTS;
                localStorage.setItem('appConfig', JSON.stringify(config));
            }
        }
    } catch (e) {
        console.warn('Failed to sync server config:', e);
    }
}

async function loadVoiceStyles() {
    try {
        const response = await fetch('/api/tts/styles');
        if (response.ok) {
            const styles = await response.json();
            const select = document.getElementById('cfg-tts-voice');
            select.innerHTML = styles.map(s => `<option value="${s}">${s}</option>`).join('');
            if (config.ttsVoice && styles.includes(config.ttsVoice)) {
                select.value = config.ttsVoice;
            } else if (styles.length > 0) {
                select.value = styles[0];
                config.ttsVoice = styles[0];
            }
        }
    } catch (e) {
        console.warn('Failed to load voice styles:', e);
    }
}


function toggleSwitch(id) {
    const el = document.getElementById(id);
    if (el) el.checked = !el.checked;
}

function setupEventListeners() {
    document.getElementById('save-cfg-btn').addEventListener('click', saveConfig);

    messageInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            unlockAudioContext(); // Unlock audio on user interaction
            sendMessage();
        }
        autoResizeInput();
    });

    messageInput.addEventListener('input', autoResizeInput);

    // Stop handling
    sendBtn.addEventListener('click', () => {
        if (isGenerating) {
            stopGeneration();
        } else {
            unlockAudioContext(); // Unlock audio on user interaction
            sendMessage();
        }
    });

    // Enter key handling (prevent duplicate listener if one exists in HTML? No, setupEventListeners covers it)
    // Note: The previous listener on sendBtn was not shown in viewed lines but likely exists or was default form submission? 
    // Ah, line 301 calls sendMessage(). I should remove the old click listener if any, or ensuring `sendMessage` checks `isGenerating` is not enough if we want STOP behavior.
    // The previous code didn't show an explicit click listener for sendBtn!
    // Wait, let me check where sendMessage is called.
    // Line 277 calls sendMessage on Enter. 
    // I need to make sure the Button Click also calls sendMessage OR stopGeneration.
    // I will Assume there isn't one and add it.
}

function autoResizeInput() {
    messageInput.style.height = 'auto';
    messageInput.style.height = Math.min(messageInput.scrollHeight, 150) + 'px';
}

function toggleSidebar() {
    document.getElementById('sidebar').classList.toggle('active');
    const overlay = document.getElementById('sidebar-overlay');
    if (overlay) overlay.classList.toggle('active');
}

function handleImageUpload(input) {
    if (input.files && input.files[0]) {
        const reader = new FileReader();
        reader.onload = function (e) {
            pendingImage = e.target.result; // Base64 string
            imagePreviewVal.src = pendingImage;
            previewContainer.style.display = 'block';
        };
        reader.readAsDataURL(input.files[0]);
    }
}

function removeImage() {
    pendingImage = null;
    document.getElementById('image-upload').value = '';
    previewContainer.style.display = 'none';
}

function clearChat() {
    if (isGenerating) return;
    messages = [];
    chatMessages.innerHTML = '';
}

/* Chat Logic */

async function sendMessage() {
    if (isGenerating) return;

    const text = messageInput.value.trim();
    if (!text && !pendingImage) return;

    // Prepare User Message
    const userMsg = {
        role: 'user',
        content: text,
        image: pendingImage
    };

    appendMessage(userMsg);
    messages.push(userMsg);

    // Reset Input
    messageInput.value = '';
    removeImage();
    autoResizeInput();

    // Prepare Assistant Placeholder
    isGenerating = true;
    updateSendButtonState();

    // Create new AbortController
    abortController = new AbortController();

    const assistantId = 'msg-' + Date.now();
    appendMessage({ role: 'assistant', content: '', id: assistantId });

    // Build API Payload
    // Always start with a system prompt to define behavior and anchor the context
    const systemMsg = { role: 'system', content: config.systemPrompt };

    // Trim old messages if history exceeds limit (Chrome extension approach)
    // historyCount = number of conversation turns (user+assistant pairs)
    const maxMessages = (parseInt(config.historyCount) || 10) * 2;
    if (messages.length > maxMessages) {
        // Remove oldest messages, keeping recent ones
        messages = messages.slice(-maxMessages);
    }

    // Map all current messages to API format
    const history = messages.map(m => {
        if (m.image) {
            // Vision format
            return {
                role: m.role,
                content: [
                    { type: 'text', text: m.content || 'Please describe this image.' },
                    { type: 'image_url', image_url: { url: m.image } }
                ]
            };
        } else {
            // Clean content for history
            let content = m.content || '';
            if (m.role === 'assistant') {
                // Remove think tags from history to prevent recursion/confusion
                content = content.replace(/<think>[\s\S]*?<\/think>/g, '').trim();
            }
            return { role: m.role, content: content };
        }
    });

    const payload = {
        model: config.model,
        messages: [systemMsg, ...history], // Prepend system message
        temperature: config.temperature,
        max_tokens: config.maxTokens,
        stream: true
    };

    // Debug: Log payload to verify what's being sent
    console.log('=== LLM Request Payload ===');
    console.log('System Prompt:', systemMsg.content);
    console.log('History Count:', history.length);
    console.log('Messages:', JSON.stringify(payload.messages, null, 2));

    try {
        await streamResponse(payload, assistantId);
    } catch (e) {
        if (e.name === 'AbortError') {
            updateMessageContent(assistantId, `**[Stopped by User]**`);
        } else {
            updateMessageContent(assistantId, `**Error:** ${e.message}`);
        }
    } finally {
        isGenerating = false;
        abortController = null;
        updateSendButtonState();
    }
}

function stopGeneration() {
    if (abortController) {
        abortController.abort();
        abortController = null;
    }
}

function updateSendButtonState() {
    if (isGenerating) {
        sendBtn.disabled = false; // Enabled so we can Click to Stop
        sendBtn.innerHTML = '<span class="material-icons-round">stop</span>';
        sendBtn.title = "Stop Generation";
        sendBtn.classList.add('stop-btn');
    } else {
        sendBtn.disabled = false;
        sendBtn.innerHTML = '<span class="material-icons-round">send</span>';
        sendBtn.title = "Send Message";
        sendBtn.classList.remove('stop-btn');
    }
}

async function streamResponse(payload, elementId) {
    // Use the Go server's API endpoint
    const response = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: abortController.signal
    });

    if (!response.ok) {
        let errorDetails = `Server Error ${response.status}: ${response.statusText}`;
        const errorBody = await response.text();
        if (errorBody) {
            errorDetails += ` - ${errorBody}`;
        }
        throw new Error(errorDetails);
    }

    await processStream(response, elementId);
}

// Helper to process the stream reader (shared by direct and proxy)
async function processStream(response, elementId) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let fullText = '';

    while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        const lines = buffer.split('\n\n');
        buffer = lines.pop();

        for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed.startsWith('data: ')) continue;

            const dataStr = trimmed.substring(6);
            if (dataStr === '[DONE]') break; // Was 'return' - caused assistant message to not be saved!

            try {
                const json = JSON.parse(dataStr);
                const delta = json.choices[0].delta;
                const content = delta.content || '';

                if (content) {
                    fullText += content;
                    let displayText = fullText;
                    if (config.hideThink) {
                        displayText = fullText.replace(/<think>[\s\S]*?<\/think>/g, '');
                        if (displayText.includes('<think>')) displayText = displayText.split('<think>')[0];
                    }
                    updateMessageContent(elementId, displayText);
                }
            } catch (e) {
                console.error('JSON Parse Error', e);
            }
        }
    }
    // Finalize
    messages.push({ role: 'assistant', content: fullText });

    // Auto-play TTS if enabled
    if (config.enableTTS && config.autoTTS && fullText) {
        let displayText = fullText;
        if (config.hideThink) {
            displayText = fullText.replace(/<think>[\s\S]*?<\/think>/g, '');
        }
        console.log("Auto-playing TTS for text length:", displayText.length);

        // Target the last message's speak button if it exists
        const lastMsg = document.getElementById(elementId);
        const btn = lastMsg ? lastMsg.querySelector('.speak-btn') : null;
        speakMessage(displayText, btn);
    }

    // Ensure action buttons are rendered properly (unhide or ensure they exist)
    // appendMessage already creates them. We don't need addSpeakButton anymore.
}

function appendMessage(msg) {
    const div = document.createElement('div');
    div.className = `message ${msg.role}`;
    if (msg.id) div.id = msg.id;

    let innerHtml = '';

    // Wrapper start
    innerHtml += `<div class="message-inner">`;

    // Bubble content
    let bubbleContent = '';
    if (msg.image) {
        bubbleContent += `<img src="${msg.image}" class="message-image">`;
    }

    const textContent = msg.content || '';
    if (msg.role === 'user') {
        bubbleContent += `<div class="message-bubble">${escapeHtml(textContent)}</div>`;
    } else {
        bubbleContent += `<div class="message-bubble"><div class="markdown-body">${marked.parse(textContent)}</div></div>`;
    }

    innerHtml += bubbleContent;

    // Action Bar (for assistant only)
    if (msg.role === 'assistant') {
        innerHtml += `
            <div class="message-actions">
                <button class="icon-btn copy-btn" onclick="copyMessage(this)" title="Copy">
                    <span class="material-icons-round">content_copy</span>
                </button>
                <button class="icon-btn speak-btn" onclick="speakMessageFromBtn(this)" title="Speak">
                    <span class="material-icons-round">volume_up</span>
                </button>
            </div>`;
    }

    // Wrapper end
    innerHtml += `</div>`;

    div.innerHTML = innerHtml;
    chatMessages.appendChild(div);
    scrollToBottom();
}

// New helper functions
// New helper functions
async function copyMessage(btn) {
    const bubble = btn.closest('.message-inner').querySelector('.markdown-body');
    if (!bubble) return;

    // Get text content (try to get clean text without HTML if possible, or just innerText)
    const text = bubble.innerText;
    try {
        await navigator.clipboard.writeText(text);
        showToast('Copied to clipboard');
    } catch (err) {
        console.warn('Clipboard API failed, trying fallback', err);
        fallbackCopyTextToClipboard(text);
    }
}

function fallbackCopyTextToClipboard(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;

    // Avoid scrolling to bottom
    textArea.style.top = "0";
    textArea.style.left = "0";
    textArea.style.position = "fixed";

    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
        var successful = document.execCommand('copy');
        var msg = successful ? 'successful' : 'unsuccessful';
        if (successful) {
            showToast('Copied to clipboard');
        } else {
            showToast('Failed to copy', true);
        }
    } catch (err) {
        console.error('Fallback: Oops, unable to copy', err);
        showToast('Failed to copy', true);
    }
    document.body.removeChild(textArea);
}

function showToast(message, isError = false) {
    let toast = document.getElementById('toast-notification');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toast-notification';
        toast.className = 'toast';
        document.body.appendChild(toast);
    }

    const icon = isError ? 'error_outline' : 'check_circle';
    const color = isError ? 'var(--danger-color)' : 'var(--success-color)';

    toast.innerHTML = `
        <span class="material-icons-round" style="color: ${color}">${icon}</span>
        <span>${message}</span>
    `;

    // Trigger reflow
    void toast.offsetWidth;

    toast.classList.add('show');

    // Hide after 3s
    if (toast.timeoutId) clearTimeout(toast.timeoutId);
    toast.timeoutId = setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

function speakMessageFromBtn(btn) {
    const bubble = btn.closest('.message-inner').querySelector('.markdown-body');
    if (bubble) {
        speakMessage(bubble.innerText, btn);
    }
}

function stopAllAudio() {
    if (currentAudio) {
        currentAudio.pause();
        currentAudio.src = '';
        currentAudio.src = '';
        currentAudio = null;
    }
    // Clear Queue
    ttsQueue = [];
    isPlayingQueue = false;
    // Reset all icons
    document.querySelectorAll('.speak-btn').forEach(btn => {
        const icon = btn.querySelector('.material-icons-round');
        if (icon) icon.textContent = 'volume_up';
        btn.title = "Speak";
    });
    currentAudioBtn = null;
}

function updateMessageContent(id, text) {
    const el = document.getElementById(id);
    if (!el) return;

    const bubble = el.querySelector('.message-bubble');
    bubble.innerHTML = `<div class="markdown-body">${marked.parse(text)}</div>`;

    // Re-highlight code blocks
    bubble.querySelectorAll('pre code').forEach((block) => {
        highlight.highlightElement(block);
    });

    scrollToBottom();
}

function scrollToBottom() {
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

// TTS: Speak a message using the Go server's /api/tts endpoint
async function speakMessage(text, btn = null) {
    // If clicking the same button that is currently playing, stop it
    if (currentAudio && btn && btn === currentAudioBtn) {
        stopAllAudio();
        return;
    }

    // Stop previous audio before starting new one
    stopAllAudio();

    if (!config.enableTTS) {
        if (!btn) return; // Don't alert on auto-play failure if disabled
        alert('TTS is disabled. Enable it in settings.');
        return;
    }

    // Clean markdown/html from text
    const cleanText = text.replace(/<[^>]*>/g, '').replace(/[#*`_~]/g, '').trim();
    if (!cleanText) return;

    // Split text into chunks (sentences)
    // Match sentences followed by space or end of string, keeping delimiter
    const rawChunks = cleanText.match(/[^.!?\n]+[.!?\n]*/g) || [cleanText];

    // Combine short chunks
    let chunks = [];
    let currentChunk = "";

    for (const part of rawChunks) {
        if ((currentChunk + part).length > config.chunkSize && currentChunk) {
            chunks.push(currentChunk.trim());
            currentChunk = "";
        }
        currentChunk += part;
    }
    if (currentChunk) chunks.push(currentChunk.trim());

    ttsQueue = chunks;
    if (btn) currentAudioBtn = btn;

    processTTSQueue();
}

async function processTTSQueue(isFirstChunk = false) {
    if (ttsQueue.length === 0) {
        console.log("TTS Queue finished");
        if (currentAudioBtn) {
            currentAudioBtn.querySelector('.material-icons-round').textContent = 'volume_up';
            currentAudioBtn.title = 'Speak';
            currentAudioBtn = null;
        }
        isPlayingQueue = false;
        return;
    }

    isPlayingQueue = true;
    const text = ttsQueue.shift();
    const btn = currentAudioBtn;

    // Safety: if manually stopped, don't auto-continue unless it's new
    if (!isPlayingQueue && !isFirstChunk) return;

    try {
        if (btn) {
            btn.querySelector('.material-icons-round').textContent = 'hourglass_empty';
            btn.disabled = true;
        }

        console.log(`Fetching TTS chunk: "${text.substring(0, 20)}..."`);
        const response = await fetch('/api/tts', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                text: text,
                lang: config.ttsLang,
                chunkSize: parseInt(config.chunkSize) || 300,
                voiceStyle: config.ttsVoice,
                speed: parseFloat(config.ttsSpeed) || 1.0
            })
        });

        if (btn) btn.disabled = false;

        if (!response.ok) {
            const errMsg = await response.text();
            console.error('TTS Error:', errMsg);

            if (isFirstChunk) {
                alert(`TTS Failed: ${errMsg}`);
                stopAllAudio(); // Stop queue
                if (btn) btn.querySelector('.material-icons-round').textContent = 'error';
                return;
            }

            // If mid-queue failure, maybe just log and stop to avoid partial confusion
            console.warn("Stopping TTS queue due to error.");
            stopAllAudio();
            if (btn) btn.querySelector('.material-icons-round').textContent = 'error';
            return;
        }

        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('audio/wav')) {
            const blob = await response.blob();
            const url = URL.createObjectURL(blob);

            const audio = new Audio(url);
            // Don't set currentAudio globally yet?
            // If we do, stopAllAudio() works.
            currentAudio = audio;

            if (btn) {
                btn.querySelector('.material-icons-round').textContent = 'stop';
                btn.title = "Stop";
            }

            audio.onended = () => {
                currentAudio = null;
                processTTSQueue();
            };

            audio.onerror = (e) => {
                console.error("Audio playback error", e);
                currentAudio = null;
                processTTSQueue();
            };

            await audio.play();
        } else {
            console.warn("TTS returned non-audio:", contentType);
            processTTSQueue();
        }
    } catch (e) {
        console.error('TTS Fetch/Play Error:', e);
        if (btn) btn.disabled = false;

        if (isFirstChunk) {
            alert(`TTS Error: ${e.message}`);
            stopAllAudio();
            if (btn) btn.querySelector('.material-icons-round').textContent = 'error';
        } else {
            processTTSQueue();
        }
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}


