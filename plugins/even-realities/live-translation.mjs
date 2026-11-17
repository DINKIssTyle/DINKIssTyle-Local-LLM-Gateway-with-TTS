// Stable speech can be translated live; drafts never request reply examples
// or archive transcripts. Keep one active translation and one latest revision.
export class LiveTranslationQueue {
  constructor({translate,onText,onFinal,onToken=()=>{},onBusy=()=>{},onFailure=()=>{},onMeasure=()=>{},intervalMs=250,now=()=>Date.now()}){
    Object.assign(this,{translate,onText,onFinal,onToken,onBusy,onFailure,onMeasure,intervalMs,now});
    this.token=0;this.segment='';this.pair='';this.finalSubmitted=false;this.finalKey='';this.lastDraftKey='';
    this.pending=null;this.active=null;this.controller=null;this.timer=null;this.nextDraftAt=0;this.latestStart=NaN;
  }
  reset(){
    this.token++;this.onToken(this.token);this.segment='';this.pair='';this.finalSubmitted=false;this.finalKey=this.lastDraftKey='';
    this.pending=null;clearTimeout(this.timer);this.timer=null;this.controller?.abort();this.nextDraftAt=0;this.latestStart=NaN;this.onBusy(false);
  }
  hasDraft(){return !!this.segment && !this.finalSubmitted;}
  submit(job){
    const prompt=String(job.prompt || '').trim();if(!prompt)return false;
    const pair=`${job.source}>${job.target}\0${job.sessionId || ''}`;
    const key=pair+'\0'+prompt;
    const draft=job.draft===true;
    const changed=job.id!==this.segment || pair!==this.pair;
    const start=Date.parse(job.startedAt);
    if(pair===this.pair && changed && Number.isFinite(start) && start<this.latestStart)return false;
    if(Number.isFinite(start))this.latestStart=start;
    if(changed){this.token++;this.onToken(this.token,{final:!draft,pairChanged:pair!==this.pair,job});this.segment=job.id;this.pair=pair;this.finalSubmitted=false;this.finalKey=this.lastDraftKey='';this.nextDraftAt=0;}
    if(draft && this.finalSubmitted)return false;
    if(!draft && this.finalSubmitted && this.finalKey===key)return false;
    if(draft && (this.lastDraftKey===key || this.pending?.key===key || (this.active?.key===key && this.active.token===this.token)))return false;
    if(!draft){this.token++;this.onToken(this.token,{final:true,job});this.finalSubmitted=true;this.finalKey=key;clearTimeout(this.timer);this.timer=null;}
    this.pending={...job,queuedAt:this.now(),prompt,draft,key,token:this.token};
    // Obsolete requests must not hold the latest sentence until their timeout.
    if(this.active && (changed || !draft))this.controller?.abort();
    this.pump();return true;
  }
  async pump(){
    if(this.active || !this.pending)return;
    const delay=this.pending.draft || this.pending.busyAttempts?this.nextDraftAt-this.now():0;
    if(delay>0){if(!this.timer)this.timer=setTimeout(()=>{this.timer=null;this.pump();},delay);return;}
    clearTimeout(this.timer);this.timer=null;
    const job=this.pending;this.pending=null;this.active=job;const startedAt=this.now();const controller=new AbortController();this.controller=controller;this.onBusy(true);this.onMeasure(job,'translation_queue_ms',startedAt-(job.queuedAt ?? startedAt));
    try{
      let abort;
      const cancelled=new Promise((_,reject)=>{abort=()=>reject(new Error('번역 취소'));controller.signal.addEventListener('abort',abort,{once:true});});
      let result;
      try{result=await Promise.race([this.translate(job,controller.signal,text=>{if(job.token===this.token && !controller.signal.aborted){if(!job.firstText){job.firstText=true;this.onMeasure(job,'first_text_ms',this.now()-startedAt);}this.onText({...job,draft:true,partial:true},text);}}),cancelled]);}
      finally{controller.signal.removeEventListener('abort',abort);}
      if(job.token!==this.token || controller.signal.aborted)return;
      if(result.busy && (job.busyAttempts || 0)<5){
        if(!this.pending)this.pending={...job,busyAttempts:(job.busyAttempts || 0)+1};
        this.nextDraftAt=this.now()+Math.min(600,150*((job.busyAttempts || 0)+1));
        this.onFailure(job,'번역 처리 중 · 최신 내용 재시도');return;
      }
      if(result.ok){if(!job.firstText)this.onMeasure(job,'first_text_ms',this.now()-startedAt);if(job.draft)this.lastDraftKey=job.key;this.onText(job,result.text);}
      else this.onFailure(job,result.reason);
      if(!job.draft)this.onFinal(job,result);
    }catch(error){
      if(job.token===this.token && !controller.signal.aborted){this.onFailure(job,error.message);if(!job.draft)this.onFinal(job,{ok:false,reason:error.message});}
    }finally{
      this.nextDraftAt=this.pending?.busyAttempts?this.nextDraftAt:this.pending && this.pending.id!==job.id?0:startedAt+this.intervalMs;this.active=null;this.controller=null;this.onBusy(false);this.pump();
    }
  }
}
