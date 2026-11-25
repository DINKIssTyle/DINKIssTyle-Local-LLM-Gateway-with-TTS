// Separate diagnostic events. Delivery is best effort and never blocks STT or
// card display; pending records are bounded and never enter prompt memory.
export class HintAudit {
 constructor({fetch,spec,version,onStatus=()=>{},schedule=setTimeout,cancel=clearTimeout,now=()=>Date.now()}={}){
  Object.assign(this,{fetch,spec,version,onStatus,schedule,cancel,now});this.pending=[];this.active=false;this.timer=null;this.sequence=0;this.nonce=Math.random().toString(36).slice(2);this.failedUntil=0;this.scope='';
 }
 record(event){
  const spec=this.spec();if(!spec?.server)return;
  const scope=JSON.stringify([spec.server,spec.headers?.Authorization || '']);
  if(scope!==this.scope){this.pending=[];this.scope=scope;this.failedUntil=0;}
  if(this.now()<this.failedUntil)return;
  const cut=(value,n)=>Array.from(String(value || '')).slice(0,n).join('');
  const entry={eventId:`${this.nonce}-${this.now()}-${++this.sequence}`,sessionId:cut(event.sessionId,100),segmentId:cut(event.segmentId,100),jobId:cut(event.jobId,120),stage:event.stage,reason:cut(event.reason,240),topic:cut(event.topic,80),kind:cut(event.kind,16),source:cut(event.source,24),evidence:cut(event.evidence,160),text:['generated','phone_display','decision_response','generation','device_error'].includes(event.stage)?cut(event.text,2000):'',version:this.version,occurredAt:new Date(this.now()).toISOString()};
  this.pending.push({entry,scope,spec});this.pending=this.pending.slice(-64);
  if(!this.timer)this.timer=this.schedule(()=>{this.timer=null;this.flush();},250);
 }
 async flush(){
  if(this.active || !this.pending.length)return;
  if(this.timer){this.cancel(this.timer);this.timer=null;}
  const scope=this.scope,items=this.pending.splice(0,16).filter(x=>x.scope===scope);if(!items.length)return;
  this.active=true;const controller=new AbortController();let rejectTimeout;const timedOut=new Promise((_,reject)=>{rejectTimeout=reject;});const timeout=this.schedule(()=>{controller.abort();rejectTimeout(Error('이력 저장 시간 초과'));},2000);
  try{
   const {spec}=items[0];const r=await Promise.race([this.fetch(`${spec.server}/api/glasses/hint-events`,{method:'POST',headers:spec.headers,signal:controller.signal,body:JSON.stringify({events:items.map(x=>x.entry)})}),timedOut]);
   if(!r.ok)throw Error(`HTTP ${r.status}`);
   if(scope===this.scope)this.onStatus('힌트 처리 이력 저장됨');
  }catch(error){if(scope===this.scope){this.failedUntil=this.now()+30000;this.pending=[];this.onStatus(`힌트 이력 저장 실패 · ${error.message}`);}}
  finally{this.cancel(timeout);this.active=false;if(this.pending.length)this.flush();}
 }
}

const clip=(value,limit)=>Array.from(String(value || '')).slice(0,limit).join('');
const reasonGroups={
 insufficient_context:['insufficient_context','unresolved_hint_subject','unresolved_hint_task'],
 no_incremental_value:['no_additional_information','no_information_value','transcript_echo','empty_or_echo','duplicate_candidate','duplicate_plan','already_visible','heading_only','navigation_only','no_summary_value'],
 insufficient_evidence:['source_does_not_answer_task','incomplete_or_empty_source','unsupported_quote_or_name','unverified_name'],
 error:['judge_timeout','judge_invalid_response','judge_empty_response','judge_stream_error','judge_http_error','judge_request_error','writer_timeout','writer_error','hint_translation_failed','missing_proof','invalid_hint_response','invalid_source_selection','mixed_source_records','invalid_quote','no_fact','sdk_transport_error','source_lookup_failed'],
 lifecycle:['candidate_expired','expired_or_canceled','topic_changed','past_topic','transcript_changed','transcript_revised','selection_evidence_changed','pending_replaced','chat_priority','chat_or_capture_priority','recent_failed_candidate','canceled','user_dismissed'],
 policy:['conversation_summary_disabled','memory_not_requested','chat_history_artifact','translation_example_only']
};
export function hintReasonGroup(event={}){
 // Diagnostic details can describe errors too; classify the underlying reason,
 // not all decision_response events as successful decisions.
 for(const [group,reasons] of Object.entries(reasonGroups))if(reasons.includes(event.reason))return group;
 // A generic judge skip does not establish *why* nothing was selected.
 return ['skipped','canceled'].includes(event.stage) || ['llm_skip','jev_skip','no_hint','not_completed'].includes(event.reason)?'other':'';
}
export const hintReasonGroupLabel=group=>({insufficient_context:'문맥 부족',no_incremental_value:'추가 가치 없음',insufficient_evidence:'근거 부족',error:'처리 오류',lifecycle:'진행·취소',policy:'정책에 따른 생략',other:'기타·사유 미확정'})[group] || '';

function sourceURL(value){
 try{const url=new URL(value);if(!['http:','https:'].includes(url.protocol) || url.username || url.password)return '';url.search='';url.hash='';return url.href.length<=240?url.href:'';}catch{return '';}
}
// Store only the source which passed validation, not every search hit. The
// trigger explains interest; it is never presented as proof of a public fact.
export function hintProvenance(job={},plan={},verified={}){
 const evidence=plan.evidence || plan.anchor || job.focusText || '';
 const entries=Array.isArray(job.evidenceEntries)?job.evidenceEntries:[];
 const provenance={version:1,task:clip(plan.question,160),trigger:{segmentId:clip(entries.find(entry=>evidence && String(entry.text || '').includes(evidence))?.id || job.id,100),text:clip(evidence,160)},transcripts:entries.slice(0,2).map(e=>({segmentId:clip(e.id,100),text:clip(e.text,100)})),knowledge:verified.source || plan.source || 'internal',sources:[],quote:clip(verified.fact,240)};
 if(verified.reference){
  const selection=verified.selection;
  provenance.sources.push({title:clip(verified.reference.title,80),url:sourceURL(verified.reference.url),...(selection?{record:selection.record,sentences:selection.sentences}:{} )});
 }
 // Keep JSON parseable under the existing 2,000-character event limit even
 // when ASR/source text contains many JSON escapes. Never slice serialized JSON.
 while(Array.from(JSON.stringify(provenance)).length>1900){
  if(provenance.quote)provenance.quote=clip(provenance.quote,Array.from(provenance.quote).length-40);
  else if(provenance.transcripts.length)provenance.transcripts.pop();
  else if(provenance.task)provenance.task=clip(provenance.task,Array.from(provenance.task).length-40);
  else if(provenance.trigger.text)provenance.trigger.text=clip(provenance.trigger.text,Array.from(provenance.trigger.text).length-40);
  else {provenance.sources=[];provenance.trigger.segmentId='';}
 }
 return JSON.stringify(provenance);
}
export function formatHintProvenance(text){
 let data;try{data=JSON.parse(text);}catch{return '출처 추적 기록 형식 오류';}
 if(data?.version!==1 || !data.trigger || !Array.isArray(data.sources) || !Array.isArray(data.transcripts))return '출처 추적 기록 형식 오류';
 const lines=[data.task?`정보 과제: ${data.task}`:'',`발생 계기 (사실 근거 아님): ${data.trigger.text || '기록 없음'}`,data.trigger.segmentId?`전사 구간: ${data.trigger.segmentId}`:''];
 for(const entry of data.transcripts)lines.push(`관련 전사 [${entry.segmentId || '?'}]: ${entry.text || ''}`);
 if(data.knowledge==='internal')lines.push('사실 근거: 모델 내부 지식 · 외부 출처 검증 없음');
 else if(!data.sources.length)lines.push('사실 근거: 출처 기록 없음');
 for(const source of data.sources){
  lines.push(`${data.knowledge==='memory'?'저장된 개인 기록':'외부 출처'}: ${source.title || '제목 없음'}`);
  const url=sourceURL(source.url);if(url)lines.push(url);
  if(source.record && Array.isArray(source.sentences))lines.push(`근거 문장: 자료 ${source.record} / 문장 ${source.sentences.join(', ')}`);
 }
 if(data.quote)lines.push(`${data.knowledge==='internal'?'생성 정보':'검증된 원문'} (일부): ${data.quote}`);
 return lines.filter(Boolean).join('\n');
}
export function formatHintHistoryEvent(event,{stageLabel=event.stage,reasonLabel=event.reason,title=event.topic || ''}={}){
 const group=hintReasonGroupLabel(hintReasonGroup(event));
 const body=event.reason==='hint_provenance'?formatHintProvenance(event.text):event.text || '';
 return `${new Date(event.occurredAt || event.createdAt).toLocaleString()} [${stageLabel}] ${title}${event.jobId?`\n힌트 연결: ${event.jobId}`:''}${group?`\n분류: ${group}`:''}${reasonLabel?`\n${reasonLabel}`:''}${event.evidence && event.reason!=='hint_provenance'?`\n발생 계기 (사실 근거 아님): ${event.evidence}`:''}${body?'\n'+body:''}`;
}
