import assert from 'node:assert/strict';
import {aiStatusIcon,normalizeAIHeaderLayout,normalizeAIIconPosition,aiIconPositionLabel} from './ai-status-icon.mjs';
import {headerLayout} from './header-layout.mjs';
const active={listening:true,audio:true};
assert.equal(normalizeAIHeaderLayout('status-icon'),'status-icon');assert.equal(normalizeAIHeaderLayout('bad'),'standard');
assert.equal(aiStatusIcon(active).key,'status-listening');
assert.equal(aiStatusIcon({...active,transcribing:true}).key,'status-transcribing');
assert.equal(aiStatusIcon({...active,working:true,phase:'생각 중 (2초)'}).key,'status-thinking');
assert.equal(aiStatusIcon({...active,working:true,phase:'답변 작성 중'}).key,'mode-ai');
assert.equal(aiStatusIcon({...active,working:true,phase:'프롬프트 처리 중',progress:32}).key,'status-prompt-2');
assert.equal(aiStatusIcon({...active,working:true,phase:'프롬프트 처리 중',progress:100}).key,'status-prompt-8');
assert.equal(aiStatusIcon({...active,working:true,phase:'프롬프트 처리 중'}).key,'status-prompt-unknown');
for(const progress of [NaN,Infinity,'32'])assert.equal(aiStatusIcon({...active,working:true,phase:'프롬프트 처리 중',progress}).key,'status-prompt-unknown');
for(const state of [{},{listening:true,audio:false},{...active,preparing:true},{...active,error:true},{...active,error:true,working:true,phase:'답변 작성 중'}])assert.equal(aiStatusIcon(state).key,'status-warning');
assert.equal(aiStatusIcon({transcribing:true}).key,'status-transcribing','final STT may finish after mic is stopped');
const compact=headerLayout(false,{iconOnly:true});assert.equal(compact.stateIconX,552);assert.equal(compact.stateIconX+24,576);assert.ok(compact.width<=compact.stateIconX);
assert.notEqual(headerLayout(true,{iconOnly:true}).key,compact.key,'translation retains the full header');
console.log('PASS: optional right-edge AI header, phase precedence, quantized real progress and explicit unknown/error states');

for(const [value,x,label] of [[0,0,'좌 · 0px'],[123,123,'123px'],[124,124,'124px'],[276,276,'중앙 · 276px'],[552,552,'우 · 552px']]){
 const layout=headerLayout(false,{iconOnly:true,iconPosition:value});
 assert.equal(layout.stateIconX,x);assert.equal(aiIconPositionLabel(value),label);
 assert.ok(layout.x+layout.width<=x || layout.x>=x+24,'blank tap target must not mask the icon');
 assert.ok(x>=0 && x+24<=576);
}
for(const invalid of [undefined,null,'',NaN,Infinity,'bad',false])assert.equal(normalizeAIIconPosition(invalid),552);
assert.equal(normalizeAIIconPosition('0'),0);assert.equal(normalizeAIIconPosition('1'),1);assert.equal(normalizeAIIconPosition(-3),0);assert.equal(normalizeAIIconPosition(999),552);
assert.equal(headerLayout(true,{iconOnly:true,iconPosition:0}).stateIconX,headerLayout(true,{iconOnly:true,iconPosition:552}).stateIconX);

for(let x=0;x<=552;x++){
 const layout=headerLayout(false,{iconOnly:true,iconPosition:x});assert.equal(layout.stateIconX,x);
 assert.ok(layout.x+layout.width<=x || layout.x>=x+24);assert.ok(layout.x>=0 && layout.x+layout.width<=576);
}
