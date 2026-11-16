// Detection belongs to STT; never infer a foreign language from a short name.
export class TranslationDirection {
  constructor(){this.reset();}
  reset(){this.partner='';this.pair='';this.latest=0;}
  route({source='auto',target='ko',detected='',engine='',text='',startedAt=''}){
    const at=Date.parse(startedAt);
    if(Number.isFinite(at) && at<this.latest)return null;
    let actual=detected.toLowerCase().split('-')[0];
    if(target==='ko' && engine==='whisper'){
      if(!/^[a-z]{2}$/.test(actual))actual=/[가-힣]/u.test(text)?'ko':source!=='auto'?source:this.partner;
      if(actual && actual!=='ko')this.partner=actual;
      const partner=source!=='auto' && source!=='ko'?source:this.partner;
      // Wait for an actual partner language instead of silently assuming English.
      if(actual==='ko' && !partner)return null;
      source=actual || source;target=actual==='ko'?partner:'ko';
    }
    if(Number.isFinite(at))this.latest=at;
    const pair=source+'>'+target,changed=!!this.pair && pair!==this.pair;
    this.pair=pair;
    return {source,target,changed};
  }
}
export function pronunciationText(text,pronunciation,target){
  return target!=='ko' && target!=='en' && pronunciation?.trim()?text+'\n발음: '+pronunciation.trim():text;
}
