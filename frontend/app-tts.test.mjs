import assert from 'node:assert/strict';
import test from 'node:test';

globalThis.window = globalThis;
globalThis.document = globalThis.document || { getElementById: () => null };
await import('./supertonic3.js');
await import('./app-tts.js');

function createServerTTSTestHarness({ fetchImpl, scheduleImpl } = {}) {
    const state = {
        currentAudioBtn: null,
        isPlayingQueue: false,
        streamingTTSActive: false,
        streamingTTSBuffer: '',
        streamingTTSCommittedIndex: 0,
        ttsSessionId: 0
    };
    const requests = [];
    let unlockCalls = 0;

    globalThis.fetch = async (url, options = {}) => {
        requests.push({ url, options, body: JSON.parse(options.body || '{}') });
        if (fetchImpl) return fetchImpl(url, options, requests.length - 1);
        return {
            ok: true,
            status: 200,
            arrayBuffer: async () => new ArrayBuffer(16)
        };
    };

    const audioCtx = {
        state: 'running',
        decodeAudioData: async () => ({ duration: 0.5 })
    };

    const origUnlock = globalThis.DKSTSupertonic3.unlockSupertonic3Audio;
    const origGetCtx = globalThis.DKSTSupertonic3.getSupertonic3AudioContext;
    const origSchedule = globalThis.DKSTSupertonic3.scheduleSupertonic3AudioBuffer;
    const origCancel = globalThis.DKSTSupertonic3.cancelSupertonic3Speech;

    globalThis.DKSTSupertonic3.unlockSupertonic3Audio = async () => { unlockCalls += 1; };
    globalThis.DKSTSupertonic3.getSupertonic3AudioContext = () => audioCtx;
    globalThis.DKSTSupertonic3.scheduleSupertonic3AudioBuffer = (buf, pause) => {
        if (scheduleImpl) return scheduleImpl(buf, pause);
        return Promise.resolve();
    };
    globalThis.DKSTSupertonic3.cancelSupertonic3Speech = () => {};

    const controller = globalThis.DKSTTTS.createTTSController({
        deps: {
            config: {
                enableTTS: true,
                ttsEngine: 'supertonic',
                ttsLang: 'ko',
                ttsVoice: 'F1',
                ttsSpeed: 1.0,
                ttsAudioFormat: 'mp3',
                ttsSteps: 5
            },
            getPlaybackState: () => state,
            setPlaybackState: (patch) => Object.assign(state, patch),
            onSyncCurrentAudioButtonUI: () => {},
            onSyncWakeLock: () => {}
        }
    });

    const cleanup = () => {
        globalThis.DKSTSupertonic3.unlockSupertonic3Audio = origUnlock;
        globalThis.DKSTSupertonic3.getSupertonic3AudioContext = origGetCtx;
        globalThis.DKSTSupertonic3.scheduleSupertonic3AudioBuffer = origSchedule;
        globalThis.DKSTSupertonic3.cancelSupertonic3Speech = origCancel;
    };

    return { controller, state, requests, getUnlockCalls: () => unlockCalls, cleanup };
}

test('server TTS sends scheduler superseding headers, AbortSignal, and uses Korean chunk sizing', async () => {
    const harness = createServerTTSTestHarness();
    try {
        const longKorean = '안녕하세요 반갑습니다 오늘 날씨가 참 좋네요. '.repeat(8).trim();
        await harness.controller.speakMessage(longKorean);

        assert.ok(harness.requests.length >= 2, `expected Korean text to split into >=2 chunks (<=120 chars), got ${harness.requests.length}`);
        for (const req of harness.requests) {
            assert.equal(req.url, '/api/tts');
            assert.equal(req.options.credentials, 'include');
            assert.ok(req.options.signal instanceof AbortSignal);
            assert.ok(req.options.headers['X-TTS-Client-ID']);
            assert.ok(Number(req.options.headers['X-TTS-Session-ID']) > 0);
            assert.ok(Number(req.options.headers['X-TTS-Request-ID']) > 0);
            assert.ok(req.body.text.length <= 120, `expected chunk <= 120 chars, got ${req.body.text.length}`);
        }
        assert.ok(harness.getUnlockCalls() >= 1);
    } finally {
        harness.cleanup();
    }
});

test('stopAllAudio aborts in-flight server TTS fetch', async () => {
    let capturedSignal = null;
    const harness = createServerTTSTestHarness({
        fetchImpl: (_url, options) => new Promise((_resolve, reject) => {
            capturedSignal = options.signal;
            options.signal.addEventListener('abort', () => {
                const err = new Error('Aborted');
                err.name = 'AbortError';
                reject(err);
            }, { once: true });
        })
    });
    try {
        const speakPromise = harness.controller.speakMessage('테스트 문장입니다.');
        await new Promise((r) => setTimeout(r, 15));
        assert.ok(capturedSignal && !capturedSignal.aborted);

        harness.controller.stopAllAudio();
        assert.equal(capturedSignal.aborted, true);
        await speakPromise;
    } finally {
        harness.cleanup();
    }
});

test('streaming server TTS pipelines next sentence fetch while previous sentence is still playing', async () => {
    const resolvers = [];
    const harness = createServerTTSTestHarness({
        scheduleImpl: () => new Promise((resolve) => {
            resolvers.push(resolve);
        })
    });
    try {
        harness.controller.initStreamingTTS();
        harness.controller.feedStreamingTTS('첫 번째 문장입니다. 두 번째 문장입니다. ');
        harness.controller.finalizeStreamingTTS('첫 번째 문장입니다. 두 번째 문장입니다. ');

        // Wait briefly for both fetches to be dispatched WITHOUT resolving playback of sentence 1.
        await new Promise((r) => setTimeout(r, 40));
        assert.equal(harness.requests.length, 2, 'second sentence should be fetched before first sentence finishes playing');
        assert.equal(harness.state.isPlayingQueue, true, 'queue should remain active until scheduled playback completes');

        for (const resolve of resolvers) resolve();
        await new Promise((r) => setTimeout(r, 20));
        assert.equal(harness.state.isPlayingQueue, false, 'queue should finish after last scheduled audio ends');
    } finally {
        harness.cleanup();
    }
});
