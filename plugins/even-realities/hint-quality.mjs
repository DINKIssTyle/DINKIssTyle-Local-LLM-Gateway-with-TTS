const key=text=>String(text || '').normalize('NFC').toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
// Reject empty existence/importance claims, not concrete explanatory statements.
// Reject whole non-answers, preserving useful factual uncertainty/limitations.
// This is a final content guard, not a dictionary of accepted topic names.
export function hintContentIssue(text,{topic='',task=''}={}){
 text=String(text || '').trim();if(!text)return 'no_information_value';
 if([topic,task].some(title=>key(title) && key(title)===key(text)))return 'heading_only';
 const compact=key(text);
 if(text.length<140 && /(?:이유|방향|원리|목표|방법|의미|개념|비교|정리|정보|설명)$/u.test(text) && compact.length>=8 && [topic,task].some(title=>key(title).includes(compact.slice(0,Math.min(12,compact.length)))))return 'heading_only';
 const sentences=text.split(/(?<=[.!?。！？])\s+|\n+/u).map(v=>v.trim()).filter(Boolean);
 const nonAnswer=sentence=>{
  if(/(?:제공된|주어진|현재의?|해당)\s*(?:대화|전사|문맥|정보|자료).{0,100}(?:알\s*수|어렵|부족|파악|특정|확인할\s*수\s*없)/u.test(sentence))return 'insufficient_context';
  if(/(?:추가|더\s*많은|구체적인)\s*(?:정보|문맥|설명).{0,25}(?:필요합니다|필요하다|제공해|알려\s*주)/u.test(sentence))return 'insufficient_context';
  if(/(?:provided|given|available)\s+(?:context|conversation|information|details).{0,100}(?:insufficient|not enough|cannot|unclear|does not (?:identify|specify))/iu.test(sentence) || /(?:need|require|provide)\s+(?:more|additional)\s+(?:information|context|details)/iu.test(sentence))return 'insufficient_context';
  if(/(?:자세한|더\s*많은|관련|지역별).{0,100}(?:사이트|홈페이지|앱|날씨누리).{0,80}(?:참고|방문|확인|이용).{0,20}(?:바랍니다|하세요|있습니다)/u.test(sentence) || /^(?:please\s+)?(?:visit|refer to|check|see).{0,100}(?:website|site|app).{0,50}(?:details|information|more)/iu.test(sentence))return 'navigation_only';
  const remainder=key(sentence).replace(key(topic),'');
  if(/^(?:에대한|관련|에는|에|의)?(?:구체적인|관련된|중요한)?(?:기록|자료|정보|근거|연구)(?:이|가|는|은)?(?:존재합니다|존재함|있습니다|있음|중요합니다|중요함)$/u.test(remainder) || /^(?:there (?:is|are) (?:some )?|\w+ has )(?:information|records|evidence)(?: about .+)?[.!]?$/iu.test(sentence))return 'no_information_value';
  return '';
 };
 const issues=sentences.map(nonAnswer);
 return issues.length && issues.every(Boolean)?issues[0]:'';
}
export function usefulHintText(text,topic=''){return !hintContentIssue(text,{topic});}
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

// Structural unresolved reference, not a dictionary of accepted/rejected names.
export function unresolvedTermTopic(topic){
  return /^(?:이|그|저)\s+.+(?:하는|되는|한다는|된다는)\s*(?:거|것)(?:[은는이가을를])?$/u.test(String(topic || '').trim());
}

export function unresolvedSpecificTask(need,evidence){
  // An unspecified person's past action is not a general knowledge question.
  // Do not infer their identity or silently invent the missing object.
  return /그(?:분|사람|것|걸|거|장비|제품)/u.test(String(evidence || '')) && /(?:낸|만든|개발한|발명한|등록한|출원한).{0,24}(?:내용|분야|명칭|이름|종류)/u.test(String(need || ''));
}

export function hintTaskIssue(topic,need){
  topic=String(topic || '').trim();need=String(need || '').trim();
  if(Array.from(topic).length>40 || /[.!?。！？]$/u.test(topic) || /(?:합니다|됩니다|입니다|다릅니다|있습니다|없습니다|안되냐|할까요|하나요|모아\s*심어|남아\s*있는\s*것)$/u.test(topic))return 'sentence_topic';
  if(/^(?:definition|meaning|mechanism|comparison|설명|정의|의미|원리)$/iu.test(need))return 'generic_task';
  return '';
}
