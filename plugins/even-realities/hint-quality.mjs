const key=text=>String(text || '').normalize('NFC').toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
// Reject empty existence/importance claims, not concrete explanatory statements.
export function usefulHintText(text,topic=''){
  text=String(text || '').trim();
  if(!text)return false;
  const remainder=key(text).replace(key(topic),'');
  if(/^(?:에대한|관련|에는|에|의)?(?:구체적인|관련된|중요한)?(?:기록|자료|정보|근거|연구)(?:이|가|는|은)?(?:존재합니다|존재함|있습니다|있음|중요합니다|중요함)$/u.test(remainder))return false;
  if(/^(?:there (?:is|are) (?:some )?|\w+ has )(?:information|records|evidence)(?: about .+)?[.!]?$/iu.test(text))return false;
  return true;
}
// Preserve exact source bytes and complete sentences; never turn a cut snippet
// into a confident statement by merely deleting its ellipsis.
export function completeHintQuote(quote){
  quote=String(quote || '').trim();
  if(!/(?:…|\.{3}|\(truncated\))\s*$/iu.test(quote))return quote;
  const prefix=quote.replace(/(?:…|\.{3}|\(truncated\))\s*$/iu,'');
  const ends=[...prefix.matchAll(/[.!?。！？](?=\s|$)/gu)];
  const last=ends.at(-1);
  return last?prefix.slice(0,last.index+1).trim():'';
}

export function acknowledgementOnly(text){
  const compact=key(text);
  return !!compact && /^(?:(?:네|예|응|어|음|아|맞아요|그렇구나|알겠어|알겠습니다|잘알겠습니다|고마워|감사합니다|고맙습니다)|(?:okay|ok|yes|yeah|hmm|thanks|thankyou|understood))+$/iu.test(compact);
}
