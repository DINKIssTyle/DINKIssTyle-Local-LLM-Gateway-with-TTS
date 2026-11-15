import assert from 'node:assert/strict';
import {SpeechSegmenter,mergePCM} from './audio-segments.mjs';
import {normalizeGlassesInput,inputSegmentOptions,GlassesInputProcessor,levelDb} from './glasses-input.mjs';
const pcm=values=>{const bytes=new Uint8Array(values.length*2),view=new DataView(bytes.buffer);values.forEach((v,i)=>view.setInt16(i*2,v,true));return bytes;};
const samples=bytes=>{const v=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength);return Array.from({length:bytes.length/2},(_,i)=>v.getInt16(i*2,true));};
assert.deepEqual(normalizeGlassesInput(),{sensitivity:'high',gain:1});
assert.deepEqual(normalizeGlassesInput({sensitivity:'invalid',gain:Infinity}),{sensitivity:'high',gain:1});
assert.deepEqual(normalizeGlassesInput({sensitivity:'very_high',gain:'4'}),{sensitivity:'very_high',gain:4});
for(const engine of ['whisper','apple','chrome-online'])for(const mode of ['ai','translate']){
 assert.equal(inputSegmentOptions(engine,mode,'glasses',{sensitivity:'high'}).threshold,0.004);
 assert.equal(inputSegmentOptions(engine,mode,'glasses',{sensitivity:'standard'}).threshold,0.008);
 assert.equal(inputSegmentOptions(engine,mode,'phone',{sensitivity:'very_high'}).threshold,0.008);
}
const raw=pcm([100,-200,300,-400]),copy=raw.slice();
const boost=new GlassesInputProcessor({gain:2});
assert.deepEqual(samples(boost.process(raw,'glasses').bytes),[200,-400,600,-800]);assert.deepEqual(raw,copy,'never modify SDK buffers');
const phone=boost.process(raw,'phone');assert.equal(phone.bytes,raw);assert.equal(phone.effectiveGain,1);assert.deepEqual(phone.bytes,copy,'phone is byte-for-byte unchanged');
assert.equal(new GlassesInputProcessor({gain:1}).process(raw,'glasses').bytes,raw);
const loud=boost.process(pcm([32767,-32768,16000,-16000]),'glasses');assert.equal(loud.limited,true);
assert.ok(samples(loud.bytes).every(v=>Math.abs(v)<=32700),'limit the whole frame before integer conversion');
assert.ok(Math.abs(samples(loud.bytes)[2]/samples(loud.bytes)[0]-16000/32767)<0.0001,'peak protection retains waveform ratios');
assert.equal(boost.process(pcm([0,0]),'glasses').rawLevel,0);assert.equal(levelDb(0),-Infinity);assert.ok(Math.abs(levelDb(.5)+6.0206)<.0001);
assert.throws(()=>boost.process(new Uint8Array(1),'glasses'));
const quiet=pcm(Array(1600).fill(150)); // RMS 0.00458: below old gate, above high.
const quietBefore=quiet.slice();
const old=new SpeechSegmenter(inputSegmentOptions('apple','translate','glasses',{sensitivity:'standard'}));
const sensitive=new SpeechSegmenter(inputSegmentOptions('apple','translate','glasses',{sensitivity:'high'}));
const phoneGate=new SpeechSegmenter(inputSegmentOptions('apple','translate','phone',{sensitivity:'high'}));
const snapshots=[];
for(let i=0;i<15;i++){
 const signal=boost.process(quiet,'glasses');old.push(signal.bytes,signal.rawLevel);sensitive.push(signal.bytes,signal.rawLevel);phoneGate.push(quiet);
 const snapshot=sensitive.snapshot();if(snapshot)snapshots.push(snapshot);
}
assert.equal(old.flush(),null,'amplification alone must not turn subthreshold noise into a voiced segment');
assert.equal(phoneGate.flush(),null,'phone keeps the original gate');
const captured=sensitive.flush();assert.equal(captured.length,quiet.length*15,'high sensitivity preserves quiet speech instead of dropping it');
assert.ok(samples(captured).every(v=>v===300));assert.deepEqual(quiet,quietBefore);
for(const snapshot of snapshots)assert.ok(samples(snapshot).every(v=>v===300),'growing preview snapshots must not amplify previously buffered PCM again');
assert.equal(mergePCM([raw,raw]).length,raw.length*2);
console.log('PASS: glasses-only sensitivity/gain, unchanged phone PCM/gate, immutable source, peak protection, quiet capture, raw-level gating and single-pass preview audio');

assert.deepEqual(normalizeGlassesInput({sensitivity:'auto',gain:'auto'}),{sensitivity:'auto',gain:'auto'});
const adaptive=new GlassesInputProcessor({sensitivity:'auto',gain:'auto'});
let automatic;
for(let i=0;i<50;i++)automatic=adaptive.process(quiet,'glasses');
assert.ok(automatic.effectiveGain>2 && automatic.effectiveGain<=4,'quiet above-threshold input rises gradually within a bounded gain');
assert.ok(automatic.threshold>=0.002 && automatic.threshold<=0.012);
const loudAuto=adaptive.process(pcm(Array(1600).fill(30000)),'glasses');
assert.ok(samples(loudAuto.bytes).every(v=>Math.abs(v)<=32700),'AGC ramps also obey the peak limiter');
for(let i=0;i<100;i++)automatic=adaptive.process(pcm(Array(1600).fill(10)),'glasses');
assert.ok(automatic.effectiveGain<1.01,'background below detection must not be amplified toward speech target');
const adaptivePhone=adaptive.process(raw,'phone');assert.equal(adaptivePhone.bytes,raw);assert.equal(adaptivePhone.threshold,0.008);
const live=new GlassesInputProcessor({gain:2});const before=live.process(quiet,'glasses');
live.configure({gain:4,sensitivity:'auto'});const changed=live.process(quiet,'glasses');
assert.ok(samples(changed.bytes)[0]<=301 && samples(changed.bytes).at(-1)===600,'hot gain changes ramp new PCM instead of introducing a step');
assert.ok(samples(before.bytes).every(v=>v===300),'a setting change never rewrites previously buffered frames');
console.log('PASS: bounded automatic sensitivity/gain, silence release, peak protection, live gain ramp and unchanged buffered/phone PCM');
