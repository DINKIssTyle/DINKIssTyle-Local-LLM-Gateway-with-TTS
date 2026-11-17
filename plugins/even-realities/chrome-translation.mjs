import {abortableWait} from './abortable-wait.mjs';
// This bridge talks only to the user's authenticated gateway. The request carries provisional or finalized text, never microphone PCM.
// Transcription may come from Whisper or the separately selected Chrome STT engine.
export async function localTranslate(fetcher,server,headers,request,signal,timeoutMs=8000,engine="chrome"){
  if(!["chrome","apple"].includes(engine))return {ok:false,reason:"잘못된 번역 엔진"};
  const label=engine==="apple"?"Apple":"Chrome";
  const controller=new AbortController();
  const abort=()=>controller.abort();
  if(signal?.aborted)throw Object.assign(new Error('번역 취소'),{name:'AbortError'});
  signal?.addEventListener('abort',abort,{once:true});
  const timer=setTimeout(abort,timeoutMs);
  try{
    const response=await abortableWait(fetcher(`${server}/api/translate/${engine}`,{method:'POST',headers,body:JSON.stringify(request),signal:controller.signal}),controller.signal);
    const data=await abortableWait(response.json(),controller.signal);
    if(response.status===429 || data.status==='busy')return {ok:false,busy:true,reason:data.error || '번역 처리 중'};
    if(!response.ok || data.status!=='ready' || typeof data.text!=='string' || !data.text.trim())return {ok:false,needsPrepare:['needs_prepare','needs-prepare','downloadable','unavailable'].includes(data.status) || /준비.*먼저|언어.*설치/i.test(data.error || ''),reason:data.error || `${label} 번역 준비가 필요합니다`};
    return {ok:true,text:data.text.trim(),source:data.source,target:data.target,timings:data.timings};
  }catch(error){
    if(signal?.aborted)throw error;
    return {ok:false,reason:controller.signal.aborted?`${label} 번역 시간 초과`:error.message};
  }finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
}
export function chromeTranslate(...args){return localTranslate(...args);}
