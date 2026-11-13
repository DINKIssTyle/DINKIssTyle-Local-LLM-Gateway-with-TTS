// Provisional text never enters the LLM or transcript store. One active
// translation plus one latest pending revision bounds work while speaking.
export class LiveTranslationQueue {
  constructor({translate,onText,onFinal,onToken=()=>{},onBusy=()=>{},onFailure=()=>{},intervalMs=500,now=()=>Date.now()}){
    Object.assign(this,{translate,onText,onFinal,onToken,onBusy,onFailure,intervalMs,now});
    this.token=0;this.segment='';this.pair='';this.finalSubmitted=false;this.finalKey='';this.lastDraftKey='';
    this.pending=null;this.active=null;this.controller=null;this.timer=null;this.nextDraftAt=0;
  }
  reset(){
    this.token++;this.onToken(this.token);this.segment='';this.pair='';this.finalSubmitted=false;this.finalKey=this.lastDraftKey='';
    this.pending=null;clearTimeout(this.timer);this.timer=null;this.controller?.abort();this.nextDraftAt=0;this.onBusy(false);
  }
  hasDraft(){return !!this.segment && !this.finalSubmitted;}
  submit(job){
    const prompt=String(job.prompt || '').trim();if(!prompt)return false;
    const pair=`${job.source}>${job.target}`;
    const key=pair+'\0'+prompt;
    const draft=job.draft===true;
    const changed=job.id!==this.segment || pair!==this.pair;
    if(changed){this.token++;this.onToken(this.token);this.segment=job.id;this.pair=pair;this.finalSubmitted=false;this.finalKey=this.lastDraftKey='';this.nextDraftAt=0;}
    if(draft && this.finalSubmitted)return false;
    if(!draft && this.finalSubmitted && this.finalKey===key)return false;
    if(draft && (this.lastDraftKey===key || this.pending?.key===key || (this.active?.key===key && this.active.token===this.token)))return false;
    if(!draft){this.token++;this.onToken(this.token);this.finalSubmitted=true;this.finalKey=key;clearTimeout(this.timer);this.timer=null;}
    this.pending={...job,prompt,draft,key,token:this.token};this.pump();return true;
  }
  async pump(){
    if(this.active || !this.pending)return;
    const delay=this.pending.draft?this.nextDraftAt-this.now():0;
    if(delay>0){if(!this.timer)this.timer=setTimeout(()=>{this.timer=null;this.pump();},delay);return;}
    clearTimeout(this.timer);this.timer=null;
    const job=this.pending;this.pending=null;this.active=job;const controller=new AbortController();this.controller=controller;this.onBusy(true);
    try{
      const result=await this.translate(job,controller.signal);
      if(job.token!==this.token || controller.signal.aborted)return;
      if(result.ok){if(job.draft)this.lastDraftKey=job.key;this.onText(job,result.text);}
      else this.onFailure(job,result.reason);
      if(!job.draft)this.onFinal(job,result);
    }catch(error){
      if(job.token===this.token && !controller.signal.aborted){this.onFailure(job,error.message);if(!job.draft)this.onFinal(job,{ok:false,reason:error.message});}
    }finally{
      this.nextDraftAt=this.pending && this.pending.id!==job.id?0:this.now()+this.intervalMs;this.active=null;this.controller=null;this.onBusy(false);this.pump();
    }
  }
}
