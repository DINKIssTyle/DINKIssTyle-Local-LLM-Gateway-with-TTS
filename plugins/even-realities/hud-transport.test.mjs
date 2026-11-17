import assert from 'node:assert/strict';
import {HUDTransport} from './hud-transport.mjs';
import {abortableWait} from './abortable-wait.mjs';
let now=0,ready=0;
const calls=[];
const transport=new HUDTransport({intervalMs:150,timeoutMs:20,now:()=>now,sleep:async ms=>{now+=ms;},onReady:()=>ready++});
// Thirty virtual minutes of rapid caption/layout changes: all native operations
// (including rebuilds) share pacing and never overlap.
let active=0;
for(let i=0;i<12000;i++)await transport.call(async()=>{
  assert.equal(++active,1);calls.push(now);await Promise.resolve();active--;return true;
});
assert.equal(now,1799850);
assert.ok(calls.slice(1).every((at,i)=>at-calls[i]>=150));
let release;
await assert.rejects(transport.call(()=>new Promise(resolve=>{release=resolve;})),/시간 초과/);
assert.equal(transport.ready,false);
let invoked=false;
await assert.rejects(transport.call(()=>{invoked=true;}),/연결 대기/);
assert.equal(invoked,false); // Never add another native request to a stuck bridge.
transport.setConnected(false);transport.setConnected(true);
assert.equal(transport.ready,false); // Reconnect cannot overlap an old native call.
release(true);await new Promise(resolve=>setImmediate(resolve));
assert.equal(transport.ready,true);assert.equal(ready,1);
let releaseDisconnected;
const inFlight=transport.call(()=>new Promise(resolve=>{releaseDisconnected=resolve;}));
await new Promise(resolve=>setImmediate(resolve));
transport.setConnected(false);
await assert.rejects(inFlight,/해제/);
assert.equal(transport.ready,false);
releaseDisconnected(true);await new Promise(resolve=>setImmediate(resolve));
transport.setConnected(true);assert.equal(transport.ready,true);
assert.equal(await transport.call(()=>true),true);
// A WebView stream that ignores abort must not hold the AI application queue.
const controller=new AbortController();let canceled=0;
const stalled=abortableWait(new Promise(()=>{}),controller.signal,()=>canceled++);
controller.abort();await assert.rejects(stalled,{name:'AbortError'});assert.equal(canceled,1);
const late=Promise.reject(Error('late native failure'));
await assert.rejects(abortableWait(late,controller.signal),{name:'AbortError'});
console.log('HUD pacing, 30-minute virtual soak, stalled SDK, reconnect and ignored abort passed');
