// Audio endpoints need not be sentence boundaries. Keep nearby finalized
// fragments as context, while replacing (never appending) each live hypothesis.
export class TranslationSentence {
  constructor({maxGapMs=1500,maxSpanMs=20000,maxChars=400}={}){
    Object.assign(this,{maxGapMs,maxSpanMs,maxChars});this.reset();
  }
  reset(){this.parts=[];this.pair='';}
  update(job){
    const start=Date.parse(job.startedAt),end=Date.parse(job.endedAt);
    if(!Number.isFinite(start) || !Number.isFinite(end))return job;
    const pair=job.source+'>'+job.target;
    if(this.pair!==pair){this.reset();this.pair=pair;}
    let index=this.parts.findIndex(part=>part.id===job.id);
    if(index<0){
      const previous=this.parts.at(-1);
      // Ignore an old final result after a newer sentence has already started.
      if(previous && start<previous.start)return null;
      if(previous && (start-previous.end>this.maxGapMs || end-this.parts[0].start>this.maxSpanMs || /[.!?。！？]["'”’)]?$/u.test(previous.prompt.trim()) || this.parts.map(p=>p.prompt).join(' ').length+job.prompt.length>this.maxChars))this.parts=[];
      this.parts.push({id:job.id,start,end,prompt:job.prompt,final:!job.draft});
      index=this.parts.length-1;
    }else{
      const part=this.parts[index];
      if(part.final && job.draft)return null;
      Object.assign(part,{end,prompt:job.prompt,final:!job.draft});
    }
    // A late final may improve context, but must not replace a newer live box
    // or send its unfinished speech to the reply LLM.
    if(index!==this.parts.length-1)return null;
    const prefix=this.parts.slice(0,index).filter(part=>part.final).map(part=>part.prompt);
    return {...job,historyId:this.parts[0].id,prompt:[...prefix,job.prompt].join(' '),stableText:[...prefix,job.stableText || ''].join(' ').trim()};
  }
}
