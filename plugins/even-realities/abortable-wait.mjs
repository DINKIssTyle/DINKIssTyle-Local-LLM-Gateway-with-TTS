// Some WebViews leave fetch/stream promises pending after abort. Release the
// application queue even when the underlying implementation ignores the signal.
export function abortableWait(promise,signal,onAbort=()=>{}){
  if(!signal)return Promise.resolve(promise);
  return new Promise((resolve,reject)=>{
    const abort=()=>{
      try{onAbort();}catch{}
      finish(reject,Object.assign(new Error('Request aborted'),{name:'AbortError'}));
    };
    const finish=(settle,value)=>{signal.removeEventListener('abort',abort);settle(value);};
    // Always observe late rejections, including an already aborted signal.
    Promise.resolve(promise).then(value=>finish(resolve,value),error=>finish(reject,error));
    if(signal.aborted)abort();else signal.addEventListener('abort',abort,{once:true});
  });
}
