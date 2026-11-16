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
  const entry={eventId:`${this.nonce}-${this.now()}-${++this.sequence}`,sessionId:cut(event.sessionId,100),segmentId:cut(event.segmentId,100),jobId:cut(event.jobId,120),stage:event.stage,reason:cut(event.reason,240),topic:cut(event.topic,80),kind:cut(event.kind,16),source:cut(event.source,24),evidence:cut(event.evidence,160),text:['generated','phone_display','decision_response'].includes(event.stage)?cut(event.text,2000):'',version:this.version,occurredAt:new Date(this.now()).toISOString()};
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
