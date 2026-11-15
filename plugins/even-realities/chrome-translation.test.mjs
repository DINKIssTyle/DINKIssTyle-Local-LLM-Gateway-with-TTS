import assert from 'node:assert/strict';
import {localTranslate,chromeTranslate,translationReplyPrompts} from './chrome-translation.mjs';
let request;
const success=await chromeTranslate(async(url,options)=>{request={url,options};return {ok:true,json:async()=>({status:'ready',text:' Hello ',source:'ko',target:'en'})};},'https://gateway',{'Authorization':'Bearer test'}, {text:'안녕',source:'ko',target:'en'});
assert.equal(success.text,'Hello');assert.equal(request.url,'https://gateway/api/translate/chrome');
assert.equal(request.options.headers.Authorization,'Bearer test');assert.deepEqual(JSON.parse(request.options.body),{text:'안녕',source:'ko',target:'en'});
const fallback=await chromeTranslate(async()=>({ok:false,json:async()=>({status:'needs_prepare',error:'준비 필요'})}),'',{},{});
assert.equal(fallback.ok,false);assert.match(fallback.reason,/준비 필요/);
const timed=await chromeTranslate((url,{signal})=>new Promise((resolve,reject)=>signal.addEventListener('abort',()=>reject(new Error('aborted')),{once:true})),'',{}, {},undefined,5);
assert.equal(timed.ok,false);assert.match(timed.reason,/시간 초과/);
const controller=new AbortController();controller.abort();
await assert.rejects(chromeTranslate(()=>assert.fail('must not fetch'),' ',{}, {},controller.signal),{name:'AbortError'});
const prompts=translationReplyPrompts('안녕','Hello','ko','en');
assert.match(prompts.systemPrompt,/translation 필드는 출력하지 마세요/);assert.match(prompts.userPrompt,/이미 완료된 번역[\s\S]*Hello/);
console.log('PASS: authenticated Chrome bridge, bounded fallback, cancellation and reply-only prompts');

let appleURL;
const apple=await localTranslate(async(url)=>{appleURL=url;return{ok:true,json:async()=>({status:'ready',text:'Apple text'})}},'https://gateway',{}, {source:'ko',target:'en',text:'안녕'},undefined,1000,'apple');
assert.equal(appleURL,'https://gateway/api/translate/apple');assert.equal(apple.text,'Apple text');
const busyResult=await localTranslate(async()=>({status:429,ok:false,json:async()=>({status:'busy',error:'processing'})}),'',{},{});
assert.equal(busyResult.busy,true);assert.notEqual(busyResult.needsPrepare,true);
const unavailable=await localTranslate(async()=>({status:503,ok:false,json:async()=>({status:'needs_prepare'})}),'',{},{});assert.equal(unavailable.needsPrepare,true);
const transient=await localTranslate(async()=>({status:500,ok:false,json:async()=>({status:'error',error:'temporary'})}),'',{},{});assert.equal(transient.needsPrepare,false);

const needsApple=await localTranslate(async()=>({status:503,ok:false,json:async()=>({status:'error',error:'서버에서 Apple 번역 준비를 먼저 실행해 주세요'})}),'',{}, {},undefined,1000,'apple');assert.equal(needsApple.needsPrepare,true);
