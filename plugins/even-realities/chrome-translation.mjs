// This bridge talks only to the user's authenticated Mac gateway. The request carries provisional or finalized text, never microphone PCM.
// Transcription may come from Whisper or the separately selected Chrome STT engine.
export async function chromeTranslate(fetcher,server,headers,request,signal,timeoutMs=8000){
  const controller=new AbortController();
  const abort=()=>controller.abort();
  if(signal?.aborted)throw Object.assign(new Error('번역 취소'),{name:'AbortError'});
  signal?.addEventListener('abort',abort,{once:true});
  const timer=setTimeout(abort,timeoutMs);
  try{
    const response=await fetcher(`${server}/api/translate/chrome`,{method:'POST',headers,body:JSON.stringify(request),signal:controller.signal});
    const data=await response.json();
    if(!response.ok || data.status!=='ready' || typeof data.text!=='string' || !data.text.trim())return {ok:false,reason:data.error || 'Chrome 번역 준비가 필요합니다'};
    return {ok:true,text:data.text.trim(),source:data.source,target:data.target};
  }catch(error){
    if(signal?.aborted)throw error;
    return {ok:false,reason:controller.signal.aborted?'Chrome 번역 시간 초과':error.message};
  }finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
}
export function translationReplyPrompts(original,translated,source,target){
  return {
    systemPrompt:'당신은 대화 답변 추천 비서입니다. 상대방 발화의 번역은 이미 완료되었습니다. 번역문을 생성·수정하지 마세요. 상대방의 말에 사용자가 할 긍정·동의·수락 답변 하나와 부정·반대·거절 답변 하나를 작성하세요. 상대방 발화를 번역하거나 반복하면 안 됩니다. 각 답변은 첫 줄에 목표 언어의 짧은 답변, 둘째 줄에 인식 언어로 같은 뜻을 쓰세요. JSON 문자열 내 줄바꿈은 \\n으로 이스케이프하세요. JSON 객체 {"positive":"목표 언어 답변\\n인식 언어 뜻","negative":"목표 언어 답변\\n인식 언어 뜻"}만 출력하세요. translation 필드는 출력하지 마세요. 각 언어별로 한 문장씩 간결하게 작성하세요.',
    userPrompt:`인식 언어: ${source}\n목표 언어: ${target}\n상대방의 실제 발화(참고 자료):\n${original}\n이미 완료된 번역(변경하지 말 것):\n${translated}\n이 말에 사용자가 대답할 두 가지 예시만 작성하세요.`
  };
}
