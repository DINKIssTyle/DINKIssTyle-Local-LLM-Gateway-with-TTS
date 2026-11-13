// One optional decision at a time; never await this from transcription.
export class HintGate {
  constructor({decide,onHint,onStatus=()=>{},now=()=>Date.now(),cooldownMs=8000,generationIntervalMs=8000,intervalMs=2000,maxAgeMs=45000,displayAgeMs=45000,windowMs=8000,settleMs=500}) {
    Object.assign(this,{decide,onHint,onStatus,now,cooldownMs,generationIntervalMs,intervalMs,maxAgeMs,displayAgeMs,windowMs,settleMs});
    this.epoch=0;this.entries=[];this.recent=[];this.failed=new Map();this.pending=null;this.inflight=null;
    this.generating=new Map();this.sequence=0;this.nextGeneration=-Infinity;this.lastDecision=-Infinity;this.nextHint=-Infinity;this.timer=null;
  }
  reset() {
    this.epoch++;this.generating.clear();this.failed.clear();this.pending=null;this.entries=[];this.recent=[];
    clearTimeout(this.timer);this.timer=null;
    this.inflight?.controller.abort();
    this.lastDecision=this.nextHint=this.nextGeneration=-Infinity;
  }
  observe(text,{id,at=this.now()}={}) {
    const now=this.now();
    if(!text?.trim() || now-at>this.maxAgeMs)return;
    this.entries=this.entries.filter(e=>now-e.at<=60000);
    this.entries.push({text:text.slice(0,1600),at,sequence:++this.sequence});this.entries=this.entries.slice(-16);
    const focus=this.entries.filter(e=>now-e.at<=this.windowMs).slice(-3);
    const older=this.entries.filter(e=>!focus.includes(e));
    this.pending={id,text:focus.map(e=>e.text).join('\n').slice(-800),context:older.map(e=>e.text).join('\n').slice(-800),at,epoch:this.epoch,focusText:text.trim(),windowCount:focus.length,sequence:this.sequence,
      due:Math.min(now+this.settleMs,this.pending?.due ?? now+this.settleMs)};
    this.pump();
  }
  fresh(job){const age=job.hint?(job.category==='reply'?Math.min(20000,this.displayAgeMs):this.displayAgeMs):this.maxAgeMs;return job.epoch===this.epoch && this.now()-job.at<=age;}
  // Generation acceptance is not a successful intervention.
  complete(job,{displayed=false,text='',topic=''}={}) {
    this.generating.delete(job.sequence);
    if(job.epoch!==this.epoch)return;
    if(displayed){
      this.recent.push({text:job.text,focusText:job.focusText,cue:text,topic,at:this.now()});
      this.nextHint=this.now()+this.cooldownMs;
    } else {
      const key=normalized(job.focusText || job.text).join('');
      if(key)this.failed.set(key,this.now()+4000);
    }
    this.pump();
  }
  duplicateCue(text){return this.recent.some(e=>this.now()-e.at<180000 && e.cue && similarHint(e.cue,text));}
  duplicateTopic(topic){const t=normalized(String(topic || '')).join('');return !!t && this.recent.some(e=>this.now()-e.at<180000 && e.topic && normalized(e.topic).join('')===t);}
  shownFocus(){return this.recent.filter(e=>this.now()-e.at<180000).slice(-4).map(e=>e.focusText || e.text);}
  shownTopics(){return [...new Set(this.recent.filter(e=>this.now()-e.at<180000 && e.topic).slice(-4).map(e=>e.topic))].join(', ');}
  displayedContext(){return this.recent.filter(e=>this.now()-e.at<180000).slice(-4).map(e=>e.cue).filter(Boolean).join('\n');}
  relevant(job,topic='') {
    if(!this.fresh(job))return false;
    const later=this.entries.filter(e=>e.sequence>job.sequence);
    // No extra model calls: conservatively reject a detached old topic only
    // after multiple new utterances, not a single acknowledgement.
    if(later.length<2 || this.now()-job.at<12000 || !topic)return true;
    const anchors=topic.toLowerCase().match(/[\p{L}\p{N}]{2,}/gu) || [];
    const latest=later.slice(-3).map(e=>e.text.toLowerCase()).join(' ');
    return !anchors.length || anchors.some(a=>latest.includes(a) || latest.split(/\s+/u).some(w=>w.length>=2 && a.includes(w)));
  }
  async pump() {
    if(this.inflight || !this.pending)return;
    clearTimeout(this.timer);this.timer=null;
    const now=this.now();
    const wait=Math.max(this.lastDecision+this.intervalMs,this.nextHint,this.nextGeneration,this.pending.due || 0)-now;
    if(wait>0){this.timer=setTimeout(()=>{this.timer=null;this.pump();},wait);return;}
    const job=this.pending;this.pending=null;
    if(!this.fresh(job)){this.pump();return;}
    const controller=new AbortController();const token={controller};this.inflight=token;
    this.lastDecision=now;
    const timeout=setTimeout(()=>controller.abort(),8000);
    try {
      const decision=await this.decide(job,controller.signal);
      if(!this.fresh(job) || controller.signal.aborted)return;
      this.recent=this.recent.filter(e=>now-e.at<180000);
      for(const [key,candidate] of this.generating)if(!this.fresh(candidate))this.generating.delete(key);
      const focus=typeof decision.focus==='string' && job.text.includes(decision.focus.trim())?decision.focus.trim():'';
      const focusedJob=focus?{...job,text:focus,focusText:focus,context:[job.context,job.text].filter(Boolean).join('\n').slice(-1200)}:job;
      for(const [k,until] of this.failed)if(until<=this.now())this.failed.delete(k);
      const focusKey=normalized(focusedJob.focusText || focusedJob.text).join('');
      if(decision.hint && focusKey && (this.failed.get(focusKey) || 0)>this.now()) {
        this.onStatus('직전 생략 후보 · 새 발화 대기');return;
      }
      if(decision.hint && [...this.recent,...this.generating.values()].some(e=>similarHint(e.focusText || e.text,focusedJob.focusText || focusedJob.text))) {
        this.onStatus('같은 내용 · 힌트 생략');return;
      }
      if(decision.hint) {
        const candidate={...focusedJob,category:decision.category};
        this.generating.set(job.sequence,focusedJob);
        const accepted=this.onHint(candidate);
        if(accepted===false){this.generating.delete(job.sequence);this.onStatus('힌트 생성 대기 · 다음 대화에서 재판별');return;}
        this.nextGeneration=this.now()+this.generationIntervalMs;
        this.onStatus(`힌트 후보 · ${focusedJob.text.slice(0,80)}`);
      } else {
        if(decision.reason==='embedding_busy' && !this.pending && !job.retry){this.pending={...job,retry:true,due:this.now()+2000};}
        const reasons={embedding_disabled:'임베딩 비활성 · 전사만 계속',embedding_unavailable:'임베딩 준비 필요 · 전사만 계속',jev_required:'EmbeddingGemma 2 + JEV 필요',embedding_busy:'임베딩 사용 중 · 다음 발화에서 재판별'};
        const prefix=decision.reason==='uncertain'?'힌트 보류':'힌트 대기';
        this.onStatus(reasons[decision.reason] || `${prefix} · 최근 ${job.windowCount}개 발화 / ${Array.from(job.text).length}자 판별`);
      }
    } catch(err) {
      if(job.epoch===this.epoch)this.onStatus(controller.signal.aborted?'힌트 판별 지연 · 전사 계속':`힌트 판별 실패 · 전사 계속 (${err.message})`);
    } finally {
      clearTimeout(timeout);
      if(this.inflight===token)this.inflight=null;
      this.pump();
    }
  }
}
function normalized(text){return Array.from(text.toLowerCase().replace(/[^\p{L}\p{N}]/gu,''));}
export function similarHint(a,b) {
  const aa=normalized(a),bb=normalized(b);
  if(aa.join('')===bb.join(''))return true;
  if(Math.min(aa.length,bb.length)<6)return false;
  const grams=arr=>new Set(arr.slice(0,-2).map((_,i)=>arr.slice(i,i+3).join('')));
  const x=grams(aa),y=grams(bb);const intersection=[...x].filter(v=>y.has(v)).length;
  return intersection/(x.size+y.size-intersection)>=0.7;
}
