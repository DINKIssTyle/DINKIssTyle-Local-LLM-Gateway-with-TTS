import {getTextWidth} from '@evenrealities/pretext';
import {wrapHUDLines,hudNoticeText} from './hud-status.mjs';
import {hintWindow} from './adaptive-hud.mjs';

const WIDTH=570,TAIL=92;
export function secondsPerHintLine(value){if(value==null || value==='')return 3;const n=Number(value);return Number.isFinite(n)?Math.max(1,Math.min(30,Math.round(n*2)/2)):3;}
export function timedHintLines(text){return wrapHUDLines(text,WIDTH);}
export function aiTitleLine(title,seconds=null){
  const label=seconds===null?'':`${Math.max(0,Math.ceil(seconds))}초`;
  const caption=hudNoticeText(String(title || 'AI 답변').replace(/\s+/gu,' ').trim(),label?WIDTH-TAIL:WIDTH);
  if(!label)return caption;
  let pad=' ';while(getTextWidth(caption+pad+' '+label)<=WIDTH)pad+=' ';
  return caption+pad+label;
}
export function hintCountdownWindow(text,title,offset,hideTranscript,seconds){
  const win=hintWindow(text,title,offset,hideTranscript);
  return {...win,headerText:aiTitleLine(win.layout.title,seconds)};
}
export class HintLifetime {
  constructor({now=()=>Date.now(),schedule=setTimeout,cancel=clearTimeout,onTick=()=>{},onClose=()=>{}}={}){Object.assign(this,{now,schedule,cancel,onTick,onClose});this.generation=0;this.timer=null;this.deadline=0;this.text='';this.title='';this.seconds=3;}
  get active(){return this.deadline>0;}
  get remaining(){return this.active?Math.max(0,Math.ceil((this.deadline-this.now())/1000)):null;}
  clear(){this.generation++;if(this.timer!==null)this.cancel(this.timer);this.timer=null;this.deadline=0;this.text=this.title='';}
  show({text,title='',enabled,secondsPerLine=3}){
    this.clear();this.text=text;this.title=title;this.seconds=secondsPerHintLine(secondsPerLine);
    if(!enabled || !text)return;
    this.deadline=this.now()+Math.max(1,timedHintLines(text).length)*this.seconds*1000;
    this.pump(this.generation);
  }
  pump(generation){
    if(generation!==this.generation || !this.active)return;
    if(this.now()>=this.deadline){const snapshot={text:this.text,title:this.title};this.clear();this.onClose(snapshot);return;}
    this.onTick();
    if(generation!==this.generation || !this.active)return;
    this.timer=this.schedule(()=>this.pump(generation),Math.min(1000,this.deadline-this.now()));
  }
}
