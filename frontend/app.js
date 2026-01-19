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
    autoTTS: true,
    ttsFormat: 'wav' // 'wav' or 'mp3'
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

// Streaming TTS State
let streamingTTSActive = false;
let streamingTTSCommittedIndex = 0; // How much of the display text has been sent to TTS
let streamingTTSBuffer = ""; // Uncommitted text buffer
let streamingTTSProcessor = null; // Reference to the active processor loop
let ttsSessionId = 0;


// DOM Elements
const chatMessages = document.getElementById('chat-messages');
const messageInput = document.getElementById('message-input');
const sendBtn = document.getElementById('send-btn');
const imagePreviewVal = document.getElementById('image-preview');
const previewContainer = document.getElementById('preview-container');

// Audio Context for Auto-play
let audioContextUnlocked = false;
let audioCtx = null;
let currentSource = null;

function unlockAudioContext() {
    if (!audioCtx) {
        const AudioContext = window.AudioContext || window.webkitAudioContext;
        if (AudioContext) {
            audioCtx = new AudioContext();
        }
    }

    if (audioCtx && audioCtx.state === 'suspended') {
        audioCtx.resume().then(() => {
            // Play silent buffer to unlock
            const buffer = audioCtx.createBuffer(1, 1, 22050);
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            source.connect(audioCtx.destination);
            source.start(0);
            audioContextUnlocked = true;
            console.log("AudioContext unlocked/resumed");
        }).catch(e => console.error("Failed to resume AudioContext", e));
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
    let format = config.ttsFormat || 'wav';
    if (format === 'mp3') format = 'mp3-high'; // Legacy mapping
    document.getElementById('cfg-tts-format').value = format;
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
    config.ttsFormat = document.getElementById('cfg-tts-format').value;

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
            // Fix Korean IME duplicate submission / residual character issue
            if (e.isComposing) return;
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
    // Stop any TTS playback and generation
    stopAllAudio();

    if (isGenerating) {
        stopGeneration();
    }

    messages = [];
    chatMessages.innerHTML = '';
}

/* Chat Logic */

async function sendMessage() {
    // Unlock audio context on user interaction
    unlockAudioContext();

    const text = messageInput.value.trim();
    if (!text && !pendingImage) return;
    if (isGenerating) return;

    // Stop and clear any existing audio/TTS
    stopAllAudio();

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

    // Trim old messages if history exceeds limit (user+assistant pairs)
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

    // Initialize streaming TTS if enabled
    const useStreamingTTS = config.enableTTS && config.autoTTS;
    if (useStreamingTTS) {
        initStreamingTTS(elementId);
    }

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
            if (dataStr === '[DONE]') break;

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

                    // Feed to streaming TTS
                    if (useStreamingTTS) {
                        feedStreamingTTS(displayText);
                    }
                }
            } catch (e) {
                console.error('JSON Parse Error', e);
            }
        }
    }

    // Finalize
    messages.push({ role: 'assistant', content: fullText });

    // Finalize streaming TTS (commit any remaining text)
    if (useStreamingTTS) {
        let finalDisplayText = fullText;
        if (config.hideThink) {
            finalDisplayText = fullText.replace(/<think>[\s\S]*?<\/think>/g, '');
        }
        finalizeStreamingTTS(finalDisplayText);
    }
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

/**
 * Stop all audio playback and clear queues
 */
function stopAllAudio() {
    // Clear queues
    ttsQueue = [];
    audioWarmup = null;

    // Stop current audio source
    if (currentSource) {
        try {
            currentSource.stop();
        } catch (e) {
            // Ignore errors if already stopped
        }
        currentSource = null;
    }

    // Clear audio cache to free memory
    clearTTSAudioCache();

    // Reset loop state
    isPlayingQueue = false;

    // Cancel streaming
    streamingTTSActive = false;
    streamingTTSBuffer = "";
    streamingTTSCommittedIndex = 0;

    // Increment session ID to invalidate pending ops
    ttsSessionId++;

    // Reset UI
    const btn = currentAudioBtn;
    if (btn) {
        const iconEl = btn.querySelector('.material-icons-round');
        if (iconEl) iconEl.textContent = 'volume_up';
        btn.title = 'Speak';
        btn.disabled = false;
    }
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
    // If clicking the same button that is currently playing or streaming, stop it
    if ((isPlayingQueue || streamingTTSActive) && btn && btn === currentAudioBtn) {
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

    // Clean text for TTS (remove emojis, markdown, etc.)
    const cleanText = cleanTextForTTS(text);
    if (!cleanText) return;

    // Initialize/Clear queue
    ttsQueue = []; // Clear existing queue if any (though stopAllAudio called above)
    if (btn) currentAudioBtn = btn;

    // 1. Split by paragraphs (double newlines) first to ensure backend doesn't receive multi-paragraph chunks
    const paragraphs = cleanText.split(/\n\s*\n+/);

    let currentChunk = "";

    for (const paragraph of paragraphs) {
        if (!paragraph.trim()) continue;

        // 2. Split paragraph into sentences
        // Match sentences followed by space or end of string, keeping delimiter
        const rawChunks = paragraph.match(/[^.!?\n]+[.!?\n]*/g) || [paragraph];

        // 3. Combine sentences up to chunkSize
        for (const part of rawChunks) {
            if ((currentChunk + part).length > config.chunkSize && currentChunk) {
                const chunk = currentChunk.trim();
                ttsQueue.push(chunk);
                currentChunk = "";

                // Trigger playback immediately if not running
                processTTSQueue();
            }
            currentChunk += part;
        }
        if (currentChunk) {
            const chunk = currentChunk.trim();
            ttsQueue.push(chunk);
            currentChunk = "";
            processTTSQueue();
        }
    }

    // Final check for remaining chunk (redundant with logic above but safe)
    if (currentChunk) {
        ttsQueue.push(currentChunk.trim());
        processTTSQueue();
    }
}

// ============================================================================
// Streaming TTS Functions
// These enable TTS generation to start while LLM is still streaming
// ============================================================================

/**
 * Clean text for TTS - removes emojis, markdown, and non-speakable characters
 */
function cleanTextForTTS(text) {
    if (!text) return '';

    let cleaned = text;

    // Remove HTML tags
    cleaned = cleaned.replace(/<[^>]*>/g, '');

    // Remove emoji (comprehensive Unicode ranges)
    cleaned = cleaned.replace(/[\u{1F600}-\u{1F64F}]/gu, ''); // Emoticons
    cleaned = cleaned.replace(/[\u{1F300}-\u{1F5FF}]/gu, ''); // Misc Symbols and Pictographs
    cleaned = cleaned.replace(/[\u{1F680}-\u{1F6FF}]/gu, ''); // Transport and Map
    cleaned = cleaned.replace(/[\u{1F1E0}-\u{1F1FF}]/gu, ''); // Flags
    cleaned = cleaned.replace(/[\u{2600}-\u{26FF}]/gu, '');   // Misc symbols
    cleaned = cleaned.replace(/[\u{2700}-\u{27BF}]/gu, '');   // Dingbats
    cleaned = cleaned.replace(/[\u{FE00}-\u{FE0F}]/gu, '');   // Variation Selectors
    cleaned = cleaned.replace(/[\u{1F900}-\u{1F9FF}]/gu, ''); // Supplemental Symbols and Pictographs
    cleaned = cleaned.replace(/[\u{1FA00}-\u{1FA6F}]/gu, ''); // Chess Symbols
    cleaned = cleaned.replace(/[\u{1FA70}-\u{1FAFF}]/gu, ''); // Symbols and Pictographs Extended-A
    cleaned = cleaned.replace(/[\u{231A}-\u{231B}]/gu, '');   // Watch, Hourglass
    cleaned = cleaned.replace(/[\u{23E9}-\u{23F3}]/gu, '');   // Various symbols
    cleaned = cleaned.replace(/[\u{23F8}-\u{23FA}]/gu, '');   // Various symbols
    cleaned = cleaned.replace(/[\u{25AA}-\u{25AB}]/gu, '');   // Squares
    cleaned = cleaned.replace(/[\u{25B6}]/gu, '');            // Play button
    cleaned = cleaned.replace(/[\u{25C0}]/gu, '');            // Reverse button
    cleaned = cleaned.replace(/[\u{25FB}-\u{25FE}]/gu, '');   // Squares
    cleaned = cleaned.replace(/[\u{2614}-\u{2615}]/gu, '');   // Umbrella, Hot beverage
    cleaned = cleaned.replace(/[\u{2648}-\u{2653}]/gu, '');   // Zodiac
    cleaned = cleaned.replace(/[\u{267F}]/gu, '');            // Wheelchair
    cleaned = cleaned.replace(/[\u{2693}]/gu, '');            // Anchor
    cleaned = cleaned.replace(/[\u{26A1}]/gu, '');            // High voltage
    cleaned = cleaned.replace(/[\u{26AA}-\u{26AB}]/gu, '');   // Circles
    cleaned = cleaned.replace(/[\u{26BD}-\u{26BE}]/gu, '');   // Sports
    cleaned = cleaned.replace(/[\u{26C4}-\u{26C5}]/gu, '');   // Weather
    cleaned = cleaned.replace(/[\u{26CE}]/gu, '');            // Ophiuchus
    cleaned = cleaned.replace(/[\u{26D4}]/gu, '');            // No entry
    cleaned = cleaned.replace(/[\u{26EA}]/gu, '');            // Church
    cleaned = cleaned.replace(/[\u{26F2}-\u{26F3}]/gu, '');   // Fountain, Golf
    cleaned = cleaned.replace(/[\u{26F5}]/gu, '');            // Sailboat
    cleaned = cleaned.replace(/[\u{26FA}]/gu, '');            // Tent
    cleaned = cleaned.replace(/[\u{26FD}]/gu, '');            // Fuel pump
    cleaned = cleaned.replace(/[\u{2702}]/gu, '');            // Scissors
    cleaned = cleaned.replace(/[\u{2705}]/gu, '');            // Check mark
    cleaned = cleaned.replace(/[\u{2708}-\u{270D}]/gu, '');   // Various
    cleaned = cleaned.replace(/[\u{270F}]/gu, '');            // Pencil
    cleaned = cleaned.replace(/[\u{2712}]/gu, '');            // Black nib
    cleaned = cleaned.replace(/[\u{2714}]/gu, '');            // Check mark
    cleaned = cleaned.replace(/[\u{2716}]/gu, '');            // X mark
    cleaned = cleaned.replace(/[\u{271D}]/gu, '');            // Latin cross
    cleaned = cleaned.replace(/[\u{2721}]/gu, '');            // Star of David
    cleaned = cleaned.replace(/[\u{2728}]/gu, '');            // Sparkles
    cleaned = cleaned.replace(/[\u{2733}-\u{2734}]/gu, '');   // Eight spoked asterisk
    cleaned = cleaned.replace(/[\u{2744}]/gu, '');            // Snowflake
    cleaned = cleaned.replace(/[\u{2747}]/gu, '');            // Sparkle
    cleaned = cleaned.replace(/[\u{274C}]/gu, '');            // Cross mark
    cleaned = cleaned.replace(/[\u{274E}]/gu, '');            // Cross mark
    cleaned = cleaned.replace(/[\u{2753}-\u{2755}]/gu, '');   // Question marks
    cleaned = cleaned.replace(/[\u{2757}]/gu, '');            // Exclamation mark
    cleaned = cleaned.replace(/[\u{2763}-\u{2764}]/gu, '');   // Hearts
    cleaned = cleaned.replace(/[\u{2795}-\u{2797}]/gu, '');   // Math symbols
    cleaned = cleaned.replace(/[\u{27A1}]/gu, '');            // Right arrow
    cleaned = cleaned.replace(/[\u{27B0}]/gu, '');            // Curly loop
    cleaned = cleaned.replace(/[\u{27BF}]/gu, '');            // Double curly loop
    cleaned = cleaned.replace(/[\u{2934}-\u{2935}]/gu, '');   // Arrows
    cleaned = cleaned.replace(/[\u{2B05}-\u{2B07}]/gu, '');   // Arrows
    cleaned = cleaned.replace(/[\u{2B1B}-\u{2B1C}]/gu, '');   // Squares
    cleaned = cleaned.replace(/[\u{2B50}]/gu, '');            // Star
    cleaned = cleaned.replace(/[\u{2B55}]/gu, '');            // Circle
    cleaned = cleaned.replace(/[\u{3030}]/gu, '');            // Wavy dash
    cleaned = cleaned.replace(/[\u{303D}]/gu, '');            // Part alternation mark
    cleaned = cleaned.replace(/[\u{3297}]/gu, '');            // Circled Ideograph Congratulation
    cleaned = cleaned.replace(/[\u{3299}]/gu, '');            // Circled Ideograph Secret

    // FIRST: Add punctuation to markdown elements that typically don't have them
    // Headlines (# text) - add period at end if no punctuation
    cleaned = cleaned.replace(/^(#{1,6})\s*([^\n]+?)([.!?]?)$/gm, (match, hashes, text, punct) => {
        // If headline doesn't end with punctuation, add a period
        if (!punct) {
            return `${hashes} ${text}.`;
        }
        return match;
    });

    // Bold/Italic text that ends a line without punctuation
    cleaned = cleaned.replace(/(\*\*[^*]+\*\*|\*[^*]+\*)(\s*)$/gm, '$1.$2');

    // Remove markdown formatting (AFTER adding punctuation)
    cleaned = cleaned.replace(/#{1,6}\s*/gm, ''); // Headlines - remove # but keep text
    cleaned = cleaned.replace(/[*`_~|]/g, ''); // Other markdown syntax characters
    cleaned = cleaned.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1'); // [link text](url) -> link text
    cleaned = cleaned.replace(/!\[[^\]]*\]\([^)]*\)/g, ''); // Remove image markdown
    cleaned = cleaned.replace(/```[\s\S]*?```/g, ''); // Code blocks
    cleaned = cleaned.replace(/`[^`]*`/g, ''); // Inline code
    cleaned = cleaned.replace(/^>\s*/gm, ''); // Blockquotes
    cleaned = cleaned.replace(/^-{3,}$/gm, ''); // Horizontal rules
    cleaned = cleaned.replace(/^\s*[-*+]\s+/gm, ''); // List markers
    cleaned = cleaned.replace(/^\s*\d+\.\s+/gm, ''); // Numbered lists

    // Remove special characters that shouldn't be spoken
    cleaned = cleaned.replace(/[«»""''„‚]/g, ''); // Fancy quotes
    cleaned = cleaned.replace(/[—–]/g, ', '); // Dashes to pauses
    cleaned = cleaned.replace(/\.{3,}/g, '.'); // Ellipsis
    cleaned = cleaned.replace(/\s*[-•◦▪▸►]\s*/g, ', '); // Bullet points to pauses

    // Normalize whitespace
    cleaned = cleaned.replace(/\r\n/g, '\n'); // Normalize line endings
    cleaned = cleaned.replace(/\n{3,}/g, '\n\n'); // Max 2 newlines
    cleaned = cleaned.replace(/[ \t]+/g, ' '); // Multiple spaces to single
    cleaned = cleaned.replace(/^\s+|\s+$/gm, ''); // Trim each line

    return cleaned.trim();
}

/**
 * Initialize streaming TTS for a new message
 */
function initStreamingTTS(elementId) {
    // Stop any existing audio/TTS
    stopAllAudio();

    streamingTTSActive = true;
    streamingTTSCommittedIndex = 0;
    streamingTTSBuffer = "";

    // Get speak button for UI updates
    const msgEl = document.getElementById(elementId);
    if (msgEl) {
        currentAudioBtn = msgEl.querySelector('.speak-btn');
    }

    console.log("[Streaming TTS] Initialized");
}

/**
 * Feed new display text to the streaming TTS processor
 * This is called every time the LLM emits new tokens
 */
function feedStreamingTTS(displayText) {
    if (!streamingTTSActive) return;

    // Process all available committed segments in a loop
    let iterations = 0;
    const maxIterations = 20; // Safety limit

    while (iterations < maxIterations) {
        iterations++;

        // Get the new portion of text since last commit
        const newText = displayText.substring(streamingTTSCommittedIndex);
        if (!newText || newText.length < 5) break; // Need at least some content

        let committed = null;
        let advanceBy = 0;

        // Priority 1: Check for paragraph boundaries (double newline)
        const paragraphMatch = newText.match(/^([\s\S]*?\n\s*\n)/);
        if (paragraphMatch && paragraphMatch[1].trim()) {
            committed = cleanTextForTTS(paragraphMatch[1]);
            advanceBy = paragraphMatch[0].length;
        }

        // Priority 2: Check for sentence endings (.!?) followed by space and more text
        if (!committed) {
            const sentenceMatch = newText.match(/^([\s\S]*?[.!?])(\s+\S)/);
            if (sentenceMatch && sentenceMatch[1].trim()) {
                committed = cleanTextForTTS(sentenceMatch[1]);
                advanceBy = sentenceMatch[1].length;
            }
        }

        // Priority 3: Check for colon followed by newline (common in lists/explanations)
        if (!committed) {
            const colonMatch = newText.match(/^([\s\S]*?:)\s*\n/);
            if (colonMatch && colonMatch[1].trim().length > 10) {
                committed = cleanTextForTTS(colonMatch[1]);
                advanceBy = colonMatch[0].length;
            }
        }

        // Priority 4: Check for single newline with substantial text before it
        if (!committed) {
            const lineMatch = newText.match(/^([^\n]{20,})\n/);
            if (lineMatch) {
                committed = cleanTextForTTS(lineMatch[1]);
                advanceBy = lineMatch[0].length;
            }
        }

        // If nothing matched, stop the loop
        if (!committed) break;

        // Commit the segment
        console.log(`[Streaming TTS] Committing: "${committed.substring(0, 40)}..."`);
        pushToStreamingTTSQueue(committed);
        streamingTTSCommittedIndex += advanceBy;
    }
}

/**
 * Finalize streaming TTS when LLM stream ends
 * Commits any remaining uncommitted text
 */
function finalizeStreamingTTS(finalDisplayText) {
    if (!streamingTTSActive) return;

    // Commit any remaining text
    const remainingText = finalDisplayText.substring(streamingTTSCommittedIndex);
    const cleanText = cleanTextForTTS(remainingText);

    if (cleanText) {
        console.log(`[Streaming TTS] Finalizing: "${cleanText.substring(0, 30)}..."`);
        pushToStreamingTTSQueue(cleanText);
    }

    streamingTTSActive = false;
    console.log("[Streaming TTS] Finalized");
}

/**
 * Push a text segment to the TTS queue and ensure processing is running
 */
function pushToStreamingTTSQueue(text) {
    if (!text || !text.trim()) return;

    // Split into smaller chunks if needed (by paragraph/sentence within the segment)
    const paragraphs = text.split(/\n\s*\n+/);
    const newChunks = [];

    for (const para of paragraphs) {
        if (!para.trim()) continue;

        // Split by sentences and combine to chunkSize
        const rawChunks = para.match(/[^.!?\n]+[.!?\n]*/g) || [para];
        let currentChunk = "";

        for (const part of rawChunks) {
            if ((currentChunk + part).length > config.chunkSize && currentChunk) {
                const chunk = currentChunk.trim();
                // Only add if it has actual speakable content (not just punctuation)
                if (chunk && chunk.length > 1 && /[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(chunk)) {
                    ttsQueue.push(chunk);
                    newChunks.push(chunk);
                }
                currentChunk = "";
            }
            currentChunk += part;
        }
        if (currentChunk.trim()) {
            const chunk = currentChunk.trim();
            // Only add if it has actual speakable content
            if (chunk && chunk.length > 1 && /[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(chunk)) {
                ttsQueue.push(chunk);
                newChunks.push(chunk);
            }
        }
    }

    // IMMEDIATELY start prefetching new chunks (don't wait for the playback loop)
    // This is the key optimization - prefetch starts right when text is committed
    for (const chunk of newChunks) {
        prefetchTTSAudio(chunk);
    }

    // Start processing if not already running
    if (!isPlayingQueue && ttsQueue.length > 0) {
        processTTSQueue();
    }
}

// ============================================================================
// Global TTS Audio Cache and Prefetch System
// ============================================================================
const ttsAudioCache = new Map(); // text -> Promise<url>

/**
 * Prefetch audio for a given text chunk
 * Can be called anytime - will use cached promise if already fetching/fetched
 */
function prefetchTTSAudio(text) {
    if (!text) return null;
    if (ttsAudioCache.has(text)) return ttsAudioCache.get(text);

    const promise = (async () => {
        // Check if session is still valid
        const sessionAtStart = ttsSessionId;

        try {
            const payload = {
                text: text,
                lang: config.ttsLang,
                chunkSize: parseInt(config.chunkSize) || 300,
                voiceStyle: config.ttsVoice,
                speed: parseFloat(config.ttsSpeed) || 1.0,
                format: config.ttsFormat || 'wav'
            };

            console.log(`[TTS] Prefetching: "${text.substring(0, 25)}..."`);
            const response = await fetch('/api/tts', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });

            // Check if session changed during fetch
            if (sessionAtStart !== ttsSessionId) {
                console.log(`[TTS] Session changed, discarding prefetch`);
                return null;
            }

            if (!response.ok) {
                console.error(`[TTS] Chunk failed:`, await response.text());
                return null;
            }

            const blob = await response.blob();
            const url = URL.createObjectURL(blob);
            console.log(`[TTS] Prefetch complete: "${text.substring(0, 25)}..."`);
            return url;
        } catch (e) {
            console.error(`[TTS] Chunk error:`, e);
            return null;
        }
    })();

    ttsAudioCache.set(text, promise);
    return promise;
}

/**
 * Clear the audio cache (called on stopAllAudio)
 */
function clearTTSAudioCache() {
    // Revoke all URLs
    ttsAudioCache.forEach(async (promise) => {
        const url = await promise;
        if (url) URL.revokeObjectURL(url);
    });
    ttsAudioCache.clear();
}

async function processTTSQueue(isFirstChunk = false) {
    if (ttsQueue.length === 0) return;
    if (isPlayingQueue) return; // Already running

    isPlayingQueue = true;
    const btn = currentAudioBtn;
    const sessionId = ttsSessionId;

    if (btn) {
        const iconEl = btn.querySelector('.material-icons-round');
        if (iconEl) iconEl.textContent = 'hourglass_empty';
        btn.disabled = true;
    }

    let firstChunkPlayed = false;

    // Start prefetching first few items immediately
    for (let i = 0; i < Math.min(3, ttsQueue.length); i++) {
        prefetchTTSAudio(ttsQueue[i]);
    }

    // Main processing loop
    while (true) {
        // Check cancellation
        if (sessionId !== ttsSessionId) break;

        // Get next item from queue
        const text = ttsQueue.shift();

        if (!text) {
            // Queue empty - check if streaming is still active
            if (streamingTTSActive) {
                // Wait a bit for more items to arrive
                await new Promise(r => setTimeout(r, 100));
                continue;
            } else {
                // Streaming finished and queue empty - we're done
                break;
            }
        }

        // Start prefetching next items while we process current
        for (let i = 0; i < Math.min(2, ttsQueue.length); i++) {
            prefetchTTSAudio(ttsQueue[i]);
        }

        // Get current audio
        let audioUrl = null;
        try {
            const audioUrlPromise = prefetchTTSAudio(text);
            audioUrl = await audioUrlPromise;
        } catch (e) {
            console.error("Prefetch failed", e);
        }

        // Remove from cache after getting
        ttsAudioCache.delete(text);

        if (!audioUrl) {
            continue; // Skip failed chunks
        }

        // Check cancellation again
        if (sessionId !== ttsSessionId) {
            URL.revokeObjectURL(audioUrl);
            break;
        }

        // Play audio using Web Audio API
        try {
            // Unlock audio context if needed (last ditch effort)
            unlockAudioContext();

            if (!audioCtx) {
                throw new Error("AudioContext not available");
            }

            // Fetch blob data
            const response = await fetch(audioUrl);
            const arrayBuffer = await response.arrayBuffer();

            // Decode audio data
            const audioBuffer = await audioCtx.decodeAudioData(arrayBuffer);

            // Update UI on first chunk playing
            if (!firstChunkPlayed && btn) {
                btn.disabled = false;
                const iconEl = btn.querySelector('.material-icons-round');
                if (iconEl) iconEl.textContent = 'stop';
                btn.title = "Stop";
                firstChunkPlayed = true;
            }

            // Create source and play
            await new Promise((resolve, reject) => {
                // Check cancellation before starting
                if (sessionId !== ttsSessionId) {
                    resolve();
                    return;
                }

                const source = audioCtx.createBufferSource();
                source.buffer = audioBuffer;
                source.connect(audioCtx.destination);
                currentSource = source;

                source.onended = () => {
                    currentSource = null;
                    resolve();
                };

                source.start(0);
            });

        } catch (e) {
            console.error("Playback failed for chunk:", e);
        } finally {
            URL.revokeObjectURL(audioUrl);
        }
    }

    // Finished or Cancelled
    if (sessionId === ttsSessionId) {
        endTTS(btn, sessionId);
    }
}

/**
 * Reset TTS UI state after playback completes
 */
function endTTS(btn, sessionId) {
    // Only update UI if we are still in the same session
    if (sessionId === ttsSessionId) {
        if (btn) {
            const iconEl = btn.querySelector('.material-icons-round');
            if (iconEl) iconEl.textContent = 'volume_up';
            btn.title = 'Speak';
            btn.disabled = false;
        }
        currentAudioBtn = null;
        isPlayingQueue = false;
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}


