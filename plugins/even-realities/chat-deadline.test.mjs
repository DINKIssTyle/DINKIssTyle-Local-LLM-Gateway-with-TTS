import assert from 'node:assert/strict';
import {chatDeadline,chatFailureMessage} from './chat-deadline.mjs';
let now=0,id=0;const timers=new Map();
const setTimer=(fn,ms)=>{timers.set(++id,{at:now+ms,fn});return id;};
const clearTimer=key=>timers.delete(key);
function advance(ms){const until=now+ms;while(true){const next=[...timers].sort((a,b)=>a[1].at-b[1].at)[0];if(!next || next[1].at>until)break;now=next[1].at;timers.delete(next[0]);next[1].fn();}now=until;}
const c=new AbortController();const d=chatDeadline(c,{chatSession:true,setTimer,clearTimer});
advance(45001);assert.equal(c.signal.aborted,false,'explicit chat must survive the old 45-second limit');
advance(200000);d.touch();advance(80000);assert.equal(c.signal.aborted,false,'reasoning/tool/heartbeat activity is valid progress');
advance(4999);assert.equal(c.signal.aborted,true,'total deadline bounds waiting even with continuous events');
assert.match(chatFailureMessage(new TypeError('Load failed'),d),/5분 30초/,'timeout must have a precise cause');d.stop();assert.equal(timers.size,0);
const idle=new AbortController();const idleDeadline=chatDeadline(idle,{chatSession:true,setTimer,clearTimer});
advance(299999);assert.equal(idle.signal.aborted,false);advance(1);assert.equal(idle.signal.aborted,true);assert.match(idleDeadline.reason(),/5분 동안/);idleDeadline.stop();
for(const [hint,ms] of [[true,30000],[false,45000]]){
 const controller=new AbortController(),deadline=chatDeadline(controller,{hint,setTimer,clearTimer});
 advance(ms-1);assert.equal(controller.signal.aborted,false);advance(1);assert.equal(controller.signal.aborted,true);deadline.stop();
}
const manual=new AbortController(),manualDeadline=chatDeadline(manual,{chatSession:true,setTimer,clearTimer});manual.abort();manualDeadline.stop();
assert.equal(manualDeadline.reason(),'','manual cancellation is not a timeout');
assert.match(chatFailureMessage(new TypeError('Load failed'),manualDeadline),/게이트웨이·모델 서버/);
assert.equal(chatFailureMessage(Error('LM_STUDIO_AUTH_ERROR'),manualDeadline),'LM_STUDIO_AUTH_ERROR');assert.equal(timers.size,0);
console.log('PASS: explicit chat survives 45 seconds, tracks stream activity, bounds total/idle wait, cleans timers and distinguishes cancellation/network/server errors');
