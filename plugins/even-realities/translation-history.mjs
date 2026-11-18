import {wrapHUDLines,sliceHUDWindow} from './hud-status.mjs';

// Display-only scrollback: bounded in both native rows and stored characters.
export class TranslationHistory {
  constructor({maxLines=30,maxChars=6000,width=570,visibleLines=5}={}){
    Object.assign(this,{maxLines,maxChars,width,visibleLines});this.reset();
  }
  reset(){this.entries=[];this.follow=true;this.offset=0;this.trimmedLines=0;}
  text(){return this.entries.flatMap(entry=>entry.lines).join('\n');}
  update(id,text,{final=false,revision=id}={}){
    text=String(text || '').trim();
    if(!text || ['…','...'].includes(text))return false;
    const index=this.entries.findIndex(entry=>entry.id===id);
    if(index>=0 && (index!==this.entries.length-1 || (this.entries[index].final && !final && this.entries[index].revision===revision)))return false;
    // Cap input before measuring, including zero-width/combining characters.
    const lines=wrapHUDLines(Array.from(text).slice(-this.maxChars).join(''),this.width).slice(-this.maxLines);
    if(!lines.length)return false;
    const entry={id,lines,final,revision};
    if(index>=0)this.entries[index]=entry;else this.entries.push(entry);
    let count=this.entries.reduce((n,e)=>n+e.lines.length,0);
    let chars=this.entries.reduce((n,e)=>n+e.lines.reduce((m,line)=>m+line.length,0),0),removed=0;
    while(count>this.maxLines || chars>this.maxChars){
      chars-=this.entries[0].lines.shift().length;count--;removed++;
      if(!this.entries[0].lines.length)this.entries.shift();
    }
    this.trimmedLines+=removed;
    if(!this.follow)this.offset=Math.max(0,this.offset-removed);
    this.offset=Math.min(this.offset,Math.max(0,count-this.visibleLines));
    return true;
  }
  window(){
    const fullText=this.text();
    return {...sliceHUDWindow(fullText,this.follow?Infinity:this.offset,this.visibleLines,this.width),fullText};
  }
  scroll(delta){
    const win=this.window();
    this.offset=Math.max(0,Math.min(win.maxOffset,win.offsetLine+delta));
    this.follow=this.offset===win.maxOffset;
  }
}
