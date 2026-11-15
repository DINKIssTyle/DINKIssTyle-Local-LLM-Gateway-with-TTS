import {speechSegmentOptions} from './audio-segments.mjs';

export const GLASSES_THRESHOLDS={standard:0.008,high:0.004,very_high:0.002};
export function normalizeGlassesInput(value){
  return {sensitivity:['standard','high','very_high','auto'].includes(value?.sensitivity)?value.sensitivity:'high',
    gain:value?.gain==='auto'?'auto':[1,2,4].includes(Number(value?.gain))?Number(value.gain):1};
}
export function inputSegmentOptions(engine,mode,source,settings){
  return {...speechSegmentOptions(engine,mode),threshold:source==='glasses'?(GLASSES_THRESHOLDS[normalizeGlassesInput(settings).sensitivity] ?? 0.004):0.008};
}
export function levelDb(rms){return rms>0?20*Math.log10(rms):-Infinity;}

// Operate on each new SDK PCM frame once, before snapshots are accumulated.
// A frame-wide peak limiter preserves its waveform instead of clipping samples.
export class GlassesInputProcessor {
  constructor(settings){this.settings=normalizeGlassesInput(settings);this.noise=0.001;this.autoGain=1;this.lastGain=null;}
  configure(settings){const next=normalizeGlassesInput(settings);this.transition=next.gain!==this.settings.gain;this.settings=next;}
  process(bytes,source){
    if(bytes.length%2)throw Error('잘못된 PCM16 오디오 프레임');
    const view=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength);
    const count=bytes.length/2;let sum=0,peak=0;
    for(let i=0;i<count;i++){const value=view.getInt16(i*2,true);sum+=value*value;peak=Math.max(peak,Math.abs(value));}
    const rawLevel=Math.sqrt(sum/Math.max(1,count))/32768;
    const seconds=count/16000;
    // Track quiet frames quickly and rising background very slowly. RMS alone
    // cannot identify a voice; stay bounded and never amplify silence to target.
    if(source==='glasses'){
      const rate=rawLevel<this.noise?2:0.03;
      this.noise+=(Math.min(rawLevel,0.008)-this.noise)*(1-Math.exp(-seconds*rate));
    }
    const threshold=source==='glasses'?(this.settings.sensitivity==='auto'?Math.max(0.002,Math.min(0.012,this.noise*2.5)):GLASSES_THRESHOLDS[this.settings.sensitivity]):0.008;
    const desired=rawLevel>=threshold?Math.max(1,Math.min(4,0.035/Math.max(rawLevel,0.0001))):1;
    this.autoGain+=(desired-this.autoGain)*(1-Math.exp(-seconds*(desired<this.autoGain?8:1)));
    const requestedGain=source==='glasses'?(this.settings.gain==='auto'?this.autoGain:this.settings.gain):1;
    const effectiveGain=requestedGain===1 || !peak?requestedGain:Math.min(requestedGain,32700/peak);
    const limited=effectiveGain<requestedGain-1e-9;
    const previous=this.lastGain;this.lastGain=effectiveGain;
    // Manual settings are constant from startup. Live changes and AGC ramp only
    // new samples; existing cumulative STT snapshots are never rewritten.
    const ramp=source==='glasses' && previous!==null && previous!==effectiveGain && (this.transition || this.settings.gain==='auto');this.transition=false;
    if((requestedGain===1 && !ramp) || !peak)return {bytes,rawLevel,outputLevel:rawLevel,peak:peak/32768,requestedGain,effectiveGain,limited,threshold};
    const output=new Uint8Array(bytes.length),target=new DataView(output.buffer);let outputSum=0;
    for(let i=0;i<count;i++){
      const gain=ramp?Math.min(peak?32700/peak:4,previous+(effectiveGain-previous)*(i+1)/count):effectiveGain;
      const value=Math.max(-32768,Math.min(32767,Math.round(view.getInt16(i*2,true)*gain)));
      target.setInt16(i*2,value,true);outputSum+=value*value;
    }
    return {bytes:output,rawLevel,outputLevel:Math.sqrt(outputSum/Math.max(1,count))/32768,peak:peak/32768,requestedGain,effectiveGain,limited,threshold};
  }
}
