import {usefulHintText} from './hint-quality.mjs';
import {HINT_LABELS,isInformationHintPlan} from './hint-policy.mjs';
import {measureTextWrap} from '@evenrealities/pretext';
import {cleanHintText} from './hud-status.mjs';

function parseObject(raw) {
  const text=String(raw).trim().replace(/^```(?:json)?\s*/i,'').replace(/\s*```$/,'');
  try {
    let value=JSON.parse(text);
    for(let depth=0;depth<3 && value && typeof value==='object' && !Array.isArray(value) && value.result!==undefined;depth++){
      value=typeof value.result==='string'?JSON.parse(value.result.trim().replace(/^```(?:json)?\s*/i,'').replace(/\s*```$/,'')):value.result;
    }
    return value;
  }catch {
    const start=text.indexOf('{'),end=text.lastIndexOf('}');
    if(start>=0 && end>start){try{return JSON.parse(text.slice(start,end+1));}catch{}}
    return null;
  }
}
export function parseHintPlan(raw) {
  if(raw.trim()==='HINT_SKIP')return null;
  const plan=parseObject(raw);
  if(!plan || plan.skip===true || !isInformationHintPlan(plan))return null;
  if(!['internal','web','memory','conversation'].includes(plan.source) || typeof plan.topic!=='string' || !plan.topic.trim() || typeof plan.question!=='string' || !plan.question.trim() || Array.from(plan.question).length>300)return null;
  if(plan.source==='conversation' && !plan.kind)return null;
  return {topic:plan.topic.trim().slice(0,80),question:plan.question.trim(),source:plan.source,...(Object.hasOwn(HINT_LABELS,plan.kind)?{kind:plan.kind}:{}),
    knownFacts:Array.isArray(plan.knownFacts)?plan.knownFacts.filter(v=>typeof v==='string').slice(0,6).map(v=>v.slice(0,200)):[]};
}
const normalized=text=>text.toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
function stripConversationalLeadIn(text) {
  return String(text)
    .replace(/^(?:네[,.]?\s+|예[,.]?\s+|질문하신\s+|문의하신\s+|말씀하신\s+|알려드리자면[,.]?\s+|확인해\s*보면[,.]?\s+)+/u,'')
    .replace(/\s*(?:더\s*궁금한\s*점이\s*있으시면\s*말씀하세요\.?|도움이\s*되셨길\s*바랍니다\.?)$/u,'')
    .trim();
}
export function hintResultKind(raw,fallback='key') {
  const kind=parseObject(raw)?.kind;
  return Object.hasOwn(HINT_LABELS,kind)?kind:Object.hasOwn(HINT_LABELS,fallback)?fallback:'key';
}
export function extractHintFact(raw,observed,plan) {
  const text=cleanHintText(raw);
  if(text==='HINT_SKIP')return '';
  const data=parseObject(text);
  const rawFact=typeof data?.fact==='string'?data.fact:(typeof data?.suggestion==='string'?data.suggestion:'');
  if(!data || data.adds_information!==true || !rawFact)return '';
  const fact=stripConversationalLeadIn(cleanHintText(rawFact));
  const compact=normalized(fact);
  if(!usefulHintText(fact,plan.topic))return '';
  if(!compact || normalized(observed).includes(compact) || plan.knownFacts.some(v=>normalized(v)===compact))return '';
  if(/\b(?:you|we|they|the speakers?) (?:are|were) (?:talking|discussing|mentioning)\b|\b(?:this|the) conversation (?:is|was) about\b/iu.test(fact))return '';
  if(/(?:이야기|논의|언급|대화)(?:하고|되고|하는|되었습니다|했습니다)|기억해\s*(?:두|놓)|확인해\s*보세요/u.test(fact))return '';
  return fact;
}

// Planning output is private; consume both gateway SSE dialects without rendering.
export async function readHintPlanResponse(res,{onProgress=()=>{},signal}={}) {
  if(!res.ok)throw new Error(`힌트 정보 계획 실패: HTTP ${res.status}`);
  if(!res.body){const d=await res.json();return d.choices?.[0]?.message?.content || d.response || '';}
  const reader=res.body.getReader(),decoder=new TextDecoder();let buffer='',answer='',event='';
  const line=value=>{
    value=value.replace(/\r$/,'');if(!value){event='';return;}
    if(value.startsWith('event:')){event=value.slice(6).trim();return;}
    if(!value.startsWith('data:'))return;
    const raw=value.slice(5).trim();if(!raw || raw==='[DONE]')return;
    if(event==='error')throw new Error(raw);
    const d=JSON.parse(raw);if(d.error)throw new Error(typeof d.error==='string'?d.error:d.error.message || '힌트 계획 오류');
    if(typeof d.final_assistant_content==='string')answer=d.final_assistant_content;
    else if(d.result?.output){answer=d.result.output.filter(v=>v.type==='message').map(v=>v.content || '').join('');}
    else if((d.type || event)==='message.delta')answer+=d.content || '';
    else answer+=d.choices?.[0]?.delta?.content || '';
    onProgress(answer);
  };
  const cancelReader=()=>{try{Promise.resolve(reader.cancel?.()).catch(()=>{});}catch{}};
  if(signal?.aborted)cancelReader();else signal?.addEventListener('abort',cancelReader,{once:true});
  try{
    while(true){
      if(signal?.aborted)throw Object.assign(Error('힌트 판별 취소'),{name:'AbortError'});
      const {done,value}=await reader.read();
      if(signal?.aborted)throw Object.assign(Error('힌트 판별 취소'),{name:'AbortError'});
      buffer+=decoder.decode(value,{stream:!done});const lines=buffer.split('\n');buffer=lines.pop();for(const v of lines)line(v);if(done){if(buffer)line(buffer);break;}
    }
    return answer;
  }finally{signal?.removeEventListener('abort',cancelReader);}

}


// Called only after JEV selects a candidate. Most cues need one LLM request;
// research is a second request only when external evidence is needed.
export function parseCueResponse(raw,observed) {
  const data=parseObject(raw);
  if(!data || data.skip || !['internal','web','memory','conversation'].includes(data.source))return null;
  if(typeof data.fact==='string' && data.source==='internal'){
    if(!['concept','person','suggestion'].includes(data.cue_type) || typeof data.topic!=='string' || !data.topic.trim())return null;
    const fact=extractHintFact(raw,observed,{knownFacts:Array.isArray(data.knownFacts)?data.knownFacts.filter(v=>typeof v==='string'):[]});
    return fact?{...data,fact}:null;
  }
  const plan=parseHintPlan(raw);
  return plan?{...plan,cue_type:data.cue_type || 'concept'}:null;
}
export function fitCue(text,width=570,lines=6) {
  const sentences=String(text).trim().match(/[^.!?。！？]+[.!?。！？]+(?:[”’"']+)?|[^.!?。！？]+$/gu) || [];
  let output='';
  for(const sentence of sentences){
    const next=(output+' '+sentence.trim()).trim();
    if(measureTextWrap(next,width).lineCount>lines)break;
    output=next;
  }
  return output;
}
