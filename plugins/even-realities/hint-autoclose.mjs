import {getTextWidth} from '@evenrealities/pretext';
import {wrapHUDLines} from './hud-status.mjs';
import {hintWindow} from './adaptive-hud.mjs';

const WIDTH=570,TAIL=92; // fixed timer reserve; never reflow as seconds tick.
export function secondsPerHintLine(value){if(value==null || value==='')return 3;const n=Number(value);return Number.isFinite(n)?Math.max(1,Math.min(30,Math.round(n*2)/2)):3;}
export function timedHintLines(text){
  const lines=wrapHUDLines(text,WIDTH);
  if(lines.length && getTextWidth(lines.at(-1))>WIDTH-TAIL)lines.splice(-1,1,...wrapHUDLines(lines.at(-1),WIDTH-TAIL));
  return lines;
}
export function hintCountdownWindow(text,title,offset,hideTranscript,seconds){
  const win=hintWindow(timedHintLines(text).join('\n'),title,offset,hideTranscript);
  const lines=[...win.lines];
  if(lines.length && win.offsetLine+lines.length===win.totalLines){
    const label=`${Math.max(0,Math.ceil(seconds))}초`;let pad=' ';
    while(getTextWidth(lines.at(-1)+pad+' '+label)<=WIDTH)pad+=' ';
    lines[lines.length-1]+=pad+label;
  }
  return {...win,visibleText:(win.layout.title?[win.layout.title,...lines]:lines).join('\n')};
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
