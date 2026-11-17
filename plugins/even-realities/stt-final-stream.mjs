import {abortableWait} from './abortable-wait.mjs';
// Recognition enables live translation/preview; only the saved event allows
// callers to release the original audio and mark the transcript as archived.
export async function readFinalTranscription(response,{signal,onRecognized=()=>{}}={}){
 if(!response.headers?.get('content-type')?.includes('text/event-stream') || !response.body)return abortableWait(response.json(),signal);
 const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='',saved=null,recognized=false;
 const cancel=()=>{try{Promise.resolve(reader.cancel?.()).catch(()=>{});}catch{}};
 const consume=line=>{
  if(saved || !line.startsWith('data:'))return;
  const value=JSON.parse(line.slice(5).trim());
  if(value.type==='stt.error')throw Object.assign(Error(value.message || '전사 저장 실패'),{retryable:value.retryable!==false});
  if(value.type==='stt.recognized' && !recognized){recognized=true;onRecognized(value.result);}
  if(value.type==='stt.saved')saved=value.result;
 };
 try{
  while(!saved){
   const {done,value}=await abortableWait(reader.read(),signal,cancel);buffer+=decoder.decode(value,{stream:!done});
   const lines=buffer.split('\n');buffer=lines.pop();for(const line of lines)consume(line.replace(/\r$/,''));
   if(buffer.length>256*1024)throw Error('전사 응답 크기 초과');
   if(done){if(buffer.trim())consume(buffer.trim());break;}
  }
  if(!saved)throw Object.assign(Error('전사 저장 완료 응답을 받지 못했습니다'),{retryable:true});
  return saved;
 }finally{cancel();try{reader.releaseLock?.();}catch{}}
}
