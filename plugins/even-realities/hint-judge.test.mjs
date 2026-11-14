import assert from 'node:assert/strict';
import {suggestedHintJudge,normalizeHintBudget,estimateHintTokens,compactJudgeContext,parseHintJudgment,requestHintJudgment} from './hint-judge.mjs';
assert.equal(normalizeHintBudget(1),300);assert.equal(normalizeHintBudget(9999),800);assert.equal(normalizeHintBudget(undefined),600);
for(const budget of [300,400,500,600,700,800])for(const sample of ['청동기 시대가 언제였는지 궁금하다. ','MacBook Air M4 weight 13-inch and 15-inch. ','🕶️こんにちは 한국어 English 123? ']){
 const c=compactJudgeContext({text:sample.repeat(100),context:sample.repeat(100)},sample.repeat(100),[{term:'피라미드'}],budget);
 assert.ok(c.estimatedTokens<=budget,JSON.stringify(c));assert.ok(c.latest);assert.ok(c.latest.endsWith(sample.trim()));
 assert.ok(estimateHintTokens('한국어한국어')>estimateHintTokens('abcdef'));
}
const context=compactJudgeContext({text:'피라미드가 왕의 무덤이래.',context:''});
const value={hint:true,topic:'피라미드',evidence:'피라미드',question:'쿠푸왕 피라미드 건립 시기와 구조',source:'web'};
const valid=parseHintJudgment(JSON.stringify(value),context);
assert.equal(valid.plan.source,'web');assert.equal(valid.focus,'피라미드');assert.ok(valid.plan.knownFacts.includes(context.latest));
assert.deepEqual(parseHintJudgment('```json\n'+JSON.stringify({result:JSON.stringify(value)})+'\n```',context),valid);
assert.equal(parseHintJudgment(JSON.stringify({...value,evidence:'없는 원문'}),context),null);
assert.equal(parseHintJudgment(JSON.stringify({...value,source:'execute_command'}),context),null);
assert.equal(parseHintJudgment('{"hint":false}',context).hint,false);
assert.equal(parseHintJudgment('not json',context),null);
let sent;
const result=await requestHintJudgment({fetch:async(url,opts)=>{sent=JSON.parse(opts.body);return {ok:true,json:async()=>({choices:[{message:{content:JSON.stringify(value)}}]})};},server:'http://test',headers:{},model:'small',job:{text:context.latest,context:''},advisory:{hint:false},limit:300});
assert.equal(result.hint,true,'negative JEV advice cannot veto LLM judgment');assert.equal(sent.glasses_hint_phase,'judge');assert.equal(sent.model,'small');assert.equal(sent.max_tokens,192);assert.equal(sent.store,false);assert.equal(sent.chat_template_kwargs.enable_thinking,false);
assert.ok(result.contextEstimatedTokens<=300);assert.equal(sent.response_format.type,'json_schema');
await assert.rejects(()=>requestHintJudgment({fetch:async(url,opts)=>new Promise((resolve,reject)=>opts.signal.addEventListener('abort',()=>reject(Object.assign(Error('aborted'),{name:'AbortError'})))),model:'small',job:{text:'피라미드',context:''},timeoutMs:10}),/aborted/);
const controller=new AbortController();controller.abort();
await assert.rejects(()=>requestHintJudgment({fetch:async(url,opts)=>{assert.equal(opts.signal.aborted,true);throw Error('cancelled');},model:'small',job:{text:'피라미드'},signal:controller.signal}),/cancelled/);
console.log('PASS: 300–800 multilingual context budgets, bounded tool-free judge, grounded plans, nested JSON, JEV-independent selection and cancellation');

assert.equal(suggestedHintJudge([{id:'qwen3.5-0.8b-mlx',label:'0.8B'},{id:'gemma-4-e2b-it-mlx',label:'Gemma'}]),'gemma-4-e2b-it-mlx');
assert.equal(suggestedHintJudge([{id:'qwen3.5-0.8b-mlx',label:'0.8B'}]),'');
