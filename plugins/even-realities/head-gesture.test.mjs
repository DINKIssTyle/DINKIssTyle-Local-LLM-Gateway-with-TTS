import assert from 'node:assert/strict';
import {validateEvenHubPageContainer} from '@evenrealities/even_hub_sdk';
import {HeadGesture,imuVector,poseSample,makeHeadCalibration,validHeadCalibration,blankHeadPage} from './head-gesture.mjs';
assert.equal(imuVector({x:0,y:NaN,z:1}),null);assert.deepEqual(imuVector({x:1,y:2}),[1,2,0]);assert.deepEqual(imuVector({}),[0,0,0],'omitted protobuf doubles represent zero');
const calibration=makeHeadCalibration(poseSample(Array.from({length:8},()=>[5,0,0])),poseSample(Array.from({length:8},()=>[5,30,0])));
assert.ok(validHeadCalibration(calibration));assert.ok(!validHeadCalibration({neutral:[0,0,0],raised:[0,0,0]}));
assert.throws(()=>makeHeadCalibration({mean:[0,0,0],noise:1},{mean:[0,1,0],noise:1}),/近|가깝/);
const detector=new HeadGesture(calibration);let now=0,toggles=0;
const sample=(y,x=5,z=0)=>{now+=100;if(detector.consume({x,y,z},now))toggles++;};
// Calibration finishes while looking up: returning once must not toggle.
for(let i=0;i<3;i++)sample(30);for(let i=0;i<4;i++)sample(0);assert.equal(toggles,0);
for(let i=0;i<3;i++)sample(30);for(let i=0;i<3;i++)sample(0);assert.equal(toggles,1);
// A bounced gesture within the refractory interval is ignored.
for(let i=0;i<3;i++)sample(30);for(let i=0;i<3;i++)sample(0);assert.equal(toggles,1);
now+=1600;for(let i=0;i<4;i++)sample(0);for(let i=0;i<3;i++)sample(30);for(let i=0;i<3;i++)sample(0);assert.equal(toggles,2);
// Lateral turns, downward motion and a single spike cannot arm a toggle.
now+=1600;for(let i=0;i<4;i++)sample(0);for(let i=0;i<4;i++)sample(30,50);for(let i=0;i<4;i++)sample(0);assert.equal(toggles,2);
for(let i=0;i<4;i++)sample(-30);for(let i=0;i<4;i++)sample(0);sample(30);for(let i=0;i<4;i++)sample(0);assert.equal(toggles,2);
// A long hold and a discontinuous stream cannot complete an old gesture.
for(let i=0;i<3;i++)sample(30);now+=5000;for(let i=0;i<4;i++)sample(0);assert.equal(toggles,2);
for(let i=0;i<3;i++)sample(30);now+=2000;for(let i=0;i<4;i++)sample(0);assert.equal(toggles,2);
// Any sensor axis and unit scale can be calibrated without hard-coded pitch.
for(const raised of [[.3,0,0],[0,0,-20],[2,3,4]]){
 const d=new HeadGesture({neutral:[0,0,0],raised});let at=0,count=0;
 for(const value of [[0,0,0],raised,[0,0,0]])for(let i=0;i<4;i++){at+=100;if(d.consume({x:value[0],y:value[1],z:value[2]},at))count++;}
 assert.equal(count,1);
}
const blank=blankHeadPage();assert.equal(validateEvenHubPageContainer(blank).valid,true);assert.equal(blank.containerTotalNum,1);assert.equal(blank.textObject[0].content,'');assert.equal(blank.textObject[0].borderWidth,0);assert.equal(blank.imageObject,undefined);
console.log('PASS: calibrated up-return gesture, dwell/cooldown, neutral re-arm, lateral/down/spike/gap rejection, arbitrary axes/units and truly blank live page');
