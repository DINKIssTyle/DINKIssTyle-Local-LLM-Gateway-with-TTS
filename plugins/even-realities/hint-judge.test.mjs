import assert from 'node:assert/strict';
import {hintPresentation,judgePrompts,suggestedHintJudge,normalizeHintBudget,estimateHintTokens,compactJudgeContext,parseHintJudgment,requestHintJudgment,hintEvidenceChoices} from './hint-judge.mjs';
assert.equal(normalizeHintBudget(1),300);assert.equal(normalizeHintBudget(9999),800);assert.equal(normalizeHintBudget(undefined),600);
for(const budget of [300,400,500,600,700,800])for(const sample of ['청동기 시대가 언제였는지 궁금하다. ','MacBook Air M4 weight 13-inch and 15-inch. ','🕶️こんにちは 한국어 English 123? ']){
 const c=compactJudgeContext({text:sample.repeat(100),context:sample.repeat(100)},sample.repeat(100),[{term:'피라미드'}],budget);
 assert.ok(c.estimatedTokens<=budget,JSON.stringify(c));assert.ok(c.latest);assert.ok(c.latest.endsWith(sample.trim()));
 assert.ok(estimateHintTokens('한국어한국어')>estimateHintTokens('abcdef'));
}
const context=compactJudgeContext({text:'피라미드가 왕의 무덤이래.',context:''});
const value={hint:true,kind:'term',topic:'피라미드',evidence:'피라미드',question:'쿠푸왕 피라미드 건립 시기와 구조',source:'web'};
const valid=parseHintJudgment(JSON.stringify(value),context);
assert.equal(valid.plan.source,'web');assert.equal(valid.focus,'피라미드');assert.ok(valid.plan.knownFacts.includes(context.latest));
assert.deepEqual(parseHintJudgment('```json\n'+JSON.stringify({result:JSON.stringify(value)})+'\n```',context),valid);
assert.equal(parseHintJudgment(JSON.stringify({...value,evidence:'없는 원문'}),context),null);
assert.equal(parseHintJudgment(JSON.stringify({...value,source:'execute_command'}),context),null);
assert.equal(parseHintJudgment('{"hint":false}',context).hint,false);
assert.equal(parseHintJudgment('not json',context),null);
let sent;
const result=await requestHintJudgment({fetch:async(url,opts)=>{sent=JSON.parse(opts.body);return {ok:true,json:async()=>({choices:[{message:{content:JSON.stringify(value)}}]})};},server:'http://test',headers:{},model:'small',job:{text:context.latest,context:''},advisory:{hint:false},limit:300});
assert.equal(result.hint,true,'negative JEV advice cannot veto LLM judgment');assert.equal(sent.glasses_hint_phase,'judge');assert.equal(sent.model,'small');assert.equal(sent.max_tokens,384);assert.equal(sent.store,false);assert.equal(sent.chat_template_kwargs.enable_thinking,false);
assert.ok(result.contextEstimatedTokens<=300);assert.equal(sent.response_format.type,'json_schema');
await assert.rejects(()=>requestHintJudgment({fetch:async(url,opts)=>new Promise((resolve,reject)=>opts.signal.addEventListener('abort',()=>reject(Object.assign(Error('aborted'),{name:'AbortError'})))),model:'small',job:{text:'피라미드',context:''},timeoutMs:10}),error=>error.code==='judge_timeout');
const controller=new AbortController();controller.abort();
await assert.rejects(()=>requestHintJudgment({fetch:async(url,opts)=>{assert.equal(opts.signal.aborted,true);throw Error('cancelled');},model:'small',job:{text:'피라미드'},signal:controller.signal}),error=>error.code==='judge_canceled');
console.log('PASS: 300–800 multilingual context budgets, bounded tool-free judge, grounded plans, nested JSON, JEV-independent selection and cancellation');

assert.equal(suggestedHintJudge([{id:'qwen3.5-0.8b-mlx',label:'0.8B'},{id:'gemma-4-e2b-it-mlx',label:'Gemma'}]),'gemma-4-e2b-it-mlx');
assert.equal(suggestedHintJudge([{id:'qwen3.5-0.8b-mlx',label:'0.8B'}]),'');

for(const [kind,label] of Object.entries({term:'용어',key:'정보',verify:'확인',schedule:'일정'})){
 const decision=parseHintJudgment(JSON.stringify({...value,kind}),context);
 const shown=hintPresentation(decision.plan);
 assert.match(shown.title,new RegExp('^\\['+label+'\\]'));
 assert.match(shown.reason,/피라미드/);
}
assert.equal(parseHintJudgment(JSON.stringify({...value,kind:'invented'}),context),null);
assert.ok(sent.response_format.json_schema.schema.required.includes('kind'));
assert.match(judgePrompts(context).systemPrompt,/Do not .*create reminders/);
assert.ok(!hintPresentation({topic:'legacy'}).title.includes('undefined'));

const uncertainPrompt=judgePrompts(context).systemPrompt;
assert.match(uncertainPrompt,/typos, missing words and recognition errors/);
assert.match(uncertainPrompt,/never correct or translate that evidence/);
assert.match(uncertainPrompt,/No category quotas/);
assert.equal(sent.response_format.json_schema.schema.properties.kind.enum[0],'key');

const isolated=compactJudgeContext({text:'디지털 시대의 변화',context:''},'[청동기 시대] 이전 힌트 본문');
assert.ok(!isolated.text.includes('청동기 시대') && !isolated.text.includes('이전 힌트 본문'));
assert.ok(!isolated.text.includes('Shown:'));

const continuityContext=compactJudgeContext({text:'이제 내일 날씨 이야기야.',context:'',activeTopic:'피라미드'});
const changedSkip=parseHintJudgment(JSON.stringify({hint:false,topic:'내일 날씨',evidence:'내일 날씨',continuity:'changed'}),continuityContext);
assert.equal(changedSkip.continuity,'changed');assert.equal(changedSkip.topic,'내일 날씨');
assert.equal(parseHintJudgment(JSON.stringify({hint:false,topic:'없는 주제',evidence:'없는 원문',continuity:'changed'}),continuityContext).topic,undefined,'ungrounded skip cannot change the current topic');
for(const budget of [300,600,800])assert.ok(compactJudgeContext({text:'대화 중인 새 발화'.repeat(100),context:'참고'.repeat(100),activeTopic:'아주 긴 현재 주제 이름입니다'.repeat(10)},'',[],budget).estimatedTokens<=budget);
assert.ok(sent.response_format.json_schema.schema.required.includes('continuity'));

const conversation=compactJudgeContext({text:'금요일 오후 3시에 회의해요.\n장소는 2층입니다.\n노트북을 가져와 주세요.',latestText:'노트북을 가져와 주세요.',context:'금요일 오후 3시에 회의해요.\n장소는 2층입니다.'},'',[],600);
assert.match(conversation.text,/Recent conversation/);assert.match(conversation.latest,/금요일 오후 3시/);assert.match(conversation.latest,/노트북/);
const schedule=parseHintJudgment(JSON.stringify({hint:true,kind:'schedule',source:'conversation',topic:'회의 준비',evidence:'금요일 오후 3시',question:'언급한 일정과 준비물 정리'}),conversation);
assert.equal(schedule.hint,false);assert.equal(schedule.reason,'conversation_summary_disabled');
assert.equal(parseHintJudgment(JSON.stringify({...value,source:'conversation',kind:'term'}),context).hint,false,'conversation-only tasks cannot be disguised as information');
for(const budget of [300,600,800]){
 const c=compactJudgeContext({text:'전체',latestText:'최근 발화입니다. '.repeat(100),context:'모아둔 대화입니다. '.repeat(100)},'',[{term:'준비'}],budget);
 assert.ok(c.estimatedTokens<=budget,JSON.stringify(c));assert.ok(c.prior);
}
const numberedChoice=conversation.evidenceChoices.findIndex(e=>e.includes('금요일'));
assert.ok(numberedChoice>0);
const chosen=parseHintJudgment(JSON.stringify({hint:true,kind:'schedule',source:'conversation',topic:'회의 준비',evidence:numberedChoice,question:'일정과 준비물 정리'}),conversation);
assert.equal(chosen.hint,false);assert.equal(parseHintJudgment(JSON.stringify({...value,evidence:999}),conversation),null);
assert.equal(sent.response_format.json_schema.schema.properties.evidence.type,'integer');

let repairs=0;
const repaired=await requestHintJudgment({server:'http://test',model:'small',job:{text:'피라미드의 큰 돌을 어떻게 옮겼을까.'},fetch:async(url,opts)=>{repairs++;const body=JSON.parse(opts.body);assert.ok(body.response_format.json_schema.schema.anyOf);return {ok:true,json:async()=>({response:JSON.stringify(repairs===1?{...value,hint:true,topic:'',evidence:0}:{...value,evidence:1})})};}});
assert.equal(repairs,2);assert.equal(repaired.repairAttempts,1);assert.ok(repaired.focus.includes('피라미드'));
let skippedCalls=0;const skipped=await requestHintJudgment({server:'http://test',job:{text:'응 고마워.'},fetch:async()=>{skippedCalls++;return {ok:true,json:async()=>({response:'{"hint":false}'})};}});assert.equal(skipped.hint,false);assert.equal(skippedCalls,1,'a legitimate skip must never trigger a retry');
let badCalls=0;await assert.rejects(()=>requestHintJudgment({job:{text:'피라미드 이야기'},fetch:async()=>{badCalls++;return {ok:true,json:async()=>({response:'{"hint":true,"topic":"","evidence":0}'})};}}),/비어 있어/);assert.equal(badCalls,2,'format repair is bounded to one retry');

const compactPrompt=judgePrompts(context).systemPrompt;
assert.ok(compactPrompt.length<2400,'classifier instructions must stay compact');
assert.doesNotMatch(compactPrompt,/예시|피라미드|EBITDA|구상성단/,'no few-shot examples or example topics in the classifier');
assert.match(compactPrompt,/unfinished Latest/);
assert.match(compactPrompt,/ignore uncertain parts/);
const news='국가자격시험에 대해서도 전수조사를 실시하고 이상징후가 있었는지 점검할 방침입니다.';
assert.deepEqual(hintEvidenceChoices(news),['',news],'never split a normal sentence or 방침 into 40-character fragments');
const fragmented=compactJudgeContext({latestText:'오늘 오후 상급기관인 고용노동부와',context:news,text:news+' 오늘 오후 상급기관인 고용노동부와'},'',[],800);
assert.ok(fragmented.evidenceChoices.includes(news));
const fromPrior=parseHintJudgment(JSON.stringify({...value,topic:'전수조사',question:'전수조사의 의미',evidence:fragmented.evidenceChoices.indexOf(news)}),fragmented);
assert.equal(fromPrior.plan.topic,'전수조사','a valid preceding sentence remains selectable despite an unfinished latest utterance');
const long='국가자격시험 전수조사 대상과 점검 방법을 안내합니다 '.repeat(10);
const chunks=hintEvidenceChoices(long).slice(1);
assert.ok(chunks.length>1);assert.ok(chunks.every(v=>Array.from(v).length<=160));
assert.equal(chunks.join(' '),long.trim(),'long clauses preserve original words and content');
assert.ok(hintEvidenceChoices('가'.repeat(400)).slice(1).every(v=>Array.from(v).length<=160),'unspaced input remains bounded');


// Dictionary advice must not bias LLM selection, including plausible titles.
const transcript='미국국립보건원에서 정의한 개념을 리디자인한다고 설명합니다. 유용한 목적을 위해 새롭게 디자인합니다.';
const plain=compactJudgeContext({text:transcript},'',[],600);
const biased=compactJudgeContext({text:transcript},'',[{term:'해서'},{term:'아아'},{term:'아주'},{term:'임의의 후보'}],600);
assert.deepEqual(biased,plain,'candidate advice has no effect on context or its budget');
assert.doesNotMatch(plain.text,/Names|임의의 후보/);
let cleanRequest;
await requestHintJudgment({server:'http://test',model:'small',job:{text:transcript},advisory:{keyword:'임의의 후보',candidates:[{term:'해서'}]},fetch:async(url,opts)=>{cleanRequest=JSON.parse(opts.body);return {ok:true,json:async()=>({response:'{"hint":false}'})};}});
assert.match(cleanRequest.messages[1].content,/미국국립보건원/);
assert.match(cleanRequest.messages[1].content,/리디자인/);
assert.doesNotMatch(JSON.stringify(cleanRequest.messages),/Names|임의의 후보/,'neither prompt contains a dictionary candidate list');
assert.match(judgePrompts(plain).systemPrompt,/units or comparison subjects are unclear/);

const diagnostics=[];
await assert.rejects(()=>requestHintJudgment({job:{text:'유전자 정보의 등록'},timeoutMs:10,onDiagnostic:data=>diagnostics.push(data),fetch:async()=>new Promise(()=>{})}),error=>error.code==='judge_timeout');
assert.equal(diagnostics.at(-1).reason,'judge_timeout');assert.equal(diagnostics.at(-1).role,'hintDecision');assert.ok(diagnostics.at(-1).elapsedMs>=0);assert.equal(diagnostics.at(-1).phase,'request');
await assert.rejects(()=>requestHintJudgment({job:{text:'유전자 등록'},onDiagnostic:data=>diagnostics.push(data),fetch:async()=>({ok:false,status:400})}),error=>error.code==='judge_http_error');
assert.equal(diagnostics.at(-1).status,400);
await assert.rejects(()=>requestHintJudgment({job:{text:'유전자 등록'},fetch:async()=>{throw new TypeError('network unavailable');}}),error=>error.code==='judge_request_error');
await assert.rejects(()=>requestHintJudgment({job:{text:'유전자 등록'},onDiagnostic:data=>diagnostics.push(data),fetch:async()=>({ok:true,json:async()=>({response:'not json'})})}),error=>error.code==='judge_invalid_response');
assert.ok(diagnostics.some(d=>d.response==='not json'));assert.equal(diagnostics.at(-1).attempt,2);
let emitted=false,canceledReader=false;
await assert.rejects(()=>requestHintJudgment({job:{text:'유전자 등록'},timeoutMs:10,onDiagnostic:data=>diagnostics.push(data),fetch:async()=>({ok:true,body:{getReader:()=>({cancel(){canceledReader=true;},read:async()=>{if(!emitted){emitted=true;return {value:Buffer.from('data: '+JSON.stringify({choices:[{delta:{content:'{"hint":'}}]})+'\n\n'),done:false};}return new Promise(()=>{});}})}})}),error=>error.code==='judge_timeout');
assert.equal(diagnostics.at(-1).response,'{"hint":');assert.equal(diagnostics.at(-1).phase,'response');assert.equal(canceledReader,true,'timeout cancels the reader as well as fetch');
const validDiagnostics=[];
const skippedDiagnostic=await requestHintJudgment({job:{text:'안녕하세요'},onDiagnostic:d=>validDiagnostics.push(d),fetch:async()=>({ok:true,json:async()=>({response:'{"hint":false}'})})});
assert.equal(skippedDiagnostic.hint,false);assert.equal(validDiagnostics.at(-1).reason,'judge_response');
assert.ok(skippedDiagnostic.elapsedMs>=0);

assert.ok(!sent.response_format.json_schema.schema.properties.kind.enum.includes('recap'));
assert.ok(!sent.response_format.json_schema.schema.properties.source.enum.includes('conversation'));
for(const source of ['internal','web','memory'])assert.equal(parseHintJudgment(JSON.stringify({...value,kind:'recap',source}),context).reason,'conversation_summary_disabled');
