import assert from 'node:assert/strict';
import {HintGate,similarHint} from './hint-gate.mjs';
const tick=()=>new Promise(resolve=>setImmediate(resolve));
let now=100000,calls=[],hints=[],statuses=[],release;
const gate=new HintGate({generationIntervalMs:0,settleMs:0,now:()=>now,intervalMs:0,cooldownMs:0,decide:async job=>{
 calls.push(job);if(job.id==='held')return await new Promise(resolve=>{release=resolve;});
 return {hint:job.text!=='일반 대화',category:'explain'};
},onHint:job=>hints.push(job),onStatus:text=>statuses.push(text)});
gate.observe('일반 대화',{id:'skip'});await tick();assert.equal(hints.length,0);
gate.observe('묵시적 갱신이 무슨 뜻일까',{id:'held'});await tick();
gate.observe('판별 중 새 발화 하나',{id:'superseded'});gate.observe('판별 중 최신 발화 둘',{id:'latest'});
assert.equal(calls.length,2,'decisions must be independent and serialized');
release({hint:true,category:'explain'});await tick();await tick();
assert.deepEqual(calls.map(job=>job.id),['skip','held','latest'],'only one latest pending decision survives');
assert.equal(calls.at(-1).text,'판별 중 최신 발화 둘');assert.ok(!calls.at(-1).text.includes('묵시적 갱신'));
for(const job of hints)gate.complete(job,{displayed:true,text:'유용한 정보 '+job.text});
const before=hints.length;gate.observe('판별 중 최신 발화 둘!',{id:'duplicate'});await tick();
assert.equal(hints.length,before,'duplicates must not generate hints');
assert.ok(similarHint('묵시적 갱신이 뭐지?', '묵시적갱신이 뭐지'));
gate.observe('새 주제 대기 중',{id:'held'});await tick();gate.reset();release({hint:true,category:'reply'});await tick();
assert.equal(hints.length,before,'mode cancellation discards a late decision');
gate.observe('이미 지난 대화',{id:'stale',at:now-gate.maxAgeMs-1});await tick();assert.ok(!calls.some(job=>job.id==='stale'));
const delayed=new HintGate({generationIntervalMs:0,settleMs:0,now:()=>now,intervalMs:0,cooldownMs:15000,decide:async()=>({hint:true,category:'check'}),onHint:job=>hints.push(job)});
delayed.observe('첫 번째 정보가 필요한 대화',{id:'first'});await tick();
delayed.complete(hints.at(-1),{displayed:true,text:'첫 정보'});
const afterFirst=hints.length;
delayed.observe('바로 다음 정보가 필요한 대화',{id:'next'});await tick();assert.equal(hints.length,afterFirst,'cooldown prevents repeated interventions');
clearTimeout(delayed.timer);now+=15001;await delayed.pump();assert.equal(hints.length,afterFirst+1);delayed.reset();
const errors=new HintGate({generationIntervalMs:0,settleMs:0,intervalMs:0,cooldownMs:0,decide:async()=>{throw Error('503');},onHint:()=>assert.fail('classifier errors must not trigger AI'),onStatus:text=>statuses.push(text)});
errors.observe('확인할 사실이 있는데',{id:'error'});await tick();assert.match(statuses.at(-1),/전사 계속/);errors.reset();gate.reset();
let windows=[];
const aggregate=new HintGate({generationIntervalMs:0,now:()=>now,settleMs:0,intervalMs:0,cooldownMs:0,decide:async job=>{windows.push(job);return{hint:false};},onHint:()=>assert.fail()});
aggregate.observe('자료에 유동성커버리지비율이 나왔어',{id:'term'});await tick();now+=2000;
aggregate.observe('다음 분기부터 적용한대',{id:'detail'});await tick();now+=2000;
aggregate.observe('응, 그렇구나',{id:'ack'});await tick();
assert.equal(windows.at(-1).text,'응, 그렇구나');assert.equal(windows.at(-1).context,'');
assert.equal(windows.at(-1).windowCount,1);
const displayJob={...windows.at(-1),hint:true};now+=30000;
assert.ok(aggregate.fresh(displayJob),'generation has its own longer display lifetime');now+=16000;assert.ok(!aggregate.fresh(displayJob));aggregate.reset();
let refusedCalls=0;
const refused=new HintGate({generationIntervalMs:0,settleMs:0,intervalMs:0,cooldownMs:0,decide:async()=>({hint:true,category:'reply'}),onHint:()=>{refusedCalls++;return false;}});
refused.observe('수락되지 않은 후보',{id:'refused-1'});await tick();refused.observe('수락되지 않은 후보',{id:'refused-2'});await tick();
assert.equal(refusedCalls,2,'a refused candidate must not be marked as already hinted');refused.reset();
let successful=[];
const lifecycle=new HintGate({generationIntervalMs:0,now:()=>now,settleMs:0,intervalMs:0,cooldownMs:15000,decide:async()=>({hint:true,category:'reply'}),onHint:job=>{successful.push(job);}});
lifecycle.observe('거절하고 싶은 제안',{id:'try1'});await tick();
assert.equal(lifecycle.recent.length,0,'acceptance must not count as display');
lifecycle.complete(successful.at(-1));assert.equal(lifecycle.recent.length,0);
clearTimeout(lifecycle.timer);now+=4001;
lifecycle.observe('거절하고 싶은 제안',{id:'try2'});await tick();assert.equal(successful.length,2,'failed candidate may retry after short backoff');
lifecycle.complete(successful.at(-1),{displayed:true,text:'이번에는 일정 때문에 어렵다고 말씀하세요.',topic:'제안'});
assert.equal(lifecycle.recent.length,1);assert.ok(lifecycle.duplicateCue('이번에는 일정 때문에 어렵다고 말씀하세요.'));
assert.ok(lifecycle.duplicateTopic('제안'));
assert.equal(lifecycle.shownTopics(),'제안');
assert.match(lifecycle.displayedContext(),/일정 때문에/);
const old={...successful.at(-1),hint:true};now+=13000;
lifecycle.observe('다음은 축구 경기 소식입니다',{id:'different1'});lifecycle.observe('축구팀 선수 이적 소식입니다',{id:'different2'});
lifecycle.confirmTopic(lifecycle.entries.at(-1),{topic:'축구',focus:'축구',continuity:'changed'});
assert.equal(lifecycle.relevant(old,'제안'),false,'confirmed topic change must reject the old scene');
assert.equal(lifecycle.relevant({...old,topicVersion:lifecycle.topicVersion},'축구'),true);
lifecycle.reset();
console.log('PASS: hint skip/selection, coalescing, cooldown, duplicate suppression, stale/cancelled decisions, isolated errors');

let budgetCalls=[];
const budget=new HintGate({now:()=>now,settleMs:0,intervalMs:0,cooldownMs:0,decide:async()=>({hint:true,category:'concept'}),onHint:j=>budgetCalls.push(j)});
budget.observe('첫 번째 후보',{id:'budget1'});await tick();
budget.observe('두 번째 다른 후보',{id:'budget2'});await tick();assert.equal(budgetCalls.length,1);
clearTimeout(budget.timer);now+=8001;await budget.pump();assert.equal(budgetCalls.length,2,'generation rate budget must bound LLM calls');budget.reset();

let focused=[];
const focusGate=new HintGate({settleMs:0,generationIntervalMs:0,intervalMs:0,cooldownMs:0,decide:async()=>({hint:true,category:'background',focus:'자료에 EBITDA가 나왔어'}),onHint:j=>focused.push(j)});
focusGate.observe('자료에 EBITDA가 나왔어\n네, 알겠습니다',{id:'focused'});await tick();
assert.equal(focused[0].text,'자료에 EBITDA가 나왔어');assert.match(focused[0].context,/네, 알겠습니다/);focusGate.reset();
const grounded=new HintGate({settleMs:0,generationIntervalMs:0,intervalMs:0,cooldownMs:0,decide:async()=>({hint:true,category:'background',focus:'전사에 없는 이름'}),onHint:j=>focused.push(j)});
grounded.observe('실제 전사 내용입니다',{id:'grounded'});await tick();assert.equal(focused.at(-1).text,'실제 전사 내용입니다');grounded.reset();

let busyCount=0,busyHints=[];
const busyGate=new HintGate({now:()=>now,settleMs:0,intervalMs:0,generationIntervalMs:0,cooldownMs:0,decide:async()=>++busyCount===1?{hint:false,reason:'embedding_busy'}:{hint:true,category:'explain'},onHint:j=>busyHints.push(j)});
busyGate.observe('일시적으로 바쁜 동안 들어온 전문 용어',{id:'busy'});await tick();
assert.equal(busyHints.length,0);clearTimeout(busyGate.timer);now+=4001;await busyGate.pump();
assert.equal(busyHints.length,1,'temporarily busy embedding must retry without another utterance');busyGate.reset();

let shortJobs=[];
const shortGate=new HintGate({now:()=>now,settleMs:0,intervalMs:0,decide:async job=>{shortJobs.push(job);return{hint:false};},onHint:()=>assert.fail()});
shortGate.observe('이전 주제는 이미 지나갔습니다',{id:'past'});await tick();now+=9000;
shortGate.observe('피라미드',{id:'current'});await tick();
assert.equal(shortJobs.at(-1).text,'피라미드','current topic detection must not require a long conversation');
assert.equal(shortJobs.at(-1).context,'','an unrelated old topic must not be supplied as context');
for(let i=0;i<4;i++){shortGate.observe(`지금 발화 ${i}`,{id:`short-${i}`});await tick();}
assert.equal(shortJobs.at(-1).windowCount,1);assert.ok(shortJobs.at(-1).text.length<=800);assert.ok(shortJobs.at(-1).context.length<=800);
shortGate.reset();

let unblocked=[];
const perFocusGate=new HintGate({now:()=>now,settleMs:0,intervalMs:0,generationIntervalMs:0,cooldownMs:15000,decide:async()=>({hint:true,category:'explain'}),onHint:j=>unblocked.push(j)});
perFocusGate.observe('실패한 주제 하나',{id:'fail-1'});await tick();
perFocusGate.complete(unblocked.at(-1));
perFocusGate.observe('완전히 새로운 주제 둘',{id:'other-1'});await tick();
assert.equal(unblocked.length,2,'different utterance must not be delayed by failed candidate backoff');
perFocusGate.reset();

let diversityNow=100000,diverse=[];
const diversity=new HintGate({now:()=>diversityNow,settleMs:0,intervalMs:0,generationIntervalMs:0,cooldownMs:8000,decide:async()=>({hint:true,category:'background'}),onHint:job=>diverse.push(job)});
diversity.observe('피라미드의 건립 목적에 대해 이야기했어',{id:'purpose'});await tick();
diversity.complete(diverse.at(-1),{displayed:true,topic:'피라미드',text:'피라미드는 왕실 무덤으로 건립되었습니다.'});
assert.ok(diversity.duplicateTopic('피라미드'),'short display cooldown remains');
diversityNow+=8001;
assert.equal(diversity.duplicateTopic('피라미드'),false,'topic must be eligible again after normal cooldown, not blocked for three minutes');
diversity.observe('피라미드의 돌을 옮기는 방법도 흥미롭네',{id:'construction'});await tick();
assert.equal(diverse.length,2,'a new aspect of the same topic must reach generation');
assert.ok(diversity.duplicateCue('피라미드는 왕실 무덤으로 건립되었습니다.'),'same fact remains suppressed for three minutes');
assert.equal(diversity.duplicateCue('석재 운반에는 썰매와 경사로가 이용된 것으로 추정됩니다.'),false);
assert.ok(diversity.duplicateHistory().some(text=>text.includes('왕실 무덤')),'history is available to comparison only');
diversityNow+=12000;assert.deepEqual(diversity.shownFocus(),[],'old source utterance no longer excludes a whole ongoing topic');
diversity.observe('피라미드의 건립 목적에 대해 이야기했어',{id:'revisit'});await tick();
assert.equal(diverse.length,3,'revisited source may supply another fact after short repetition window');diversity.reset();
console.log('PASS: ongoing topics allow new aspects after cooldown while duplicate facts, short repeats, and memory bounds remain protected');
let keywordJob;
const keywordGate=new HintGate({generationIntervalMs:0,settleMs:0,intervalMs:0,cooldownMs:0,
 decide:async()=>({hint:true,category:'background',focus:'청동기 시대에는 청동을 썼대.',keyword:'청동기시대',keywordSurface:'청동기 시대'}),onHint:job=>{keywordJob=job;}});
keywordGate.observe('청동기 시대에는 청동을 썼대.',{id:'dictionary'});await tick();
assert.equal(keywordJob.keyword,'청동기시대');assert.match(keywordJob.text,/청동을 썼대/);keywordGate.reset();
const inventedGate=new HintGate({generationIntervalMs:0,settleMs:0,intervalMs:0,cooldownMs:0,
 decide:async()=>({hint:true,category:'background',keyword:'없는명칭',keywordSurface:'등장하지않음'}),onHint:job=>{keywordJob=job;}});
inventedGate.observe('피라미드 이야기를 들었어.',{id:'invented'});await tick();assert.equal(keywordJob.keyword,undefined);inventedGate.reset();
console.log('PASS: canonical dictionary keyword retains original evidence; unobserved keyword evidence is rejected');
let busy=true,deferredHint;
const deferGate=new HintGate({generationIntervalMs:0,settleMs:0,intervalMs:0,cooldownMs:0,decide:async()=>busy?{hint:false,defer:true}:{hint:true,category:'background',plan:{topic:'피라미드',question:'건립 시기',source:'web',knownFacts:[]}},onHint:job=>{deferredHint=job;}});
deferGate.observe('피라미드가 왕의 무덤이래.',{id:'deferred'});await tick();assert.ok(deferGate.pending);assert.equal(deferredHint,undefined);
busy=false;clearTimeout(deferGate.timer);deferGate.pending.due=0;await deferGate.pump();assert.equal(deferredHint.hintPlan.topic,'피라미드');deferGate.reset();
let aspect='건립 시기',aspectTime=Date.now(),aspectJobs=[];
const aspectGate=new HintGate({now:()=>aspectTime,generationIntervalMs:0,settleMs:0,intervalMs:0,cooldownMs:0,decide:async()=>({hint:true,category:'background',focus:'피라미드',plan:{topic:'피라미드',question:aspect,source:'web',knownFacts:[]}}),onHint:job=>{aspectJobs.push(job);}});
aspectGate.observe('피라미드 건립에 관한 대화',{id:'aspect1'});await tick();aspectGate.complete(aspectJobs.at(-1),{displayed:true,text:'기자의 대피라미드 건립 시기',topic:'피라미드'});
aspectTime+=1000;aspect='석재 운반에 사용한 기술과 공법';aspectGate.observe('피라미드 돌을 운반한 방법',{id:'aspect2'});await tick();assert.equal(aspectJobs.length,2,'same quoted name must allow a genuinely different research aspect');aspectGate.complete(aspectJobs.at(-1),{displayed:true,text:'경사로와 수로 활용',topic:'피라미드'});
aspectTime+=1000;aspectGate.observe('피라미드 돌 운반 기술',{id:'aspect3'});await tick();assert.equal(aspectJobs.length,2,'same research aspect must still coalesce');aspectGate.reset();

// Stable preview revisions replace one utterance; corrected/withdrawn evidence is invalid.
let previewDecide,previewJobs=[];
const previews=new HintGate({settleMs:0,intervalMs:0,generationIntervalMs:0,cooldownMs:0,
 decide:(job,signal)=>new Promise(resolve=>{signal.addEventListener('abort',()=>resolve({hint:false}),{once:true});previewDecide=()=>resolve({hint:true,focus:'피라미드',plan:{topic:'피라미드',question:'건립 시기',source:'web'}});}),onHint:job=>previewJobs.push(job)});
previews.observe('피라미드 건립 시기가 궁금해',{id:'rev',preview:true});
previews.observe('피아노 연주에 대해 이야기하자',{id:'rev',preview:true});
previewDecide();await tick();assert.equal(previewJobs.length,0,'corrected preview cannot generate stale hint');
assert.equal(previews.entries.length,1);previews.reset();
const promoted=new HintGate({settleMs:0,intervalMs:0,generationIntervalMs:0,cooldownMs:0,decide:async()=>({hint:true,focus:'청동기 시대',plan:{topic:'청동기 시대',question:'지역별 시기',source:'web',kind:'term',evidence:'청동기 시대'}}),onHint:job=>previewJobs.push(job)});
promoted.observe('청동기 시대에 청동을 사용했대',{id:'same',preview:true});await tick();
const previewJob=previewJobs.at(-1);assert.equal(previewJob.hintPlan.kind,'term');
promoted.observe('청동기 시대에 청동을 사용했대',{id:'same'});assert.equal(promoted.entries.length,1);
assert.equal(promoted.relevant(previewJob,'청동기 시대'),true);
promoted.withdraw('same');assert.equal(promoted.relevant(previewJob,'청동기 시대'),false);promoted.reset();
promoted.observe('청동기 시대에 청동을 사용했대',{id:'short-correction',preview:true});await tick();
const correctedJob=previewJobs.at(-1);
promoted.validatePreview('short-correction','청동기 시대에 청동을 사용했대 더 이어지는 이야기');
assert.equal(promoted.relevant(correctedJob,'청동기 시대'),true);
promoted.validatePreview('short-correction','피아노');
assert.equal(promoted.relevant(correctedJob,'청동기 시대'),false,'a correction too short for a new judgment must still invalidate the old one');promoted.reset();
console.log('PASS: stable preview promotion, classification propagation, revision replacement and stale/withdrawn evidence rejection');

// New topics must never inherit an old topic as Latest, even while a hint is generating.
let topicNow=1000,selected=[];
const currentOnly=new HintGate({now:()=>topicNow,settleMs:0,intervalMs:0,cooldownMs:0,decide:async j=>{selected.push(j);return j.text.includes('청동기')?{hint:false,topic:'청동기 시대',focus:'청동기 시대',continuity:'unknown'}:j.text.includes('디지털')?{hint:false,topic:'디지털 시대',focus:'디지털 시대',continuity:'changed'}:{hint:false,topic:'디지털 시대',focus:'그게',continuity:'same'};},onHint:()=>assert.fail()});
currentOnly.observe('청동기 시대의 시작이 궁금하대',{id:'bronze'});await tick();
const bronzeJob={...selected.at(-1),topicVersion:currentOnly.topicVersion,hint:true};
topicNow+=1000;currentOnly.observe('이제 디지털 시대의 변화로 넘어갑니다.',{id:'digital'});await tick();
assert.equal(selected.at(-1).text,'이제 디지털 시대의 변화로 넘어갑니다.');
assert.equal(selected.at(-1).context,'');assert.equal(currentOnly.relevant(bronzeJob,'청동기 시대'),false,'no 12-second grace and no generic-word match');
topicNow+=1000;currentOnly.observe('그게 어떤 변화인가요?',{id:'reference'});await tick();
assert.match(selected.at(-1).context,/디지털 시대/);assert.ok(!selected.at(-1).context.includes('청동기'));
assert.equal(currentOnly.relevant(bronzeJob,'청동기 시대'),false,'a reference to the new topic cannot revive the old one');
currentOnly.complete({...bronzeJob,decisionKey:'청동기 시대 시작 시기'},{displayed:true,text:'청동기 시대는 지역별 시작 시기가 다릅니다.',topic:'청동기 시대'});
assert.equal(currentOnly.duplicateHistory().length,1);topicNow+=180001;assert.equal(currentOnly.duplicateHistory().length,0,'learning repetition becomes eligible again after the recent-history window');currentOnly.reset();
console.log('PASS: current-only topic evidence, immediate topic-switch rejection, reference-only context and expiring comparison history');

assert.equal(similarHint('행사는 11월 20일에 열립니다.','행사는 11월 15일에 열립니다.'),false);
assert.equal(similarHint('이 제품은 방수를 지원하지 않습니다.','이 제품은 방수를 지원합니다.'),false);

const retained=new HintGate({settleMs:0,intervalMs:0,decide:async()=>({hint:false}),onHint:()=>{}});
retained.complete({epoch:retained.epoch,sequence:1,text:'피라미드',focusText:'피라미드'},{displayed:true,text:'쿠푸왕의 무덤입니다.',topic:'피라미드'});
retained.reset();assert.equal(retained.duplicateHistory().length,1,'dismiss/MIC/wake cancellation must retain the recent comparison ledger');
retained.reset({forgetHistory:true});assert.equal(retained.duplicateHistory().length,0,'explicit scope/settings reset may clear history');

// Continuing speech need not repeat the entire topic name while generation runs.
let continuationNow=10000,continuationJobs=[],topicChanges=0;
const continuation=new HintGate({now:()=>continuationNow,settleMs:0,intervalMs:0,generationIntervalMs:0,cooldownMs:0,onTopicChange:()=>topicChanges++,
 decide:async j=>j.text.includes('내일 날씨')?{hint:false,topic:'내일 날씨',focus:'내일 날씨',continuity:'changed'}:j.text.includes('큰 돌')?{hint:false,topic:'피라미드',focus:'큰 돌',continuity:'same'}:{hint:true,focus:'피라미드',continuity:'unknown',plan:{topic:'피라미드',question:'석재 운반 기술',source:'web'}},onHint:j=>continuationJobs.push(j)});
continuation.observe('피라미드 건축이 대단하대',{id:'c1'});await tick();const firstContinuation=continuationJobs.at(-1);
continuationNow+=2000;continuation.observe('그렇게 큰 돌을 위로 옮겼다니 놀라워',{id:'c2'});await tick();
assert.equal(continuation.relevant(firstContinuation),true,'same-topic continuation must keep a completed hint displayable without a literal topic-name match');
assert.equal(topicChanges,0);
continuationNow+=2000;continuation.observe('이제 내일 날씨 이야기야',{id:'c3'});await tick();
assert.equal(continuation.relevant(firstContinuation),false);assert.equal(continuation.rejectionReason(firstContinuation),'topic_changed');assert.equal(topicChanges,1);
continuation.reset();
const timing=new HintGate({now:()=>continuationNow,decide:async()=>({hint:false}),onHint:()=>{}});
timing.complete({epoch:timing.epoch,sequence:1,text:'처음'},{displayed:true,text:'첫 힌트',topic:'처음'});
assert.equal(timing.displayDelay(),8000);continuationNow+=3000;assert.equal(timing.displayDelay(),5000);continuationNow+=5000;assert.equal(timing.displayDelay(),0);timing.reset();

const scopedTopic=new HintGate({settleMs:0,intervalMs:0,decide:async()=>({hint:false}),onHint:()=>{}});
scopedTopic.confirmTopic({id:'one',sequence:1,text:'피라미드'},{topic:'피라미드',focus:'피라미드',continuity:'unknown'});
const beforeRegionalChange=scopedTopic.topicVersion;
scopedTopic.confirmTopic({id:'two',sequence:2,text:'이제 이집트 피라미드와는 다른 지역 이야기'},{topic:'이집트 피라미드와 다른 지역',focus:'다른 지역',continuity:'changed'});
assert.ok(scopedTopic.topicVersion>beforeRegionalChange,'an explicit changed decision takes precedence over a shared name prefix');scopedTopic.reset();
