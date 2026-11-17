import {usefulHintText,completeHintQuote,hintContentIssue} from './hint-quality.mjs';
import {unwrapHintObject,HINT_LABELS} from './hint-judge.mjs';

const textKey=text=>String(text || '').normalize('NFC').replace(/\s+/gu,' ').trim();
const nameKey=text=>textKey(text).toLocaleLowerCase().replace(/\s/gu,'');
const copyKey=text=>textKey(text).replace(/[\p{P}\s]/gu,'');
const numberKey=text=>(String(text).match(/[+-]?\d+(?:[.,]\d+)*/gu) || []).join('|');

// Model confidence and query echoes are not evidence. A quote must come from
// one server-provided source record, and its label must exist in that record.
export function validateHintGrounding(raw,proof){
  if(proof?.version!==1 || !['known','quote','conversation'].includes(proof.mode))return {ok:false,reason:'missing_proof'};
  const data=unwrapHintObject(raw);
  if(proof.mode==='quote' && proof.task_oriented===true){
    let selection=data;
    if(!selection){try{const value=JSON.parse(String(raw).trim().replace(/^```(?:json)?\s*/i,'').replace(/\s*```$/,''));if(Array.isArray(value))selection=value;}catch{}}
    return selection?validateContextHintSelection(selection,proof):{ok:false,reason:'invalid_hint_response'};
  }
  if(!data)return {ok:false,reason:'invalid_hint_response'};
  if(proof.mode==='conversation')return conversationHint(data,proof);
  if(proof.mode==='quote' && Object.hasOwn(data,'selection')){
    if(!Number.isInteger(data.selection) || data.selection<0)return {ok:false,reason:'invalid_source_selection'};
    if(data.selection===0)return {ok:false,reason:'source_does_not_answer_task'};
    const choices=(proof.records || []).slice(0,4).flatMap((record,index)=>(Array.isArray(record.spans)?record.spans:[]).slice(0,16).map((_,sentence)=>({record:index+1,sentences:[sentence+1],adds_information:true})));
    const selected=choices[data.selection-1];
    return selected?validateHintGrounding(JSON.stringify(selected),proof):{ok:false,reason:'invalid_source_selection'};
  }
  if(proof.mode==='quote' && data.record===0 && data.adds_information===false && Array.isArray(data.sentences) && !data.sentences.length)return {ok:false,reason:'source_does_not_answer_task'};
  if(data.adds_information!==true)return {ok:false,reason:data.adds_information===false?'no_additional_information':'no_fact'};
  const kind=Object.hasOwn(HINT_LABELS,proof.kind)?proof.kind:data.kind;
  if(proof.mode==='known'){
    if(typeof data.fact!=='string' || !data.fact.trim())return {ok:false,reason:'no_fact'};
    if(typeof proof.topic!=='string' || !proof.topic.trim())return {ok:false,reason:'unknown_name'};
    const allowed=[proof.topic,...(Array.isArray(proof.aliases)?proof.aliases:[])].map(nameKey);
    if(!proof.task_oriented && data.topic && !allowed.includes(nameKey(data.topic)))return {ok:false,reason:'unverified_name'};
    const issue=hintContentIssue(data.fact,{topic:proof.topic,task:proof.task});
    if(issue)return {ok:false,reason:issue};
    return {ok:true,fact:data.fact.trim(),topic:proof.topic.trim(),kind,source:'internal'};
  }
  if(Object.hasOwn(data,'record') || Object.hasOwn(data,'sentences')){
    if(!Number.isInteger(data.record) || data.record<1 || data.record>Math.min(4,proof.records?.length || 0))return {ok:false,reason:'invalid_source_selection'};
    const record=proof.records[data.record-1],indices=data.sentences;
    if(!Array.isArray(indices) || !indices.length || indices.length>2 || !Array.isArray(record.spans))return {ok:false,reason:'invalid_source_selection'};
    if(indices.some((index,i)=>!Number.isInteger(index) || index<1 || index>record.spans.length || (i && index!==indices[i-1]+1)))return {ok:false,reason:'invalid_source_selection'};
    const selected=indices.map(index=>record.spans[index-1]);
    if(selected.some(text=>typeof text!=='string' || !textKey(record.text).includes(textKey(text))))return {ok:false,reason:'unsupported_quote_or_name'};
    // Never trust generated fact/topic alongside references. Resolve exclusively
    // against the immutable proof; run the existing URL/name/quality checks.
    return validateHintGrounding(JSON.stringify({fact:selected.join(' '),support_quote:selected.join(' '),topic:proof.topic,adds_information:true}),{...proof,records:[record]});
  }
  if(typeof data.fact!=='string' || !data.fact.trim())return {ok:false,reason:'no_fact'};
  const quote=typeof data.support_quote==='string'?data.support_quote.trim():data.fact.trim();
  const topic=typeof data.topic==='string'?data.topic.trim():String(proof.topic || '').trim();
  if(nameKey(topic)!==nameKey(proof.topic))return {ok:false,reason:'unverified_name'};
  if(Array.from(quote).length<12 || Array.from(quote).length>800 || topic.length<2 || topic.length>80)return {ok:false,reason:'invalid_quote'};
  for(const record of Array.isArray(proof.records)?proof.records.slice(0,4):[]){
    if(typeof record.text!=='string')continue;
    // Recover punctuation-only copy edits by showing the original record,
    // never the model's modified sentence. Changed words/numbers still fail.
    const exact=textKey(record.text).includes(textKey(quote));
    const punctuationOnly=!exact && copyKey(record.text)===copyKey(quote) && numberKey(record.text)===numberKey(quote);
    if(!exact && !punctuationOnly)continue;
    if(proof.source!=='memory' && !/^https?:\/\//i.test(record.url || ''))continue;
    if(!nameKey(quote).includes(nameKey(topic)) && !nameKey(record.title).includes(nameKey(topic)))continue;
    // Display the verified extract itself, never an unsupported paraphrase.
    const fact=completeHintQuote(punctuationOnly?record.text.trim():quote);
    if(Array.from(fact).length<12 || !usefulHintText(fact,topic))return {ok:false,reason:'incomplete_or_empty_source'};
    return {ok:true,fact,topic,kind,source:proof.source,reference:{title:record.title || '',url:record.url || ''}};
  }
  return {ok:false,reason:'unsupported_quote_or_name'};
}

function conversationHint(data,proof){
  if(proof.source!=='conversation' || !['schedule','recap'].includes(proof.kind) || !Array.isArray(data.points) || !data.points.length || data.points.length>4)return {ok:false,reason:'invalid_summary'};
  const source=proof.records?.[0]?.text;
  if(typeof source!=='string' || !source.trim())return {ok:false,reason:'missing_summary_source'};
  // Summary usefulness is compression/organization, not new external facts.
  // A boolean alone cannot waive the independent source checks below.
  if(data.skip===true)return {ok:false,reason:'explicit_skip'};
  const points=[];let recovered=false;
  const negative=text=>/않|못|없|아니|미정|불가|안\s*(?:돼|되|됨|됩|해)|\b(?:not|never|cannot|without|no)\b/iu.test(text);
  for(const point of data.points){
    const text=typeof point?.text==='string'?point.text.trim():'';
    if(point.type!==undefined && !['summary','schedule','check'].includes(point.type))return {ok:false,reason:'invalid_summary'};
    const refs=Array.isArray(point?.evidence)?point.evidence:typeof point?.evidence==='string'?[point.evidence]:[];
    const evidence=refs.map(ref=>Number.isInteger(ref)?proof.spans?.[ref-1] || '':ref);
    if(!text || text.length>300 || !evidence.length || evidence.length>3 || evidence.some(e=>typeof e!=='string' || e.trim().length<2 || !textKey(source).includes(textKey(e))))return {ok:false,reason:'unsupported_summary'};
    const support=evidence.join(' '),numbers=new Set(numberKey(support).split('|'));
    const outputNumbers=new Set(numberKey(text).split('|'));
    const valuesChanged=[...outputNumbers].some(n=>n && !numbers.has(n)) || negative(text)!==negative(support);
    const grams=value=>{const chars=Array.from(nameKey(value).replace(/^(?:결론|비교|이유|요점|결정|핵심|일정|회의|준비|준비물|내용|참고)[:：]\s*/u,'').replace(/[^\p{L}\p{N}]/gu,''));return new Set(chars.slice(1).map((_,i)=>chars[i]+chars[i+1]));};
    const outputGrams=grams(text),sourceGrams=grams(support);
    const overlap=[...outputGrams].filter(g=>sourceGrams.has(g)).length;
    // A heading alone is not a summary. If the model omits the useful values
    // or changes critical facts, show its validated source spans instead.
    const contentPresent=overlap>=Math.min(2,sourceGrams.size) && overlap/Math.max(1,outputGrams.size)>=.18;
    // A long exact copy is not a digest; retain other useful structured points.
    if(point.type==='summary' && Array.from(text).length>100 && textKey(text)===textKey(support))continue;
    recovered ||= valuesChanged || !contentPresent;
    const body=valuesChanged || !contentPresent?evidence.map(e=>e.trim()).join(' / '):text;
    const labels={summary:'핵심',schedule:'일정',check:'확인 필요'};
    points.push(point.type?`${labels[point.type]}: ${body.replace(/^(?:핵심|일정|확인 필요|summary|schedule|check)[:：]\s*/iu,'')}`:body);
  }
  if(!points.length)return {ok:false,reason:'no_summary_value'};
  const fact=points.join('\n');
  const generic=/^(?:대화|내용|상황|관련|자료|정보|핵심|요점|일정|회의|준비|진행|검토|사항|정리|요약|및|중|입니다|합니다|[\s·:：,./])+$/u;
  if(points.every(p=>generic.test(p)) || (!recovered && points.length===1 && source.replace(/\s/g,'').length<60 && textKey(source)===textKey(fact)))return {ok:false,reason:'no_summary_value'};
  return {ok:true,fact,topic:proof.topic,kind:proof.kind,source:'conversation'};
}

// Contextual tasks are grounded by actual conversation AND exact source text.
// A broad cue such as movies is not a claimed proper name in retrieved records.
function validateContextHintSelection(data,proof){
  const records=(proof.records || []).slice(0,4);
  let selected,normalized='';
  if(!Array.isArray(data) && (Object.hasOwn(data,'record') || Object.hasOwn(data,'sentences'))){
    if(['selections','selection','choices','source_selections','source_selection','selected_items'].some(key=>Object.hasOwn(data,key)))return {ok:false,reason:'invalid_source_selection'};
    if(data.record===0 && Array.isArray(data.sentences) && !data.sentences.length)return {ok:false,reason:'source_does_not_answer_task'};
    if(!Number.isInteger(data.record) || data.record<1 || data.record>records.length || !Array.isArray(data.sentences) || !data.sentences.length || data.sentences.length>3)return {ok:false,reason:'invalid_source_selection'};
    const record=records[data.record-1],ids=data.sentences;
    if(ids.some((id,i)=>!Number.isInteger(id) || id<1 || id>Math.min(16,record.spans?.length || 0) || (i && id<=ids[i-1])))return {ok:false,reason:'invalid_source_selection'};
    selected=ids.map(id=>({record,index:data.record-1,sentence:id,text:record.spans[id-1]}));
  }else{
    const aliases=['selections','selection','choices','source_selections','source_selection','selected_items'];
    const keys=Array.isArray(data)?[]:aliases.filter(key=>Object.hasOwn(data,key));
    if(!Array.isArray(data) && keys.length!==1)return {ok:false,reason:'invalid_source_selection'};
    const value=Array.isArray(data)?data:data[keys[0]];
    const ids=Number.isInteger(value)?(value?[value]:[]):value;
    if(!Array.isArray(ids) || ids.length>3 || ids.some(id=>!Number.isInteger(id) || id<1) || new Set(ids).size!==ids.length)return {ok:false,reason:'invalid_source_selection'};
    if(!ids.length)return {ok:false,reason:'source_does_not_answer_task'};
    const choices=records.flatMap((record,index)=>(Array.isArray(record.spans)?record.spans:[]).slice(0,16).map((text,sentence)=>({record,index,sentence:sentence+1,text})));
    selected=[...ids].sort((a,b)=>a-b).map(id=>choices[id-1]);
    if(selected.some(choice=>!choice))return {ok:false,reason:'invalid_source_selection'};
    if(selected.some(choice=>choice.index!==selected[0].index))return {ok:false,reason:'mixed_source_records'};
    if(Array.isArray(data) || keys[0]!=='selections')normalized=Array.isArray(data)?'flat_array':keys[0];
  }
  const record=selected[0].record;
  if(proof.source!=='memory' && !/^https?:\/\//i.test(record.url || ''))return {ok:false,reason:'unsupported_quote_or_name'};
  const texts=[];
  for(const choice of selected){
    if(typeof choice.text!=='string' || typeof record.text!=='string' || !textKey(record.text).includes(textKey(choice.text)))return {ok:false,reason:'unsupported_quote_or_name'};
    const text=completeHintQuote(choice.text);
    if(Array.from(text).length<12 || Array.from(text).length>800)return {ok:false,reason:'incomplete_or_empty_source'};
    const issue=hintContentIssue(text,{topic:record.title,task:proof.task});
    if(issue)return {ok:false,reason:issue};
    texts.push(text);
  }
  const issue=hintContentIssue(texts.join('\n'),{topic:record.title,task:proof.task});
  if(issue)return {ok:false,reason:issue};
  const topic=Array.from(String(record.title || '').trim()).slice(0,80).join('');
  if(!topic)return {ok:false,reason:'unverified_name'};
  return {ok:true,normalized,selection:{record:selected[0].index+1,sentences:selected.map(choice=>choice.sentence)},fact:texts.join('\n'),topic,kind:Object.hasOwn(HINT_LABELS,proof.kind)?proof.kind:'key',source:proof.source,reference:{title:record.title,url:record.url || ''}};
}
