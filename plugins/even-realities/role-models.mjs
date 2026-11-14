export const ROLE_MODEL_KEYS=['chat','hintDecision','hint','translation','translationReply'];
export function normalizeRoleModels(value){
  return Object.fromEntries(ROLE_MODEL_KEYS.map(role=>[role,typeof value?.[role]==='string'?value[role].trim():'']));
}
export function resolveRoleModel(role,config){return normalizeRoleModels(config.roleModels)[role] || String(config.model || '').trim();}
export function llmRequestRole({mode,chatSession=false,translated=false}={}){
  return mode==='translate'?(translated?'translationReply':'translation'):(chatSession?'chat':'hint');
}
export function roleModelOptions(models,base,current=''){
  const baseLabel=models.find(m=>m.id===base)?.label || base || '미선택';
  const ordered=models.filter(m=>m.id===base).concat(models.filter(m=>m.id!==base));
  const options=[{id:'',label:`기본 모델 사용 (${baseLabel})`},...ordered];
  if(current && !models.some(m=>m.id===current))options.push({id:current,label:`${current} · 현재 목록에 없음`});
  return options;
}
export function translationOnlyPrompts(original,source,target,context=''){
  return {
    systemPrompt:'당신은 실시간 통역 비서입니다. 입력 발화 전체를 목표 언어로 자연스럽게 번역하세요. 질문에 답하거나 긍정·부정 답변 예시를 만들지 마세요. 참고 문맥은 대명사나 생략된 대상을 이해하는 용도이며 번역 대상이 아닙니다. 원문과 같은 언어라면 자연스럽게 다듬으세요. 반드시 JSON 객체 {"translation":"번역문"}만 출력하세요. 제목·마크다운·설명은 넣지 마세요.',
    userPrompt:`인식 언어: ${source}\n목표 언어: ${target}\n짧은 참고 문맥:\n${context || '(없음)'}\n번역할 발화:\n${original}`
  };
}
