import {acknowledgementOnly,unresolvedTermTopic,unresolvedSpecificTask,hintTaskIssue} from './hint-quality.mjs';
import {judgePrompts,hintJudgeRepairPrompt} from './prompts.mjs';
export {judgePrompts} from './prompts.mjs';
import {HINT_LABELS,INFORMATION_HINT_KINDS,isInformationHintPlan} from './hint-policy.mjs';
export {HINT_LABELS} from './hint-policy.mjs';
import {readHintPlanResponse} from './hint-enrichment.mjs';

export function hintPresentation(plan={}){
  const kind=Object.hasOwn(HINT_LABELS,plan.kind)?plan.kind:'key';
  const reasons={term:'낯선 용어의 뜻·용법을 설명',key:'대화 이해에 필요한 정보를 보충',verify:'언급한 사실의 근거를 확인',schedule:'구체적인 날짜·기한을 확인',recap:'최근 대화의 요점·결정·비교를 요약'};
  const evidence=String(plan.evidence || '').slice(0,160);
  return {title:`[${HINT_LABELS[kind]}] ${plan.topic || '힌트'}`,reason:`${evidence?`‘${evidence}’ · `:''}${reasons[kind]}`};
}

export const JEV_ONLY='__jev__';
export function suggestedHintJudge(models){return models.find(model=>/gemma.*4.*e2b/i.test(model.id+' '+model.label))?.id || '';}
export const normalizeHintBudget=value=>Math.max(300,Math.min(800,Math.round((Number(value)||600)/100)*100));
// Conservative multilingual estimate, not a model-specific tokenizer. Never use chars/4 for Korean.
export function estimateHintTokens(text){
  let tokens=0;
  for(const c of String(text))tokens+=/[\u1100-\u11ff\u3000-\u9fff\uac00-\ud7af]/u.test(c)?2:/[a-zA-Z0-9 ]/.test(c)?.5:c.codePointAt(0)>127?3:1;
  return Math.ceil(tokens);
}
function tailWithin(text,budget){
  const chars=Array.from(String(text || ''));let cost=0,i=chars.length;
  while(i>0){const n=estimateHintTokens(chars[i-1]);if(cost+n>budget)break;cost+=n;i--;}
  // If the budget starts inside a word, discard that partial word rather
  // than feeding a manufactured name to the classifier.
  if(i>0 && /[\p{L}\p{N}]/u.test(chars[i-1]) && /[\p{L}\p{N}]/u.test(chars[i] || '')){
    const boundary=chars.slice(i).findIndex(c=>/\s/u.test(c));
    if(boundary>=0)i+=boundary+1;
  }
  return chars.slice(i).join('').trim();
}
function headWithin(text,budget){
  let output='',cost=0;
  for(const c of String(text || '')){const n=estimateHintTokens(c);if(cost+n>budget)break;cost+=n;output+=c;}
  return output.trim();
}
export function compactJudgeContext(job,shown='',_candidates=[],limit=600){
  const budget=normalizeHintBudget(limit),reserve=120;
  const collected=typeof job.latestText==='string';
  const current=tailWithin(collected?job.latestText:job.text,Math.floor((budget-reserve)*(collected ? .45 : .76)));
  const remaining=budget-reserve-estimateHintTokens(current);
  const prior=tailWithin(job.context,Math.floor(remaining*(collected ? .8 : .35)));
  const topic=headWithin(job.activeTopic,24);
  const compact=value=>String(value || '').normalize('NFC').toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
  const activeTopic=compact([prior,current].join(' ')).includes(compact(topic))?topic:'';
  const latest=collected?[prior,current].filter(Boolean).join('\n'):current;
  const evidenceChoices=hintEvidenceChoices(latest);
  const numbered=value=>hintEvidenceChoices(value).slice(1).map(part=>`[${evidenceChoices.indexOf(part)}] ${part}`).join('\n');
  const text=`CurrentTopic (continuity only):\n${activeTopic}\n${collected?'Recent conversation':'Reference (pronoun resolution only)'}:\n${collected?numbered(prior):prior}\nLatest:\n${numbered(current)}`;
  return {text,latest,prior,evidenceChoices,estimatedTokens:estimateHintTokens(text)+evidenceChoices.length*2,budget};
}
export function unwrapHintObject(raw){
  let value=raw;
  for(let depth=0;depth<4;depth++){
    if(typeof value==='string'){
      const text=value.trim().replace(/^```(?:json)?\s*/i,'').replace(/\s*```$/,'');
      try{value=JSON.parse(text);}catch{return null;}
    }
    if(!value || typeof value!=='object' || Array.isArray(value))return null;
    if(typeof value.result==='string' || (value.result && typeof value.result==='object')){value=value.result;continue;}
    return value;
  }
  return null;
}
export function parseHintJudgment(raw,context,onInvalid=()=>{}){
  let data=unwrapHintObject(raw);
  const modern=data && Object.hasOwn(data,'span');
  if(modern){
    if(!Number.isInteger(data.span) || data.span<0 || data.span>=context.evidenceChoices.length)return null;
    if(data.span===0)return {hint:false,reason:'llm_skip',backend:'llm',continuity:'unknown'};
    const topicKey=value=>String(value || '').normalize('NFC').toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
    let spanRecovered=false;
    if(typeof data.topic==='string' && topicKey(data.topic).length>=2 && !topicKey(context.evidenceChoices[data.span]).includes(topicKey(data.topic))){
      const matches=context.evidenceChoices.flatMap((text,index)=>index>0 && topicKey(text).includes(topicKey(data.topic))?[index]:[]);
      if(matches.length!==1){onInvalid('topic_span_mismatch');return null;}
      data={...data,span:matches[0]};spanRecovered=true;
    }
    const evidence=context.evidenceChoices[data.span];
    if(acknowledgementOnly(evidence))return {hint:false,reason:'llm_skip',backend:'llm',continuity:'unknown'};
    const taskIssue=hintTaskIssue(data.topic,data.need);
    if(taskIssue){onInvalid(taskIssue);return null;}
    if(spanRecovered)data={...data,spanRecovered:true};
    const kindAliases={mechanism:'term',definition:'term',comparison:'term',factcheck:'verify',fact_check:'verify'};
    data={...data,kind:kindAliases[data.kind] || data.kind,hint:true,evidence:data.span,question:data.need};
  }
  if(data && Number.isInteger(data.evidence))data={...data,evidence:context.evidenceChoices?.[data.evidence] ?? ''};
  const continuity=['same','changed','unknown'].includes(data?.continuity)?data.continuity:'unknown';
  if(data?.hint===false){
    const grounded=typeof data.topic==='string' && data.topic.trim().length<=80 && typeof data.evidence==='string' && data.evidence.trim() && data.evidence.length<=160 && context.latest.includes(data.evidence.trim());
    return {hint:false,reason:'llm_skip',backend:'llm',continuity,...(grounded?{topic:data.topic.trim(),focus:data.evidence.trim()}:{})};
  }
  if(data?.hint===true && (data.kind==='recap' || data.source==='conversation'))return {hint:false,reason:'conversation_summary_disabled',backend:'llm',continuity};
  if(data?.hint!==true || !isInformationHintPlan(data))return null;
  if(data.kind==='term' && unresolvedTermTopic(data.topic))return {hint:false,reason:'unresolved_hint_subject',backend:'llm',continuity};
  for(const [key,max] of [['topic',80],['evidence',160],['question',180]])if(typeof data[key]!=='string' || !data[key].trim() || Array.from(data[key]).length>max)return null;
  let evidence=data.evidence.trim();
  if(context.latest && evidence.includes(context.latest))evidence=context.latest;
  // The model must ground its selection in text it actually saw, from the current window.
  if(!context.latest.includes(evidence))return null;
  const key=value=>String(value).normalize('NFC').toLocaleLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
  // A source number cannot ground an unrelated topic or an empty sentence ending.
  if(!key(evidence).includes(key(data.topic)) || key(data.topic).length<2)return null;
  if(data.source==='internal' && unresolvedSpecificTask(data.question,evidence))return {hint:false,reason:'unresolved_hint_subject',backend:'llm',continuity};
  if(data.kind!==undefined && !Object.hasOwn(HINT_LABELS,data.kind))return null;
  // A definition/claim needs external verification even when the small model
  // accidentally calls it a conversation summary. Correct the route, not the evidence.
  const source=data.source;
  const plan={kind:data.kind || 'key',evidence,topic:data.topic.trim(),question:data.question.trim(),source,knownFacts:[context.latest]};
  return {hint:true,...(data.spanRecovered?{spanRecovered:true}:{}),...(modern?{}:{continuity}),category:data.source==='memory'?'recall':'background',focus:evidence,reason:'llm_topic',backend:'llm',plan};
}
const schema={type:'object',properties:{span:{type:'integer'},topic:{type:'string',maxLength:40},need:{type:'string',maxLength:100},kind:{type:'string',enum:INFORMATION_HINT_KINDS},source:{type:'string',enum:['internal','web','memory']}},required:['span','topic','need','kind','source'],additionalProperties:false};
// Keep sentences intact. Only oversized sentences need clause/word boundaries;
// the 160-code-point hard ceiling matches the evidence validator.
export function hintEvidenceChoices(text){
  const choices=[''];
  for(const sentence of String(text).match(/[^.!?。！？\n]+[.!?。！？]?/gu) || []){
    let rest=sentence.trim();
    while(rest){
      const chars=Array.from(rest);
      if(chars.length<=140){if(chars.length>=2)choices.push(rest);break;}
      const head=chars.slice(0,140).join('');
      const clauses=[...head.matchAll(/[,，;；:：]\s+|\s+/gu)].filter(m=>m.index>=Math.min(40,head.length/2));
      const match=clauses.at(-1);
      const end=match?match.index+match[0].length:head.length;
      const part=rest.slice(0,end).trim();if(part.length>=2)choices.push(part);
      rest=rest.slice(end).trimStart();
    }
  }
  return [...new Set(choices)].slice(0,32);
}
export async function requestHintJudgment({fetch,server,headers,model,job,shown='',limit=600,signal,preferWeb=true,sensitivity='balanced',targetLang='ko',timeoutMs=12000,onDiagnostic=()=>{}}){
  const context=compactJudgeContext(job,shown,[],limit);
  const prompts=judgePrompts(context,{preferWeb,sensitivity,targetLang});
  const requestSchema={...schema,properties:{...schema.properties,span:{type:'integer',enum:context.evidenceChoices.map((_,index)=>index)}}};
  const controller=new AbortController(),started=Date.now();
  let timedOut=false,attempt=0,requestMs=0,readMs=0,lastRaw='',phase='request',lastDiagnostic='',invalidReason='';
  const diagnostic=(reason,extra={})=>{const key=JSON.stringify([reason,Math.min(attempt+1,2),phase,extra]);if(key===lastDiagnostic)return;lastDiagnostic=key;try{onDiagnostic({reason,model,role:'hintDecision',attempt:Math.min(attempt+1,2),phase,elapsedMs:Date.now()-started,requestMs,readMs,...extra});}catch{}};
  let rejectAbort;
  const interrupted=new Promise((_,reject)=>{rejectAbort=reject;});
  const abort=()=>{
    controller.abort();
    const error=Object.assign(Error(timedOut?'힌트 판별 시간 초과':'힌트 판별 취소'),{name:'AbortError',code:timedOut?'judge_timeout':'judge_canceled'});
    rejectAbort(error);
  };
  if(signal?.aborted)abort();else signal?.addEventListener('abort',abort,{once:true});
  const timer=setTimeout(()=>{timedOut=true;abort();},timeoutMs);
  try{
    for(attempt=0;attempt<2;attempt++){
      if(controller.signal.aborted)await interrupted;
      const repair=hintJudgeRepairPrompt(attempt,invalidReason);
      phase='request';requestMs=readMs=0;lastRaw='';
      const requestStart=Date.now();
      const response=await Promise.race([fetch(`${server}/api/chat`,{method:'POST',headers,signal:controller.signal,body:JSON.stringify({model,stream:true,store:false,glasses_hint:true,glasses_hint_phase:'judge',max_tokens:384,temperature:.1,chat_template_kwargs:{enable_thinking:false},response_format:{type:'json_schema',json_schema:{name:'hint_judgment',strict:true,schema:requestSchema}},messages:[{role:'system',content:prompts.systemPrompt+repair},{role:'user',content:prompts.userPrompt}]})}),interrupted]);
      requestMs=Date.now()-requestStart;
      if(!response.ok)throw Object.assign(Error(`힌트 판별 HTTP ${response.status}`),{code:'judge_http_error',status:response.status});
      phase='response';const readStart=Date.now();
      lastRaw=await Promise.race([readHintPlanResponse(response,{signal:controller.signal,onProgress:text=>{lastRaw=text;}}),interrupted]);
      readMs=Date.now()-readStart;
      if(controller.signal.aborted)await interrupted;
      invalidReason='';
      const result=parseHintJudgment(lastRaw,context,reason=>{invalidReason=reason;});
      diagnostic(result?'judge_response':'judge_invalid_response',{response:Array.from(String(lastRaw)).slice(0,1200).join(''),...(invalidReason?{validation:invalidReason}:{}),...(result?.spanRecovered?{spanRecovered:true}:{})});
      if(result)return {...result,repairAttempts:attempt,elapsedMs:Date.now()-started,observedText:context.latest,contextEstimatedTokens:context.estimatedTokens,contextBudget:context.budget};
    }
    throw Object.assign(Error('힌트 판별의 제목·작업·원문 근거가 비어 있어 표시할 수 없습니다'),{code:'judge_invalid_response'});
  }catch(error){
    if(controller.signal.aborted)error=Object.assign(Error(timedOut?'힌트 판별 시간 초과':'힌트 판별 취소'),{name:'AbortError',code:timedOut?'judge_timeout':'judge_canceled'});
    if(!error.code || !String(error.code).startsWith('judge_'))error.code=error instanceof SyntaxError?'judge_invalid_response':'judge_request_error';
    diagnostic(error.code,{status:error.status || undefined,response:Array.from(String(lastRaw)).slice(0,1200).join('')});
    throw error;
  }finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
}
