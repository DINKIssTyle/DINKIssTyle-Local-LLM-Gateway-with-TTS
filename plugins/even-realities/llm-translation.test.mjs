import assert from 'node:assert/strict';
import {llmTranslation as sendTranslation,stableTranslationPreview} from './llm-translation.mjs';
import {expandGatewayRequest} from './test-support/gateway-prompts.mjs';
const llmTranslation=options=>sendTranslation({...options,fetch:(url,opts)=>options.fetch(url,{...opts,body:JSON.stringify(expandGatewayRequest(JSON.parse(opts.body)))})});
assert.equal(stableTranslationPreview('Hello. unfinished','Hello. '),'Hello.');
assert.equal(stableTranslationPreview('No punctuation yet','No punctuation yet'),'');
assert.equal(stableTranslationPreview('First. Next. Tail','First. Next. '),'First. Next.');
let sent,reads=0,cancels=0;const partial=[];
const result=await llmTranslation({server:'http://gateway',headers:{},model:'translation-role',job:{draft:true,prompt:'Hello.',source:'en',target:'ko'},onPartial:t=>partial.push(t),fetch:async(url,opts)=>{
 sent=JSON.parse(opts.body);return {ok:true,body:{getReader:()=>({cancel:async()=>{cancels++;},read:async()=>{
  if(reads++===0)return {done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:'{"translation":"안녕'})+'\n\n')};
  if(reads===2)return {done:false,value:Buffer.from('data: '+JSON.stringify({type:'request.complete',final_assistant_content:'{"translation":"안녕하세요."}'})+'\n\n')};
  throw Error('must not read after completion');
 }})}};
}});
assert.equal(result.ok,true);assert.equal(result.text,'안녕하세요.');assert.equal(sent.glasses_task,'translation_preview');assert.equal(sent.chat_template_kwargs.enable_thinking,false);assert.equal(sent.model,'translation-role');assert.equal(cancels,1);assert.deepEqual(partial,['안녕','안녕하세요.']);
const busy=await llmTranslation({job:{prompt:'hi'},fetch:async()=>({status:429}),server:'',headers:{}});assert.equal(busy.busy,true);
const cancelled=new AbortController();cancelled.abort();const aborted=await assert.rejects(()=>llmTranslation({job:{},signal:cancelled.signal,fetch:async(_,o)=>{assert.equal(o.signal.aborted,true);throw Error('aborted');}}),/aborted/);
console.log('PASS: stable sentence-only previews, streamed translations, terminal completion, dedicated role/policy, busy and cancellation');
const incomplete=await llmTranslation({job:{prompt:'test'},fetch:async()=>({ok:true,json:async()=>({response:'{"translation":"unfinished'})})});
assert.equal(incomplete.ok,false,'truncated JSON cannot be cached as a completed sentence');
