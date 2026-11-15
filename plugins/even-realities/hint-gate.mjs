// Prior speech is available only to resolve an explicit reference, never as another topic.
export function refersToPrior(text){return /(?:^|\s)(?:그(?:것|거|건|걸|게|곳|때|쪽|분)|이(?:것|거|건|걸|게)|저(?:것|거|곳)|거기|그\s+(?:사람|제품|시대)|it|that|this|they|them|their|there|these|those|he|she|his|her)(?:[은는이가을를의도]?)(?=\s|[,.!?。！？]|$)/iu.test(String(text));}
const compactEvidence=text=>String(text || '').normalize('NFC').toLowerCase().replace(/\s/gu,'');
const compactTopic=text=>String(text || '').normalize('NFC').toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
const briefAcknowledgement=text=>/^(?:네|예|응|어|음|아|맞아|맞아요|그렇구나|알겠어|알겠습니다|고마워|감사합니다|okay|ok|yes|yeah|hmm)[.!?。！？\s]*$/iu.test(String(text).trim());
// One optional decision at a time; never await this from transcription.
export class HintGate {
  constructor({decide,onHint,onStatus=()=>{},onTopicChange=()=>{},now=()=>Date.now(),cooldownMs=8000,generationIntervalMs=8000,intervalMs=2000,maxAgeMs=45000,displayAgeMs=45000,windowMs=8000,settleMs=500,repeatFocusMs=20000}) {
    Object.assign(this,{decide,onHint,onStatus,onTopicChange,now,cooldownMs,generationIntervalMs,intervalMs,maxAgeMs,displayAgeMs,windowMs,settleMs,repeatFocusMs});
    this.epoch=0;this.topicVersion=0;this.currentTopic='';this.topicDecisionSequence=0;this.entries=[];this.recent=[];this.failed=new Map();this.pending=null;this.inflight=null;
    this.generating=new Map();this.sequence=0;this.nextGeneration=-Infinity;this.lastDecision=-Infinity;this.nextHint=-Infinity;this.timer=null;
  }
  reset({forgetHistory=false}={}) {
    this.epoch++;this.topicVersion++;this.currentTopic='';this.topicDecisionSequence=0;this.generating.clear();this.failed.clear();this.pending=null;this.entries=[];
    this.recent=forgetHistory?[]:this.recent.filter(e=>this.now()-e.at<180000);
    clearTimeout(this.timer);this.timer=null;
    this.inflight?.controller.abort();
    this.lastDecision=this.nextHint=this.nextGeneration=-Infinity;
  }
  observe(text,{id,at=this.now(),preview=false}={}) {
    const now=this.now();
    if(!text?.trim() || now-at>this.maxAgeMs)return;
    this.entries=this.entries.filter(e=>now-e.at<=60000);
    const previous=id?this.entries.find(e=>e.id===id):null;
    // Promote an unchanged preview without treating it as a second utterance.
    if(previous?.text===text.slice(0,1600)){previous.preview=preview;previous.at=at;return;}
    if(previous)this.entries=this.entries.filter(e=>e!==previous);
    this.entries.push({id,preview,text:text.slice(0,1600),at,sequence:++this.sequence});this.entries=this.entries.slice(-16);
    const previousUtterance=this.entries.filter(e=>e.id!==id && now-e.at<=this.windowMs).at(-1);
    const context=refersToPrior(text)?previousUtterance?.text.slice(-200) || '':'';
    this.pending={id,preview,topicVersion:this.topicVersion,text:text.slice(-800),context,at,epoch:this.epoch,focusText:text.trim(),windowCount:1,sequence:this.sequence,
      due:Math.min(now+this.settleMs,this.pending?.due ?? now+this.settleMs)};
    this.pump();
  }
  withdraw(id){
    this.entries=this.entries.filter(e=>e.id!==id);
    if(this.pending?.id===id)this.pending=null;
  }
  validatePreview(id,text){
    const current=this.entries.find(e=>e.id===id);
    // Even a short correction must invalidate old evidence, without starting
    // another judgment for every character in the new draft.
    if(current?.preview)current.text=String(text).slice(0,1600);
  }
  fresh(job){const age=job.hint?(job.category==='reply'?Math.min(20000,this.displayAgeMs):this.displayAgeMs):this.maxAgeMs;return job.epoch===this.epoch && this.now()-job.at<=age;}
  // Generation acceptance is not a successful intervention.
  complete(job,{displayed=false,text='',topic=''}={}) {
    this.generating.delete(job.sequence);
    if(job.epoch!==this.epoch)return;
    if(displayed){
      this.recent.push({text:job.text,focusText:job.focusText,decisionKey:job.decisionKey,cue:text,topic,at:this.now()});this.recent=this.recent.slice(-24);
      this.nextHint=this.now()+this.cooldownMs;
    } else {
      const key=normalized(job.decisionKey || job.focusText || job.text).join('');
      if(key)this.failed.set(key,this.now()+4000);
    }
    this.pump();
  }
  duplicateCue(text){return this.recent.some(e=>this.now()-e.at<180000 && e.cue && similarHint(e.cue,text));}
  duplicateTopic(topic){const t=normalized(String(topic || '')).join('');return !!t && this.recent.some(e=>this.now()-e.at<this.cooldownMs && e.topic && normalized(e.topic).join('')===t);}
  shownFocus(){return this.recent.filter(e=>this.now()-e.at<this.repeatFocusMs).slice(-4).map(e=>e.focusText || e.text);}
  shownTopics(){return [...new Set(this.recent.filter(e=>this.now()-e.at<180000 && e.topic).slice(-4).map(e=>e.topic))].join(', ');}
  diversityContext(){return this.recent.filter(e=>this.now()-e.at<180000 && e.cue).slice(-6).map(e=>`[${e.topic || '정보'}] ${e.cue}`).join('\n').slice(-1200);}
  displayedContext(){return this.recent.filter(e=>this.now()-e.at<180000).slice(-4).map(e=>e.cue).filter(Boolean).join('\n');}
  confirmTopic(job,decision){
    const topic=decision.plan?.topic || decision.topic || '';
    const evidence=decision.focus || '';
    if(!topic || !evidence || !job.text.includes(evidence) || job.sequence<this.topicDecisionSequence)return;
    const latest=this.entries.filter(e=>!briefAcknowledgement(e.text)).at(-1);
    if(latest && latest.sequence>job.sequence && latest.id!==job.id)return;
    const before=compactTopic(this.currentTopic),next=compactTopic(topic);
    const compatible=before===next || Math.min(before.length,next.length)>=3 && (before.includes(next) || next.includes(before));
    // A missing name in the next sentence is not proof of a topic change.
    // Changed is accepted only from a grounded current classifier decision.
    const changed=decision.continuity==='changed' || (!decision.continuity && !compatible && compactTopic(job.text).includes(next));
    if(!before || changed){this.topicVersion++;if(before)this.onTopicChange({topic,version:this.topicVersion});}
    if(!before || changed || decision.continuity==='same' || compatible)this.currentTopic=topic;
    this.topicDecisionSequence=job.sequence;
  }
  rejectionReason(job){
    if(!this.fresh(job))return 'expired_or_canceled';
    if(job.topicVersion!==undefined && job.topicVersion!==this.topicVersion)return 'topic_changed';
    if(job.preview){
      const current=this.entries.find(e=>e.id===job.id);
      if(!current || !compactEvidence(current.text).includes(compactEvidence(job.focusText || job.text)))return 'transcript_changed';
    }
    return '';
  }
  relevant(job,topic=''){return !this.rejectionReason(job);}
  displayDelay(){return Math.max(0,this.nextHint-this.now());}
  duplicateHistory(kind='fact'){
    return this.recent.filter(e=>this.now()-e.at<180000).slice(-6).map(e=>kind==='plan'?e.decisionKey:e.cue).filter(Boolean);
  }

  async pump() {
    if(this.inflight || !this.pending)return;
    clearTimeout(this.timer);this.timer=null;
    const now=this.now();
    const wait=Math.max(this.lastDecision+this.intervalMs,this.nextHint,this.nextGeneration,this.pending.due || 0)-now;
    if(wait>0){this.timer=setTimeout(()=>{this.timer=null;this.pump();},wait);return;}
    const job={...this.pending,topicVersion:this.topicVersion,activeTopic:this.currentTopic};this.pending=null;
    if(!this.fresh(job)){this.pump();return;}
    const controller=new AbortController();const token={controller};this.inflight=token;
    this.lastDecision=now;
    const timeout=setTimeout(()=>controller.abort(),8000);
    try {
      const decision=await this.decide(job,controller.signal);
      if(!this.fresh(job) || controller.signal.aborted)return;
      if(decision.defer){
        if(!this.pending || this.pending.sequence<=job.sequence)this.pending={...job,due:this.now()+1000};
        return;
      }
      this.confirmTopic(job,decision);
      this.recent=this.recent.filter(e=>now-e.at<180000);
      for(const [key,candidate] of this.generating)if(!this.fresh(candidate))this.generating.delete(key);
      const focus=typeof decision.focus==='string' && job.text.includes(decision.focus.trim())?decision.focus.trim():'';
      const versionedJob={...job,observedText:job.text,topicVersion:this.topicDecisionSequence===job.sequence?this.topicVersion:job.topicVersion};
      let focusedJob=focus?{...versionedJob,text:focus,focusText:focus,context:[job.context,job.text].filter(Boolean).join('\n').slice(-1200)}:versionedJob;
      if(typeof decision.keyword==='string' && decision.keyword.length<=80 && typeof decision.keywordSurface==='string' && decision.keywordSurface.trim() && job.text.includes(decision.keywordSurface.trim()))focusedJob={...focusedJob,keyword:decision.keyword,keywordSurface:decision.keywordSurface};
      if(decision.plan)focusedJob={...focusedJob,decisionKey:`${decision.plan.topic} ${decision.plan.question}`};
      if(!this.relevant(focusedJob,decision.plan?.topic))return;
      for(const [k,until] of this.failed)if(until<=this.now())this.failed.delete(k);
      const focusKey=normalized(focusedJob.decisionKey || focusedJob.focusText || focusedJob.text).join('');
      if(decision.hint && focusKey && (this.failed.get(focusKey) || 0)>this.now()) {
        this.onStatus('직전 생략 후보 · 새 발화 대기');return;
      }
      if(decision.hint && [...this.recent.filter(e=>this.now()-e.at<this.repeatFocusMs),...this.generating.values()].some(e=>similarHint(e.decisionKey || e.focusText || e.text,focusedJob.decisionKey || focusedJob.focusText || focusedJob.text))) {
        this.onStatus('같은 내용 · 힌트 생략');return;
      }
      if(decision.hint) {
        const candidate={...focusedJob,category:decision.category,...(decision.plan?{hintPlan:decision.plan}:{})};
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
export function sameHintCriticalAtoms(a,b){
  const numbers=text=>(String(text).toLowerCase().match(/[+-]?[0-9]+(?:[.,][0-9]+)*(?:\s*[%\p{L}]+)?/gu) || []).map(v=>v.replace(/ /g,'')).sort().join('|');
  const negative=text=>/않|못|없|아니|불가|미지원|안\s*(?:됩니다|돼|되|지원)|\b(?:not|never|cannot|without|no)\b/iu.test(text);
  const direction=text=>[/시작|개시|착수|시초|\b(?:start|begin|opening)\b/iu,/종료|끝|마감|\b(?:end|finish|closing)\b/iu,/최대|최고|\b(?:maximum|highest|largest)\b/iu,/최소|최저|\b(?:minimum|lowest|smallest)\b/iu].reduce((bits,pattern,i)=>bits|(pattern.test(text)?1<<i:0),0);
  const da=direction(a),db=direction(b);
  return numbers(a)===numbers(b) && negative(a)===negative(b) && (!da || !db || da===db);
}
export function similarHint(a,b) {
  if(!sameHintCriticalAtoms(a,b))return false;
  const aa=normalized(a),bb=normalized(b);
  if(aa.join('')===bb.join(''))return true;
  if(Math.min(aa.length,bb.length)<6)return false;
  const grams=arr=>new Set(arr.slice(0,-2).map((_,i)=>arr.slice(i,i+3).join('')));
  const x=grams(aa),y=grams(bb);const intersection=[...x].filter(v=>y.has(v)).length;
  return intersection/(x.size+y.size-intersection)>=0.7;
}
