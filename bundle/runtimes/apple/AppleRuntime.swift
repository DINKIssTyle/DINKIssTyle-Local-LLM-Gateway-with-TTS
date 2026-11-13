import AppKit
import SwiftUI
import Speech
import Translation
import AVFoundation
import Foundation

struct RuntimeError: LocalizedError {
 let message:String
 var errorDescription:String? {message}
}

@MainActor final class SpeechSession {
 let id:String, owner:String
 let transcriber:SpeechTranscriber, analyzer:SpeechAnalyzer, format:AVAudioFormat
 let continuation:AsyncStream<AnalyzerInput>.Continuation
 var task:Task<Void,Never>?
 var converter:AVAudioConverter?
 var bytes=0, finished=false, ending=false, error=""
 var touched=Date()
 struct Span {let start:Double,end:Double,text:String,final:Bool}
 var spans=[Span]()
 init(id:String,owner:String,transcriber:SpeechTranscriber,format:AVAudioFormat){
  self.id=id;self.owner=owner;self.transcriber=transcriber;self.format=format
  let (stream,builder)=AsyncStream.makeStream(of:AnalyzerInput.self)
  continuation=builder
  analyzer=SpeechAnalyzer(inputSequence:stream,modules:[transcriber])
  task=Task { [weak self] in
   guard let self else{return}
   do {
    for try await result in transcriber.results {
     let start=CMTimeGetSeconds(result.range.start),end=CMTimeGetSeconds(CMTimeRangeGetEnd(result.range))
     spans.removeAll{($0.start==start || ($0.start<end && $0.end>start)) && (!$0.final || result.isFinal)}
     spans.append(Span(start:start,end:end,text:String(result.text.characters),final:result.isFinal));spans.sort{$0.start<$1.start}
    }
   }catch {if !finished {self.error=error.localizedDescription}}
   finished=true
  }
 }
 func append(_ data:Data) throws {
  guard !finished else {throw RuntimeError(message:error.isEmpty ? "Apple STT 세션이 종료되었습니다":"Apple STT: "+error)}
  guard data.count>bytes else{return}
  let delta=data.subdata(in:bytes..<data.count)
  guard delta.count%2==0 else{throw RuntimeError(message:"잘못된 PCM16 프레임")}
  let source=AVAudioFormat(commonFormat:.pcmFormatFloat32,sampleRate:16000,channels:1,interleaved:false)!
  let frames=delta.count/2
  let pcm=AVAudioPCMBuffer(pcmFormat:source,frameCapacity:AVAudioFrameCount(frames))!;pcm.frameLength=AVAudioFrameCount(frames)
  delta.withUnsafeBytes{raw in
   for index in 0..<frames {
    let low=UInt16(raw[index*2]),high=UInt16(raw[index*2+1])<<8
    pcm.floatChannelData![0][index]=Float(Int16(bitPattern:low|high))/32768
   }
  }
  if converter==nil {converter=AVAudioConverter(from:source,to:format)}
  guard let converter else {throw RuntimeError(message:"Apple STT 오디오 포맷 변환 불가")}
  let output=AVAudioPCMBuffer(pcmFormat:format,frameCapacity:AVAudioFrameCount(ceil(Double(frames)*format.sampleRate/16000)+1024))!
  var supplied=false,conversionError:NSError?
  let status=converter.convert(to:output,error:&conversionError){_,state in
   if supplied {state.pointee = .noDataNow;return nil};supplied=true;state.pointee = .haveData;return pcm
  }
  if status == .error {throw conversionError ?? RuntimeError(message:"Apple STT PCM 변환 실패") as NSError}
  if output.frameLength>0 {continuation.yield(AnalyzerInput(buffer:output,bufferStartTime:CMTime(value:Int64(bytes/2),timescale:16000)))}
  bytes=data.count;touched=Date()
 }
 func finish() async {
  guard !ending && !finished else{return};ending=true;continuation.finish()
  do {try await analyzer.finalizeAndFinishThroughEndOfInput()}catch{self.error=error.localizedDescription}
  await task?.value;finished=true
 }
 func stop() async {finished=true;continuation.finish();await analyzer.cancelAndFinishNow();task?.cancel()}
 func snapshot()->[String:Any]{
  let stable=spans.filter{$0.final}.map{$0.text}.joined(separator:" ")
  let draft=spans.filter{!$0.final}.map{$0.text}.joined(separator:" ")
  return ["status":error.isEmpty ? "ready":"error","error":error,"text":spans.map{$0.text}.joined(separator:" "),"stableText":stable,"draftText":draft,"finished":finished]
 }
}

@MainActor final class Preparation:NSObject,NSWindowDelegate {
 var window:NSWindow?,done=false
 let completion:(Result<Void,Error>)->Void
 init(completion:@escaping(Result<Void,Error>)->Void){self.completion=completion}
 func finish(_ result:Result<Void,Error>){guard !done else{return};done=true;completion(result);window?.close();window=nil}
 func windowWillClose(_ notification:Notification){finish(.failure(RuntimeError(message:"Apple 번역 준비를 취소했습니다")))}
}
struct PreparationView:View {
 let source:String,target:String,completion:(Result<Void,Error>)->Void
 var body:some View {
  VStack(spacing:16){Text("Apple 번역 준비").font(.title2);Text("\(source.uppercased()) → \(target.uppercased())");Text("언어 다운로드 안내가 나타나면 허용해 주세요.");ProgressView()}.padding(28).frame(width:380,height:180)
   .translationTask(source:Locale.Language(identifier:source),target:Locale.Language(identifier:target)){session in
    do {try await session.prepareTranslation();completion(.success(()))}catch{completion(.failure(error))}
   }
 }
}

@MainActor final class RuntimeController {
 var sessions=[String:SpeechSession](),order=[String](),active:SpeechSession?
 var translators=[String:TranslationSession](),preparation:Preparation?
 var speechWork:Task<[String:Any],Error>?
 var timer:Timer?
 init(){timer=Timer.scheduledTimer(withTimeInterval:5,repeats:true){[weak self]_ in Task{@MainActor in
  if let s=self?.active,!s.finished,Date().timeIntervalSince(s.touched)>20 {await s.stop()}
 }}}
 func locale(_ language:String) async throws->Locale {
  guard SpeechTranscriber.isAvailable,let locale=await SpeechTranscriber.supportedLocale(equivalentTo:Locale(identifier:language)) else {throw RuntimeError(message:"Apple STT가 지원하지 않는 기기 또는 언어입니다: "+language)}
  return locale
 }
 func transcriber(_ language:String) async throws->SpeechTranscriber {
  SpeechTranscriber(locale:try await locale(language),preset:.progressiveTranscription)
 }
 func prepareSpeech(_ language:String) async throws->[String:Any]{
  let locale=try await locale(language)
  let reserved=await AssetInventory.reservedLocales
  if !reserved.contains(locale) && reserved.count>=AssetInventory.maximumReservedLocales {
   let activeLocales=active?.finished==false ? active!.transcriber.selectedLocales : []
   guard let old=reserved.first(where:{!activeLocales.contains($0)}) else {throw RuntimeError(message:"먼저 녹음을 중지한 뒤 다른 Apple STT 언어를 준비해 주세요")}
   await AssetInventory.release(reservedLocale:old)
  }
  _ = try await AssetInventory.reserve(locale:locale)
  let module=SpeechTranscriber(locale:locale,preset:.progressiveTranscription)
  if let request=try await AssetInventory.assetInstallationRequest(supporting:[module]) {try await request.downloadAndInstall()}
  let status=await AssetInventory.status(forModules:[module])
  guard status == .installed else {throw RuntimeError(message:"Apple STT 모델 준비 필요: "+String(describing:status))}
  return ["status":"ready","language":locale.identifier]
 }
 func speech(_ p:[String:Any]) async throws->[String:Any]{
  let id=p["session"] as? String ?? "",owner=p["owner"] as? String ?? ""
  if p["op"] as? String == "sttStop" {if let s=sessions[id],s.owner==owner {await s.stop()};return ["status":"ready"]}
  if p["op"] as? String == "sttRead" {guard let s=sessions[id],s.owner==owner else{throw RuntimeError(message:"Apple STT 세션 없음")};return s.snapshot()}
  guard !id.isEmpty,!owner.isEmpty else{throw RuntimeError(message:"잘못된 Apple STT 세션")}
  let s:SpeechSession
  if let existing=sessions[id] {guard existing.owner==owner else{throw RuntimeError(message:"잘못된 Apple STT 소유자")};s=existing} else {
   if let active,!active.finished {
    guard active.owner==owner || Date().timeIntervalSince(active.touched)>20 else{throw RuntimeError(message:"다른 Apple STT 세션이 사용 중입니다")}
    await active.stop()
   }
   let module=try await transcriber(p["language"] as? String ?? "ko")
   guard await AssetInventory.status(forModules:[module]) == .installed else{throw RuntimeError(message:"Apple STT 준비 버튼으로 언어 모델을 준비해 주세요")}
   guard let format=await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith:[module]) else{throw RuntimeError(message:"Apple STT 오디오 포맷 없음")}
   s=SpeechSession(id:id,owner:owner,transcriber:module,format:format);sessions[id]=s;active=s;order.append(id)
   while order.count>4 {if let old=sessions.removeValue(forKey:order.removeFirst()){await old.stop()}}
  }
  if !s.finished {
   guard let data=Data(base64Encoded:p["audio"] as? String ?? "") else{throw RuntimeError(message:"잘못된 PCM 데이터")}
   try s.append(data)
   if p["final"] as? Bool == true {await s.finish()}
  }
  return s.snapshot()
 }
 func prepareTranslation(_ source:String,_ target:String) async throws {
  let availability=LanguageAvailability()
  let status=await availability.status(from:Locale.Language(identifier:source),to:Locale.Language(identifier:target))
  if status == .unsupported {throw RuntimeError(message:"Apple 번역이 지원하지 않는 언어 방향입니다")}
  if status != .installed {
   guard preparation==nil else{throw RuntimeError(message:"Apple 번역 준비 창이 이미 열려 있습니다")}
   try await withCheckedThrowingContinuation{(continuation:CheckedContinuation<Void,Error>) in
    let prep=Preparation{[weak self]result in self?.preparation=nil;continuation.resume(with:result)};preparation=prep
    let window=NSWindow(contentRect:NSRect(x:0,y:0,width:420,height:240),styleMask:[.titled,.closable],backing:.buffered,defer:false)
    window.title="DKST · Apple 번역 준비";window.isReleasedWhenClosed=false;window.delegate=prep;prep.window=window
    window.contentView=NSHostingView(rootView:PreparationView(source:source,target:target){result in Task{@MainActor in prep.finish(result)}})
    window.center();window.makeKeyAndOrderFront(nil);NSApp.activate(ignoringOtherApps:true)
   }
  }
  translators[source+">"+target]=TranslationSession(installedSource:Locale.Language(identifier:source),target:Locale.Language(identifier:target))
  while translators.count>4 {let key=translators.keys.first!;translators.removeValue(forKey:key)?.cancel()}
 }
 func handle(_ p:[String:Any]) async throws->[String:Any]{
  switch p["op"] as? String ?? "" {
  case "status":
   let speech=await SpeechTranscriber.supportedLocales.map(\.identifier)
   let translation=await LanguageAvailability().supportedLanguages.map{$0.minimalIdentifier}
   return ["status":"ready","available":true,"speechAvailable":SpeechTranscriber.isAvailable,"speechLanguages":speech,"translationLanguages":translation]
  case "sttPrepare":return try await prepareSpeech(p["language"] as? String ?? "ko")
  case "sttPush","sttRead","sttStop":
   let previous=speechWork
   let work=Task{@MainActor in if let previous {_ = try? await previous.value};return try await self.speech(p)}
   speechWork=work
   return try await work.value
  case "translationPrepare","translate":
   let source=p["source"] as? String ?? "",target=p["target"] as? String ?? "",key=source+">"+target
   if source==target {return ["status":"ready","text":p["text"] as? String ?? ""]}
   if p["op"] as? String == "translationPrepare" {try await prepareTranslation(source,target);return ["status":"ready"]}
   if translators[key]==nil {
    guard await LanguageAvailability().status(from:Locale.Language(identifier:source),to:Locale.Language(identifier:target)) == .installed else {throw RuntimeError(message:"Mac에서 Apple 번역 준비를 먼저 실행해 주세요")}
    translators[key]=TranslationSession(installedSource:Locale.Language(identifier:source),target:Locale.Language(identifier:target))
   }
   do {let response=try await translators[key]!.translate(p["text"] as? String ?? "");return ["status":"ready","text":response.targetText]}
   catch {translators.removeValue(forKey:key)?.cancel();throw error}
  default:throw RuntimeError(message:"잘못된 Apple 엔진 명령")
  }
 }
}

@main struct AppleRuntime {
 @MainActor static func main(){
  let app=NSApplication.shared;app.setActivationPolicy(.accessory)
  let controller=RuntimeController()
  DispatchQueue.global(qos:.userInitiated).async {
   while let line=readLine(){
    guard line.utf8.count<8*1024*1024,let data=line.data(using:.utf8),let request=(try? JSONSerialization.jsonObject(with:data)) as? [String:Any] else{continue}
    Task{@MainActor in
     var response:[String:Any]
     do{response=try await controller.handle(request)}catch{response=["status":"error","error":error.localizedDescription]}
     response["id"]=request["id"]
     if let data=try? JSONSerialization.data(withJSONObject:response),let text=String(data:data,encoding:.utf8){print(text);fflush(stdout)}
    }
   }
   Task{@MainActor in NSApp.terminate(nil)}
  }
  app.run()
 }
}
