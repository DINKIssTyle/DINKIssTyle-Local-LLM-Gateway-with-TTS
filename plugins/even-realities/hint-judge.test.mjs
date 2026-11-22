import assert from 'node:assert/strict';
import {hintPresentation,suggestedHintJudge,normalizeHintBudget,estimateHintTokens,compactJudgeContext,parseHintJudgment,requestHintJudgment as sendHintJudgment,hintEvidenceChoices} from './hint-judge.mjs';
import {judgePrompts} from './test-support/gateway-prompts.mjs';
import {expandGatewayRequest} from './test-support/gateway-prompts.mjs';
const requestHintJudgment=options=>sendHintJudgment({...options,fetch:(url,opts)=>options.fetch(url,{...opts,body:JSON.stringify(expandGatewayRequest(JSON.parse(opts.body)))})});
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
assert.equal(result.hint,true,'negative JEV advice cannot veto LLM judgment');assert.equal(sent.glasses_hint_phase,'judge');assert.equal(sent.model,'small');assert.equal(sent.max_tokens,1024);assert.equal(sent.store,false);assert.notEqual(sent.chat_template_kwargs?.enable_thinking,false);
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
assert.match(judgePrompts(context).systemPrompt,/No automatic recaps or reminders/);
assert.ok(!hintPresentation({topic:'legacy'}).title.includes('undefined'));

const uncertainPrompt=judgePrompts(context).systemPrompt;
assert.match(uncertainPrompt,/ASR may be wrong/);
assert.match(uncertainPrompt,/never invent a title, person, date, number or missing negation/);
assert.match(uncertainPrompt,/Plan the useful task before the label/);
assert.equal(sent.response_format.json_schema.schema.properties.kind.enum[0],'key');

const isolated=compactJudgeContext({text:'디지털 시대의 변화',context:''},'[청동기 시대] 이전 힌트 본문');
assert.ok(!isolated.text.includes('청동기 시대') && !isolated.text.includes('이전 힌트 본문'));
assert.ok(!isolated.text.includes('Shown:'));

const continuityContext=compactJudgeContext({text:'이제 내일 날씨 이야기야.',context:'',activeTopic:'피라미드'});
const changedSkip=parseHintJudgment(JSON.stringify({hint:false,topic:'내일 날씨',evidence:'내일 날씨',continuity:'changed'}),continuityContext);
assert.equal(changedSkip.continuity,'changed');assert.equal(changedSkip.topic,'내일 날씨');
assert.equal(parseHintJudgment(JSON.stringify({hint:false,topic:'없는 주제',evidence:'없는 원문',continuity:'changed'}),continuityContext).topic,undefined,'ungrounded skip cannot change the current topic');
for(const budget of [300,600,800])assert.ok(compactJudgeContext({text:'대화 중인 새 발화'.repeat(100),context:'참고'.repeat(100),activeTopic:'아주 긴 현재 주제 이름입니다'.repeat(10)},'',[],budget).estimatedTokens<=budget);
assert.ok(sent.response_format.json_schema.schema.required.includes('anchor'));

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
assert.equal(sent.response_format.json_schema.schema.properties.anchor.type,'string');

let repairs=0;
const repaired=await requestHintJudgment({server:'http://test',model:'small',job:{text:'피라미드의 큰 돌을 어떻게 옮겼을까.'},fetch:async(url,opts)=>{repairs++;const body=JSON.parse(opts.body);assert.ok(!body.response_format.json_schema.schema.anyOf);return {ok:true,json:async()=>({response:JSON.stringify(repairs===1?{...value,hint:true,topic:'',evidence:0}:{...value,evidence:1})})};}});
assert.equal(repairs,2);assert.equal(repaired.repairAttempts,1);assert.ok(repaired.focus.includes('피라미드'));
let skippedCalls=0;const skipped=await requestHintJudgment({server:'http://test',job:{text:'응 고마워.'},fetch:async()=>{skippedCalls++;return {ok:true,json:async()=>({response:'{"hint":false}'})};}});assert.equal(skipped.hint,false);assert.equal(skippedCalls,1,'a legitimate skip must never trigger a retry');
let badCalls=0;await assert.rejects(()=>requestHintJudgment({job:{text:'피라미드 이야기'},fetch:async()=>{badCalls++;return {ok:true,json:async()=>({response:'{"hint":true,"topic":"","evidence":0}'})};}}),/비어 있어/);assert.equal(badCalls,2,'format repair is bounded to one retry');

const compactPrompt=judgePrompts(context).systemPrompt;
assert.ok(compactPrompt.length<5500,'planner instructions and its source-research reference must stay bounded');
assert.doesNotMatch(compactPrompt,/예시|피라미드|EBITDA|구상성단/,'no few-shot examples or example topics in the classifier');
assert.match(compactPrompt,/unfinished Latest/);
assert.match(compactPrompt,/use clear context/);
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
assert.match(judgePrompts(plain).systemPrompt,/keep location, time and metric distinct/);

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

const biology=compactJudgeContext({latestText:'열수 분출공에서는 고압의 뜨거운 물이 나옵니다.',context:'원시 수프 가설도 생명의 기원에 관한 설명입니다.'},'',[],600);
const span=biology.evidenceChoices.findIndex(text=>text.includes('열수'));
const selection={span,topic:'열수 분출공',need:'원시 수프 가설과 에너지원 및 조건의 차이',kind:'term',source:'web'};
const selected=parseHintJudgment(JSON.stringify(selection),biology);
assert.ok(selected.hint);assert.equal(selected.plan.question,selection.need);
assert.equal(selected.plan.evidence,biology.evidenceChoices[span]);
assert.equal(parseHintJudgment(JSON.stringify({...selection,span:0,topic:'',need:''}),biology).hint,false);
for(const bad of [{span:99},{span:[span]},{span:null},{span:-1},{topic:'스마트 안경'},{topic:'소인성'},{need:''}])assert.equal(parseHintJudgment(JSON.stringify({...selection,...bad}),biology),null);
const fragment=compactJudgeContext({text:'있습니다.'});
assert.equal(parseHintJudgment(JSON.stringify({hint:true,evidence:1,topic:'UFO',question:'UFO 정의',kind:'term',source:'web'}),fragment),null);
assert.equal(parseHintJudgment(JSON.stringify({...value,evidence:0}),context),null,'old positive zero is never auto-repaired');
const staleTopic=compactJudgeContext({text:'열수 분출공 이야기',activeTopic:'피라미드'});
assert.doesNotMatch(staleTopic.text,/피라미드/);
assert.ok(!sent.response_format.json_schema.schema.anyOf);
assert.deepEqual(sent.response_format.json_schema.schema.required,['task','anchor','source','kind','search_query']);
console.log('PASS: compact span contract, real biology comparison, grounded topic, positive-zero rejection and stale topic removal');

assert.equal(parseHintJudgment(JSON.stringify({...selection,kind:'mechanism'}),biology).plan.kind,'term');
const thanks=compactJudgeContext({text:'네 감사합니다. 잘 알겠습니다.'});
assert.equal(parseHintJudgment(JSON.stringify({span:2,topic:'잘 알겠습니다.',need:'',kind:'key',source:'internal'}),thanks).hint,false);

const failedDiagnostics=[];
await assert.rejects(()=>requestHintJudgment({job:{text:'피라미드'},onDiagnostic:event=>failedDiagnostics.push(event),fetch:async()=>({ok:true,json:async()=>({response:'{"span":99}'})})}),/비어 있어/);
assert.equal(failedDiagnostics.filter(event=>event.reason==='judge_invalid_response').length,2,'each attempt emits once, without a duplicated terminal response');

// Fabricated fixtures: unresolved referring clauses are not named terms.
const unclear=compactJudgeContext({text:'그 조건을 만족하는 거라면 만들 수 있다고 합니다.'});
assert.equal(parseHintJudgment(JSON.stringify({span:1,topic:'그 조건을 만족하는 거',need:'그 의미 설명',kind:'term',source:'internal'}),unclear).reason,'unresolved_hint_subject');
const patent=compactJudgeContext({text:'어떤 제작자가 도구 특허를 냈다고 하는데 명칭은 아직 말하지 않았습니다.'});
const genericPatent=parseHintJudgment(JSON.stringify({span:1,topic:'특허',need:'특허를 등록할 때 요구되는 기본 요건',kind:'term',source:'web'}),patent);
assert.ok(genericPatent.hint);assert.equal(genericPatent.plan.topic,'특허');
assert.match(judgePrompts(patent).systemPrompt,/Skip unclear identities/);
assert.match(judgePrompts(patent).systemPrompt,/evidence, not a card title or word to define/);

const unknownInventor=compactJudgeContext({text:'그 제작자는 그걸로 도구 특허를 등록했다고 합니다.'});
assert.equal(parseHintJudgment(JSON.stringify({span:1,topic:'도구 특허',need:'등록한 특허의 구체적인 내용',kind:'term',source:'internal'}),unknownInventor).reason,'unresolved_hint_subject');
assert.ok(parseHintJudgment(JSON.stringify({span:1,topic:'특허',need:'특허 등록을 위한 일반적인 요건',kind:'term',source:'web'}),unknownInventor).hint,'an unidentified inventor does not prevent general patent guidance');
console.log('PASS: unresolved clauses and unidentified past-action lookups skip without a schema retry; general concepts remain selectable');

const numbered=compactJudgeContext({text:'조경에서는 나무를 모아 심습니다. 다음은 색소에 관한 이야기입니다.'});
const correctedSpan=parseHintJudgment(JSON.stringify({span:2,topic:'조경',need:'모아 심기가 경관에 주는 효과',kind:'term',source:'web'}),numbered);
assert.ok(correctedSpan.hint);assert.ok(correctedSpan.spanRecovered);assert.match(correctedSpan.focus,/조경/);
const ambiguous=compactJudgeContext({text:'조경은 공간을 다룹니다. 조경에는 나무를 씁니다. 다른 이야기입니다.'});
assert.equal(parseHintJudgment(JSON.stringify({span:3,topic:'조경',need:'식재의 기본 원리',kind:'term',source:'web'}),ambiguous),null,'multiple possible source numbers are never guessed');
let issue='';
assert.equal(parseHintJudgment(JSON.stringify({span:1,topic:'조경에서는 나무를 모아 심습니다.',need:'식재 원리',kind:'term',source:'web'}),numbered,r=>{issue=r;}),null);assert.equal(issue,'sentence_topic');
assert.equal(parseHintJudgment(JSON.stringify({span:1,topic:'조경',need:'mechanism',kind:'term',source:'internal'}),numbered,r=>{issue=r;}),null);assert.equal(issue,'generic_task');
let countRepair=0;
const repairTask=await requestHintJudgment({job:{text:'나무의 잎은 계절에 따라 달라집니다.'},fetch:async(url,opts)=>{
 const sent=JSON.parse(opts.body);countRepair++;
 if(countRepair===2)assert.match(sent.messages[0].content,/not just a category/);
 return {ok:true,json:async()=>({response:JSON.stringify(countRepair===1?{span:1,topic:'나무',need:'mechanism',kind:'term',source:'internal'}:{span:1,topic:'나무',need:'계절에 따라 잎 색이 달라지는 생물학적 원리',kind:'term',source:'web'})})};
}});
assert.equal(countRepair,2);assert.equal(repairTask.plan.topic,'나무');
console.log('PASS: uniquely recoverable source-number errors, sentence topics and empty research labels repaired within one retry');

let failedReselection=0;
await assert.rejects(()=>requestHintJudgment({job:{text:'나무가 있습니다.'},fetch:async()=>{failedReselection++;return {ok:true,json:async()=>({response:JSON.stringify({span:1,topic:'나무가 있습니다.',need:'mechanism',kind:'term',source:'internal'})})};}}),error=>error.code==='judge_invalid_response');
assert.equal(failedReselection,2,'semantic reselection never starts an unbounded loop');

const movieContext=compactJudgeContext({text:'주말에 영화 한 편 볼까 합니다.'});
const taskPlan=parseHintJudgment(JSON.stringify({anchor:'영화 한 편',task:'현재 한국 극장 관객 수 순위와 집계 기준일',source:'web',kind:'key'}),movieContext);
assert.ok(taskPlan.hint);assert.ok(taskPlan.plan.taskOriented);assert.equal(taskPlan.focus,'영화 한 편');
assert.equal(taskPlan.plan.question,'현재 한국 극장 관객 수 순위와 집계 기준일');
assert.equal(taskPlan.plan.topic,taskPlan.plan.question,'research task, not copied noun, identifies the pending card');
assert.equal(parseHintJudgment('{"task":"","anchor":"쉬는 중","source":"internal","kind":""}',movieContext).hint,false);
for(const bad of [{anchor:'없는 영화 제목'},{task:'mechanism'},{source:'run_command'},{task:'가'.repeat(181)}])assert.equal(parseHintJudgment(JSON.stringify({anchor:'영화 한 편',task:'현재 한국 극장 관객 수 순위와 집계 기준일',source:'web',kind:'key',...bad}),movieContext),null);
assert.equal(parseHintJudgment(JSON.stringify({anchor:'영화 한 편',task:'현재 한국 극장 관객 수 순위와 집계 기준일',source:'web',kind:'other'}),movieContext).plan.kind,'key','display taxonomy cannot veto a useful grounded task');
assert.ok(!movieContext.text.includes('CurrentTopic'));

// Source-focused search remains separate from the wearer-facing information task.
const sourceTask={task:'장치의 최대 입력 전력 확인',anchor:'피라미드',source:'web',kind:'verify',search_query:'장치 제조사 사용 설명서 입력 전력'};
const sourcePlan=parseHintJudgment(JSON.stringify(sourceTask),context).plan;
assert.equal(sourcePlan.question,sourceTask.task);
assert.equal(sourcePlan.searchQuery,sourceTask.search_query);
for(const search_query of [123,'가'.repeat(301)])assert.equal(parseHintJudgment(JSON.stringify({...sourceTask,search_query}),context),null);
assert.equal(parseHintJudgment(JSON.stringify({...sourceTask,source:'memory'}),context).plan.searchQuery,undefined);

// A server deadline/stream error must not become a grounding failure or start a schema retry.
for(const code of ['judge_timeout','judge_stream_error']){
 let calls=0;
 await assert.rejects(()=>requestHintJudgment({job:{text:'손잡이의 지렛대 원리'},fetch:async()=>{calls++;return {ok:true,body:{getReader:()=>{let done=false;return {read:async()=>done?{done:true}:(done=true,{done:false,value:Buffer.from('data: '+JSON.stringify({error:{code,message:'upstream interrupted'}})+'\n\n')})};}}};}}),error=>error.code===code);
 assert.equal(calls,1,'transport errors do not justify a schema-repair request');
}
let emptyCalls=0;
await assert.rejects(()=>requestHintJudgment({job:{text:'손잡이 원리'},fetch:async()=>{emptyCalls++;return {ok:true,json:async()=>({response:''})};}}),error=>error.code==='judge_empty_response');
assert.equal(emptyCalls,1,'a missing final answer is not insufficient transcript evidence');
