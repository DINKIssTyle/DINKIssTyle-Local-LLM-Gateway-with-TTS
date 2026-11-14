const axes=['x','y','z'];
export function imuVector(data){return data && typeof data==='object' && !Array.isArray(data) && axes.every(k=>data[k]===undefined || typeof data[k]==='number' && Number.isFinite(data[k]))?axes.map(k=>data[k] ?? 0):null;}
const subtract=(a,b)=>a.map((v,i)=>v-b[i]);
const norm=v=>Math.hypot(...v);
export function poseSample(values){
  if(values.length<8)throw Error('IMU 표본이 부족합니다');
  const mean=axes.map((_,i)=>values.reduce((s,v)=>s+v[i],0)/values.length);
  const noise=Math.max(...values.map(v=>norm(subtract(v,mean))));
  return {mean,noise};
}
export function makeHeadCalibration(neutral,raised){
  const distance=norm(subtract(raised.mean,neutral.mean));
  if(!Number.isFinite(distance) || distance<Math.max(1e-6,8*neutral.noise,8*raised.noise))throw Error('두 자세가 너무 가깝거나 흔들렸습니다. 정면부터 다시 보정하세요.');
  return {neutral:neutral.mean,raised:raised.mean};
}
export function validHeadCalibration(value){return !!value && ['neutral','raised'].every(key=>Array.isArray(value[key]) && value[key].length===3 && value[key].every(Number.isFinite)) && norm(subtract(value.raised,value.neutral))>1e-6;}
// Projection onto the wearer's calibrated up direction avoids assuming SDK axis names/units.
export class HeadGesture {
  constructor(calibration){this.configure(calibration);}
  configure(calibration){this.calibration=validHeadCalibration(calibration)?calibration:null;this.reset();}
  reset(){this.phase='neutral';this.since=null;this.last=null;this.raisedAt=0;this.cooldownUntil=0;}
  consume(data,now=Date.now()){
    const value=imuVector(data);if(!value || !this.calibration)return false;
    if(this.last!==null && (now<=this.last || now-this.last>1500)){this.phase='neutral';this.since=null;}
    this.last=now;if(now<this.cooldownUntil)return false;
    const up=subtract(this.calibration.raised,this.calibration.neutral),size=norm(up),delta=subtract(value,this.calibration.neutral);
    const projection=delta.reduce((sum,v,i)=>sum+v*up[i],0)/(size*size);
    const lateral=norm(delta.map((v,i)=>v-projection*up[i]))/size;
    const neutral=Math.abs(projection)<=.25 && lateral<=.45;
    const raised=projection>=.65 && projection<=1.7 && lateral<=.45;
    if(this.phase==='neutral'){
      if(!neutral){this.since=null;return false;}
      if(this.since===null)this.since=now;
      if(now-this.since>=150){this.phase='ready';this.since=null;}
    }else if(this.phase==='ready'){
      if(!raised){this.since=null;return false;}
      if(this.since===null)this.since=now;
      if(now-this.since>=150){this.phase='raised';this.raisedAt=now;this.since=null;}
    }else{
      if(now-this.raisedAt>4000){this.phase='neutral';this.since=null;return false;}
      if(!neutral){this.since=null;return false;}
      if(this.since===null)this.since=now;
      if(now-this.since>=150){this.phase='neutral';this.since=null;this.cooldownUntil=now+1500;return true;}
    }
    return false;
  }
}
export function blankHeadPage(){return {containerTotalNum:1,textObject:[{containerID:1,containerName:'head-blank',xPosition:0,yPosition:0,width:576,height:288,paddingLength:0,borderWidth:0,content:'',isEventCapture:1}],menuObject:{menuItems:[{itemID:91,itemName:'화면/STT 재개'}]}};}
