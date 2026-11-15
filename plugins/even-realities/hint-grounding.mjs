import {unwrapHintObject} from './hint-judge.mjs';

const textKey=text=>String(text || '').normalize('NFC').replace(/\s+/gu,' ').trim();
const nameKey=text=>textKey(text).toLocaleLowerCase().replace(/\s/gu,'');

// Model confidence and query echoes are not evidence. A quote must come from
// one server-provided source record, and its label must exist in that record.
export function validateHintGrounding(raw,proof){
  if(proof?.version!==1 || !['known','quote'].includes(proof.mode))return {ok:false,reason:'missing_proof'};
  const data=unwrapHintObject(raw);
  if(!data || data.adds_information!==true || typeof data.fact!=='string' || !data.fact.trim())return {ok:false,reason:'no_fact'};
  if(proof.mode==='known'){
    if(typeof proof.topic!=='string' || !proof.topic.trim())return {ok:false,reason:'unknown_name'};
    const allowed=[proof.topic,...(Array.isArray(proof.aliases)?proof.aliases:[])].map(nameKey);
    if(data.topic && !allowed.includes(nameKey(data.topic)))return {ok:false,reason:'unverified_name'};
    return {ok:true,fact:data.fact.trim(),topic:proof.topic.trim(),kind:data.kind,source:'internal'};
  }
  const quote=typeof data.support_quote==='string'?data.support_quote.trim():data.fact.trim();
  const topic=typeof data.topic==='string'?data.topic.trim():String(proof.topic || '').trim();
  if(nameKey(topic)!==nameKey(proof.topic))return {ok:false,reason:'unverified_name'};
  if(Array.from(quote).length<12 || Array.from(quote).length>800 || topic.length<2 || topic.length>80)return {ok:false,reason:'invalid_quote'};
  for(const record of Array.isArray(proof.records)?proof.records.slice(0,4):[]){
    if(typeof record.text!=='string' || !textKey(record.text).includes(textKey(quote)))continue;
    if(proof.source!=='memory' && !/^https?:\/\//i.test(record.url || ''))continue;
    if(!nameKey(quote).includes(nameKey(topic)) && !nameKey(record.title).includes(nameKey(topic)))continue;
    // Display the verified extract itself, never an unsupported paraphrase.
    return {ok:true,fact:quote,topic,kind:data.kind,source:proof.source,reference:{title:record.title || '',url:record.url || ''}};
  }
  return {ok:false,reason:'unsupported_quote_or_name'};
}
