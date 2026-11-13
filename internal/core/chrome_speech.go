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
	var result chromeSpeechResult
	if !chromeSTTEngine(req.Engine) {
		return result, errors.New("잘못된 Chrome STT 엔진")
	}
	if len(req.Language) != 2 {
		return result, errors.New("Chrome STT 인식 언어를 선택하세요")
	}
	if !s.mu.TryLock() {
		return result, errSTTBusy
	}
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if op == "stop" && s.tab == nil {
		return chromeSpeechResult{Status: "ready"}, nil
	}
	if err := s.startLocked(); err != nil {
		return result, err
	}
	task, cancel := context.WithCancel(s.tab)
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
	err := chromedp.Run(task, action)
	if err != nil {
		return result, err
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
	if err != nil {
		return output, err
	}
	if !preview {
		req.Audio = ""
		deadline := time.Now().Add(time.Duration(float64(len(audio))/32000*float64(time.Second)) + 5*time.Second)
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
			result = next
		}
		if !result.Finished {
			return output, errors.New("Chrome STT 최종 결과 지연")
		}
	}
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
 const sessions=new Map();let active=null;
 const supportsTrack=()=>Number((navigator.userAgent.match(/Chrome\/(\d+)/)||[])[1] || 0)>=135 && !!self.MediaStreamTrackGenerator && !!self.AudioData;
 const locale=lang=>({ko:'ko-KR',en:'en-US',ja:'ja-JP',zh:'zh-CN',es:'es-ES',fr:'fr-FR',de:'de-DE',vi:'vi-VN',th:'th-TH'}[lang] || lang);
 const snapshot=s=>({status:s.error?'error':'ready',error:s.error || '',text:(s.final+' '+s.draft).trim(),stableText:s.final,draftText:s.draft,finished:s.finished,debug:{started:!!s.started,bytes:s.bytes,events:s.events,track:s.track.getSettings(),id:s.id,visible:document.visibilityState}});
 async function prepare(p){
  if(!SR)return {status:'unsupported',error:'Chrome SpeechRecognition API가 없습니다'};
  if(!supportsTrack())return {status:'unsupported',error:'Chrome 오디오 트랙 입력을 지원하지 않습니다'};
  return {status:'ready'};
 }

 async function make(p){
  if(active && active.id!==p.id && !active.finished){
   if(active.owner!==p.owner && Date.now()-active.touched<15000)throw Error('다른 Chrome STT 세션이 사용 중입니다');
   active.finished=true;active.recognition.abort();active.track.stop();active.writer.close().catch(()=>{});
  }
  if(!supportsTrack())throw Error('이 Chrome은 PCM 오디오 트랙 입력을 지원하지 않습니다');
  const track=new MediaStreamTrackGenerator({kind:'audio'});const writer=track.writable.getWriter();
  const recognition=new SR();recognition.lang=locale(p.language);recognition.continuous=true;recognition.interimResults=true;
  recognition.processLocally=false;
  const s={id:p.id,owner:p.owner,recognition,track,writer,bytes:0,final:'',draft:'',finished:false,ending:false,error:'',events:[],touched:Date.now()};
  for(const event of ['start','audiostart','soundstart','speechstart','speechend','audioend','end','result','nomatch','error'])recognition.addEventListener(event,e=>s.events.push(event+(e.error?':'+e.error:'')));
  recognition.onresult=e=>{
   let committed='',draft='';
   for(let i=0;i<e.results.length;i++){if(e.results[i].isFinal)committed+=e.results[i][0].transcript+' ';else draft+=e.results[i][0].transcript;}
   s.final=committed.trim();s.draft=draft;
  };
  recognition.onerror=e=>{if(e.error!=='no-speech' && !(e.error==='aborted' && s.finished))s.error='Chrome STT: '+e.error;};
  recognition.onend=()=>{s.finished=true;s.track.stop();s.writer.close().catch(()=>{});};
  track.addEventListener('ended',()=>s.events.push('track-ended'));track.addEventListener('mute',()=>s.events.push('track-mute'));
  let readyResolve,readyReject;const ready=new Promise((resolve,reject)=>{readyResolve=resolve;readyReject=reject;});s.ready=ready;s.queue=Promise.resolve();
  const timer=setTimeout(()=>{if(!s.started && !s.finished){s.error='Chrome STT 시작 응답 없음';s.finished=true;recognition.abort();track.stop();writer.close().catch(()=>{});readyReject(Error(s.error));}},8000);
  recognition.onstart=()=>{s.started=true;clearTimeout(timer);readyResolve();if(s.ending)setTimeout(()=>{if(!s.finished)recognition.stop();},300);};
  const original=recognition.onerror;recognition.onerror=e=>{original(e);clearTimeout(timer);if(!s.started){s.finished=true;track.stop();writer.close().catch(()=>{});readyReject(Error(s.error || e.error));}};
  try{recognition.start(track);}catch(e){clearTimeout(timer);track.stop();writer.close().catch(()=>{});throw e;}
  sessions.set(p.id,s);active=s;
  const silence=new AudioData({format:'s16',sampleRate:16000,numberOfFrames:1600,numberOfChannels:1,timestamp:0,data:new Uint8Array(3200)});
  try{await writer.write(silence);}finally{silence.close();}
  await ready;
  while(sessions.size>4){const key=sessions.keys().next().value;sessions.delete(key);}
  return s;
 }
 setInterval(()=>{if(active && !active.finished && Date.now()-active.touched>15000){active.finished=true;active.recognition.abort();active.track.stop();active.writer.close().catch(()=>{});}},5000);
 window.chromeSpeech={prepare,
  stop(p){const s=sessions.get(p.id);if(s && s.owner===p.owner && !s.finished){s.finished=true;s.recognition.abort();s.track.stop();s.writer.close().catch(()=>{});}return {status:'ready'};},
  async push(p){try{
   let s=sessions.get(p.id);
   if(!s)s=await make(p);
   const work=async()=>{
   if(s.finished)return snapshot(s);
   if(!s.started)await s.ready;
   s.touched=Date.now();
   const raw=atob(p.audio || '');
   if(raw.length>s.bytes){
    const bytes=new Uint8Array(raw.length-s.bytes);for(let i=0;i<bytes.length;i++)bytes[i]=raw.charCodeAt(s.bytes+i);
    const frame=new AudioData({format:'s16',sampleRate:16000,numberOfFrames:bytes.length/2,numberOfChannels:1,timestamp:100000+s.bytes/32000*1e6,data:bytes});
    try{await s.writer.write(frame);}finally{frame.close();}s.bytes=raw.length;
   }
   if(p.final && !s.ending){s.ending=true;if(s.started)setTimeout(()=>{if(!s.finished)s.recognition.stop();},300);}
   return snapshot(s);
   };
   const result=s.queue.then(work,work);s.queue=result.catch(()=>{});return await result;
  }catch(e){return {status:'error',error:e.name+': '+e.message};}},
  read(p){const s=sessions.get(p.id);return s?snapshot(s):{status:'error',error:'Chrome STT 세션이 없습니다'};}
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
