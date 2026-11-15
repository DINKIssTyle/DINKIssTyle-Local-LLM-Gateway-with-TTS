// Commit only complete sentences in the STT's stable prefix. The unfinished
// sentence keeps its full context; extending it never retranslates old sentences.
export function translationChunks(prompt,stableText=''){
  prompt=String(prompt || '').trim();
  const stable=String(stableText || '').trim();
  if(!stable || !prompt.startsWith(stable))return [prompt].filter(Boolean);
  const chunks=[];
  let from=0;
  for(const match of stable.matchAll(/(?:[。！？]+["'”’)]*|[.!?]+["'”’)]*(?=\s|$))/gu)){
    const end=match.index+match[0].length;
    const sentence=stable.slice(from,end).trim();
    // Avoid treating common abbreviations as sentence endpoints.
    if(/\b(?:Mr|Mrs|Ms|Dr|Prof|Sr|Jr|St|vs|etc|e\.g|i\.e)\.$/iu.test(sentence) || /\b[A-Z]\.$/u.test(sentence))continue;
    if(sentence)chunks.push(sentence);
    from=end;
  }
  const tail=prompt.slice(from).trim();if(tail)chunks.push(tail);
  return chunks;
}

export class StableTranslation {
  constructor(translate,{maxEntries=32,maxChars=8000}={}){
    Object.assign(this,{translate,maxEntries,maxChars});this.reset();
  }
  reset(){this.scope='';this.cache=new Map();}
  async run(job,signal){
    const scope=[job.engine,job.source,job.target,job.historyId || job.id,job.cacheScope || ''].join('\0');
    if(scope!==this.scope){this.scope=scope;this.cache=new Map();}
    // A cancelled older call must not populate a newer segment's cache.
    const cache=this.cache;
    const texts=[];
    for(const source of translationChunks(job.prompt,job.draft?job.stableText:job.prompt)){
      if(signal.aborted)throw new Error('번역 취소');
      let text=cache.get(source);
      if(text){cache.delete(source);cache.set(source,text);}
      if(!text){
        const result=await this.translate({...job,prompt:source,onChunk:job.onPartial?(partial)=>job.onPartial([...texts,partial].join(' ')):undefined},signal);
        if(signal.aborted)throw new Error('번역 취소');
        if(!result.ok)return result;
        text=result.text;cache.set(source,text);
        let chars=[...cache].reduce((n,[key,value])=>n+key.length+value.length,0);
        while(cache.size>this.maxEntries || chars>this.maxChars){
          const [key,value]=cache.entries().next().value;cache.delete(key);chars-=key.length+value.length;
        }
      }
      texts.push(text);
    }
    return {ok:true,text:texts.join(' ')};
  }
}
