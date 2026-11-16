import assert from 'node:assert/strict';
import {validateHintGrounding} from './hint-grounding.mjs';
const source={version:1,mode:'quote',source:'web',topic:'피라미드',records:[{title:'피라미드 건축',url:'https://example.com/pyramids',text:'피라미드 건축에서는 무거운 석재를 옮기는 데 경사로를 활용한 것으로 추정된다.'}]};
const good={fact:source.records[0].text,support_quote:source.records[0].text,topic:'피라미드',kind:'key',adds_information:true};
assert.equal(validateHintGrounding(JSON.stringify(good),source).ok,true);
assert.equal(validateHintGrounding(JSON.stringify(good),{...source,topic:'달무늬 소자'}).ok,false,'a writer cannot switch away from the requested subject');
const invented={...good,fact:'달무늬 소자는 고대 건축에 쓰인 특별한 장치이다.',topic:'달무늬 소자'};
assert.equal(validateHintGrounding(JSON.stringify(invented),source).ok,false,'a real source about another subject cannot validate an invented name');
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:'이 소자는 특수한 동력으로 작동한다.'}),source).ok,false);
const misleading={...good,fact:'근거에 없는 새로운 인과를 추가합니다.'};
assert.equal(validateHintGrounding(JSON.stringify(misleading),source).fact,good.support_quote,'the model paraphrase must not be displayed in extract mode');
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:'Query: 달무늬 소자의 의미는 무엇인가',topic:'달무늬 소자'}),source).ok,false,'query echoes are not source content');
assert.equal(validateHintGrounding(JSON.stringify(good),null).reason,'missing_proof');
assert.equal(validateHintGrounding('HINT_SKIP',source).ok,false);
const known={version:1,mode:'known',source:'internal',topic:'피라미드',aliases:['피라미드']};
assert.equal(validateHintGrounding(JSON.stringify(good),known).ok,true);
assert.equal(validateHintGrounding(JSON.stringify({...good,topic:'달무늬 소자'}),known).ok,false);
const otherRecord={...source,records:[{title:'달무늬 소자',url:'https://example.com/a',text:'다른 내용의 연속된 문장입니다.'},source.records[0]]};
assert.equal(validateHintGrounding(JSON.stringify(invented),otherRecord).ok,false,'a title in a different record cannot validate a quote');
console.log('PASS: source-bound names and exact excerpts, no query echo/cross-source support, verified canonical names and unsupported paraphrase rejection');

const punctuation={...good,support_quote:source.records[0].text.replace(/\.$/,'!')};
assert.equal(validateHintGrounding(JSON.stringify(punctuation),source).fact,source.records[0].text,'punctuation-only edits display original source');
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:source.records[0].text.replace('석재','나무')}),source).ok,false,'changed words remain unsupported');

const numericSource={...source,records:[{...source.records[0],text:'피라미드 모형의 높이는 1.5미터로 기록되어 있습니다.'}]};
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:numericSource.records[0].text.replace('1.5','15')}),numericSource).ok,false,'decimal changes are not punctuation repair');

const meeting='금요일 오후 3시에 회의해요. 장소는 2층 회의실입니다. 노트북을 가져와 주세요.';
const conversationProof={version:1,mode:'conversation',source:'conversation',topic:'회의 준비',kind:'schedule',records:[{title:'최근 대화',text:meeting}]};
const schedule={adds_information:true,kind:'key',topic:'회의 준비',points:[{text:'회의: 금요일 오후 3시, 2층 회의실',evidence:['금요일 오후 3시에 회의해요.','장소는 2층 회의실입니다.']},{text:'준비물: 노트북',evidence:['노트북을 가져와 주세요.']}]};
const validated=validateHintGrounding(JSON.stringify(schedule),conversationProof);
assert.equal(validated.ok,true);assert.equal(validated.kind,'schedule','the selected operation owns the label, not writer default key');assert.match(validated.fact,/회의:.*\n준비물/s);
assert.equal(validateHintGrounding(JSON.stringify({...schedule,points:[{text:'회의는 4시에 확정',evidence:['금요일 오후 3시에 회의해요.']}]}),conversationProof).fact,'금요일 오후 3시에 회의해요.');
assert.equal(validateHintGrounding(JSON.stringify({...schedule,points:[{text:'외부에서 가져온 내용',evidence:['없는 원문']}]}),conversationProof).ok,false);
assert.equal(validateHintGrounding(JSON.stringify({...schedule,points:[{text:'예약됨',evidence:['예약이 안됩니다.']}]}),{...conversationProof,records:[{text:'예약이 안됩니다.'}]}).fact,'예약이 안됩니다.');
assert.equal(validateHintGrounding(JSON.stringify(good),{...known,kind:'term'}).kind,'term','verified term definitions cannot silently become key');

const summaryNoNewFact={...schedule,adds_information:false};
assert.equal(validateHintGrounding(JSON.stringify(summaryNoNewFact),conversationProof).ok,true,'a grounded schedule does not need to add external facts');
const numberedProof={...conversationProof,spans:['금요일 오후 3시에 회의해요.','장소는 2층 회의실입니다.','노트북을 가져와 주세요.']};
const numberedSummary={points:[{text:'금요일 오후 3시 · 2층 회의실',evidence:[1,2]},{text:'노트북 지참',evidence:[3]}]};
assert.equal(validateHintGrounding(JSON.stringify(numberedSummary),numberedProof).ok,true);
assert.equal(validateHintGrounding(JSON.stringify({...numberedSummary,points:[{text:'임의 내용',evidence:[999]}]}),numberedProof).ok,false);
assert.equal(validateHintGrounding(JSON.stringify({points:[{text:'회의 준비 사항',evidence:[1,2]}]}),numberedProof).fact,'금요일 오후 3시에 회의해요. / 장소는 2층 회의실입니다.');
assert.equal(validateHintGrounding(JSON.stringify({skip:true,...numberedSummary}),numberedProof).ok,false);
assert.equal(validateHintGrounding(JSON.stringify({...good,adds_information:false}),source).ok,false,'external facts still require explicit inclusion and verified quotes');

// Compression may omit an unrelated quantity from the cited sentence. Only
// invented/changed quantities should force recovery to the original evidence.
const mixedSource='금요일 오후 3시에 회의하고 5시에 식사합니다. 노트북을 준비합니다. 서울 부산 KTX는 1시간 30분이라고 말했습니다.';
const mixedProof={version:1,mode:'conversation',source:'conversation',topic:'대화 정리',kind:'recap',records:[{text:mixedSource}],spans:['금요일 오후 3시에 회의하고 5시에 식사합니다.','노트북을 준비합니다.','서울 부산 KTX는 1시간 30분이라고 말했습니다.']};
const mixedSummary={points:[{type:'schedule',text:'금요일 오후 3시 회의',evidence:[1]},{type:'summary',text:'노트북 준비',evidence:[2]},{type:'check',text:'서울 부산 KTX 소요시간 1시간 30분',evidence:[3]}]};
const digest=validateHintGrounding(JSON.stringify(mixedSummary),mixedProof);
assert.equal(digest.ok,true);assert.match(digest.fact,/일정: 금요일 오후 3시 회의/);assert.doesNotMatch(digest.fact,/5시에 식사/,'omitting another event is not a numeric fabrication');assert.match(digest.fact,/핵심: 노트북 준비/);assert.match(digest.fact,/확인 필요: 서울 부산 KTX/);assert.doesNotMatch(digest.fact,/검증 완료|사실 확인 완료/);
const changedTime=validateHintGrounding(JSON.stringify({points:[{type:'schedule',text:'금요일 오후 8시 회의',evidence:[1]}]}),mixedProof);
assert.ok(changedTime.ok);assert.doesNotMatch(changedTime.fact,/8시/,'invented times never reach the display');
assert.equal(validateHintGrounding(JSON.stringify({points:[{type:'verified',text:'검증 완료',evidence:[3]}]}),mixedProof).ok,false,'conversation alone cannot produce a verified category');
const longCopy='전체 원문을 그대로 여러 차례 늘어놓는 문장입니다. '.repeat(8).trim();
const copyProof={...mixedProof,records:[{text:longCopy}],spans:[longCopy]};
assert.equal(validateHintGrounding(JSON.stringify({points:[{type:'summary',text:longCopy,evidence:[1]}]}),copyProof).reason,'no_summary_value','a long verbatim transcript is not a summary');
