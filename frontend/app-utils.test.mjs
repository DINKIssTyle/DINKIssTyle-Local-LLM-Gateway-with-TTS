import assert from 'node:assert/strict';
import test from 'node:test';

globalThis.window = globalThis;
await import('./app-utils.js');

const { normalizeMarkdownForRender, selectRetrievalConversationContext } = globalThis.DKSTAppUtils;

test('repairs punctuation-wrapped strong emphasis next to a Korean particle', () => {
    const source = "- **모델 공개**: **'Qwen3.8-Max'**를 공개했습니다.";
    const normalized = normalizeMarkdownForRender(source);
    assert.equal(normalized, "- **모델 공개**: <strong>'Qwen3.8-Max'</strong>를 공개했습니다.");
});

test('does not rewrite punctuation-like emphasis inside code fences', () => {
    const source = "```md\n**'Qwen3.8-Max'**를\n```";
    assert.equal(normalizeMarkdownForRender(source), source);
});

test('keeps ordinary strong emphasis as markdown', () => {
    const source = '**모델 공개**를 확인했습니다.';
    assert.equal(normalizeMarkdownForRender(source), source);
});

test('injects the previous completed user-assistant turn for retrieval follow-ups', () => {
    const messages = [
        { role: 'user', content: '네팔 홍수', turnId: 'turn-1' },
        { role: 'assistant', content: '네팔 라수와 지역에서 대규모 홍수가 발생했습니다.', turnId: 'turn-1' },
        { role: 'user', content: '각국의 반응은?', turnId: 'turn-2' }
    ];

    assert.deepEqual(selectRetrievalConversationContext(messages), messages);
});

test('does not inject an incomplete assistant placeholder into retrieval context', () => {
    const currentUser = { role: 'user', content: '각국의 반응은?', turnId: 'turn-2' };
    const messages = [
        { role: 'user', content: '네팔 홍수', turnId: 'turn-1' },
        { role: 'assistant', content: '', turnId: 'turn-1' },
        currentUser
    ];

    assert.deepEqual(selectRetrievalConversationContext(messages), [messages[0], currentUser]);
});

test('uses only the immediately preceding completed turn in retrieval context', () => {
    const messages = [
        { role: 'user', content: '이전 주제', turnId: 'turn-0' },
        { role: 'assistant', content: '이전 답변', turnId: 'turn-0' },
        { role: 'user', content: '네팔 홍수', turnId: 'turn-1' },
        { role: 'assistant', content: '네팔 홍수 답변', turnId: 'turn-1' },
        { role: 'user', content: '각국의 반응은?', turnId: 'turn-2' }
    ];

    assert.deepEqual(
        selectRetrievalConversationContext(messages).map((message) => message.content),
        ['네팔 홍수', '네팔 홍수 답변', '각국의 반응은?']
    );
});

test('does not emit an orphan assistant message when its user turn is unavailable', () => {
    const currentUser = { role: 'user', content: '현재 질문', turnId: 'turn-2' };
    const messages = [
        { role: 'assistant', content: '복원된 답변', turnId: 'missing-turn' },
        currentUser
    ];

    assert.deepEqual(selectRetrievalConversationContext(messages), [currentUser]);
});

test('retry context never jumps over the most recent failed request', () => {
    const messages = [
        { role: 'user', content: '미국 뉴스', turnId: 'a' },
        { role: 'assistant', content: '뉴스 답변', turnId: 'a' },
        { role: 'user', content: '루멘텀 회사 설명', turnId: 'b' },
        { role: 'assistant', content: '', turnId: 'b' },
        { role: 'user', content: '다시 시도', turnId: 'c' }
    ];
    assert.deepEqual(selectRetrievalConversationContext(messages).map(m => m.content), ['루멘텀 회사 설명', '다시 시도']);
});

test('keeps article title pipes, brackets and URL punctuation inside markdown links', () => {
    for (const source of [
        '- [LG·SKT·업스테이지 독파모 3파전 | 연합뉴스](https://example.com/a)',
        '- [보고서 1. - 최신](https://example.com/a?q=1#section)',
        '- [\\[특징주\\] 뉴스 | 매체](https://example.com/a_(b))'
    ]) assert.equal(normalizeMarkdownForRender(source), source);
});
