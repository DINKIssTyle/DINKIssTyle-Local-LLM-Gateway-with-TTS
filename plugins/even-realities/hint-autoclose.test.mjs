import assert from 'node:assert/strict';
import {getTextWidth} from '@evenrealities/pretext';
import {HintLifetime,secondsPerHintLine,timedHintLines,hintCountdownWindow} from './hint-autoclose.mjs';
import {hintWindow} from './adaptive-hud.mjs';
assert.equal(secondsPerHintLine(NaN),3);assert.equal(secondsPerHintLine(0),1);assert.equal(secondsPerHintLine(100),30);assert.equal(secondsPerHintLine(2.7),2.5);
let now=1000,events=[],callbacks=[],ticks=0;
const life=new HintLifetime({now:()=>now,schedule:(fn,ms)=>{callbacks.push({fn,ms});return callbacks.length;},cancel:()=>{},onTick:()=>ticks++,onClose:hint=>events.push(hint)});
life.show({text:'첫 줄\n둘째 줄',title:'제목',enabled:true,secondsPerLine:3});assert.equal(life.remaining,6);assert.equal(life.deadline,7000);assert.equal(ticks,1);
const stale=callbacks.at(-1).fn;now+=2001;stale();assert.equal(life.remaining,4);
life.show({text:'새 힌트',title:'새 제목',enabled:true,secondsPerLine:5});const deadline=life.deadline;stale();assert.equal(life.deadline,deadline);assert.equal(events.length,0,'old timer cannot close its replacement');
now=deadline+10000;callbacks.at(-1).fn();assert.equal(life.active,false);assert.equal(events.length,1);assert.equal(events[0].text,'새 힌트','background delay must expire using wall clock');
life.show({text:'유지할 힌트',title:'제목',enabled:true,secondsPerLine:1});const pending=callbacks.at(-1).fn;
life.show({text:'유지할 힌트',title:'제목',enabled:false});now+=100000;pending();assert.equal(events.length,1);assert.equal(life.remaining,null,'disabling must keep the hint open');
life.show({text:'취소할 힌트',title:'제목',enabled:true});const cancel=callbacks.at(-1).fn;life.clear();now+=100000;cancel();assert.equal(events.length,1,'mode/chat clear must cancel stale closure');
for(const text of ['짧은 힌트입니다.','한글 '.repeat(45),'Long English answer '.repeat(30),'日本語と中文 '.repeat(20)]){
 const timed=timedHintLines(text);assert.ok(getTextWidth(timed.at(-1))<=478);
 assert.equal(timed.join('').replace(/\s/gu,''),text.replace(/\s/gu,''),'timer reservation must preserve every source character');
 const a=hintCountdownWindow(text,'제목',Infinity,false,270),b=hintCountdownWindow(text,'제목',Infinity,false,9);
 assert.equal(a.layout.height,b.layout.height);assert.equal(a.totalLines,b.totalLines,'digit changes must never resize/reflow');
 assert.ok(a.visibleText.endsWith('270초'));assert.ok(b.visibleText.endsWith('9초'));
 assert.ok(getTextWidth(b.visibleText.split('\n').at(-1))>560,'countdown must align to the right edge');
 for(const line of a.visibleText.split('\n'))assert.ok(getTextWidth(line)<=570);
 assert.equal(a.lines.join(''),hintWindow(timed.join('\n'),'제목',Infinity,false).lines.join(''),'source lines remain separate from countdown metadata');
}
const long=Array.from({length:8},(_,i)=>'본문 '+i).join('\n');
assert.ok(!hintCountdownWindow(long,'제목',0,false,24).visibleText.includes('24초'),'countdown belongs to the answer end, not an extra footer row');
assert.ok(hintCountdownWindow(long,'제목',Infinity,false,24).visibleText.endsWith('24초'));
console.log('PASS: seconds per wrapped body line, stable right-aligned end-line countdown, no text loss, wall-clock expiry, replacement/disable/cancel safety');

life.show({text:'직접 AI 답변',enabled:true,secondsPerLine:2});assert.equal(life.active,true);assert.equal(life.title,'');
const plain=hintCountdownWindow('첫 문단\n\n마지막 문단','',0,false,6);assert.ok(plain.visibleText.includes('\n\n'),'direct reply paragraph breaks remain intact');assert.ok(plain.visibleText.endsWith('6초'));
