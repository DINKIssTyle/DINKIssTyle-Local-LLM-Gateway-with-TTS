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

test('chunkSpeechText breaks text into sentences under max length and language defaults', () => {
    const text = '첫 번째 문장입니다. 두 번째 문장입니다! 세 번째 질문인가요?';
    const chunks = chunkSpeechText(text, 100);
    assert.ok(chunks.length >= 1);
    assert.equal(chunks.join(' '), text);

    const koChunks = chunkSpeechText(text, 'ko');
    assert.ok(koChunks.length >= 1);
    assert.ok(koChunks.every(c => c.length <= 120));

    const enText = 'This is the first sentence. This is the second sentence. Is this the third sentence?';
    const enChunks = chunkSpeechText(enText, 'en');
    assert.ok(enChunks.length >= 1);
    assert.ok(enChunks.every(c => c.length <= 300));
});

test('streaming boundary extractor extracts complete sentences without cumulative repetition', () => {
    const tokens = ['안', '녕하', '세요! ', '오늘 ', '날씨가 ', '좋네요. ', '좋은 ', '하루 ', '되세요.'];
    let cumulative = '';
    let committedIndex = 0;
    const extractedChunks = [];

    const sentenceRegex = /([\s\S]+?[.!?。！？\n])(?:\s+|$)/g;

    for (const token of tokens) {
        cumulative += token;
        if (cumulative.length <= committedIndex) continue;

        const uncommitted = cumulative.slice(committedIndex);
        sentenceRegex.lastIndex = 0;
        let match;
        let lastMatchedEnd = 0;

        while ((match = sentenceRegex.exec(uncommitted)) !== null) {
            extractedChunks.push(match[1].trim());
            lastMatchedEnd = sentenceRegex.lastIndex;
        }

        if (lastMatchedEnd > 0) {
            committedIndex += lastMatchedEnd;
        }
    }

    // After stream finishes:
    if (cumulative.length > committedIndex) {
        extractedChunks.push(cumulative.slice(committedIndex).trim());
    }

    assert.deepEqual(extractedChunks, [
        '안녕하세요!',
        '오늘 날씨가 좋네요.',
        '좋은 하루 되세요.'
    ]);
});
