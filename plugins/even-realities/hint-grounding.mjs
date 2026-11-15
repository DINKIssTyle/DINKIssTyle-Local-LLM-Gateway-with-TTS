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
  if(!data)return {ok:false,reason:'no_fact'};
  if(proof.mode==='conversation')return conversationHint(data,proof);
  if(data.adds_information!==true)return {ok:false,reason:'no_fact'};
  const kind=Object.hasOwn(HINT_LABELS,proof.kind)?proof.kind:data.kind;
  if(typeof data.fact!=='string' || !data.fact.trim())return {ok:false,reason:'no_fact'};
  if(proof.mode==='known'){
    if(typeof proof.topic!=='string' || !proof.topic.trim())return {ok:false,reason:'unknown_name'};
    const allowed=[proof.topic,...(Array.isArray(proof.aliases)?proof.aliases:[])].map(nameKey);
    if(data.topic && !allowed.includes(nameKey(data.topic)))return {ok:false,reason:'unverified_name'};
    return {ok:true,fact:data.fact.trim(),topic:proof.topic.trim(),kind,source:'internal'};
  }
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
    return {ok:true,fact:punctuationOnly?record.text.trim():quote,topic,kind,source:proof.source,reference:{title:record.title || '',url:record.url || ''}};
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
    const refs=Array.isArray(point?.evidence)?point.evidence:typeof point?.evidence==='string'?[point.evidence]:[];
    const evidence=refs.map(ref=>Number.isInteger(ref)?proof.spans?.[ref-1] || '':ref);
    if(!text || text.length>300 || !evidence.length || evidence.length>3 || evidence.some(e=>typeof e!=='string' || e.trim().length<2 || !textKey(source).includes(textKey(e))))return {ok:false,reason:'unsupported_summary'};
    const support=evidence.join(' '),numbers=new Set(numberKey(support).split('|'));
    const outputNumbers=new Set(numberKey(text).split('|'));
    const valuesChanged=[...outputNumbers].some(n=>n && !numbers.has(n)) || [...numbers].some(n=>n && !outputNumbers.has(n)) || negative(text)!==negative(support);
    const grams=value=>{const chars=Array.from(nameKey(value).replace(/[^\p{L}\p{N}]/gu,''));return new Set(chars.slice(1).map((_,i)=>chars[i]+chars[i+1]));};
    const outputGrams=grams(text),sourceGrams=grams(support);
    const overlap=[...outputGrams].filter(g=>sourceGrams.has(g)).length;
    // A heading alone is not a summary. If the model omits the useful values
    // or changes critical facts, show its validated source spans instead.
    const contentPresent=overlap>=Math.min(2,sourceGrams.size) && overlap/Math.max(1,outputGrams.size)>=.18;
    recovered ||= valuesChanged || !contentPresent;
    points.push(valuesChanged || !contentPresent?evidence.map(e=>e.trim()).join(' / '):text);
  }
  const fact=points.join('\n');
  const generic=/^(?:대화|내용|상황|관련|자료|정보|핵심|요점|일정|회의|준비|진행|검토|사항|정리|요약|및|중|입니다|합니다|[\s·:：,./])+$/u;
  if(points.every(p=>generic.test(p)) || (!recovered && points.length===1 && source.replace(/\s/g,'').length<60 && textKey(source)===textKey(fact)))return {ok:false,reason:'no_summary_value'};
  return {ok:true,fact,topic:proof.topic,kind:proof.kind,source:'conversation'};
}
