// Only finalized transcripts use this route. It is deliberately limited to
// questions and information requests; ambient statements stay on the hint path.
export function directQuestionIntent(text) {
  const source=String(text || '').trim();
  if(!source)return {direct:false,search:false};
  const compact=source.replace(/\s/gu,'');
  const questionMark=/[?？]["'”’」』]*\s*$/u.test(source);
  const ending=/(?:인가요|입니까|습니까|일까요|는걸까|일까)[.!。?？]*$/u.test(compact);
  const reported=/(?:라고|냐고|달라고|라는).{0,35}(?:말했|물었|했어|했대|했다|하더라|적혀있|써있|버튼이).*[.!。]?$/u.test(compact)
    && !/(?:알려|검색해|찾아|설명해|정리해|계산해|비교해|요약해|번역해)(?:줘|주세요|주라)[.!?。？!]*$/u.test(compact);
  if(reported && !ending)return {direct:false,search:false};
  const request=/(?:검색|검샏|찾아|알려|설명|말해|비교|계산|요약|정리|번역)(?:을|를)?(?:좀)?(?:해)?(?:줘|주세요|주라|줄래|줄수있어|주시겠어요)(?!야|라고|라는)|찾아봐(?:줘|주세요)?/u.test(compact);
  let search=/(?:검색|검샏)(?:을)?(?:좀)?(?:해)?(?:줘|주세요|주라|줄래|줄수있어)/u.test(compact)
    || /\b(?:search(?:\s+for)?|look\s+up)\b/i.test(source);
  if(/(?:인터넷|웹).*(?:찾아봐|찾아줘|찾아주세요)/u.test(compact))search=true;
  if(/(?:내기록|저장된기억|지난대화|전사기록|우리대화)/u.test(compact))search=false;
  const interrogative=/(?:무엇|뭐|누구|누가|언제|어디|어떻게|왜|얼마|몇|어느|어떤|무슨).*(?:야|이야|요|까|니|지|해|어|돼|되|이지|라)[.!。?？]*$/u.test(compact);
  const englishExclamation=/^what\s+(?:a|an)\b/i.test(source) && !questionMark;
  const english=/^(?:what|when|where|who|why|how|which|can\s+you|could\s+you|would\s+you|is\s+there|are\s+there)\b/i.test(source)
    || /\b(?:please\s+)?(?:tell\s+me|explain\s+to\s+me|search\s+for|look\s+up)\b/i.test(source);
  return {direct:request||ending||interrogative||(!englishExclamation && english),search};
}
