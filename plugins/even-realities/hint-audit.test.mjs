import assert from 'node:assert/strict';
import {HintAudit} from './hint-audit.mjs';
let sent=[],timers=[],now=10000,spec={server:'http://one',headers:{Authorization:'Bearer one'}};
const history=new HintAudit({version:'test',spec:()=>spec,now:()=>now,schedule:fn=>{timers.push(fn);return fn;},cancel:()=>{},fetch:async(url,o)=>{sent.push({url,body:JSON.parse(o.body)});return {ok:true};}});
for(let i=0;i<70;i++)history.record({jobId:'job'+i,stage:'decision',text:'should not store',topic:'topic'});
assert.equal(history.pending.length,64);await history.flush();while(history.active)await new Promise(r=>setImmediate(r));
assert.ok(sent.length);assert.ok(sent.every(x=>x.body.events.length<=16));assert.ok(sent.every(x=>x.body.events.every(e=>!e.text)));assert.ok(sent.every(x=>!JSON.stringify(x.body).includes('Bearer')));
history.record({jobId:'card',stage:'phone_display',text:'shown body'});await history.flush();while(history.active)await new Promise(r=>setImmediate(r));assert.equal(sent.at(-1).body.events[0].text,'shown body');
history.record({jobId:'old',stage:'decision'});spec={server:'http://two',headers:{Authorization:'Bearer two'}};history.record({jobId:'new',stage:'decision'});await history.flush();while(history.active)await new Promise(r=>setImmediate(r));assert.ok(sent.at(-1).url.startsWith('http://two'));assert.equal(sent.at(-1).body.events.length,1);assert.equal(sent.at(-1).body.events[0].jobId,'new');
let timeouts=[],status;
const hung=new HintAudit({version:'test',spec:()=>spec,schedule:fn=>{timeouts.push(fn);return fn;},cancel:()=>{},now:()=>now,onStatus:s=>status=s,fetch:()=>new Promise(()=>{})});
hung.record({jobId:'hung',stage:'selected'});const flush=hung.flush();timeouts.at(-1)();await flush;assert.equal(hung.active,false);assert.match(status,/시간 초과/);hung.record({jobId:'during-backoff',stage:'selected'});assert.equal(hung.pending.length,0);now+=30001;hung.record({jobId:'recover',stage:'selected'});assert.equal(hung.pending.length,1);
console.log('PASS: bounded asynchronous hint history, stage bodies, credential isolation, account scope and uncooperative timeout/backoff');

history.record({jobId:'diagnostic',stage:'decision_response',reason:'judge_invalid_response',text:JSON.stringify({response:'not json',elapsedMs:25})});
await history.flush();while(history.active)await new Promise(r=>setImmediate(r));
assert.ok(sent.some(x=>x.body.events.some(e=>e.stage==='decision_response' && e.text.includes('elapsedMs'))));
