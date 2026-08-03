import assert from 'node:assert/strict';
import test from 'node:test';

globalThis.window = globalThis;
await import('./app-utils.js');

const { normalizeMarkdownForRender } = globalThis.DKSTAppUtils;

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
