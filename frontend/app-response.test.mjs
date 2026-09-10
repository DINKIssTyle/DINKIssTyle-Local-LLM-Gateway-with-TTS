import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';

globalThis.window = globalThis;
await import('./app-chat-streaming.js');
await import('./app-tts.js');
await import('./supertonic3.js');

const { appendStreamDelta } = globalThis.DKSTChatStreaming.createChatStreamingController();
const { cleanTextForTTS } = globalThis.DKSTTTS.createTTSController();

test('delta assembly preserves repeated characters, spaces and overlapping phrases', () => {
    for (const parts of [['하', '하', '하'], ['10', '0'], ['a ', ' ', 'b'], ['반복되는 문장입니다.', '반복되는 문장입니다.']]) {
        assert.equal(parts.reduce(appendStreamDelta, ''), parts.join(''));
    }
});

test('replayed SSE/poll events do not update the UI twice', () => {
    const source = readFileSync(new URL('./app.js', import.meta.url), 'utf8');
    const start = source.indexOf('function applyCurrentChatSessionEvent(');
    const end = source.indexOf('\nasync function ', start);
    // Execute the production handler; an old event must return before touching UI dependencies.
    const context = { AppState: { session: { eventSeq: 42 } } };
    runInNewContext(source.slice(start, end), context);
    for (const EventSeq of [41, 42]) {
        context.applyCurrentChatSessionEvent({ EventSeq, EventType: 'message.delta', PayloadJSON: '{"content":"하"}' });
        context.applyCurrentChatSessionEvent({ EventSeq, EventType: 'tool_call.success' });
    }
});

test('TTS omits complete and unfinished reasoning before stripping HTML', () => {
    assert.equal(cleanTextForTTS('<think>비공개 추론입니다.</think><b>최종 답변입니다.</b>'), '최종 답변입니다.');
    assert.equal(cleanTextForTTS('<THINK>아직 추론 중입니다.'), '');
    assert.equal(cleanTextForTTS('답변입니다. <think>추론 중'), '답변입니다.');
});

test('streaming TTS keeps reasoning silent across multiple sentence chunks', () => {
    const state = { streamingTTSActive: true, streamingTTSCommittedIndex: 0 };
    const controller = globalThis.DKSTTTS.createTTSController({ deps: {
        getPlaybackState: () => state,
        setPlaybackState: (patch) => Object.assign(state, patch)
    } });
    controller.feedStreamingTTS('<think>첫 번째 추론입니다. ');
    controller.feedStreamingTTS('<think>첫 번째 추론입니다. 두 번째 추론입니다. ');
    assert.equal(state.streamingTTSCommittedIndex, 0);
});

test('server completion immediately replaces provisional text and source links', () => {
    const source = readFileSync(new URL('./app.js', import.meta.url), 'utf8');
    const start = source.indexOf('function applyServerCompletion(');
    const end = source.indexOf('\nfunction finalizeStream(', start);
    const rendered = [], saved = [];
    const context = {
        AppState: { session: { replay: { messageBuffers: new Map() } } },
        finalizeMessageContent: (id, text) => rendered.push(text),
        upsertChatMessageState: message => saved.push(message),
        setAssistantActionBarReady: () => {}
    };
    runInNewContext(source.slice(start, end), context);
    const ctx = { elementId: 'answer', turnId: 'turn', fullText: 'provisional URL' };
    const final = '답변\n\n[출처](https://example.com/correct)';
    context.applyServerCompletion({ final_assistant_content: final }, ctx);
    assert.equal(ctx.fullText, final);
    assert.equal(rendered.at(-1), final);
    assert.equal(saved.at(-1).content, final);
    context.applyServerCompletion({ final_assistant_content: '' }, ctx);
    assert.equal(rendered.at(-1), '');
    assert.equal(ctx.fullText, '');
});

test('reconnect probe uses server liveness rather than upstream model health', async () => {
    const source = readFileSync(new URL('./app.js', import.meta.url), 'utf8');
    const start = source.indexOf('async function probeServerReachability(');
    const end = source.indexOf('\nfunction ', start);
    const requests = [];
    const context = {
        fetch: async url => { requests.push(url); return { ok: true }; }
    };
    runInNewContext(source.slice(start, end), context);
    assert.equal(await context.probeServerReachability(), true);
    assert.deepEqual(requests, ['/api/health/live']);
});
