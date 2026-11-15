// Advance only on server acknowledgement. Unknown/older gateways receive full
// snapshots. A 409 restarts from zero; final archival audio is never delta-only.
export class AudioUpload {
 constructor(){this.reset();}
 reset(){this.supported=false;this.entries=new Map();}
 key(snapshot){return [snapshot.sessionId,snapshot.segmentId,snapshot.engine,snapshot.sourceLang].join('\0');}
 prepare(snapshot){
  const key=this.key(snapshot),offset=this.supported?(this.entries.get(key)||0):0;
  const from=offset<=snapshot.bytes.length?offset:0;
  return {bytes:snapshot.bytes.subarray(from),audioDelta:this.supported,audioOffset:from};
 }
 acknowledge(snapshot,data){
  this.supported=data.audioDelta===true;
  if(!this.supported)return;
  const key=this.key(snapshot);this.entries.delete(key);this.entries.set(key,data.audioBytes || snapshot.bytes.length);
  while(this.entries.size>16)this.entries.delete(this.entries.keys().next().value);
 }
 resync(snapshot){this.entries.delete(this.key(snapshot));}
}
