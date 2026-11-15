import assert from 'node:assert/strict';
import {getTextWidth} from '@evenrealities/pretext';
import {adaptiveHUD,hintWindow} from './adaptive-hud.mjs';
const idle=adaptiveHUD();assert.equal(idle.show,false);assert.equal(idle.height,0);assert.equal(idle.transcriptY,27);assert.equal(idle.transcriptHeight,81);assert.equal(idle.transcriptLines,3);
for(let lines=1;lines<=8;lines++){
 const text=Array.from({length:lines},(_,i)=>`내용 ${i+1}`).join('\n');
 const layout=adaptiveHUD({title:'청동기 시대',text});
 assert.equal(layout.height,Math.min(lines,5)*27+39);assert.equal(layout.transcriptY,27+layout.height);assert.ok(layout.transcriptY+layout.transcriptHeight<=288);assert.equal(layout.transcriptHeight,81);assert.equal(layout.transcriptLines,3);
 const win=hintWindow(text,'청동기 시대',Infinity);
 assert.equal(win.layout.title,'청동기 시대');assert.ok(win.visibleText.split('\n').length<=5);
 if(lines>5)assert.ok(win.visibleText.endsWith('내용 '+lines),'scrolling keeps title pinned and shows the last five body rows');
}
for(const text of ['피라미드는 왕실 무덤입니다.','Long English terms '.repeat(25),'한글과 English, 日本語, 中文 '.repeat(20)]){
 const win=hintWindow(text,'아주 긴 주제 제목입니다. '.repeat(30));assert.ok(getTextWidth(win.layout.title)<=570);assert.ok(win.layout.title.endsWith('…'));
 for(const line of win.visibleText.split('\n'))assert.ok(getTextWidth(line)<=570);
}
assert.deepEqual(adaptiveHUD({mode:'translate',title:'ignored',text:'translation'}),{key:'translate',show:true,showTranscript:true,hint:false,height:141,transcriptY:168,transcriptHeight:120,transcriptLines:4,contentLines:5,title:'',titleHeight:0,bodyHeight:141});
assert.equal(adaptiveHUD({text:'짧은 답변'}).height,66);
assert.equal(adaptiveHUD({text:'긴 답변\n'.repeat(20)}).height,174);
assert.equal(adaptiveHUD({title:'주제 A',text:'짧은 내용'}).key,adaptiveHUD({title:'주제 B',text:'다른 내용'}).key,'only geometry changes need a native rebuild');
assert.notEqual(idle.key,adaptiveHUD({title:'주제',text:'내용'}).key);
console.log('PASS: compact waiting display, content-sized hint/chat boxes, three-row transcript cap, blank unused canvas, title-pinned scrolling and unchanged translation');

const hidden=adaptiveHUD({hideTranscript:true});assert.equal(hidden.show,false);assert.equal(hidden.showTranscript,false);assert.equal(hidden.transcriptHeight,0);assert.equal(hidden.transcriptLines,0);
const largeHidden=hintWindow('1\n2\n3\n4\n5\n6\n7\n8','제목',0,true);assert.equal(largeHidden.layout.height,255);assert.equal(largeHidden.visibleText.split('\n').length,8);
assert.equal(adaptiveHUD({mode:'translate',hideTranscript:true}).showTranscript,true,'translation reply examples remain visible');
