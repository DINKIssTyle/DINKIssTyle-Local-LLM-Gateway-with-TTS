// Only skip translation when a supported non-Latin script clearly matches.
// Latin-script targets still need a translator; Latin script alone is not English.
export function hintNeedsLocalization(text,target='ko'){
 const value=String(text || ''),letters=(value.match(/\p{L}/gu) || []).length;
 const count=pattern=>(value.match(pattern) || []).length;
 if(target==='ko' && count(/[\uac00-\ud7af]/gu)>letters*.35)return false;
 if(target==='ja' && count(/[\u3040-\u30ff]/gu)>0 && !count(/[\uac00-\ud7af]/gu))return false;
 if(target==='zh' && count(/\p{Script=Han}/gu)>letters*.5 && !count(/[\u3040-\u30ff\uac00-\ud7af]/gu))return false;
 if(target==='th' && count(/\p{Script=Thai}/gu)>letters*.5)return false;
 return true;
}
export async function localizeVerifiedHint({text,target='ko',translate,signal}){
 if(signal?.aborted)throw Object.assign(Error('Hint localization canceled'),{name:'AbortError'});
 if(!hintNeedsLocalization(text,target))return {ok:true,text,translated:false};
 const result=await translate(text,target);
 if(signal?.aborted)throw Object.assign(Error('Hint localization canceled'),{name:'AbortError'});
 if(!result?.ok || !String(result.text || '').trim())return {ok:false,reason:'hint_translation_failed'};
 return {ok:true,text:result.text.trim(),translated:true};
}
