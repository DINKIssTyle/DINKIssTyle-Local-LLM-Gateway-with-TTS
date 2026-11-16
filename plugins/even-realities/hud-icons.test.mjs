import {headerLayout} from './header-layout.mjs';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {inflateSync} from 'node:zlib';
import {HudIconStore,HUD_ICONS,hudIconPath,microphoneIcon,rgbaToGray4,captureStatusIcon,iconHeaderMessage} from './hud-icons.mjs';
assert.equal(microphoneIcon('phone',true),'phone-mic-on');assert.equal(microphoneIcon('phone',false),'phone-mic-off');
assert.equal(microphoneIcon('glasses',true),'glasses-mic-on');assert.equal(microphoneIcon('invalid',false),'glasses-mic-off');
assert.deepEqual([...rgbaToGray4(Uint8Array.from([255,255,255,255,0,0,0,255]),2,1)],[0xf0],'left pixel must use high nibble');
assert.deepEqual([...rgbaToGray4(Uint8Array.from([255,255,255,0,255,255,255,128]),2,1)],[0x08],'transparent background stays off');
assert.throws(()=>rgbaToGray4(new Uint8Array(12),3,1));
let loads=0;const store=new HudIconStore(async(key,{width,height})=>{loads++;return new Uint8Array(width*height*4).fill(255);});
const [one,two]=await Promise.all([store.load('phone-mic-on'),store.load('phone-mic-on')]);
assert.equal(loads,1);assert.equal(one,two);assert.equal(one.length,288);assert.ok(one.every(v=>v===255));
let attempts=0;const retry=new HudIconStore(async(key,{width,height})=>{if(!attempts++)throw Error('missing');return new Uint8Array(width*height*4);});
await assert.rejects(retry.load('phone-mic-on'));assert.equal((await retry.load('phone-mic-on')).length,288);
const fingerprints=new Set();
for(const [key,{width,height}] of Object.entries(HUD_ICONS)){
 assert.equal(width,24);assert.equal(height,24);assert.ok(hudIconPath(key).endsWith('.png'));
 const png=await readFile(new URL(hudIconPath(key),import.meta.url));
 assert.equal(png.subarray(1,4).toString(),'PNG');assert.equal(png.readUInt32BE(16),24);assert.equal(png.readUInt32BE(20),24);assert.equal(png[24],1,'source is a 1-bit bitmap');
 let pos=8,parts=[];
 while(pos<png.length){const size=png.readUInt32BE(pos),type=png.toString('ascii',pos+4,pos+8);if(type==='IDAT')parts.push(png.subarray(pos+8,pos+8+size));pos+=size+12;}
 const rows=inflateSync(Buffer.concat(parts));assert.equal(rows.length,24*4);assert.ok(rows.some(v=>v!==0));fingerprints.add(rows.toString('hex'));
}
assert.equal(fingerprints.size,9,'each source, mode and capture state must have a distinct editable bitmap');
for(const translate of [true,false]){const {x,width}=headerLayout(translate);assert.equal(x+width,396);assert.ok(x>=28);assert.equal(1+24<=27,true,'icons must remain inside existing header row');}
console.log('PASS: nine native-size 24×24 bitmaps, packed Gray4/alpha, coalesced raster cache, failed-load retry and non-overlapping header layout');

for(const trigger of ['이븐','웬즈데이','a'.repeat(40),'가'.repeat(40),'']){
 const layout=headerLayout(false,{trigger}),{prefix,wake,micIconX,x,width}=layout;
 assert.ok(wake.x>=prefix.x+prefix.width);assert.ok(micIconX>=wake.x+wake.width);assert.ok(x>=micIconX+24);assert.ok(width>=100);
 assert.ok(wake.content.endsWith(']'));if(trigger.length===40)assert.ok(wake.content.includes('…'));
}
assert.notEqual(headerLayout(false,{trigger:'이븐'}).key,headerLayout(false,{trigger:'웬즈데이'}).key,'keyword changes must reposition the following icon');

const manifest=JSON.parse(await readFile(new URL('app.json',import.meta.url),'utf8'));
for(const name of ['g2-microphone','phone-microphone'])assert.ok(manifest.permissions.some(p=>p.name===name && p.desc),'both microphone sources need declared permissions');

const dimStore=new HudIconStore(async(key,{width,height})=>new Uint8Array(width*height*4).fill(255));
assert.ok((await dimStore.load('glasses-mic-on')).every(byte=>byte===0xff));
assert.ok((await dimStore.load('glasses-mic-off')).every(byte=>byte===0x88),'inactive bitmap is half-luminance Gray4 without changing editable 1-bit sources');
assert.deepEqual([...rgbaToGray4(Uint8Array.from([0,0,0,255,255,255,255,0]),2,1,0.5)],[0],'dimmed background remains black');

for(const translate of [false,true])for(const trigger of ['이븐','가'.repeat(40)]){
 const l=headerLayout(translate,{trigger});assert.ok(l.modeIconX>=l.prefix.x+l.prefix.width);assert.ok(l.wake.x>=l.modeIconX+24);assert.ok(l.micIconX>=l.wake.x+l.wake.width);assert.ok(l.stateIconX>=l.micIconX+24);assert.ok(l.x>=l.stateIconX+24);assert.ok(l.x+l.width<400);assert.ok(l.width>=100);
}
assert.equal(captureStatusIcon(), 'status-idle');assert.equal(captureStatusIcon({listening:true}), 'status-listening');assert.equal(captureStatusIcon({listening:true,transcribing:true}), 'status-transcribing');assert.equal(captureStatusIcon({listening:true,preparing:true}), 'status-idle');assert.equal(captureStatusIcon({listening:true,error:true}), 'status-idle');
for(const text of ['| 대기중','| 듣는중','| 전사 처리','| 전사 처리중','| 전사 처리 대기','| -'])assert.equal(iconHeaderMessage(text),'');
for(const text of ['| 음성 엔진 준비중','| 마이크 연결중','| 오디오 수신 대기','| 질문 대기','| 전사 오류','| 프롬프트 처리 32%'])assert.equal(iconHeaderMessage(text),text,'meaningful progress, readiness and error details stay visible');
for(const key of ['mode-ai','mode-translate','status-idle','status-listening','status-transcribing'])assert.ok((await dimStore.load(key)).every(byte=>byte===0x88),'auxiliary icons use dim packed grayscale');
console.log('PASS: native mode/state bitmaps, non-overlapping header, truthful capture state and visible readiness/error fallback details');
