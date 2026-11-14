import {speechSegmentOptions} from './audio-segments.mjs';

export const GLASSES_THRESHOLDS={standard:0.008,high:0.004,very_high:0.002};
export function normalizeGlassesInput(value){
  return {sensitivity:['standard','high','very_high'].includes(value?.sensitivity)?value.sensitivity:'high',
    gain:[1,2,4].includes(Number(value?.gain))?Number(value.gain):1};
}
export function inputSegmentOptions(engine,mode,source,settings){
  return {...speechSegmentOptions(engine,mode),threshold:source==='glasses'?GLASSES_THRESHOLDS[normalizeGlassesInput(settings).sensitivity]:0.008};
}
export function levelDb(rms){return rms>0?20*Math.log10(rms):-Infinity;}

// Operate on each new SDK PCM frame once, before snapshots are accumulated.
// A frame-wide peak limiter preserves its waveform instead of clipping samples.
export class GlassesInputProcessor {
  constructor(settings){this.settings=normalizeGlassesInput(settings);}
  process(bytes,source){
    if(bytes.length%2)throw Error('잘못된 PCM16 오디오 프레임');
    const view=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength);
    const count=bytes.length/2;let sum=0,peak=0;
    for(let i=0;i<count;i++){const value=view.getInt16(i*2,true);sum+=value*value;peak=Math.max(peak,Math.abs(value));}
    const rawLevel=Math.sqrt(sum/Math.max(1,count))/32768;
    const requestedGain=source==='glasses'?this.settings.gain:1;
    const effectiveGain=requestedGain===1 || !peak?requestedGain:Math.min(requestedGain,32700/peak);
    const limited=effectiveGain<requestedGain-1e-9;
    if(requestedGain===1 || !peak)return {bytes,rawLevel,outputLevel:rawLevel,peak:peak/32768,requestedGain,effectiveGain,limited};
    const output=new Uint8Array(bytes.length),target=new DataView(output.buffer);let outputSum=0;
    for(let i=0;i<count;i++){
      const value=Math.max(-32768,Math.min(32767,Math.round(view.getInt16(i*2,true)*effectiveGain)));
      target.setInt16(i*2,value,true);outputSum+=value*value;
    }
    return {bytes:output,rawLevel,outputLevel:Math.sqrt(outputSum/Math.max(1,count))/32768,peak:peak/32768,requestedGain,effectiveGain,limited};
  }
}
