import test from 'node:test';
import assert from 'node:assert/strict';
import './supertonic3.js';

const {
    SUPERTONIC3_VOICES,
    boundaryPause,
    cleanSpeechText,
    chunkSpeechText
} = globalThis.DKSTSupertonic3;

test('SUPERTONIC3_VOICES contains 10 male and female voices', () => {
    assert.equal(SUPERTONIC3_VOICES.length, 10);
    assert.equal(SUPERTONIC3_VOICES[0].id, 'M1');
    assert.equal(SUPERTONIC3_VOICES[5].id, 'F1');
});

test('boundaryPause assigns appropriate pauses for punctuation', () => {
    assert.equal(boundaryPause('정말인가요?'), 0.28);
    assert.equal(boundaryPause('반갑습니다!'), 0.24);
    assert.equal(boundaryPause('오늘 날씨가 좋습니다.'), 0.22);
    assert.equal(boundaryPause('그리고;'), 0.16);
    assert.equal(boundaryPause('다음:'), 0.13);
    assert.equal(boundaryPause('사과, 배, 포도,'), 0.09);
    assert.equal(boundaryPause('일반 문장'), 0.08);
});

test('cleanSpeechText strips URLs, citations, and extra whitespace', () => {
    const input = '참고 문서는 https://example.com/docs 이며 [1] 또는 [출처 필요] 입니다.';
    const cleaned = cleanSpeechText(input);
    assert.match(cleaned, /참고 문서는/);
    assert.ok(!cleaned.includes('https://example.com/docs'));
    assert.ok(!cleaned.includes('[1]'));
    assert.ok(!cleaned.includes('[출처 필요]'));
});

test('chunkSpeechText breaks text into sentences under max length', () => {
    const text = '첫 번째 문장입니다. 두 번째 문장입니다! 세 번째 질문인가요?';
    const chunks = chunkSpeechText(text, 100);
    assert.ok(chunks.length >= 1);
    assert.equal(chunks.join(' '), text);
});
