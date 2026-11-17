// At most one native display operation may be outstanding. A timeout releases
// JS, but does not pretend to cancel a native Bluetooth operation: quarantine
// it until it settles. Callers keep only the latest coalesced display state.
export class HUDTransport{
  constructor({timeoutMs=5000,intervalMs=150,onReady=()=>{},now=()=>Date.now(),sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms))}={}){
    Object.assign(this,{timeoutMs,intervalMs,onReady,now,sleep});
    this.connected=true;this.pending=false;this.fault=false;this.last=-Infinity;this.epoch=0;
  }
  get ready(){return this.connected&&!this.pending&&!this.fault;}
  setConnected(value){
    if(value===this.connected)return;
    this.connected=value;this.epoch++;
    if(!value)this.interrupt?.();
    else{this.fault=false;if(!this.pending)this.onReady();}
  }
  async call(operation){
    if(!this.ready)throw Error('안경 화면 연결 대기 중');
    this.pending=true;
    const epoch=this.epoch;
    let expired=false,timer;
    try{
      const delay=Math.max(0,this.intervalMs-(this.now()-this.last));
      if(delay>0)await this.sleep(delay);
      if(!this.connected||epoch!==this.epoch){this.pending=false;throw Error('안경 연결이 변경되었습니다');}
      this.last=this.now();
      const native=Promise.resolve().then(operation);
      const settled=native.finally(()=>{
        this.pending=false;
        if(expired&&this.connected){this.fault=false;queueMicrotask(()=>this.onReady());}
      });
      const interrupted=new Promise((_,reject)=>{
        this.interrupt=()=>{expired=true;reject(Error('안경 연결이 해제되었습니다'));};
        timer=setTimeout(()=>{expired=true;this.fault=true;reject(Error('안경 화면 응답 대기 시간 초과'));},this.timeoutMs);
      });
      const value=await Promise.race([settled,interrupted]);
      if(epoch!==this.epoch)throw Error('안경 연결이 변경되었습니다');
      return value;
    }finally{clearTimeout(timer);this.interrupt=null;}
  }
}
