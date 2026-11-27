import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./app-mic.js', import.meta.url), 'utf8');
const flush = async () => { for (let i = 0; i < 15; i++) await Promise.resolve(); };
function setup(settings = {}) {
    const timers = new Map();
    const recognitions = [];
    const contexts = [];
    const tracks = [];
    const requests = [];
    const alerts = [];
    const sent = [];
    let timerID = 0;
    class Recognition {
        constructor() { recognitions.push(this); }
        start() { this.onstart?.(); }
        abort() { this.onend?.(); }
        result(text) { this.onresult({ resultIndex: 0, results: [{ isFinal: true, 0: { transcript: text } }] }); }
    }
    class AudioContext {
        constructor() { this.sampleRate = 16000; this.state = 'running'; contexts.push(this); }
        async resume() {}
        async close() { this.state = 'closed'; }
        createMediaStreamSource() { return { connect() {}, disconnect() {} }; }
        createScriptProcessor() { return this.processor = { connect() {}, disconnect() {} }; }
    }
    const classes = { toggle() {}, add() {}, remove() {} };
    const input = { value: '', classList: classes, dispatchEvent() {} };
    const state = { input: {}, chat: {}, audio: {} };
    const config = { ttsLang: 'ko', sttMethod: 'browser', sttContinuous: false, ...settings };
    const window = {
        SpeechRecognition: Recognition, AudioContext, document: { body: { classList: classes } },
        navigator: { mediaDevices: { async getUserMedia() {
            const track = { stopped: false, stop() { this.stopped = true; } }; tracks.push(track);
            return { getTracks: () => [track] };
        } } },
        async fetch(url, options) {
            requests.push({ url, options });
            return { ok: true, json: async () => url.includes('/status') ? { status: 'ready', ...(url.includes('qwen3-asr') ? { modelId: 'qwen3-asr-0.6b-mlx' } : {}) } : { text: '안녕하세요' } };
        },
        alert: text => alerts.push(text),
        setTimeout(fn) { timers.set(++timerID, fn); return timerID; },
        clearTimeout(id) { timers.delete(id); },
        setInterval() { return ++timerID; }, clearInterval() {},
        btoa: text => Buffer.from(text, 'binary').toString('base64')
    };
    vm.runInNewContext(source, { window, console, Event, AbortController });
    const controller = window.DKSTMic.createMicController({ refs: { messageInput: input }, deps: {
        AppState: state, config, t: key => key,
        async sendMessage(options) { sent.push({ text: input.value, options }); input.value = ''; }
    } });
    const tick = async () => { const current = [...timers.values()]; timers.clear(); for (const fn of current) fn(); await flush(); };
    return { controller, state, config, window, contexts, recognitions, tracks, requests, alerts, sent, input, tick, timers };
}

test('browser final text is sent once with voice-input metadata', async () => {
    const h = setup();
    h.controller.start();
    h.recognitions[0].result('테스트');
    await h.recognitions[0].onend();
    assert.equal(h.sent.length, 1);
    assert.equal(h.sent[0].text, '테스트');
    assert.equal(h.sent[0].options.fromVoiceInput, true);
    assert.equal(h.state.input.isSTTActive, false);
});

test('continuous browser listening waits for generation and playback, then resumes', async () => {
    const h = setup({ sttContinuous: true });
    h.controller.start();
    h.recognitions[0].result('질문');
    await h.recognitions[0].onend();
    h.state.chat.isGenerating = true;
    await h.tick();
    assert.equal(h.recognitions.length, 1);
    h.state.chat.isGenerating = false;
    h.state.audio.isPlayingQueue = true;
    await h.tick();
    assert.equal(h.recognitions.length, 1);
    h.state.audio.isPlayingQueue = false;
    await h.tick();
    assert.equal(h.recognitions.length, 2);
    h.controller.stop({ suppressAutoSend: true });
    assert.equal(h.timers.size, 0);
    assert.equal(h.sent.length, 1);
});

test('stopping ignores late recognition events and cancels automatic sending', async () => {
    const h = setup({ sttContinuous: true });
    h.controller.start();
    const recognition = h.recognitions[0];
    recognition.result('취소');
    h.controller.stop({ suppressAutoSend: true, forceAbort: true });
    recognition.result('늦은 이벤트');
    await recognition.onend();
    assert.equal(h.input.value, '취소');
    assert.equal(h.sent.length, 0);
    assert.equal(h.timers.size, 0);
});

test('local Whisper sends PCM16 at 16 kHz after speech and silence, releasing microphone', async () => {
    const h = setup({ sttMethod: 'local' });
    h.controller.start();
    await flush();
    const capture = h.contexts[0];
    const process = capture.processor.onaudioprocess;
    const frame = value => ({ inputBuffer: { getChannelData: () => new Float32Array(4096).fill(value) } });
    process(frame(0.1));
    for (let i = 0; i < 4; i++) process(frame(0));
    await flush();
    const request = h.requests.find(r => r.url.endsWith('/transcribe'));
    const payload = JSON.parse(request.options.body);
    assert.equal(payload.format, 'pcm');
    assert.equal(payload.sampleRate, 16000);
    assert.equal(payload.language, 'ko');
    assert.equal(Buffer.from(payload.audio, 'base64').readInt16LE(0), 3276);
    assert.equal(h.tracks[0].stopped, true);
    assert.equal(capture.state, 'closed');
    assert.equal(h.sent.length, 1);
    assert.equal(h.sent[0].text, '안녕하세요');
});

test('permission resolving after stop releases the stream and never starts recording', async () => {
    const h = setup({ sttMethod: 'local' });
    let resolvePermission;
    let stopped = false;
    h.window.navigator.mediaDevices.getUserMedia = () => new Promise(resolve => { resolvePermission = resolve; });
    h.controller.start();
    await flush();
    h.controller.stop({ suppressAutoSend: true });
    resolvePermission({ getTracks: () => [{ stop() { stopped = true; } }] });
    await flush();
    assert.equal(stopped, true);
    assert.equal(h.contexts[0].state, 'closed');
    assert.equal(h.contexts[0].processor, undefined);
});

test('missing Whisper assets stop listening and explain how to install them', async () => {
    const h = setup({ sttMethod: 'local' });
    h.window.fetch = async () => ({ ok: true, json: async () => ({ status: 'missing' }) });
    h.controller.start();
    await flush();
    assert.equal(h.state.input.isSTTActive, false);
    assert.match(h.alerts[0], /sttModelMissing/);
    assert.equal(h.contexts[0].state, 'closed');
    assert.equal(h.tracks.length, 0);
});

test('stopping local inference aborts its request and ignores a late transcript', async () => {
    const h = setup({ sttMethod: 'local', sttContinuous: true });
    let resolveTranscript;
    h.window.fetch = async (url, options) => {
        h.requests.push({ url, options });
        if (url.endsWith('/status')) return { ok: true, json: async () => ({ status: 'ready' }) };
        return new Promise(resolve => { resolveTranscript = resolve; });
    };
    h.controller.start(); await flush();
    const process = h.contexts[0].processor.onaudioprocess;
    process({ inputBuffer: { getChannelData: () => new Float32Array(4096).fill(0.1) } });
    for (let i = 0; i < 4; i++) process({ inputBuffer: { getChannelData: () => new Float32Array(4096) } });
    h.controller.stop({ suppressAutoSend: true, forceAbort: true });
    assert.equal(h.requests.at(-1).options.signal.aborted, true);
    resolveTranscript({ ok: true, json: async () => ({ text: '늦은 결과' }) });
    await flush();
    assert.equal(h.sent.length, 0);
    assert.equal(h.input.value, '');
    assert.equal(h.timers.size, 0);
});

test('continuous Whisper ignores playback audio and reuses capture for the next utterance', async () => {
    const h = setup({ sttMethod: 'local', sttContinuous: true });
    h.controller.start(); await flush();
    const process = h.contexts[0].processor.onaudioprocess;
    const frame = value => ({ inputBuffer: { getChannelData: () => new Float32Array(4096).fill(value) } });
    const utterance = () => { process(frame(0.1)); for (let i = 0; i < 4; i++) process(frame(0)); };
    h.state.audio.isPlayingQueue = true;
    utterance(); await flush();
    assert.equal(h.sent.length, 0);
    h.state.audio.isPlayingQueue = false;
    utterance(); await flush();
    assert.equal(h.sent.length, 1);
    assert.equal(h.tracks[0].stopped, false);
    await h.tick();
    assert.equal(h.contexts.length, 1);
    utterance(); await flush();
    assert.equal(h.sent.length, 2);
    h.controller.stop({ suppressAutoSend: true });
    assert.equal(h.tracks[0].stopped, true);
    assert.equal(h.contexts[0].state, 'closed');
});

test('Qwen3-ASR uses its own status and PCM endpoint in single and continuous modes', async () => {
    for (const continuous of [false, true]) {
        const h = setup({ sttMethod: 'qwen3-asr', sttContinuous: continuous });
        h.controller.start(); await flush();
        assert.equal(h.requests[0].url, '/api/stt/status?engine=qwen3-asr');
        assert.equal(h.recognitions.length, 0);
        const process = h.contexts[0].processor.onaudioprocess;
        const frame = value => ({ inputBuffer: { getChannelData: () => new Float32Array(4096).fill(value) } });
        const utterance = () => { process(frame(0.1)); for (let i=0;i<4;i++) process(frame(0)); };
        utterance(); await flush();
        const payload = JSON.parse(h.requests.find(r=>r.url.endsWith('/transcribe')).options.body);
        assert.equal(payload.engine, 'qwen3-asr');
        assert.equal(payload.fallback, false);
        assert.equal(payload.sampleRate, 16000);
        assert.equal(payload.format, 'pcm');
        assert.equal(payload.language, 'ko');
        assert.equal(Buffer.from(payload.audio,'base64').readInt16LE(0),3276);
        assert.equal(h.sent[0].text, '안녕하세요');
        assert.equal(h.tracks[0].stopped, !continuous);
        if (continuous) {
            let resolveTranscript;
            h.window.fetch = (url,options) => {
                h.requests.push({url,options});
                return new Promise(resolve=>{resolveTranscript=resolve;});
            };
            utterance(); await flush();
            const request=h.requests.at(-1);
            h.controller.stop({suppressAutoSend:true,forceAbort:true});
            assert.equal(request.options.signal.aborted,true);
            resolveTranscript({ok:true,json:async()=>({text:'늦은 결과'})});
            await flush();
            assert.equal(h.sent.length,1);
            assert.equal(h.tracks[0].stopped,true);
        }
    }
});

for (const [name,status,errorKey] of [
    ['missing assets',{status:'missing',modelId:'qwen3-asr-0.6b-mlx'},'sttModelMissing'],
    ['unsupported host',{status:'unsupported',modelId:'qwen3-asr-0.6b-mlx'},'qwenSTTUnsupported'],
    ['old server returning Whisper',{status:'ready',modelId:'whisper-base'},'qwenSTTUnsupported'],
]) test(`Qwen3-ASR ${name} stops without opening the microphone or falling back`,async()=>{
    const h=setup({sttMethod:'qwen3-asr'});
    h.window.fetch=async()=>({ok:true,json:async()=>status});
    h.controller.start();await flush();
    assert.equal(h.state.input.isSTTActive,false);
    assert.equal(h.tracks.length,0);
    assert.equal(h.contexts[0].state,'closed');
    assert.match(h.alerts[0],new RegExp(errorKey));
    assert.equal(h.recognitions.length,0);
});

test('continuous STT shows speaking placeholder during TTS playback and live interim text while listening', async () => {
    const intervals = [];
    const h = setup({ sttContinuous: true });
    h.window.setInterval = (fn) => { intervals.push(fn); return intervals.length; };

    h.controller.start();
    assert.match(h.input.placeholder, /^input\.listening\./);

    // Browser interim transcript appears immediately in messageInput while speaking
    h.recognitions[0].onresult({
        resultIndex: 0,
        results: [{ isFinal: false, 0: { transcript: '오늘 날씨' } }]
    });
    assert.equal(h.input.value, '오늘 날씨');

    // Final transcript replaces interim cleanly and sends on end
    h.recognitions[0].onresult({
        resultIndex: 0,
        results: [{ isFinal: true, 0: { transcript: '오늘 날씨 알려줘' } }]
    });
    assert.equal(h.input.value, '오늘 날씨 알려줘');
    await h.recognitions[0].onend();
    assert.equal(h.sent.length, 1);
    assert.equal(h.sent[0].text, '오늘 날씨 알려줘');

    // While TTS plays back, placeholder switches to speaking
    h.state.audio.isPlayingQueue = true;
    intervals[0]();
    assert.match(h.input.placeholder, /^input\.speaking\./);

    // Once TTS finishes, placeholder switches back to listening
    h.state.audio.isPlayingQueue = false;
    intervals[0]();
    assert.match(h.input.placeholder, /^input\.listening\./);
});

test('local STT displays live /api/stt/preview text in messageInput during speech before final transcribe', async () => {
    const h = setup({ sttMethod: 'local', sttContinuous: false });
    h.window.fetch = async (url, options) => {
        h.requests.push({ url, options });
        if (url.includes('/status')) return { ok: true, json: async () => ({ status: 'ready' }) };
        if (url.includes('/preview')) return { ok: true, json: async () => ({ text: '지금 말하고 있는' }) };
        return { ok: true, json: async () => ({ text: '지금 말하고 있는 문장입니다' }) };
    };
    h.controller.start();
    await flush();
    const process = h.contexts[0].processor.onaudioprocess;
    const frame = value => ({ inputBuffer: { getChannelData: () => new Float32Array(4096).fill(value) } });

    // 4 voiced frames = 16384 samples (~1.02s >= 0.8s threshold) -> triggers /api/stt/preview
    for (let i = 0; i < 4; i++) process(frame(0.1));
    await flush();
    assert.ok(h.requests.some(r => r.url.endsWith('/preview')));
    assert.equal(h.input.value, '지금 말하고 있는');

    // Trailing silence -> triggers /api/stt/transcribe and replaces preview cleanly without duplication
    for (let i = 0; i < 4; i++) process(frame(0));
    await flush();
    assert.equal(h.sent.length, 1);
    assert.equal(h.sent[0].text, '지금 말하고 있는 문장입니다');
});
