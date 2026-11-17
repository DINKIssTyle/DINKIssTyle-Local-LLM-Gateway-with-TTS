export const normalizeAIHeaderLayout=value=>value==='status-icon'?'status-icon':'standard';
export function aiStatusIcon({error=false,preparing=false,listening=false,audio=false,transcribing=false,working=false,phase='',progress=null}={}){
 if(error)return {key:'status-warning',label:'오류 · 연결 확인'};
 if(working){
  if(/프롬프트|prompt_processing/iu.test(phase)){
   const p=typeof progress==='number'&&Number.isFinite(progress)?Math.max(0,Math.min(100,progress)):null;
   return {key:p===null?'status-prompt-unknown':`status-prompt-${Math.floor(p/12.5)}`,label:p===null?'프롬프트 처리중':`프롬프트 처리중 ${Math.round(p)}%`};
  }
  if(/답변.*(?:작성|생성)|answering/iu.test(phase))return {key:'mode-ai',label:'답변 생성중'};
  return {key:'status-thinking',label:'생각중'};
 }
 if(transcribing && !preparing)return {key:'status-transcribing',label:'전사중'};
 if(preparing || !listening || !audio)return {key:'status-warning',label:preparing?'마이크·음성 엔진 준비중':!listening?'마이크 꺼짐':'오디오 수신 대기'};
 return {key:'status-listening',label:'듣는중'};
}
