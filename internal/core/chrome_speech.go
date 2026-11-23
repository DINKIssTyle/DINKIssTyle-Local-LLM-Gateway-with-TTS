package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type chromeSpeechRequest struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Language string `json:"language"`
	Engine   string `json:"engine"`
	Audio    string `json:"audio,omitempty"`
	Final    bool   `json:"final,omitempty"`
}
type chromeSpeechResult struct {
	LockMS     float64                `json:"-"`
	StableText string                 `json:"stableText"`
	DraftText  string                 `json:"draftText"`
	Text       string                 `json:"text"`
	Status     string                 `json:"status"`
	Error      string                 `json:"error,omitempty"`
	Finished   bool                   `json:"finished"`
	Debug      map[string]interface{} `json:"debug,omitempty"`
}

func chromeSTTEngine(engine string) bool {
	return engine == "chrome-online"
}
func (s *chromeTranslator) speech(ctx context.Context, req chromeSpeechRequest, op string) (chromeSpeechResult, error) {
	releaseMemory := chromeModelMemory.use()
	defer releaseMemory()
	var result chromeSpeechResult
	lockStarted := time.Now()
	if !chromeSTTEngine(req.Engine) {
		return result, errors.New("잘못된 Chrome STT 엔진")
	}
	if len(req.Language) != 2 {
		return result, errors.New("Chrome STT 인식 언어를 선택하세요")
	}
	locked := s.speechMu.TryLock()
	if !locked {
		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(25 * time.Millisecond):
			}
			if s.speechMu.TryLock() {
				locked = true
				break
			}
		}
	}
	if !locked {
		return result, errSTTBusy
	}
	defer s.speechMu.Unlock()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if op == "stop" {
		s.mu.RLock()
		empty := s.tab == nil
		s.mu.RUnlock()
		if empty {
			return chromeSpeechResult{Status: "ready"}, nil
		}
	}
	tab, release, err := s.leaseBrowser(ctx)
	if err != nil {
		if errors.Is(err, errChromeTranslationBusy) {
			return result, errSTTBusy
		}
		return result, err
	}
	defer release()
	lockMS := float64(time.Since(lockStarted).Microseconds()) / 1000

	task, cancel := context.WithCancel(tab)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	payload, _ := json.Marshal(req)
	action := chromedp.ActionFunc(func(ctx context.Context) error {
		value, exception, err := cdpruntime.Evaluate("window.chromeSpeech." + op + "(" + string(payload) + ")").WithAwaitPromise(true).WithReturnByValue(true).WithUserGesture(true).Do(ctx)
		if err != nil {
			return err
		}
		if exception != nil {
			return errors.New("Chrome 음성 인식 실행 실패: " + exception.Text)
		}
		if value == nil {
			return errors.New("Chrome STT 응답 없음")
		}
		return json.Unmarshal(value.Value, &result)
	})
	err = chromedp.Run(task, action)
	result.LockMS = lockMS
	if err != nil {
		return result, err
	}
	if result.Status == "busy" {
		return result, errSTTBusy
	}
	if result.Error != "" {
		return result, errors.New(result.Error)
	}
	return result, nil
}
func handleChromeSpeechPrepare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req chromeSpeechRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || !chromeSTTEngine(req.Engine) || len(req.Language) != 2 {
			http.Error(w, "invalid Chrome STT settings", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		result, err := chromeTranslation.speech(ctx, req, "prepare")
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			result.Error = err.Error()
			w.WriteHeader(503)
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}
func runChromeSpeech(ctx context.Context, metadata STTTranscribeRequest, audio []byte, preview bool, user string) (sttWorkerReply, error) {
	var output sttWorkerReply
	started := time.Now()
	if metadata.Format != "" && metadata.Format != "pcm" {
		return output, errors.New("Chrome STT는 PCM16 입력이 필요합니다")
	}
	session, segment := metadata.SessionID, metadata.SegmentID
	if segment == "" {
		segment = fmt.Sprintf("legacy-%d", time.Now().UnixNano())
	}
	owner := fmt.Sprintf("%x", sha256.Sum256([]byte(user+"\x00"+session)))
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(owner+"\x00"+segment)))
	req := chromeSpeechRequest{ID: key, Owner: owner, Language: metadata.Language, Engine: metadata.Engine, Audio: base64.StdEncoding.EncodeToString(audio), Final: !preview}
	if req.Language == "" {
		req.Language = "ko"
	}
	result, err := chromeTranslation.speech(ctx, req, "push")
	output.LockMS += result.LockMS
	if err != nil {
		return output, err
	}
	if !preview {
		req.Audio = ""
		deadline := time.Now().Add(time.Duration(float64(len(audio))/32000*float64(time.Second)) + 7*time.Second)
		for !result.Finished && time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return output, ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
			next, pollErr := chromeTranslation.speech(ctx, req, "read")
			if pollErr == errSTTBusy {
				continue
			}
			if pollErr != nil {
				return output, pollErr
			}
			output.LockMS += next.LockMS
			result = next
		}
		if !result.Finished {
			return output, errors.New("Chrome STT 최종 결과 지연")
		}
	}
	output.EngineMS = float64(time.Since(started).Microseconds())/1000 - output.LockMS
	output.Text = strings.TrimSpace(result.Text)
	output.StableText = result.StableText
	output.DraftText = result.DraftText
	if !preview && output.Text == "" {
		output.Rejected = true
		output.Reason = "no_speech"
	}
	return output, nil
}

const chromeSpeechScript = `
(() => {
 const SR=self.SpeechRecognition || self.webkitSpeechRecognition;
 const segments=new Map();let activeStream=null;
 const supportsTrack=()=>Number((navigator.userAgent.match(/Chrome\/(\d+)/)||[])[1] || 0)>=135 && !!self.MediaStreamTrackGenerator && !!self.AudioData;
 const locale=lang=>({ko:'ko-KR',en:'en-US',ja:'ja-JP',zh:'zh-CN',es:'es-ES',fr:'fr-FR',de:'de-DE',vi:'vi-VN',th:'th-TH'}[lang] || lang);
 const snapshot=seg=>{
  const st=seg.stream;
  const finalText=(seg.finalText || '').trim();
  const draftText=seg.finished?'':(seg.draftText || '');
  return {status:st.error?'error':'ready',error:st.error || '',text:(finalText+' '+draftText).trim(),stableText:finalText,draftText,finished:seg.finished,debug:{started:!!st.started,bytes:seg.bytes,events:st.events.slice(-12),track:st.track.getSettings(),id:seg.id,streamKey:st.key,visible:document.visibilityState}};
 };
 async function prepare(p){
  if(!SR)return {status:'unsupported',error:'Chrome SpeechRecognition API가 없습니다'};
  if(!supportsTrack())return {status:'unsupported',error:'Chrome 오디오 트랙 입력을 지원하지 않습니다'};
  return {status:'ready'};
 }
 function closeStream(st){
  if(!st || st.finished)return;
  st.finished=true;
  try{st.recognition.abort();}catch{}
  try{st.track.stop();}catch{}
  st.writer.close().catch(()=>{});
 }
 async function writePcmChunks(st,bytes){
  const chunkSize=6400; // 200ms @ 16kHz s16 mono
  for(let offset=0;offset<bytes.length;offset+=chunkSize){
   const slice=bytes.subarray(offset,Math.min(offset+chunkSize,bytes.length));
   const frames=Math.floor(slice.length/2);
   if(frames<=0)continue;
   const frame=new AudioData({format:'s16',sampleRate:16000,numberOfFrames:frames,numberOfChannels:1,timestamp:st.timestampUs,data:slice});
   st.timestampUs+=Math.round(frames*1e6/16000);
   try{await st.writer.write(frame);}finally{frame.close();}
  }
 }
 async function ensureStream(p){
  const lang=locale(p.language);
  const key=p.owner+':'+lang;
  if(activeStream && !activeStream.finished){
   if(activeStream.key===key){
    activeStream.touched=Date.now();
    return activeStream;
   }
   const current=segments.get(activeStream.currentSegId);
   if(activeStream.owner!==p.owner && !current?.finished && Date.now()-activeStream.touched<15000)return null;
   closeStream(activeStream);
  }
  if(!supportsTrack())throw Error('이 Chrome은 PCM 오디오 트랙 입력을 지원하지 않습니다');
  const track=new MediaStreamTrackGenerator({kind:'audio'});const writer=track.writable.getWriter();
  const recognition=new SR();recognition.lang=lang;recognition.continuous=true;recognition.interimResults=true;
  recognition.processLocally=false;
  const st={key,owner:p.owner,lang,recognition,track,writer,timestampUs:0,finalCount:0,currentSegId:p.id,started:false,finished:false,error:'',events:[],touched:Date.now(),queue:Promise.resolve()};
  for(const event of ['start','audiostart','soundstart','speechstart','speechend','audioend','end','result','nomatch','error'])recognition.addEventListener(event,e=>st.events.push(event+(e.error?':'+e.error:'')));
  recognition.onresult=e=>{
   let firstDraft=e.results.length;
   for(let i=0;i<e.results.length;i++){if(!e.results[i].isFinal){firstDraft=i;break;}}
   st.finalCount=firstDraft;
   const seg=segments.get(st.currentSegId);
   if(!seg || seg.finished)return;
   let committed='',draft='';
   for(let i=seg.startResultIndex;i<e.results.length;i++){
    if(e.results[i].isFinal)committed+=e.results[i][0].transcript+' ';
    else draft+=e.results[i][0].transcript;
   }
   seg.finalText=committed.trim();
   seg.draftText=draft;
   if(seg.ending && !draft && seg.finalText){
    seg.finished=true;
   }
  };
  const baseOnError=e=>{if(e.error!=='no-speech' && !(e.error==='aborted' && st.finished))st.error='Chrome STT: '+e.error;};
  recognition.onend=()=>{
   st.finished=true;
   const seg=segments.get(st.currentSegId);
   if(seg && !seg.finished){
    if(!seg.finalText && seg.draftText)seg.finalText=seg.draftText;
    seg.draftText='';
    seg.finished=true;
   }
   try{st.track.stop();}catch{}
   st.writer.close().catch(()=>{});
  };
  track.addEventListener('ended',()=>st.events.push('track-ended'));track.addEventListener('mute',()=>st.events.push('track-mute'));
  let readyResolve,readyReject;const ready=new Promise((resolve,reject)=>{readyResolve=resolve;readyReject=reject;});st.ready=ready;
  const timer=setTimeout(()=>{if(!st.started && !st.finished){st.error='Chrome STT 시작 응답 없음';closeStream(st);readyReject(Error(st.error));}},8000);
  recognition.onstart=()=>{st.started=true;clearTimeout(timer);readyResolve();};
  recognition.onerror=e=>{baseOnError(e);clearTimeout(timer);if(!st.started){closeStream(st);readyReject(Error(st.error || e.error));}};
  try{recognition.start(track);}catch(e){clearTimeout(timer);try{track.stop();}catch{}writer.close().catch(()=>{});throw e;}
  activeStream=st;
  await writePcmChunks(st,new Uint8Array(3200)); // 100ms initial silence
  await ready;
  return st;
 }
 function ensureSegment(p,st){
  let seg=segments.get(p.id);
  if(seg && seg.stream===st)return seg;
  const prev=segments.get(st.currentSegId);
  if(prev && !prev.finished){
   if(!prev.finalText && prev.draftText)prev.finalText=prev.draftText;
   prev.draftText='';
   prev.finished=true;
  }
  st.currentSegId=p.id;
  seg={id:p.id,owner:p.owner,stream:st,bytes:0,startResultIndex:st.finalCount,finalText:'',draftText:'',finished:false,ending:false};
  segments.set(p.id,seg);
  while(segments.size>8){const key=segments.keys().next().value;if(key!==p.id)segments.delete(key);else break;}
  return seg;
 }
 setInterval(()=>{if(activeStream && !activeStream.finished && Date.now()-activeStream.touched>45000){closeStream(activeStream);}},5000);
 window.chromeSpeech={prepare,
  stop(p){
   const seg=segments.get(p.id);
   if(seg && !seg.finished){
    if(!seg.finalText && seg.draftText)seg.finalText=seg.draftText;
    seg.draftText='';seg.finished=true;
   }
   if(activeStream && activeStream.owner===p.owner && !activeStream.finished)closeStream(activeStream);
   return {status:'ready'};
  },
  async push(p){try{
   const st=await ensureStream(p);
   if(!st)return {status:'busy',error:'다른 Chrome STT 세션이 사용 중입니다'};
   const seg=ensureSegment(p,st);
   const work=async()=>{
    if(seg.finished)return snapshot(seg);
    if(!st.started)await st.ready;
    st.touched=Date.now();
    const raw=atob(p.audio || '');
    if(raw.length>seg.bytes){
     const bytes=new Uint8Array(raw.length-seg.bytes);for(let i=0;i<bytes.length;i++)bytes[i]=raw.charCodeAt(seg.bytes+i);
     await writePcmChunks(st,bytes);
     seg.bytes=raw.length;
    }
    if(p.final && !seg.ending){
     seg.ending=true;
     await writePcmChunks(st,new Uint8Array(12800)); // 400ms trailing silence to flush endpointing
     setTimeout(()=>{
      if(!seg.finished){
       if(!seg.finalText && seg.draftText)seg.finalText=seg.draftText;
       else if(seg.finalText && seg.draftText)seg.finalText=(seg.finalText+' '+seg.draftText).trim();
       seg.draftText='';
       seg.finished=true;
      }
     },900);
    }
    return snapshot(seg);
   };
   const result=st.queue.then(work,work);st.queue=result.catch(()=>{});return await result;
  }catch(e){return {status:'error',error:e.name+': '+e.message};}},
  read(p){const seg=segments.get(p.id);return seg?snapshot(seg):{status:'error',error:'Chrome STT 세션이 없습니다'};}
 };
})();
`

func handleChromeSpeechStop() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var metadata STTTranscribeRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&metadata); err != nil || metadata.SessionID == "" || metadata.SegmentID == "" || len(metadata.SessionID) > 100 || len(metadata.SegmentID) > 100 {
			http.Error(w, "invalid speech session", 400)
			return
		}
		owner := fmt.Sprintf("%x", sha256.Sum256([]byte(r.Header.Get("X-User-ID")+"\x00"+metadata.SessionID)))
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(owner+"\x00"+metadata.SegmentID)))
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		result, err := chromeTranslation.speech(ctx, chromeSpeechRequest{ID: key, Owner: owner, Language: "ko", Engine: "chrome-online"}, "stop")
		if err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}
