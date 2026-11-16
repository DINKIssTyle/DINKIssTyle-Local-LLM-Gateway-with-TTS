import assert from 'node:assert/strict';
import {cuePrompts,parseCueResponse,fitCue,hintPlanPrompts,parseHintPlan,hintAnswerPrompts,hintResultKind,extractHintFact,readHintPlanResponse} from './hint-enrichment.mjs';
const observed='피라미드 영상이 나오네. 거기 청동기 시대 얘기도 나오더라.';
const plan=parseHintPlan(JSON.stringify({topic:'피라미드',question:'이집트 피라미드는 어떤 목적으로 건립되었나?',knownFacts:['피라미드 영상이 나옴'],source:'internal'}));
assert.equal(plan.source,'internal');
assert.match(hintPlanPrompts(observed,'','background').systemPrompt,/Do not summarize/);
for(const prompts of [hintPlanPrompts(observed,'','background'),hintAnswerPrompts(plan),cuePrompts(observed,'','background'),cuePrompts(observed,'','background','',{preferWeb:true}),cuePrompts(observed,'','reply')]){
 assert.match(prompts.systemPrompt,/Role: autonomous information provider/);
 assert.match(prompts.systemPrompt,/not a user request/);
 assert.match(prompts.systemPrompt,/question field is an internal research task/);
}
assert.match(hintAnswerPrompts(plan).userPrompt,/Internal research task \(not a user question\)/);
const prior='[피라미드] 피라미드는 왕실 무덤으로 건립되었습니다.';
for(const prompts of [cuePrompts('피라미드 건축 방식이 궁금한 대화','','background',prior),cuePrompts('피라미드 건축 방식이 궁금한 대화','','background',prior,{preferWeb:true}),hintAnswerPrompts(plan,prior)]){
 assert.ok(!prompts.userPrompt.includes(prior),'old hint text must not enter generative prompts');
 assert.match(prompts.systemPrompt,/New information is allowed on the same topic/);
 assert.match(prompts.systemPrompt,/Avoid repeating the same fact in different words/);
 assert.doesNotMatch(prompts.userPrompt,/제외할 직전 주제|다시 다루지 말 것|새로운 토픽만/,'do not exclude an entire topic');
}
assert.doesNotMatch(hintAnswerPrompts(plan).userPrompt,/청동기 시대 얘기도/,'raw transcript must not be the final answer task');
assert.match(hintAnswerPrompts(plan).userPrompt,/research task/);
assert.equal(parseHintPlan('HINT_SKIP'),null);assert.equal(parseHintPlan('{"source":"execute_command"}'),null);
assert.equal(extractHintFact(JSON.stringify({fact:observed,adds_information:true}),observed,plan),'','copied transcription must not be a hint');
assert.equal(extractHintFact(JSON.stringify({fact:'피라미드에 대해 이야기하고 있습니다.',adds_information:true}),observed,plan),'','generic summaries must be suppressed');
assert.equal(extractHintFact(JSON.stringify({fact:'피라미드 영상이 나옴',adds_information:true}),observed,plan),'','known facts must not be repeated');
assert.equal(extractHintFact('피라미드 이야기를 요약합니다.',observed,plan),'','unstructured output must not bypass checks');
assert.equal(extractHintFact(JSON.stringify({fact:'새 정보',adds_information:false}),observed,plan),'');
assert.ok(extractHintFact(JSON.stringify({fact:'이집트 피라미드는 왕실 무덤으로 건립되었습니다.',adds_information:true}),observed,plan));
assert.equal(parseHintPlan('```json\n'+JSON.stringify(plan)+'\n```').topic,'피라미드');
let index=0;const content=JSON.stringify(plan);
const chunks=['data: '+JSON.stringify({type:'message.delta',content:content.slice(0,20)})+'\n\n','data: '+JSON.stringify({type:'message.delta',content:content.slice(20)})+'\n\n','data: '+JSON.stringify({type:'request.complete',final_assistant_content:content})+'\n\n'];
const reply={ok:true,body:{getReader:()=>({read:async()=>index<chunks.length?{done:false,value:Buffer.from(chunks[index++])}:{done:true}})}};
assert.equal(await readHintPlanResponse(reply),content,'planning must handle native deltas and final replacement');
console.log('PASS: missing-information planning, evidence routes, no raw transcript in final task, summary/echo rejection, structured fact extraction');

const cue={topic:'제안',cue_type:'suggestion',source:'internal',fact:'“금요일까지 검토하고 답드려도 될까요?”라고 물어보세요.',adds_information:true};
assert.match(cuePrompts('제안을 거절하고 싶어','','reply').userPrompt,/possible spoken reply/);
assert.ok(parseCueResponse(JSON.stringify(cue),'제안을 거절하고 싶어').fact);
assert.equal(parseCueResponse(JSON.stringify({...cue,source:'web'}),'전사'),null,'external facts require evidence research');
assert.equal(parseCueResponse(JSON.stringify({...cue,cue_type:'unknown'}),'전사'),null);
assert.equal(parseCueResponse('{"skip":true}','전사'),null);
assert.equal(parseCueResponse(JSON.stringify({...plan,source:'web'}),'전사').source,'web');
assert.equal(fitCue('짧은 완결 문장입니다. '+ '너무 긴 문장 '.repeat(100)+'.'),'짧은 완결 문장입니다.');
assert.equal(fitCue('너무 긴 문장 '.repeat(100)), '','never show a cut-off sentence');
assert.ok(!cuePrompts('최근 발화','이전 문맥','background','이미 나온 도움').userPrompt.includes('이미 나온 도움'));
console.log('PASS: single-pass typed cues, evidence-only research, complete sentences in six-line display');

const web=cuePrompts('그 제품 무게 얘기야','맥북 에어 M4','background','',{preferWeb:true});
assert.match(web.systemPrompt,/one specific/);
assert.match(web.systemPrompt,/web for/);
assert.match(web.systemPrompt,/current conversation|Observed speech/);
assert.doesNotMatch(cuePrompts('거절하고 싶어','','reply','',{preferWeb:true}).systemPrompt,/웹검색 우선 설정입니다/);
assert.equal(parseCueResponse('다음은 결과입니다:\n'+JSON.stringify(cue),'전사').topic,'제안');
assert.equal(extractHintFact('확인 결과:\n'+JSON.stringify({suggestion:'금요일까지 검토 후 회신드리겠습니다.',adds_information:true}),observed,plan),'금요일까지 검토 후 회신드리겠습니다.');
assert.equal(extractHintFact(JSON.stringify({fact:'네, 질문하신 기자의 대피라미드는 쿠푸왕의 무덤입니다. 도움이 되셨길 바랍니다.',adds_information:true}),observed,plan),'기자의 대피라미드는 쿠푸왕의 무덤입니다.');
for(const preferWeb of [false,true])assert.match(cuePrompts('청동기 시대에는 청동을 사용했대.','','background','',{preferWeb,keyword:'청동기시대'}).userPrompt,/dictionary\/JEV suggestion.*청동기시대/);
const wrappedCue={topic:'피라미드',cue_type:'concept',question:'피라미드의 건축 기술',knownFacts:['왕의 무덤'],source:'web'};
assert.equal(parseCueResponse('```json\n'+JSON.stringify({result:JSON.stringify(wrappedCue)})+'\n```','피라미드가 왕의 무덤이래').topic,'피라미드');
assert.ok(extractHintFact(JSON.stringify({result:JSON.stringify({fact:'기자의 대피라미드는 쿠푸왕을 위해 건립됐습니다.',adds_information:true})}),'피라미드가 왕의 무덤이래',{knownFacts:[]}));

// A writer may correct the planning label to match the actual information.
assert.equal(hintResultKind('{"fact":"돌을 올리는 원리","kind":"key"}','term'),'key');
assert.equal(hintResultKind('```json\n{"result":"{\\\"kind\\\":\\\"verify\\\"}"}\n```','term'),'verify');
assert.equal(hintResultKind('{"kind":"unsupported"}','verify'),'verify');
assert.equal(hintResultKind('{"fact":"legacy writer"}','term'),'term');
assert.equal(hintResultKind('HINT_SKIP',undefined),'key');
for(const prompts of [hintPlanPrompts(observed,'','background'),hintAnswerPrompts(plan),cuePrompts(observed,'','background')]){
 assert.match(prompts.systemPrompt,/recognition errors|Automatic transcripts/);
 assert.match(prompts.systemPrompt,/negation/);
 assert.match(prompts.systemPrompt,/Ignore only the uncertain portion/);
}
assert.match(hintAnswerPrompts(plan).systemPrompt,/Do not change the task/);
assert.match(hintAnswerPrompts(plan).userPrompt,/not verified/);

assert.equal(parseCueResponse(JSON.stringify({...cue,kind:'key'}),'전사').kind,'key');
assert.equal(parseHintPlan(JSON.stringify({...plan,kind:'verify'})).kind,'verify');
assert.match(cuePrompts(observed,'','background').systemPrompt,/Provide information missing/);

for(const source of ['internal','conversation'])assert.match(hintAnswerPrompts({...plan,source,kind:source==='conversation'?'recap':'term'}).systemPrompt,/500 Unicode characters/);
const conversationPlan={topic:'회의 준비',source:'conversation',kind:'schedule',question:'회의 날짜와 준비물 정리',knownFacts:['금요일 3시 회의, 노트북 지참']};
assert.equal(parseCueResponse(JSON.stringify(conversationPlan),'금요일 3시 회의'),null,'automatic conversation-only plans are disabled');

assert.equal(extractHintFact(JSON.stringify({fact:'The conversation is about pyramid construction.',adds_information:true}),observed,plan),'','English meta summaries are not information');
assert.equal(parseHintPlan(JSON.stringify({...plan,source:'conversation',kind:'key'})),null);
assert.equal(parseHintPlan(JSON.stringify({...plan,source:'internal',kind:'recap'})),null);

assert.equal(extractHintFact(JSON.stringify({fact:'피라미드에 대한 구체적인 기록이 존재함.',adds_information:true}),observed,plan),'');
