import assert from 'node:assert/strict';
import {HintStack,hintCardCount,stackedHintPresentation} from './hint-stack.mjs';

assert.equal(hintCardCount(undefined),1);assert.equal(hintCardCount(3),1);assert.equal(hintCardCount('2'),2);
let now=0,next=0;const timers=new Map();
const stack=new HintStack({now:()=>now,schedule:(fn,ms)=>{timers.set(++next,{fn,at:now+ms});return next;},cancel:id=>timers.delete(id)});
function advance(ms){const target=now+ms;while(true){const due=[...timers].filter(([,job])=>job.at<=target).sort((a,b)=>a[1].at-b[1].at)[0];if(!due)break;timers.delete(due[0]);now=due[1].at;due[1].fn();}now=target;}
stack.configure({autoClose:true,secondsPerLine:3});
const older=stack.add({title:'먼저 온 카드',text:'첫 줄\n둘째 줄'});advance(1000);
const newest=stack.add({title:'나중에 온 카드',text:'짧은 한 줄'});
assert.deepEqual(stack.cards.map(card=>card.id),[newest,older]);
const oldDeadline=stack.cards[1].deadline;
advance(3000);assert.equal(stack.cards.length,1);assert.equal(stack.cards[0].id,older);
assert.equal(stack.cards[0].deadline,oldDeadline);assert.equal(stack.remaining(stack.cards[0]),2,'another card expiring must not reset this card');
advance(2000);assert.equal(stack.cards.length,0);assert.equal(timers.size,0);

stack.configure({autoClose:false});
stack.add({title:'A',text:'A 내용'});stack.add({title:'B',text:'B 내용'});stack.add({title:'C',text:'C 내용'});
assert.deepEqual(stack.cards.map(card=>card.title),['C','B']);
stack.remove(stack.cards[1].id);assert.equal(stack.cards[0].title,'C');
stack.configure({limit:1});stack.add({title:'D',text:'D 내용'});assert.equal(stack.cards.length,1);stack.clear();

stack.configure({limit:2});
stack.add({title:'[확인] 오래된 제목',text:Array.from({length:12},(_,i)=>`오래된 본문 ${i}`).join('\n')});
stack.add({title:'[핵심] 새로운 제목',text:Array.from({length:12},(_,i)=>`새로운 본문 ${i}`).join('\n')});
for(const hidden of [false,true]){
 const view=stackedHintPresentation(stack,hidden);
 assert.equal(view.cards.length,2);assert.equal(view.titleHeight,27);
 assert.ok(view.transcriptY+view.transcriptHeight<=288);
 assert.ok(view.height<=261-(hidden?0:81));
 assert.equal(view.nativeBody.split('\n')[view.cards[0].rows],'','second fixed title must have a blank body row beneath it');
 assert.ok(!view.nativeBody.includes('제목'),'fixed titles must not scroll with the body');
 assert.equal(view.cards[1].titleY,27+27+view.cards[0].rows*27);
 assert.ok(view.cards.every(card=>card.rows>=1));
}
advance(5000);assert.ok(stack.cards.every(card=>card.offset===1),'both cards scroll independently');
stack.scroll(1);const firstOffset=stack.cards[0].offset;advance(2500);
assert.equal(stack.cards[0].offset,firstOffset,'manual top-card scrolling pauses only its own auto scroll');
assert.equal(stack.cards[1].offset,2);
const id=stack.cards[0].id;stack.remove(id);const single=stackedHintPresentation(stack,false);
assert.equal(single.cards.length,1);assert.equal(single.titleHeight,33);
stack.clear();assert.equal(stackedHintPresentation(stack),null);assert.equal(timers.size,0);
console.log('PASS: bounded newest-first cards, independent deadlines/expiry/scroll, eviction/selection and two pinned titles on one body within the 576×288 canvas');

stack.configure({autoClose:true,secondsPerLine:1});
stack.add({title:'빠른 읽기',text:Array.from({length:8},(_,i)=>`읽을 줄 ${i}`).join('\n')});
stack.add({title:'두 번째 빠른 읽기',text:Array.from({length:8},(_,i)=>`다음 줄 ${i}`).join('\n')});
stackedHintPresentation(stack,false);const initialOffset=stack.cards[0].offset;
advance(2000);assert.ok(stack.cards[0].offset>initialOffset,'short per-line times must shorten the initial scroll delay for compact cards');stack.clear();
