import assert from 'node:assert/strict';
import test from 'node:test';
import {StartupSTT} from './startup-stt.mjs';

test('saved login and first login both wait for bridge and restored settings', async()=>{
 for(const order of [['authenticated','connected','restored'],['connected','restored','authenticated'],['restored','authenticated','connected']]) {
  const ready={connected:false,restored:false,authenticated:false,busy:false};let calls=0;
  const start=new StartupSTT({state:()=>ready,start:async()=>calls++});
  for(let i=0;i<order.length;i++){ready[order[i]]=true;await start.attempt();assert.equal(calls,i===2?1:0);}
  await start.attempt();assert.equal(calls,1,'status refresh cannot toggle capture');
 }
});
test('concurrent readiness notifications cannot toggle off an opening microphone',async()=>{
 let release,calls=0;const ready={connected:true,restored:true,authenticated:true,busy:false};
 const start=new StartupSTT({state:()=>ready,start:()=>{calls++;return new Promise(resolve=>release=resolve);}});
 const first=start.attempt();await start.attempt();assert.equal(calls,1);release();await first;await start.attempt();assert.equal(calls,1);
});
test('manual control, already-running capture and failed starts do not get restarted',async()=>{
 let calls=0;const ready={connected:true,restored:true,authenticated:true,busy:false};
 const canceled=new StartupSTT({state:()=>ready,start:async()=>calls++});canceled.cancel();await canceled.attempt();assert.equal(calls,0);
 const busy=new StartupSTT({state:()=>({...ready,busy:true}),start:async()=>calls++});await busy.attempt();assert.equal(calls,0);
 const failed=new StartupSTT({state:()=>ready,start:async()=>{calls++;throw Error('mic denied');}});await assert.rejects(failed.attempt(),/mic denied/);await failed.attempt();assert.equal(calls,1);
});
