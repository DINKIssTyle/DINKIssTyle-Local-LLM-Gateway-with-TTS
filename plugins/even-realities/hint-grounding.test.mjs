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

const vague={...good,fact:'피라미드에 대한 구체적인 기록이 존재함.'};
assert.equal(validateHintGrounding(JSON.stringify(vague),known).reason,'no_information_value');
const cut='피라미드에 관한 잘린 설명만 있어서...';
const cutSource={...source,records:[{...source.records[0],text:cut}]};
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:cut}),cutSource).reason,'incomplete_or_empty_source');
const completeThenCut=source.records[0].text+' 다른 설명은 아직...';
const mixedQuoteSource={...source,records:[{...source.records[0],text:completeThenCut}]};
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:completeThenCut}),mixedQuoteSource).fact,source.records[0].text);
console.log('PASS: vague existence claims and incomplete snippets rejected; complete source prefix remains exact');

const headingCut='3.2. 피라미드 구조에 대한 잘린 설명...';
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:headingCut}),{...source,records:[{...source.records[0],text:headingCut}]}).ok,false,'a numbered heading alone is not a complete explanatory quote');

// Reproduce the reported task drift without accepting the invented quote.
const archaeology='고고학에서는 물리학적 측정을 이용해 시료의 특성을 비교합니다.';
const physicsProof={version:1,mode:'quote',source:'web',topic:'물리학',kind:'term',task:'고고학에서 물리학을 활용하는 사례',records:[{title:'물리학의 고고학 활용',url:'https://example.com/archaeology',text:archaeology,spans:[archaeology]}]};
const unrelated='물리에서 미분은 변화율을 분석하거나 연속적인 양의 변화를 수학적으로 기술하는 데 사용됩니다.';
assert.equal(validateHintGrounding(JSON.stringify({fact:unrelated,support_quote:unrelated,topic:'물리학',adds_information:true}),physicsProof).reason,'unsupported_quote_or_name');
const referenced=validateHintGrounding(JSON.stringify({record:1,sentences:[1],adds_information:true}),physicsProof);
assert.equal(referenced.ok,true);assert.equal(referenced.fact,archaeology);assert.equal(referenced.kind,'term');
assert.equal(validateHintGrounding(JSON.stringify({record:1,sentences:[1],fact:unrelated,support_quote:unrelated,topic:'위조',adds_information:true}),physicsProof).fact,archaeology,'only immutable source text is displayed');
for(const bad of [{record:0},{record:2},{sentences:[0]},{sentences:[2]},{sentences:[1,1]},{sentences:[]},{sentences:['1']}])assert.equal(validateHintGrounding(JSON.stringify({record:1,sentences:[1],adds_information:true,...bad}),physicsProof).ok,false);
assert.equal(validateHintGrounding(JSON.stringify({record:1,sentences:[1],adds_information:true}),{...physicsProof,records:[{...physicsProof.records[0],spans:[unrelated]}]}).ok,false,'a span outside the actual source is never trusted');
console.log('PASS: reported unsupported quotation remains blocked; source selection reconstructs exact text without trusting generated fact/topic');

assert.equal(validateHintGrounding(JSON.stringify({record:0,sentences:[],adds_information:false}),physicsProof).reason,'source_does_not_answer_task');

assert.equal(validateHintGrounding(JSON.stringify({selection:1}),physicsProof).fact,archaeology);
assert.equal(validateHintGrounding(JSON.stringify({selection:0}),physicsProof).reason,'source_does_not_answer_task');
for(const selection of [-1,2,'1',null,1.5])assert.equal(validateHintGrounding(JSON.stringify({selection}),physicsProof).ok,false);

// Movie-going is a context cue, not a requirement for a chart title to repeat
// the word movie. Fictional data test the protocol, not actual rankings.
const day='가상 기준일의 국내 극장 관객 수 통계입니다.';
const ranking='가상 작품 A가 관객 수 집계에서 첫 번째입니다.';
const contextual={version:1,mode:'quote',source:'web',task_oriented:true,topic:'영화',kind:'key',records:[{title:'가상 일별 관객 수 집계',url:'https://example.com/chart',text:day+' '+ranking,spans:[day,ranking]},{title:'다른 지표',url:'https://example.com/booking',text:'가상 예매율 집계는 관객 수 순위와 다른 자료입니다.',spans:['가상 예매율 집계는 관객 수 순위와 다른 자료입니다.']}]};
const chart=validateHintGrounding('{"selections":[1,2]}',contextual);
assert.ok(chart.ok);assert.equal(chart.fact,day+'\n'+ranking);assert.equal(chart.topic,contextual.records[0].title);
assert.equal(validateHintGrounding('{"selections":[1,3]}',contextual).ok,false,'one card cannot combine ranks/metrics from unrelated sources');
for(const selections of [[1,1],[99],['1'],[1,2,3,4]])assert.equal(validateHintGrounding(JSON.stringify({selections}),contextual).ok,false);
assert.equal(validateHintGrounding('{"selections":[]}',contextual).reason,'source_does_not_answer_task');
assert.equal(validateHintGrounding('{"selection":1}',contextual).ok,true,'a single reference remains compatible');
assert.equal(validateHintGrounding('{"selections":[1]}',{...contextual,records:[{...contextual.records[0],spans:['출처에 존재하지 않는 다른 내용을 여기서 만들었습니다.']}]}).ok,false);
assert.equal(validateHintGrounding('{"selections":[1]}',{...contextual,records:[{...contextual.records[0],url:'javascript:bad'}]}).ok,false);
assert.equal(validateHintGrounding(JSON.stringify({...good,support_quote:day,topic:'영화'}),{...source,topic:'영화',records:contextual.records}).ok,false,'legacy name verification stays strict');
console.log('PASS: situation-based chart cards, source-backed headings, date/result excerpts, no cross-source metrics and unchanged legacy name verification');

// One record with local sentence IDs: no guessing global IDs or mixing sources.
const local=validateHintGrounding('{"record":1,"sentences":[1,2]}',contextual);
assert.equal(local.fact,day+'\n'+ranking);assert.deepEqual(local.selection,{record:1,sentences:[1,2]});
assert.equal(validateHintGrounding('{"record":0,"sentences":[]}',contextual).reason,'source_does_not_answer_task');
for(const payload of [{record:0,sentences:[1]},{record:3,sentences:[1]},{record:1,sentences:[2,1]},{record:1,sentences:[1,1]},{record:1,sentences:[3]},{record:1,sentences:['1']},{record:1,sentences:[1],selections:[3]}])assert.equal(validateHintGrounding(JSON.stringify(payload),contextual).ok,false);
for(const alias of ['choices','selection','source_selections','source_selection','selected_items']){
 const result=validateHintGrounding(JSON.stringify({[alias]:[1,2]}),contextual);
 assert.ok(result.ok);assert.equal(result.normalized,alias);assert.equal(result.fact,local.fact);
 assert.equal(validateHintGrounding(JSON.stringify({[alias]:[1,3]}),contextual).reason,'mixed_source_records');
}
assert.equal(validateHintGrounding('{"choices":[1],"selections":[2]}',contextual).ok,false,'ambiguous alternative selections must not be guessed');
assert.equal(validateHintGrounding('{"1":[1,2]}',contextual).ok,false,'numeric object keys do not imply record or sentence meanings');
assert.equal(validateHintGrounding('[1,2]',contextual).fact,local.fact);
assert.equal(validateHintGrounding('JSON with selections: [1]',contextual).reason,'invalid_hint_response','never extract arbitrary numbers from prose');
const missing='제공된 대화 내용만으로는 해당 물체의 종류를 파악하기 어렵습니다.';
assert.equal(validateHintGrounding(JSON.stringify({fact:missing,adds_information:true}),{...known,task_oriented:true,topic:'물체의 종류 알아보기'}).reason,'insufficient_context');
const heading='교육의 목표와 논의를 시작할 방향';
assert.equal(validateHintGrounding(JSON.stringify({fact:heading,adds_information:true}),{...known,task_oriented:true,topic:heading}).reason,'heading_only');
const nav='지역별 더 자세한 날씨는 기상청 날씨누리 앱을 참고하시기 바랍니다.';
const navProof={...contextual,records:[{title:'날씨 서비스',url:'https://example.com/weather',text:nav,spans:[nav]}]};
assert.equal(validateHintGrounding('{"record":1,"sentences":[1]}',navProof).reason,'navigation_only');
const limitation='이 실험만으로 인과관계를 확정할 수는 없으며, 대조군을 둔 반복 실험이 필요합니다.';
assert.ok(validateHintGrounding(JSON.stringify({fact:limitation,adds_information:true}),{...known,task_oriented:true,topic:'실험 해석의 한계'}).ok,'concrete factual limitations are useful information');
console.log('PASS: canonical per-record references, safe flat aliases, source isolation, no prose recovery, insufficient-context/title/navigation rejection and useful uncertainty');

assert.equal(validateHintGrounding(JSON.stringify({fact:'지역 교육의 목표에 대한 논의와 시작 방향',adds_information:true}),{...known,task_oriented:true,topic:'지역 교육의 목표에 대한 논의를 시작해야 하는 이유와 실제 실행 방향',task:'지역 교육의 목표에 대한 논의를 시작할 구체적인 방법'}).reason,'heading_only','shortened task headings are not a substantive answer');
