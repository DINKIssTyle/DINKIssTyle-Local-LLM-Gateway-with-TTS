import assert from 'node:assert/strict';
import {getTextWidth} from '@evenrealities/pretext';
import {rightAlignedStatusText,modeHeaderText,translationHeaderText,hudNoticeText} from './hud-status.mjs';
const base={source:'en',target:'ko',running:true};
const prompt=translationHeaderText({...base,phase:'프롬프트 처리 중 (32%)'});
assert.equal(prompt,'[번역] EN→KO | 작동중 | 프롬프트 처리 32%');
assert.equal(hudNoticeText(prompt),prompt,'percentage must fit without truncation');
assert.ok(getTextWidth(prompt)<=396);
for(const phase of ['생각 중.','생각 중..','생각 중...','답변 작성 중','프롬프트 처리 중 (100%)']){
 assert.ok(translationHeaderText({...base,phase}).startsWith('[번역] EN→KO | 작동중 | '));
}
assert.equal(translationHeaderText({...base,phase:'생각 중.'}),translationHeaderText({...base,phase:'생각 중...'}),'animated dots must not move the header');
assert.equal(translationHeaderText({...base,running:false}),'[번역] EN→KO | 대기중 | -');
assert.equal(translationHeaderText({...base,phase:'프롬프트 처리 중'}),'[번역] EN→KO | 작동중 | 프롬프트 처리');
assert.equal(translationHeaderText({...base,error:'오류'}),'[번역] EN→KO | 작동중 | 오류');
assert.equal(translationHeaderText({...base,source:'ko',target:'en',translating:true}),'[번역] KO→EN | 작동중 | 번역 처리');
console.log('PASS: fixed translation header fields, actual prompt percentages, no animated dots, native pixel width');

for(const mode of ['transcript','ai','translate']){
 const header=modeHeaderText({mode,...base,phase:'프롬프트 처리 중 (32%)'});
 assert.match(header,/ \| 작동중 \| 프롬프트 처리 32%$/);
 assert.equal(hudNoticeText(header),header);
 assert.match(modeHeaderText({mode,...base}),/ \| 작동중 \| 듣는중$/);
 assert.match(modeHeaderText({mode}),/ \| 대기중 \| -$/);
}
assert.equal(modeHeaderText({mode:'ai',running:true,phase:'웹에서 근거 찾는 중'}),'[AI] | 작동중 | 웹검색');
assert.equal(modeHeaderText({mode:'ai',hint:false,running:true}),'[AI] | 작동중 | 듣는중');

for(const battery of [0,9,75,100,undefined]){
 const date=new Date(2026,10,13,16,14),text=rightAlignedStatusText(battery,date);
 assert.ok(text.startsWith(' '));
 assert.ok(getTextWidth(text)<=174);
 assert.ok(174-getTextWidth(text)<getTextWidth(' ')+.01,'right edge must remain within one native space glyph');
 assert.match(text.trim(),/^16:14 \/ /);
}
