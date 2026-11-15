import assert from 'node:assert/strict';
import {LiveTranslationQueue} from './live-translation.mjs';
const tick=()=>new Promise(resolve=>setImmediate(resolve));
let finish,requests=[],shown=[],finals=[];
const q=new LiveTranslationQueue({intervalMs:0,translate:async job=>{
 requests.push(job.prompt);
 if(job.prompt==='first')return await new Promise(resolve=>{finish=resolve;});
 return {ok:true,text:'translated '+job.prompt};
},onText:(j,text)=>shown.push({draft:j.draft,text}),onFinal:(j,r)=>finals.push(j.prompt)});
const job=(prompt,extra={})=>({id:'utterance',prompt,source:'en',target:'ko',draft:true,...extra});
q.submit(job('first'));q.submit(job('second'));q.submit(job('latest'));
assert.deepEqual(requests,['first']);
finish({ok:true,text:'first draft'});await tick();await tick();
assert.deepEqual(requests,['first','latest'],'keep only newest pending draft');
assert.deepEqual(finals,[],'drafts must not generate reply examples');
q.submit(job('corrected final',{draft:false}));await tick();
assert.equal(shown.at(-1).text,'translated corrected final');assert.deepEqual(finals,['corrected final']);
assert.equal(q.submit(job('late draft')),false,'interim result cannot replace a final');
assert.equal(q.submit(job('corrected final',{draft:false})),false,'duplicate final is idempotent');
q.submit(job('revised final',{draft:false}));await tick();assert.equal(finals.at(-1),'revised final');
q.reset();shown=[];finals=[];requests=[];
q.submit(job('first'));q.submit(job('authoritative',{draft:false}));finish({ok:true,text:'stale provisional'});await tick();await tick();
assert.ok(shown.every(v=>v.text!=='stale provisional'),'late draft cannot overwrite requested final');assert.deepEqual(finals,['authoritative']);
q.reset();shown=[];q.submit(job('first'));q.reset();finish({ok:true,text:'cancelled'});await tick();assert.equal(shown.length,0,'mode/session reset rejects old result');
let attempts=0;
const retry=new LiveTranslationQueue({intervalMs:0,translate:async()=>++attempts===1?{ok:false,reason:'busy'}:{ok:true,text:'recovered'},onText:(j,t)=>shown.push(t),onFinal:()=>assert.fail('failed draft must not call the LLM')});
retry.submit(job('same'));await tick();retry.submit(job('same'));await tick();assert.equal(attempts,2);assert.equal(shown.at(-1),'recovered');retry.reset();
let now=0,limited=[];
const throttled=new LiveTranslationQueue({now:()=>now,intervalMs:500,translate:async j=>{limited.push(j.prompt);return{ok:true,text:j.prompt};},onText:()=>{},onFinal:()=>{}});
throttled.submit(job('one'));await tick();throttled.submit(job('two'));throttled.submit(job('three'));assert.deepEqual(limited,['one']);
now=501;clearTimeout(throttled.timer);throttled.timer=null;await throttled.pump();assert.deepEqual(limited,['one','three']);throttled.reset();q.reset();
console.log('PASS: live drafts coalesce, final correction replaces, no interim LLM calls, stale/reset rejection, bounded retry and rate');
// A hung transport need not cooperate with abort for a new segment to run.
let obsoleteSignal,oldFinish;const latestShown=[];
const fresh=new LiveTranslationQueue({intervalMs:0,translate:(j,s)=>{
 if(j.id==='old'){obsoleteSignal=s;return new Promise(resolve=>oldFinish=resolve);}
 return Promise.resolve({ok:true,text:j.prompt});
},onText:(j,t)=>latestShown.push(t),onFinal:()=>{}});
fresh.submit(job('obsolete',{id:'old',startedAt:'2026-11-14T01:00:00Z'}));
fresh.submit(job('current',{id:'new',startedAt:'2026-11-14T01:00:05Z'}));
await tick();await tick();
assert.equal(obsoleteSignal.aborted,true);assert.deepEqual(latestShown,['current'],'new caption bypasses obsolete translation timeout');
assert.equal(fresh.submit(job('late saved final',{id:'old',draft:false,startedAt:'2026-11-14T01:00:00Z'})),false,'archived old audio cannot move live display backwards');
oldFinish({ok:true,text:'late old translation'});await tick();assert.deepEqual(latestShown,['current']);fresh.reset();
console.log('PASS: obsolete translation aborts immediately and late archive finals cannot displace newer live captions');
// Throttle starts, not finishes: an 800ms translation doesn't add another 500ms.
let clock=0,releaseSlow,started=[];
const startLimited=new LiveTranslationQueue({intervalMs:250,now:()=>clock,translate:j=>{started.push(clock);if(j.prompt==='slow')return new Promise(r=>releaseSlow=r);return Promise.resolve({ok:true,text:j.prompt});},onText:()=>{},onFinal:()=>{}});
startLimited.submit(job('slow'));clock=100;startLimited.submit(job('next'));clock=800;releaseSlow({ok:true,text:'slow'});await tick();assert.deepEqual(started,[0,800]);startLimited.reset();
let busyCalls=0,busyFinals=[];const busyClock={value:0};
const busyQueue=new LiveTranslationQueue({now:()=>busyClock.value,intervalMs:0,translate:async()=>++busyCalls===1?{ok:false,busy:true}:{ok:true,text:'done'},onText:()=>{},onFinal:(j,r)=>busyFinals.push(r)});
busyQueue.submit(job('final',{draft:false}));await tick();assert.equal(busyFinals.length,0,'busy must not prematurely invoke fallback LLM');
busyClock.value=151;clearTimeout(busyQueue.timer);busyQueue.timer=null;await busyQueue.pump();assert.equal(busyFinals[0].ok,true);busyQueue.reset();
let stalePublish,releaseStale,seen=[];
const partialQueue=new LiveTranslationQueue({intervalMs:0,translate:(j,s,publish)=>{if(j.id==='old'){stalePublish=publish;return new Promise(r=>releaseStale=r);}return Promise.resolve({ok:true,text:'new'});},onText:(j,t)=>seen.push(t),onFinal:()=>{}});
partialQueue.submit(job('old',{id:'old'}));partialQueue.submit(job('new',{id:'new'}));await tick();stalePublish('obsolete');releaseStale({ok:true,text:'obsolete'});await tick();assert.deepEqual(seen,['new']);partialQueue.reset();
console.log('PASS: start-based throttle, final busy retry and stale streamed text suppression');
