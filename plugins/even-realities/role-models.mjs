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
