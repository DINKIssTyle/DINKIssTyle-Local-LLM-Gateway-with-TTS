import {sliceHUDWindow} from './hud-status.mjs';
import {adaptiveHUD} from './adaptive-hud.mjs';
import {timedHintLines,secondsPerHintLine,aiTitleLine} from './hint-autoclose.mjs';

export const hintCardCount=value=>Number(value)===2?2:1;

// Independent card deadlines and scroll positions, driven by one optional timer.
export class HintStack {
  constructor({now=()=>Date.now(),schedule=setTimeout,cancel=clearTimeout,onChange=()=>{}}={}){
    Object.assign(this,{now,schedule,cancel,onChange});this.cards=[];this.limit=2;this.autoClose=false;this.seconds=3;this.nextID=0;this.timer=null;this.generation=0;
  }
  configure({limit=this.limit,autoClose=this.autoClose,secondsPerLine=this.seconds}={}){
    const seconds=secondsPerHintLine(secondsPerLine),restart=autoClose!==this.autoClose || seconds!==this.seconds;
    this.limit=hintCardCount(limit);this.autoClose=!!autoClose;this.seconds=seconds;
    this.cards=this.cards.slice(0,this.limit);
    if(restart)for(const card of this.cards)card.deadline=this.deadline(card.text);
    this.retime();this.onChange();
  }
  deadline(text){return this.autoClose?this.now()+Math.max(1,timedHintLines(text).length)*this.seconds*1000:0;}
  add({title,text,auditMeta=null}){
    text=String(text || '').trim().slice(0,2000);if(!text)return null;
    this.expire();const card={id:`hint-${++this.nextID}`,title:String(title || '힌트').slice(0,160),text,auditMeta,deadline:this.deadline(text),offset:0,maxOffset:0,nextAuto:this.now()+5000};
    this.cards.unshift(card);this.cards=this.cards.slice(0,this.limit);
    this.retime();this.onChange();return card.id;
  }
  clear(){this.cards=[];this.retime();this.onChange();}
  remove(id=this.cards[0]?.id){
    const before=this.cards.length;this.cards=this.cards.filter(card=>card.id!==id);
    if(before===this.cards.length)return false;
    this.retime();this.onChange();return true;
  }
  remaining(card){return card.deadline?Math.max(0,Math.ceil((card.deadline-this.now())/1000)):null;}
  expire(){this.cards=this.cards.filter(card=>!card.deadline || card.deadline>this.now());}
  scroll(delta,id=this.cards[0]?.id){
    const card=this.cards.find(card=>card.id===id);if(!card)return;
    card.offset=Math.max(0,Math.min(card.maxOffset,card.offset+delta));card.nextAuto=this.now()+5000;
    this.retime();this.onChange();
  }
  setView(cards){
    for(const view of cards){const card=this.cards.find(card=>card.id===view.id);if(!card)continue;
      if(!card.maxOffset && view.maxOffset)card.nextAuto=this.now()+(this.autoClose?Math.min(5000,view.rows*this.seconds*1000):5000);
      card.maxOffset=view.maxOffset;card.offset=Math.min(card.offset,card.maxOffset);
    }
    this.arm();
  }
  retime(){this.generation++;if(this.timer!==null)this.cancel(this.timer);this.timer=null;this.arm();}
  arm(){
    if(this.timer!==null || !this.cards.length)return;
    const now=this.now(),events=this.cards.flatMap(card=>[...(card.deadline?[Math.min(card.deadline,now+1000)]:[]),...(card.maxOffset?[card.nextAuto]:[])]);
    if(!events.length)return;
    const generation=this.generation;
    this.timer=this.schedule(()=>{
      if(generation!==this.generation)return;this.timer=null;this.expire();
      for(const card of this.cards)if(card.maxOffset && card.nextAuto<=this.now()){
        card.offset=card.offset>=card.maxOffset?0:card.offset+1;
        card.nextAuto=this.now()+(this.autoClose?Math.min(2500,this.seconds*1000):2500);
      }
      this.onChange();this.arm();
    },Math.max(1,Math.min(...events)-now));
  }
}

// Shared layout coordinates for the preview and native page. The native page
// gives each card its own title/body frames and consolidates dense header text.
export function stackedHintPresentation(stack,hideTranscript=false){
  const source=stack.cards.slice(0,2);if(!source.length)return null;
  let cards,height,titleHeight,bodyHeight;
  if(source.length===1){
    const card=source[0],layout=adaptiveHUD({title:card.title,text:card.text,hideTranscript});
    const win=sliceHUDWindow(card.text,card.offset,layout.contentLines,570);
    cards=[{...card,...win,rows:layout.contentLines,titleY:27,titleHeight:layout.titleHeight,remaining:stack.remaining(card)}];
    ({height,titleHeight,bodyHeight}=layout);
  }else{
    const budget=Math.floor((261-(hideTranscript?0:81))/27)-2;
    const total=source.map(card=>sliceHUDWindow(card.text,0,1,570).totalLines);
    const rows=[Math.min(total[0],Math.ceil(budget/2)),Math.min(total[1],Math.floor(budget/2))];
    let free=budget-rows[0]-rows[1];
    for(let i=0;i<2;i++){const extra=Math.min(free,total[i]-rows[i]);rows[i]+=extra;free-=extra;}
    let y=27;
    cards=source.map((card,i)=>{const win=sliceHUDWindow(card.text,card.offset,rows[i],570);
      const view={...card,...win,rows:rows[i],titleY:y,titleHeight:27,remaining:stack.remaining(card)};y+=(1+rows[i])*27;return view;});
    height=y-27;titleHeight=27;bodyHeight=height-titleHeight;
  }
  stack.setView(cards);
  // Combined presentation text for generic consumers. The native two-card page
  // uses each card's visibleText in a separate frame, without overlaying titles.
  const nativeBody=cards.map((card,i)=>i===cards.length-1?card.visibleText:[...card.visibleText.split('\n'),...Array(Math.max(0,card.rows-card.visibleText.split('\n').length)).fill(''),''].join('\n')).join('\n');
  const titles=cards.map(card=>aiTitleLine(card.title,card.remaining));
  return {key:`stack-${cards.length}-${cards.map(card=>card.rows).join('-')}-${hideTranscript}`,show:true,showTranscript:!hideTranscript,hint:true,height,titleHeight,bodyHeight,
    title:cards[0].title,contentLines:cards[0].rows,transcriptY:27+height,transcriptHeight:hideTranscript?0:81,transcriptLines:hideTranscript?0:3,cards,nativeBody,titles};
}
