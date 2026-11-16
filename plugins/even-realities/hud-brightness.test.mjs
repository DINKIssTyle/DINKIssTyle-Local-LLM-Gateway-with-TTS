import assert from 'node:assert/strict';
import {normalizeHUDDimming,hudBrightness,hudPreviewColor,hudBorderBrightness,hudBorderPreviewColor} from './hud-brightness.mjs';
assert.deepEqual(normalizeHUDDimming(),{hudDimEnabled:true,hudDimLevel:1,hudFullTranscript:false,hudFullAI:true});
for(const value of [undefined,'bad',Infinity])assert.equal(normalizeHUDDimming({hudDimLevel:value}).hudDimLevel,1);
assert.equal(normalizeHUDDimming({hudDimLevel:0}).hudDimLevel,0);
assert.equal(normalizeHUDDimming({hudDimLevel:99}).hudDimLevel,4);
for(const enabled of [true,false])for(const level of [0,1,2,3,4])for(const transcript of [true,false])for(const ai of [true,false]){
 const c={hudDimEnabled:enabled,hudDimLevel:level,hudFullTranscript:transcript,hudFullAI:ai},dim=enabled?level:4;
 for(const id of [2,3,5,6,7,8,11])assert.equal(hudBrightness(id,c),dim,'all persistent status text and microphone/state/mode icons dim together');
 assert.equal(hudBrightness(4,c),transcript?4:dim);
 for(const id of [1,9,10])assert.equal(hudBrightness(id,c),ai?4:dim,'both hint cards and answers respect the exemption');
 for(const id of [1,4])assert.equal(hudBrightness(id,c,{translate:true}),4,'translation and suggested replies stay readable');
 assert.equal(hudBrightness(8,c,{wakeActive:true}),4,'activated wake word retains its established bright feedback');
}
assert.equal(hudPreviewColor(1),'rgb(0, 64, 26)');assert.equal(hudPreviewColor(4),'var(--accent-green)');
console.log('PASS: four visible brightness levels, independent content exemptions, dim toggle, preserved translation and active wake word');

assert.equal(hudPreviewColor(0),'rgb(0, 17, 7)','lowest preview remains dimly visible');

assert.equal(hudBorderBrightness({hudDimEnabled:true,hudDimLevel:0}),4,'lowest dim level must not erase AI/answer/translation frames');
assert.equal(hudBorderBrightness({hudDimEnabled:true,hudDimLevel:1}),4,'25% maps to Gray4 level 4, not text level 1');
assert.equal(hudBorderBrightness({hudDimEnabled:false}),15);
for(const level of [0,1,2,3,4])assert.ok(hudBorderBrightness({hudDimLevel:level})>=4 && hudBorderBrightness({hudDimLevel:level})<=15);
assert.equal(hudBorderPreviewColor({hudDimLevel:0}),'rgb(0, 68, 27)');
