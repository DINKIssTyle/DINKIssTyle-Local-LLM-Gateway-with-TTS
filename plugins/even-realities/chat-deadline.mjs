// Explicit chat may load a large model, reason, and run tools. Match the
// gateway's five-minute budget with transport grace; ambient work stays short.
export function chatDeadline(controller,{chatSession=false,hint=false,setTimer=setTimeout,clearTimer=clearTimeout}={}){
  const maxMs=chatSession?330000:hint?30000:45000;
  const idleMs=chatSession?300000:0;
  let reason='',idleTimer;
  const expire=text=>{reason=text;controller.abort();};
  const maxTimer=setTimer(()=>expire(chatSession?'AI 채팅 응답 대기 시간 초과(5분 30초)':'AI 응답 처리 시간 초과'),maxMs);
  function touch(){
    if(!idleMs || controller.signal.aborted)return;
    clearTimer(idleTimer);
    idleTimer=setTimer(()=>expire('서버 응답이 5분 동안 없어 연결을 종료했습니다'),idleMs);
  }
  touch();
  return {touch,reason:()=>reason,stop(){clearTimer(maxTimer);clearTimer(idleTimer);}};
}

export function chatFailureMessage(error,deadline){
  if(deadline.reason())return deadline.reason();
  if(error instanceof TypeError || /load failed|failed to fetch|fetch failed|networkerror|network connection/i.test(error?.message || ''))
    return '서버 응답을 받는 중 연결 오류가 발생했습니다. 게이트웨이·모델 서버 연결을 확인해 주세요. 요청이 서버에서 계속 처리 중일 수 있습니다.';
  return error?.message || 'AI 응답 오류';
}
