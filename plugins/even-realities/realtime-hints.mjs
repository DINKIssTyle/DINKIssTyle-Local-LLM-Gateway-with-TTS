function previewWakeMatch(text,word){
  const alias=String(word || '').normalize('NFC').replace(/\s/gu,'');
  if(!alias)return null;
  const pattern=Array.from(alias,c=>c.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')).join('\\s*');
  const source=String(text).normalize('NFC');
  for(const match of source.matchAll(new RegExp(pattern,'giu'))){
    if(!/^[a-z]/i.test(alias))return match;
    const before=Array.from(source.slice(0,match.index)).at(-1) || '';
    const after=Array.from(source.slice(match.index+match[0].length))[0] || '';
    if(!/[\p{L}\p{N}]/u.test(before) && !/[\p{L}\p{N}]/u.test(after))return match;
  }
  return null;
}

function previewPromptAfterWake(text,aliases=[]){
  const source=String(text || '').normalize('NFC');
  for(const word of aliases){
    const match=previewWakeMatch(source,word);
    if(match){
      return source.slice(match.index+match[0].length).replace(/^[\s,.:;!?~"'”’]+/u,'').replace(/^(?:야|아)(?=$|[\s,.!?])/u,'').replace(/^[\s,.:;!?]+/u,'').trim();
    }
  }
  return '';
}

// A preview can prepare a request, but never authorizes a chat or stores a transcript.
export class PreviewIntent {
  reset(){this.id='';this.previous='';this.selected='';this.wake=false;}
  constructor(){this.reset();}
  observe({id,text='',stableText='',aliases=[]}){
    if(this.id!==id){this.reset();this.id=id;}
    let common=0;
    while(common<text.length && common<this.previous.length && text[common]===this.previous[common])common++;
    const stable=String(stableText || text.slice(0,common)).trim();
    this.previous=text;
    const wake=aliases.some(word=>Boolean(previewWakeMatch(stable,word) || previewWakeMatch(text,word)));
    const newWake=wake && !this.wake;this.wake=wake;
    const prompt=wake?previewPromptAfterWake(text || stable,aliases):'';
    const changed=stable!==this.selected && (Math.abs(stable.length-this.selected.length)>=8 || !stable.startsWith(this.selected));
    const hintText=!wake && stable.length>=8 && changed?stable:'';
    if(hintText)this.selected=hintText;
    return {wake,newWake,prompt,hintText};
  }
}

export class ChatSettingsCache {
  constructor({ttlMs=15000,now=()=>Date.now()}={}){this.ttlMs=ttlMs;this.now=now;this.entry=null;}
  clear(){this.entry=null;}
  read(key,load,signal){
    if(signal?.aborted)return Promise.reject(Object.assign(Error('cancelled'),{name:'AbortError'}));
    let entry=this.entry;
    if(!entry || entry.key!==key || (!entry.pending && this.now()>=entry.expires)){
      entry={key,pending:true,expires:0};this.entry=entry;
      entry.promise=Promise.resolve().then(load).then(value=>{entry.pending=false;entry.expires=this.now()+this.ttlMs;return value;},error=>{if(this.entry===entry)this.entry=null;throw error;});
    }
    if(!signal)return entry.promise;
    return new Promise((resolve,reject)=>{
      const abort=()=>{cleanup();reject(Object.assign(Error('cancelled'),{name:'AbortError'}));};
      const cleanup=()=>signal.removeEventListener('abort',abort);
      signal.addEventListener('abort',abort,{once:true});
      entry.promise.then(value=>{cleanup();resolve(value);},error=>{cleanup();reject(error);});
    });
  }
}

// History is sent only to the local comparison endpoint, never to a generative LLM.
export async function compareHintDuplicate({fetch,server,headers,text,history,kind='fact',signal,timeoutMs=1400}){
  if(!history.length)return false;
  const controller=new AbortController(),abort=()=>controller.abort();
  if(signal?.aborted)controller.abort();else signal?.addEventListener('abort',abort,{once:true});
  const timer=setTimeout(abort,timeoutMs);
  try{
    const response=await fetch(`${server}/api/glasses/hint-compare`,{method:'POST',headers,signal:controller.signal,
      body:JSON.stringify({kind,text:Array.from(text).slice(0,600).join(''),history:history.slice(-6).map(value=>Array.from(value).slice(0,600).join(''))})});
    if(!response.ok)return false; // Old gateways and a busy/offline encoder retain local checks.
    const result=await response.json();
    return !controller.signal.aborted && result.checked===true && result.duplicate===true;
  }catch(error){if(signal?.aborted)throw error;return false;}
  finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
}

// Keep timings only: never record recognized speech, prompts, credentials or answers.
export class VoiceLatency {
  constructor({now=()=>Date.now(),limit=20,onUpdate=()=>{}}={}){Object.assign(this,{now,limit,onUpdate});this.entries=[];}
  mark(id,stage,at=this.now()){
    if(!id)return;
    let entry=this.entries.find(item=>item.id===id);
    if(!entry){entry={id,stages:{}};this.entries.push(entry);this.entries=this.entries.slice(-this.limit);}
    if(entry.stages[stage]===undefined)entry.stages[stage]=at;
    this.onUpdate(this.summary(entry));
  }
  summary(entry=this.entries.at(-1)){
    if(!entry)return {};
    const s=entry.stages,result={};
    if(s.chat_queued===undefined)return result;
    for(const [label,start,end] of [['발화 종료 대기','last_voice','clip_end'],['음성 확정','clip_end','stt_final'],['전사 처리','stt_sent','stt_final'],['AI 대기','chat_queued','chat_start'],['설정 조회','chat_start','settings_ready'],['전송 준비','settings_ready','chat_sent'],['첫 표시','chat_sent','first_text'],['호출어 감지→전송','preview_wake','chat_sent']]){
      if(s[start]!==undefined && s[end]!==undefined)result[label]=Math.max(0,Math.round(s[end]-s[start]));
    }
    return result;
  }
}
