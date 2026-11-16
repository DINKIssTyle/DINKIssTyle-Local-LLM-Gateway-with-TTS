import assert from 'node:assert/strict';
import { getTextWidth } from '@evenrealities/pretext';
import { rightAlignedStatusText, modeHeaderText, translationHeaderText, hudNoticeText, stripToolTagsForGlasses, stripSourcesForGlasses } from './hud-status.mjs';
const base = { source: 'en', target: 'ko', running: true };
const prompt = translationHeaderText({ ...base, phase: '프롬프트 처리 중 (32%)' });
assert.equal(prompt, '[번역] EN→KO ● | 프롬프트 처리 32%');
assert.equal(hudNoticeText(prompt), prompt, 'percentage must fit without truncation');
assert.ok(getTextWidth(prompt) <= 396);
for (const phase of ['생각 중.', '생각 중..', '생각 중...', '답변 작성 중', '프롬프트 처리 중 (100%)']) {
    assert.ok(translationHeaderText({ ...base, phase }).startsWith('[번역] EN→KO ● | '));
}
assert.equal(translationHeaderText({ ...base, phase: '생각 중.' }), translationHeaderText({ ...base, phase: '생각 중...' }), 'animated dots must not move the header');
assert.equal(translationHeaderText({ ...base, running: false }), '[번역] EN→KO ○ | -');
assert.equal(translationHeaderText({ ...base, phase: '프롬프트 처리 중' }), '[번역] EN→KO ● | 프롬프트 처리');
assert.equal(translationHeaderText({ ...base, error: '오류' }), '[번역] EN→KO ● | 오류');
assert.equal(translationHeaderText({ ...base, source: 'ko', target: 'en', translating: true }), '[번역] KO→EN ● | 번역 처리');
console.log('PASS: fixed translation header fields, actual prompt percentages, no animated dots, native pixel width');

for (const mode of ['transcript', 'ai', 'translate']) {
    const header = modeHeaderText({ mode, ...base, phase: '프롬프트 처리 중 (32%)' });
    assert.match(header, / ● \| 프롬프트 처리 32%$/);
    assert.equal(hudNoticeText(header), header);
    assert.match(modeHeaderText({ mode, ...base }), / ● \| 듣는중$/);
    assert.match(modeHeaderText({ mode }), / ○ \| -$/);
}
assert.equal(modeHeaderText({ mode: 'ai', running: true, trigger: '이븐' }), '[AI ▶이븐] ● | 듣는중');
assert.equal(modeHeaderText({ mode: 'ai', running: true, phase: '웹에서 근거 찾는 중' }), '[AI] ● | 웹검색중');
assert.equal(modeHeaderText({ mode: 'ai', running: true, phase: '생각 중 (4초)' }), '[AI] ● | 생각중 4초');
assert.equal(modeHeaderText({ mode: 'ai', running: true, phase: '웹 검색 중', phaseDetail: '검색어: 맥북 에어 M4' }), '[AI] ● | 웹검색중 (맥북 에어 M4)');
assert.equal(modeHeaderText({ mode: 'ai', running: true, phase: '메모리 검색 중', phaseDetail: '메모리 검색: 회의' }), '[AI] ● | 기억 조회중 (회의)');
assert.equal(modeHeaderText({ mode: 'ai', running: true, phase: '현재 시간을 확인했습니다.' }), '[AI] ● | 시간 확인');
assert.equal(modeHeaderText({ mode: 'ai', running: true, phase: '도구 실행이 완료되었습니다.' }), '[AI] ● | 도구 완료');
assert.equal(modeHeaderText({ mode: 'ai', hint: false, running: true }), '[AI] ● | 듣는중');

for (const battery of [0, 9, 75, 100, undefined]) {
    const date = new Date(2026, 10, 13, 16, 14), text = rightAlignedStatusText(battery, date);
    assert.ok(text.startsWith(' '));
    assert.ok(getTextWidth(text) <= 174);
    assert.ok(174 - getTextWidth(text) < getTextWidth(' ') + .01, 'right edge must remain within one native space glyph');
    assert.match(text.trim(), /^16:14 \| /);
}

assert.equal(stripToolTagsForGlasses('<tool_call>{"name":"search_web","arguments":{"query":"날씨"}}</tool_call>서울은 맑음입니다.'), '서울은 맑음입니다.');
assert.equal(stripToolTagsForGlasses('<search_web>{"query":"환율"}</search_web>'), '');
assert.equal(stripToolTagsForGlasses('잠시 확인 중<tool_call><function=read_web_page><parameter=url>https://a.b'), '잠시 확인 중');
assert.equal(stripSourcesForGlasses('<|call:search_web|>{"query":"test"}<|end|>결과는 10입니다.'), '결과는 10입니다.');
