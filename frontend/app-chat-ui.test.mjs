import assert from 'node:assert/strict';
import test from 'node:test';

function createMockChatEnvironment({ scrollHeight = 2400, clientHeight = 600, initialScrollTop = 0, streamingScrollMode = 'label-top' } = {}) {
    const rafQueue = new Map();
    let nextRafId = 1;
    const listeners = new Map();

    const chatMessages = {
        scrollTop: initialScrollTop,
        scrollHeight,
        clientHeight,
        classList: {
            _set: new Set(),
            add(name) { this._set.add(name); },
            remove(name) { this._set.delete(name); },
            contains(name) { return this._set.has(name); },
            toggle(name, force) {
                if (force) this._set.add(name);
                else this._set.delete(name);
            }
        },
        style: {
            setProperty() {},
            removeProperty() {}
        },
        addEventListener(type, fn) {
            if (!listeners.has(type)) listeners.set(type, []);
            listeners.get(type).push(fn);
        },
        querySelectorAll() { return []; }
    };

    globalThis.window = globalThis;
    globalThis.document = {
        activeElement: null,
        getElementById() { return null; }
    };
    globalThis.matchMedia = () => ({ matches: false });
    globalThis.requestAnimationFrame = (cb) => {
        const id = nextRafId++;
        rafQueue.set(id, cb);
        return id;
    };
    globalThis.cancelAnimationFrame = (id) => {
        rafQueue.delete(id);
    };

    const flushAnimationFrames = (maxSteps = 40) => {
        let steps = 0;
        while (rafQueue.size > 0 && steps < maxSteps) {
            const entries = Array.from(rafQueue.entries());
            rafQueue.clear();
            for (const [, cb] of entries) {
                cb();
            }
            steps += 1;
        }
    };

    const refreshChatScrollMetrics = () => {
        const distanceFromBottom = Math.max(0, chatMessages.scrollHeight - chatMessages.clientHeight - chatMessages.scrollTop);
        return {
            scrollTop: chatMessages.scrollTop,
            scrollHeight: chatMessages.scrollHeight,
            clientHeight: chatMessages.clientHeight,
            distanceFromBottom,
            nearBottom: distanceFromBottom <= 80,
            longScrollable: (chatMessages.scrollHeight - chatMessages.clientHeight) > 120
        };
    };

    return {
        chatMessages,
        flushAnimationFrames,
        deps: {
            getStreamingScrollMode: () => streamingScrollMode,
            refreshChatScrollMetrics,
            commitChatScrollMetrics() {},
            updateReasoningControlVisibility() {}
        }
    };
}

globalThis.window = globalThis;
await import('./app-chat-ui.js');

test('scrollToBottom(true, { immediate: true }) snaps to bottom in label-top mode without enabling auto-scroll', () => {
    const env = createMockChatEnvironment({ scrollHeight: 2400, clientHeight: 600, initialScrollTop: 0, streamingScrollMode: 'label-top' });
    const controller = globalThis.DKSTChatUI.createChatUIController({
        refs: { chatMessages: env.chatMessages },
        deps: env.deps
    });

    controller.scrollToBottom(true, { immediate: true });
    assert.equal(env.chatMessages.scrollTop, 1800);

    // Subsequent non-forced scrollToBottom in label-top mode should not move scrollTop
    env.chatMessages.scrollHeight = 3000;
    controller.scrollToBottom(false);
    env.flushAnimationFrames();
    assert.equal(env.chatMessages.scrollTop, 1800);
});

test('scrollToBottom(true) completes animated scroll to bottom in label-top mode', () => {
    const env = createMockChatEnvironment({ scrollHeight: 2400, clientHeight: 600, initialScrollTop: 0, streamingScrollMode: 'label-top' });
    const controller = globalThis.DKSTChatUI.createChatUIController({
        refs: { chatMessages: env.chatMessages },
        deps: env.deps
    });

    controller.scrollToBottom(true);
    env.flushAnimationFrames();
    assert.ok(Math.abs(env.chatMessages.scrollTop - 1800) <= 1, `expected scrollTop near 1800, got ${env.chatMessages.scrollTop}`);
});

test('splitAssistantSources separates trailing Korean and English source lists cleanly', async () => {
    await import('./app-chat-streaming.js');
    const { splitAssistantSources } = globalThis.DKSTChatStreaming;

    const input = [
        '리센느는 5인조 걸그룹입니다.',
        '',
        '검색 출처:',
        '- [리센느 \\[공식\\] 프로필](https://example.com/rescene)',
        '- [위키](https://example.com/wiki)'
    ].join('\n');
    const parsed = splitAssistantSources(input);
    assert.equal(parsed.bodyText, '리센느는 5인조 걸그룹입니다.');
    assert.deepEqual(parsed.sources, [
        { title: '리센느 [공식] 프로필', url: 'https://example.com/rescene' },
        { title: '위키', url: 'https://example.com/wiki' }
    ]);

    const prose = '출처를 확인해보니 2024년 데뷔입니다.';
    const parsedProse = splitAssistantSources(prose);
    assert.equal(parsedProse.bodyText, prose);
    assert.deepEqual(parsedProse.sources, []);
});
