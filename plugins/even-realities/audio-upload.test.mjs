import assert from 'node:assert/strict';
import {AudioUpload} from './audio-upload.mjs';
const u=new AudioUpload(),job={sessionId:'session',segmentId:'one',engine:'apple',sourceLang:'en',bytes:Uint8Array.of(1,2,3,4)};
assert.equal(u.prepare(job).audioDelta,false);u.acknowledge(job,{audioDelta:true,audioBytes:4});
const next={...job,bytes:Uint8Array.of(1,2,3,4,5,6)};
assert.deepEqual([...u.prepare(next).bytes],[5,6]);assert.equal(u.prepare(next).audioOffset,4);
// No ack means a retry repeats the same delta, never skips audio.
assert.equal(u.prepare(next).audioOffset,4);u.resync(next);assert.equal(u.prepare(next).audioOffset,0);
u.acknowledge(next,{audioDelta:true,audioBytes:6});assert.equal(u.prepare({...next,segmentId:'two'}).audioOffset,0);
u.reset();assert.equal(u.prepare(next).audioDelta,false);
for(let i=0;i<25;i++)u.acknowledge({...next,segmentId:String(i)},{audioDelta:true,audioBytes:6});assert.equal(u.entries.size,16);
console.log('PASS: negotiated delta audio, acknowledged offsets, retry/resync, scope isolation and bounded metadata');

// 8 seconds at 4 snapshots/second: negotiated uploads approach one copy of
// PCM instead of repeatedly sending every prefix. Final audio is separate.
const transport=new AudioUpload();let uploaded=0,legacy=0;
for(let i=1;i<=32;i++){const snap={...job,bytes:new Uint8Array(i*8000)};const upload=transport.prepare(snap);uploaded+=upload.bytes.length;legacy+=snap.bytes.length;transport.acknowledge(snap,{audioDelta:true,audioBytes:snap.bytes.length});}
assert.equal(uploaded,256000);assert.equal(legacy,4224000);
