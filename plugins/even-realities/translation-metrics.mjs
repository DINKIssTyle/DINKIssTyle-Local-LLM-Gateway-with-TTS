// Bounded numeric diagnostics only. No speech, translations, prompts or secrets.
export class TranslationMetrics {
 constructor({limit=32,onUpdate=()=>{}}={}){this.limit=limit;this.onUpdate=onUpdate;this.entries=new Map();}
 record(id,values){
  if(!id)return;
  const allowed=['stt_first_result_ms','stt_http_ms','stt_server_ms','stt_lock_ms','stt_engine_ms','save_ms','upload_bytes','audio_bytes','translation_queue_ms','translation_http_ms','translation_server_ms','first_text_ms','sdk_ms','sdk_failed'];
  const entry=this.entries.get(id)||{};
  for(const key of allowed)if(Number.isFinite(values[key]))entry[key]=Math.max(0,Math.round(values[key]));
  this.entries.delete(id);this.entries.set(id,entry);while(this.entries.size>this.limit)this.entries.delete(this.entries.keys().next().value);
  this.onUpdate({...entry});
 }
}
