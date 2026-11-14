const superseded=()=>Object.assign(new Error('준비 설정이 변경되었습니다'),{name:'AbortError'});

// One native preparation at a time, plus only the latest requested settings.
// Do not start competing language downloads when controls change rapidly.
export class EnginePreparation {
  constructor(run,onState=()=>{}){Object.assign(this,{run,onState});this.active=null;this.pending=null;this.wanted='';this.ready='';}
  clear(){this.wanted='';this.ready='';if(this.pending){this.pending.reject(superseded());this.pending=null;}}
  ensure(spec,{force=false}={}){
    const key=JSON.stringify(spec);this.wanted=key;
    if(this.pending && this.pending.key!==key){this.pending.reject(superseded());this.pending=null;}
    if(this.active?.key===key){this.onState('preparing',spec);return this.active.promise;}
    if(this.pending?.key===key)return this.pending.promise;
    if(!force && !this.active && this.ready===key)return Promise.resolve();
    let resolve,reject;
    const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});
    // Settings events may supersede a request before its caller attaches a wait.
    promise.catch(()=>{});
    this.pending={key,spec,promise,resolve,reject};this.onState('preparing',spec);this.pump();return promise;
  }
  async pump(){
    if(this.active || !this.pending)return;
    const job=this.pending;this.pending=null;this.active=job;this.ready='';
    const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),185000);
    try{
      await this.run(job.spec,controller.signal);this.ready=job.key;
      if(this.wanted===job.key)this.onState('ready',job.spec);
      job.resolve();
    }catch(error){if(this.wanted===job.key)this.onState('error',job.spec,error);job.reject(error);}
    finally{clearTimeout(timer);this.active=null;this.pump();}
  }
}

// Cancelling capture cancels only that wait, not a shared language download.
export async function waitForPreparation(promise,signal){
  if(!signal)return promise;
  if(signal.aborted)throw superseded();
  let abort;
  try{return await Promise.race([promise,new Promise((_,reject)=>{abort=()=>reject(superseded());signal.addEventListener('abort',abort,{once:true});})]);}
  finally{signal.removeEventListener('abort',abort);}
}
