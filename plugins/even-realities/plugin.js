import {TranslationDirection,pronunciationText} from './translation-direction.mjs';
import {isInformationHintPlan} from './hint-policy.mjs';
import {localizeVerifiedHint,hintNeedsLocalization} from './hint-localization.mjs';
import {limitHUDResponse} from './ai-response-limit.mjs';
import {normalizeHUDDimming,hudBrightness,hudPreviewColor,hudLuminance,hudBorderBrightness,hudBorderPreviewColor} from './hud-brightness.mjs';
import {HintAudit} from './hint-audit.mjs';
import {llmTranslation,stableTranslationPreview} from './llm-translation.mjs';
import {TranslationMetrics} from './translation-metrics.mjs';
import {AudioUpload} from './audio-upload.mjs';
import {validateHintGrounding} from './hint-grounding.mjs';
import {HintStack,hintCardCount,stackedHintPresentation} from './hint-stack.mjs';
import {compareHintDuplicate,PreviewIntent,VoiceLatency} from './realtime-hints.mjs';
import {HeadGesture,imuVector,poseSample,makeHeadCalibration,validHeadCalibration,blankHeadPage} from './head-gesture.mjs';
import {HintLifetime,secondsPerHintLine,timedHintLines,hintCountdownWindow,aiTitleLine} from './hint-autoclose.mjs';
import {adaptiveHUD,hintWindow} from './adaptive-hud.mjs';
import {hintPresentation,JEV_ONLY,normalizeHintBudget,requestHintJudgment,suggestedHintJudge} from './hint-judge.mjs';
import {SETTINGS_MENUS,settingsMenuItems,settingsPickerPage} from './settings-menu.mjs';
import {GlassesInputProcessor,normalizeGlassesInput,inputSegmentOptions,levelDb,GLASSES_THRESHOLDS} from './glasses-input.mjs';
import {aiStatusIcon,normalizeAIHeaderLayout,normalizeAIIconPosition,aiIconPositionLabel} from './ai-status-icon.mjs';
import {headerLayout} from './header-layout.mjs';
import {HudIconStore,microphoneIcon,hudIconPath,captureStatusIcon,iconHeaderMessage} from './hud-icons.mjs';
import {EnginePreparation,waitForPreparation} from './engine-preparation.mjs';
import {HUDTransport} from './hud-transport.mjs';
import {readFinalTranscription} from './stt-final-stream.mjs';
import {abortableWait} from './abortable-wait.mjs';
import {chatDeadline,chatFailureMessage} from './chat-deadline.mjs';
import {ROLE_MODEL_KEYS,normalizeRoleModels,resolveRoleModel,llmRequestRole,roleModelOptions} from './role-models.mjs';
import {LiveTranslationQueue} from './live-translation.mjs';
import {StableTranslation} from './stable-translation.mjs';
import {TranslationSentence} from './translation-sentence.mjs';
import {TranslationHistory} from './translation-history.mjs';
import {ChatMemory,routeAIUtterance} from './ai-conversation.mjs';
import {localTranslate} from './chrome-translation.mjs';
import {filterSpeakerPCM} from './speaker-filter.mjs';
import { mergeTranslationDisplay, hasCompleteTranslationReplies, parseTranslationDisplay, translationReplyText, translationPhoneText } from './translation-display.mjs';
import { directQuestionIntent } from './direct-question.mjs';
import { parseCueResponse, parseHintPlan, hintResultKind, extractHintFact, readHintPlanResponse } from './hint-enrichment.mjs';
import { HintGate,hintActivityTiming } from './hint-gate.mjs';
import { waitForEvenAppBridge, AudioInputSource, OsEventTypeList } from '@evenrealities/even_hub_sdk';
import { normalizeModels } from './model-list.mjs';
import { SpeechSegmenter, mergePCM } from './audio-segments.mjs';
import { modeHeaderText, translationHeaderText, rightAlignedStatusText, hudStatusText, hudNoticeText, withAnimatedDots, extractVisibleAndReasoning, stripSourcesForGlasses, formatToolStatusText, sliceHUDWindow, normalizeGlassesMode, nextGlassesMode, glassesModeMenuIdToMode, targetLanguageLabel, glassesModeShortLabel, glassesModeBadgeText } from './hud-status.mjs';
import { LocalAgreement } from './local-agreement.mjs';

/**
 * DKST AI Gateway Plugin for Even Realities G2
 * Powered by @evenrealities/even_hub_sdk
 */

(function () {
  'use strict';

  // --- Configuration & State ---
  const STORAGE_KEY = 'dkst_even_realities_settings';
  let config = {
    serverUrl: DKST_DEFAULT_SERVER_URL || window.location.origin,
    username: '',
    apiKey: '',
    model: '',
    roleModels:{hintDecision:JEV_ONLY},
    hintDecisionAuto:true,
    hintContextTokens:600,
    connectionMode: 'sse',
    conciseMode: true,
    aiHintMode: true,
    hintSensitivity: 'balanced',
    hintWebSearch: true,
    aiHeaderLayout:'standard',aiIconPosition:552,hudDimEnabled:true,hudDimLevel:1,hudFullTranscript:false,hudFullAI:true,
    hideTranscript:false,
    triggerTranscriptOnly:false,
    hintAutoClose:false,
    hintCardCount:1,
    hintSecondsPerLine:3,
    headGestureEnabled:false,
    headGestureCalibration:null,
    glassesMode: 'ai',
    translateSourceLang: 'auto',
    translateTargetLang: 'ko',
    translateExcludeSelf: true,
    chromeTranslation: true,
    translationEngine: "",
    voiceMode: 'continuous',
    triggerWord: '이븐',
    speechFilter: 'strict',
    sttEngine: 'whisper',
    audioInputSource:'glasses',
    glassesInput:{sensitivity:'high',gain:1},
    sttFallback: true,
    speakerScope: 'conversation',
    enableTTS: false,
    authToken: ''
  };

  let activeSttEngine='whisper';
  let activeAudioInputSource=AudioInputSource.Glasses;
  let activeGlassesInput=normalizeGlassesInput(config.glassesInput),activeInputProcessor=new GlassesInputProcessor(activeGlassesInput),lastInputMeterAt=0;
  let bridge = null;
  let settingsEdited = false;
  let pendingNativeSave = false;
  let previousBodyOverflow = '';
  let recordReturnFocus = null;
  let displayReady = false;
  let micBusy = false;
  let capturePreparing = false;
  let captureGeneration = 0;
  let captureAbort = null;
  let captureTask=null,inputSwitchPromise=null;
  let headPaused=false,headResumeWanted=false,headTransition=null,nativeHeadBlank=false;
  let headImuActive=false,headImuWanted=false,headImuPromise=null;
  let headCalibrationPhase='idle',headCapture=null,headNeutral=null,headCalibrationTimer=null;
  const headDetector=new HeadGesture();
  let pendingSettings={}, resumeAfterSettings=false, recognitionEpoch=0;
  let settingsPicker=null, nativePickerKey='',nativeMenuSource='';
  let pendingMicOpen = null;
  let micStopPromise = null;
  let micMayBeOpen = false;
  const pendingMicCloses = new Set();
  let micCloseUncertain = false;
  const pendingHUD = new Map();
  let statusTimer = null;
  let glassesSN = null;
  let glassesBattery = null;
  let unsubscribeDeviceStatus = null;
  let lastStatus = '';
  let sendingHUD = false;
  const hudTransport = new HUDTransport({onReady:()=>{
    nativeBodyLayoutKey='';nativeHeaderLayoutKey='';nativePickerKey='';nativeHeadBlank=false;
    lastHudIconKeys.clear();
    if(displayReady)flushHUD();
  }});
  let isRecording = false;
  let audioBuffer = [];
  let isThinking = false;
  let sessionId = '';
  let recordedBytes = 0;
  let receivedCaptureAudio=false;
  let recordingStarted = 0;
  let lastVoiceFrameAt = 0;
  let captureTimer = null;
  let manualTimer = null;
  let segmenter = null;
  let pendingClips = [];
  let draining = false;
  let activeFinalClip = null;
  let activeUtterance = null;
  let pendingPreview = null;
  let previewBusy = false;
  let draftUtteranceId = '';
  let captionFinal = '';
  let captureFailure = '';
  let captionStable = '';
  let captionDraft = '';
  let answerText = '';
  let answerTitle = '';
  let answerIsWakeInput=false;
  let answerVoicePrompt='';
  let answerIsHint=false;
  let updatingHintStack=false;
  let answerAutoCloseEligible=false;
  let translationDisplay={translation:'',positive:'',negative:''};
  let translationReplyVersion=0;
  let translationToken=0,translating=false,liveTranslationDraft=false,translationFailure='';
  let answerHoldUntil = 0;
  let answerTimer = null;
  let hintDrainTimer=null;
  let processingAI = false;
  let activeAIJob = null;
  const pendingAI = [];
  const knownAIJobs = new Set();
  let pendingTrigger = false;
  const voiceWakeJobs=new Map();let nextWakeJob=0,lastWakeBrightness=null,previewWakePrompt='';
  function wakeBrightness(){return hudTextBrightness(8);}
  function hudTextBrightness(id){return hudBrightness(id,config,{translate:normalizeGlassesMode(config.glassesMode)==='translate',wakeActive:!!(previewWakeId || pendingTrigger || voiceWakeJobs.size)});}
  function syncHUDDimmingControls(){
    Object.assign(config,normalizeHUDDimming(config));
    els.hudDimEnabled.checked=config.hudDimEnabled;els.hudDimLevel.value=String(config.hudDimLevel);
    els.hudDimLevel.disabled=!config.hudDimEnabled;
    els.hudDimValue.textContent=config.hudDimLevel===0?'최저':`${config.hudDimLevel*25}%`;
    els.hudFullTranscript.checked=config.hudFullTranscript;els.hudFullAI.checked=config.hudFullAI;
  }
  function updateHUDDimming(){
    settingsEdited=true;
    Object.assign(config,normalizeHUDDimming({hudDimEnabled:els.hudDimEnabled.checked,hudDimLevel:els.hudDimLevel.value,hudFullTranscript:els.hudFullTranscript.checked,hudFullAI:els.hudFullAI.checked}));
    syncHUDDimmingControls();renderLiveHUD();persistSettings().catch(console.warn);
  }
  function applyHUDPreviewBrightness(){
    for(const [id,level] of [['hud-status-text',hudTextBrightness(2)],['hud-mode-label',hudTextBrightness(7)],['hud-header-message',hudTextBrightness(3)],['hud-transcript-body',hudTextBrightness(4)]]){
      const el=document.getElementById(id);if(el){el.style.color=hudPreviewColor(level);el.style.textShadow=level===4?'':'none';}
    }
    for(const suffix of ['', '-older'])for(const prefix of ['hud-ai-badge','hud-ai-meta','hud-ai-body']){
      const el=document.getElementById(prefix+suffix);if(el){el.style.color=hudPreviewColor(hudTextBrightness(prefix==='hud-ai-body'?1:9));el.style.textShadow=hudTextBrightness(1)===4?'':'none';}
    }
    if(els.hudScreen){els.hudScreen.style.setProperty('--hud-ai-color',hudBorderPreviewColor(config));els.hudScreen.style.setProperty('--hud-transcript-color',hudPreviewColor(hudTextBrightness(4)));}
  }
  let aiStatusError='';
  let aiIconPositionApplied=null,aiIconPositionTimer=null;
  let lastTranscriptBrightness=null;
  function wakeInputDisplay(name,question=''){const q=String(question || '').trim();return `${name}: ${q?`${q} …`:'듣는 중...'}`;}
  function finishWakeJob(token){if(token!==undefined)voiceWakeJobs.delete(token);queueHudIcons();}
  let transcriptText = '';
  const transcriptContext = [];
  const chatMemory=new ChatMemory();
  let chatEpoch=1;
  function chatScope(){return [config.serverUrl,config.username,resolveRoleModel('chat',config)].join('\0');}
  let availableModels=[];
  function aiPresentationText(){return hintLifetime.active?timedHintLines(answerText).join('\n'):answerText;}
  function triggerTranscriptText(){return [...voiceWakeJobs.values()].at(-1) || '';}
  function transcriptDisplayHidden(){
    if(!config.hideTranscript)return false;
    return !(config.triggerTranscriptOnly && triggerTranscriptText() && !answerText);
  }
  function hintStackActive(){return config.hintCardCount===2 && answerIsHint && hintStack.cards.length>0 && normalizeGlassesMode(config.glassesMode)==='ai';}
  function currentBodyLayout(){if(hintStackActive())return stackedHintPresentation(hintStack,transcriptDisplayHidden());return adaptiveHUD({mode:normalizeGlassesMode(config.glassesMode),title:answerTitle,hideTitle:answerIsWakeInput,text:aiPresentationText(),hideTranscript:transcriptDisplayHidden()});}
  function aiVisibleLines(){return currentBodyLayout().contentLines || 6;}
  let nativeTranslateLayout=false,nativeHeaderLayoutKey='',nativeBodyLayoutKey='';
  const AI_SCROLL_START_MS = 5000;
  const AI_SCROLL_LINE_MS = 2500;
  // Inner width: 576 - 2 × (2px padding + 1px border).
  const AI_BOX_WIDTH_PX = 570;

  const TRANSCRIPT_WIDTH_PX = 576;
  let aiPhaseTitle = '';
  let aiPhaseDetail = '';
  let aiReasoningPreview = '';
  let aiReasoningStartMs = 0;
  let aiAnimStep = 0;
  let aiAnimTimer = null;
  let aiScrollOffset = 0;
  let aiScrollTimer = null;
  let lastAIHUDContent = '';
  let lastAITitleContent='';
  let lastSecondAITitleContent='';
  let lastTranscriptHUDContent = '';
  let transcriptStatusMessage = '';
  let previewAbortController = null;
  let clipAbortController = null;
  let aiAbortController = null;
  let aiAbortedByModeSwitch = false;
  const MAX_PENDING_CLIPS = 64;
  const MAX_PENDING_AUDIO_BYTES = 32000 * 120; // Pause after two minutes of unsaved PCM.

  // --- DOM Elements ---
  const els = {
    serverUrl: document.getElementById('server-url'),
    btnTestConnection: document.getElementById('btn-test-connection'),
    authUsername: document.getElementById('auth-username'),
    authPassword: document.getElementById('auth-password'),
    btnLogin: document.getElementById('btn-login'),
    loginStatusLabel: document.getElementById('login-status-label'),
    apiKey: document.getElementById('api-key'),
    modelSelect: document.getElementById('model-select'),
    roleModels:{chat:document.getElementById('model-ai-chat'),hintDecision:document.getElementById('model-hint-decision'),hint:document.getElementById('model-ai-hint'),translation:document.getElementById('model-translation'),translationReply:document.getElementById('model-translation-reply')},
    btnRefreshModels: document.getElementById('btn-refresh-models'),
    connectionMode: document.getElementById('connection-mode'),
    optConciseMode: document.getElementById('opt-concise-mode'),
    optEnableTts: document.getElementById('opt-enable-tts'),
    btnSaveSettings: document.getElementById('btn-save-settings'),
    connectionStatus: document.getElementById('connection-status'),
    connectionText: document.getElementById('connection-text'),
    hudScreen: document.getElementById('hud-screen'),
    hudStatus: document.getElementById('hud-status-text'),
    hudNotice: document.getElementById('hud-notice'),
    hudModeIcon:document.getElementById('hud-mode-icon'),hudStateIcon:document.getElementById('hud-state-icon'),hudMicIcon:document.getElementById('hud-mic-icon'),
    hudModeLabel:document.getElementById('hud-mode-label'),hudTriggerKeyword:document.getElementById('hud-trigger-keyword'),hudHeaderMessage:document.getElementById('hud-header-message'),
    hudPageIndicator: document.getElementById('hud-page-indicator'),
    glassesMode: document.getElementById('glasses-mode'),
    translateExcludeSelf: document.getElementById('translate-exclude-self'),
    chromeTranslation: document.getElementById('chrome-translation'),
    translationEngine:document.getElementById('translation-engine'),
    btnPrepareChrome: document.getElementById('prepare-chrome-translation'),
    chromeTranslationStatus: document.getElementById('chrome-translation-status'),
    translateSourceLang: document.getElementById('translate-source-lang'),
    translateTargetLang: document.getElementById('translate-target-lang'),
    translateLangRow: document.getElementById('translate-lang-row'),
    modeBtnAi: document.getElementById('mode-btn-ai'),
    modeBtnTranslate: document.getElementById('mode-btn-translate'),
    modeDescription: document.getElementById('mode-description'),
    voiceMode: document.getElementById('voice-mode'),
    hintContextTokens:document.getElementById('hint-context-tokens'),
    hintJudgeStatus:document.getElementById('hint-judge-status'),
    hintSensitivity: document.getElementById('hint-sensitivity'),
    hintWebSearch: document.getElementById('hint-web-search'),
    aiIconPosition:document.getElementById('ai-icon-position'),aiIconPositionValue:document.getElementById('ai-icon-position-value'),aiHeaderLayout:document.getElementById('ai-header-layout'),hudHeader:document.getElementById('hud-status-row'),hudDimEnabled:document.getElementById('hud-dim-enabled'),hudDimLevel:document.getElementById('hud-dim-level'),hudDimValue:document.getElementById('hud-dim-value'),hudFullTranscript:document.getElementById('hud-full-transcript'),hudFullAI:document.getElementById('hud-full-ai'),
    hideTranscript:document.getElementById('hide-transcript'),
    triggerTranscriptOnly:document.getElementById('trigger-transcript-only'),
    headGestureEnabled:document.getElementById('head-gesture-enabled'),
    headCalibrate:document.getElementById('head-calibrate'),headStatus:document.getElementById('head-gesture-status'),headResume:document.getElementById('btn-head-resume'),
    hintCardCount:document.getElementById('hint-card-count'),
    hintAutoClose:document.getElementById('hint-auto-close'),
    hintSecondsPerLine:document.getElementById('hint-seconds-per-line'),
    hintStatus: document.getElementById('hint-status'),
    chatForm: document.getElementById('text-chat-form'),
    chatInput: document.getElementById('chat-input'),
    chatQuestion: document.getElementById('chat-question'),
    chatOutput: document.getElementById('chat-output'),
    triggerWord: document.getElementById('trigger-word'),
    speechFilter: document.getElementById('speech-filter'),
    sttEngine: document.getElementById('stt-engine'),
    audioInputSource:document.getElementById('audio-input-source'),
    glassesInputSettings:document.getElementById('glasses-input-settings'),glassesSensitivity:document.getElementById('glasses-sensitivity'),glassesGain:document.getElementById('glasses-gain'),glassesInputMeter:document.getElementById('glasses-input-meter'),glassesInputLevel:document.getElementById('glasses-input-level'),
    sttFallback: document.getElementById('stt-fallback'),
    sttEngineStatus: document.getElementById('stt-engine-status'),
    btnPrepareStt: document.getElementById('prepare-chrome-stt'),
    speakerScope: document.getElementById('speaker-scope'),
    transcriptModal: document.getElementById('transcript-modal'),
    btnCloseTranscripts: document.getElementById('btn-close-transcripts'),
    transcriptDate: document.getElementById('transcript-date'),
    transcriptLog: document.getElementById('transcript-log'),
    btnTranscripts: document.getElementById('btn-transcripts'),
    btnExportTranscripts: document.getElementById('btn-export-transcripts'),
    voiceBtn: document.getElementById('voice-btn'),
    actionHint: document.getElementById('action-hint')
  };

  const previewIntent=new PreviewIntent();
  let previewWakeId='';
  const loadedModelPromises=new Map(),loadedModelInstances=new Map(),modelLoadErrors=new Map();
  function modelLoadKey(model){return JSON.stringify([config.serverUrl,getAuthHeaders().Authorization || '',model]);}
  function isModelUnloadedError(status,detail=''){
    return [400,404,409,502,503].includes(status) && /not\s*loaded|unloaded|no\s*models?\s*loaded|model.*not\s*found|failed\s*to\s*load|load\s*model/i.test(detail);
  }
  async function ensureModelLoaded(model,signal,force=false){
    const id=String(model || '').trim();
    if(!id || id===JEV_ONLY || !config.serverUrl)return false;
    const key=modelLoadKey(id);
    // Keep an in-flight load shared even when a failed inference asks to retry.
    if(signal?.aborted)throw Object.assign(Error('cancelled'),{name:'AbortError'});
    let entry=loadedModelPromises.get(key);
    if(!entry){
      const status=document.getElementById('model-load-status');
      if(status)status.textContent=`모델 준비 확인 중 · ${id}`;
      const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),190000);
      entry=(async()=>{
        try{
          const res=await gatewayFetch(`${config.serverUrl}/api/models/ensure`,{
            method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),
            body:JSON.stringify({model:id}),signal:controller.signal
          });
          if(!res.ok)throw Error((await res.text()).trim().slice(0,400) || `HTTP ${res.status}`);
          const ready=await res.json();
          if(ready.status!=='ready' || !ready.instance_id)throw Error('서버가 모델 준비를 확인하지 못했습니다. 게이트웨이를 업데이트하세요.');
          loadedModelInstances.set(key,ready.instance_id);modelLoadErrors.delete(key);
          if(status)status.textContent=`모델 준비 완료 · ${ready.instance_id}`;
          while(loadedModelInstances.size>32)loadedModelInstances.delete(loadedModelInstances.keys().next().value);
          return true;
        }catch(error){modelLoadErrors.set(key,error.message);while(modelLoadErrors.size>32)modelLoadErrors.delete(modelLoadErrors.keys().next().value);if(status)status.textContent=`모델 준비 실패 · ${id} · ${error.message}`;return false;}
        finally{clearTimeout(timer);loadedModelPromises.delete(key);}
      })();
      loadedModelPromises.set(key,entry);
      while(loadedModelPromises.size>16)loadedModelPromises.delete(loadedModelPromises.keys().next().value);
    }
    if(!signal)return entry;
    if(signal.aborted)throw Object.assign(Error('cancelled'),{name:'AbortError'});
    return new Promise((resolve,reject)=>{
      const onAbort=()=>{cleanup();reject(Object.assign(Error('cancelled'),{name:'AbortError'}));};
      const cleanup=()=>signal.removeEventListener('abort',onAbort);
      signal.addEventListener('abort',onAbort,{once:true});
      entry.then(ok=>{cleanup();resolve(ok);},err=>{cleanup();reject(err);});
    });
  }
  function prewarmConfiguredModels(){
    const models=new Set([config.model,...Object.values(config.roleModels || {})].map(v=>String(v || '').trim()).filter(v=>v && v!==JEV_ONLY));
    for(const model of models)ensureModelLoaded(model).catch(()=>{});
  }
  const recentHintNotes=[];
  let hintComparisonScope=null;
  let currentHintAudit=null;
  const deviceHintAcks=new Set();
  const hintAudit=new HintAudit({fetch:gatewayFetch,spec:()=>({server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'})}),version:document.getElementById('plugin-build-version')?.textContent?.trim() || 'unknown',onStatus:text=>{const el=document.getElementById('hint-history-status');if(el)el.textContent=text;}});
  function auditHint(job,stage,details={}){
    if(!job)return;
    const plan=job.hintPlan || {};
    hintAudit.record({sessionId:job.auditSession || sessionId,segmentId:job.id || '',jobId:job.auditJobID || `${job.epoch ?? chatEpoch}-${job.sequence ?? job.id ?? 'manual'}`,stage,topic:plan.topic || '',kind:plan.kind || '',source:plan.source || '',evidence:plan.evidence || job.focusText || '',...details});
  }
  function visibleHintAudits(){return !answerIsHint?[]:hintStackActive()?hintStack.cards.map(card=>card.auditMeta).filter(Boolean):[currentHintAudit].filter(Boolean);}
  function hintDeviceDisplayed(meta){if(!meta || deviceHintAcks.has(meta.auditJobID))return;deviceHintAcks.add(meta.auditJobID);while(deviceHintAcks.size>100)deviceHintAcks.delete(deviceHintAcks.values().next().value);auditHint(meta,'device_display',{reason:'sdk_ack'});}

  const voiceLatency=new VoiceLatency({onUpdate:summary=>{
    const el=document.getElementById('voice-latency');
    if(el && Object.keys(summary).length)el.textContent=Object.entries(summary).map(([label,ms])=>`${label}: ${ms}ms`).join(' · ');
  }});
  function rememberHint(plan){
    const presentation=hintPresentation(plan);
    recentHintNotes.unshift({time:new Date().toLocaleTimeString(),title:presentation.title,reason:presentation.reason});
    recentHintNotes.length=Math.min(recentHintNotes.length,12);
    const el=document.getElementById('hint-evidence-log');
    if(el)el.textContent=recentHintNotes.map(note=>`${note.time} ${note.title}\n${note.reason}`).join('\n\n');
  }
  function reconcileHintTranscript(id,text){
    hintGate.validatePreview(id,text);
    if(activeAIJob?.hint && hintGate.revisedEvidence(activeAIJob,id,text)){
      activeAIJob.cancelReason='transcript_revised';aiAbortController?.abort();
    }
    if(!answerIsHint)return;
    if(hintStackActive()){
      for(const card of [...hintStack.cards])if(card.auditMeta && hintGate.revisedEvidence(card.auditMeta,id,text)){
        auditHint(card.auditMeta,'canceled',{reason:'transcript_revised'});hintStack.remove(card.id);
      }
    }else if(currentHintAudit && hintGate.revisedEvidence(currentHintAudit,id,text)){
      auditHint(currentHintAudit,'canceled',{reason:'transcript_revised'});
      stopAIAutoScroll();hintLifetime.clear();clearHintStack();answerText='';answerTitle='';answerAutoCloseEligible=false;
      if(els.chatOutput)els.chatOutput.textContent='';renderLiveHUD();
    }
  }
  function observeAIPreview(snapshot,data,agreed){
    if(snapshot.mode!=='ai')return;
    reconcileHintTranscript(snapshot.segmentId,data.text);
    if(pendingTrigger){
      previewWakePrompt=String(data.text || '').trim();
      const wakeName=triggerLabels()[0] || 'AI';
      clearHintStack();hintLifetime.clear();answerAutoCloseEligible=false;answerVoicePrompt='';
      answerIsWakeInput=true;answerTitle=`${wakeName} 질문`;answerText=wakeInputDisplay(wakeName,previewWakePrompt);
      if(els.chatOutput)els.chatOutput.textContent=answerText;
      setHUDNotice(previewWakePrompt?'질문 인식 중':'질문 대기');
      return;
    }
    const intent=previewIntent.observe({id:snapshot.segmentId,text:data.text,stableText:agreed.stable,aliases:triggerLabels()});
    if(intent.newWake){
      previewWakeId=snapshot.segmentId;
      voiceLatency.mark(snapshot.segmentId,'preview_wake');
      hintGate.reset();
      if(activeAIJob?.hint){activeAIJob.cancelReason='chat_priority';aiAbortController?.abort();}
      for(let i=pendingAI.length-1;i>=0;i--)if(pendingAI[i].hint)pendingAI.splice(i,1);
    }else if(!intent.wake){
      if(previewWakeId){previewWakeId='';previewWakePrompt='';if(answerTitle.endsWith(' 질문') && !processingAI){answerIsWakeInput=false;answerTitle='';answerText='';if(els.chatOutput)els.chatOutput.textContent='';}}
    }
    if(intent.wake){
      previewWakePrompt=intent.prompt || '';
      const wakeName=triggerLabels()[0] || 'AI';
      clearHintStack();hintLifetime.clear();answerAutoCloseEligible=false;answerVoicePrompt='';
      answerIsWakeInput=true;answerTitle=`${wakeName} 질문`;answerText=wakeInputDisplay(wakeName,previewWakePrompt);
      if(els.chatOutput)els.chatOutput.textContent=answerText;
      setHUDNotice(previewWakePrompt?'질문 인식 중':'질문 대기');
    }
    if(intent.hintText && !previewWakeId)hintGate.observe(intent.hintText,{id:snapshot.segmentId,preview:true});
  }

  const hintStack=new HintStack({onChange:()=>{if(!updatingHintStack && answerIsHint)syncHintStack();}});
  function syncHintStack(){
    const top=hintStack.cards[0];
    currentHintAudit=top?.auditMeta || null;
    answerIsWakeInput=false;answerText=top?.text || '';answerTitle=top?.title || '';answerVoicePrompt='';
    answerIsHint=!!top;answerAutoCloseEligible=!!top;
    if(els.chatOutput)els.chatOutput.textContent=answerText;
    renderLiveHUD();
  }
  function clearHintStack(){answerIsWakeInput=false;answerIsHint=false;currentHintAudit=null;hintStack.clear();}
  function displayHintCard(note,text){
    hintLifetime.clear();stopAIAutoScroll();aiScrollOffset=0;
    if(config.hintCardCount===2){
      updatingHintStack=true;
      try{hintStack.configure({limit:2,autoClose:config.hintAutoClose===true,secondsPerLine:config.hintSecondsPerLine});hintStack.add({title:hintPresentation(note).title,text,auditMeta:note.auditMeta});}
      finally{updatingHintStack=false;}
      currentHintAudit=note.auditMeta || null;answerIsHint=true;syncHintStack();
    }else{
      clearHintStack();currentHintAudit=note.auditMeta || null;answerIsHint=true;answerVoicePrompt='';answerTitle=hintPresentation(note).title;answerText=text;answerAutoCloseEligible=true;startHintLifetime();
      renderLiveHUD();
    }
  }
  function updateHintCardCount(){
    settingsEdited=true;stopAIAutoScroll();aiScrollOffset=0;const title=answerTitle,text=answerText,wasHint=answerIsHint,auditMeta=currentHintAudit;
    config.hintCardCount=hintCardCount(els.hintCardCount.value);els.hintCardCount.value=String(config.hintCardCount);
    if(wasHint && text){
      hintLifetime.clear();clearHintStack();answerIsHint=true;
      if(config.hintCardCount===2){
        updatingHintStack=true;
        try{hintStack.configure({limit:2,autoClose:config.hintAutoClose===true,secondsPerLine:config.hintSecondsPerLine});hintStack.add({title,text,auditMeta});}
        finally{updatingHintStack=false;}
        syncHintStack();
      }else{currentHintAudit=auditMeta;answerText=text;answerTitle=title;answerAutoCloseEligible=true;startHintLifetime();const win=sliceHUDWindow(text,0,aiVisibleLines(),AI_BOX_WIDTH_PX);if(win.maxOffset>0)startAIAutoScroll(win.maxOffset);}
    }
    renderLiveHUD();persistSettings().catch(console.warn);
  }

  const hintLifetime=new HintLifetime({
    onTick:()=>renderLiveHUD(),
    onClose:hint=>{
      if(answerTitle!==hint.title || answerText!==hint.text)return;
      answerText=answerTitle=answerVoicePrompt='';answerIsHint=false;answerAutoCloseEligible=false;aiScrollOffset=0;answerHoldUntil=0;
      stopAIAutoScroll();clearTimeout(answerTimer);renderLiveHUD();
    }
  });
  function startHintLifetime(){
    if(hintStackActive())return;
    hintLifetime.show({text:answerText,title:answerTitle,enabled:config.hintAutoClose===true && answerAutoCloseEligible,secondsPerLine:config.hintSecondsPerLine});
  }
  function updateHintLifetimeSettings(){
    settingsEdited=true;config.hintAutoClose=els.hintAutoClose.checked===true;
    config.hintSecondsPerLine=secondsPerHintLine(els.hintSecondsPerLine.value);
    els.hintSecondsPerLine.value=String(config.hintSecondsPerLine);els.hintSecondsPerLine.disabled=!config.hintAutoClose;
    if(hintStackActive())hintStack.configure({autoClose:config.hintAutoClose===true,secondsPerLine:config.hintSecondsPerLine});
    else if(answerAutoCloseEligible && answerText)startHintLifetime();else hintLifetime.clear();
    renderLiveHUD();persistSettings().catch(console.warn);
  }
  const hintGate=new HintGate({
    timing:()=>hintActivityTiming(config.hintSensitivity,!!resolveRoleModel('hintDecision',config) && resolveRoleModel('hintDecision',config)!==JEV_ONLY),
    collectContext:true,decisionTimeoutMs:16000,windowMs:20000,settleMs:1400,collectWaitMs:2200,minContextChars:45,
    decide:async(job,signal)=>{
      ensureHintComparisonScope();
      const model=resolveRoleModel('hintDecision',config);
      const useLLM=!!model && model!==JEV_ONLY;
      job.auditSession=sessionId;job.auditJobID=`${sessionId}-${job.epoch}-${job.sequence}`;
      aiStatusError='';if(iconOnlyHeader())queueHudIcons();
      const readAdvice=async(advisory,adviceSignal)=>{
        const res=await gatewayFetch(`${config.serverUrl}/api/glasses/hint-decision`,{
          method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),signal:adviceSignal,
          body:JSON.stringify({text:job.text,context:job.context,sensitivity:config.hintSensitivity,shown:hintGate.shownFocus(),advisory,keywordsOnly:advisory})
        });
        if(!res.ok)throw new Error(`HTTP ${res.status}${res.status===404?' · 게이트웨이 업데이트 필요':''}`);
        return res.json();
      };
      if(!useLLM){try{const result=await readAdvice(false,signal);auditHint(job,'decision',{reason:result.hint?'jev_selected':result.reason || 'jev_skip',kind:result.category || ''});return result;}catch(error){if(!signal.aborted)aiStatusError=String(error.message || error);throw error;}}
      if(processingAI && !activeAIJob?.hint)return {hint:false,defer:true,reason:'answer_priority'};
      if(previewWakeId || pendingTrigger || activeAIJob && !activeAIJob.hint || pendingAI.some(job=>job.chatSession))return {hint:false,reason:'chat_priority'};
      if(signal.aborted)return {hint:false,reason:'cancelled'};
      try{
        els.hintJudgeStatus.textContent=`${model} · 토픽 판별 중`;
        const result=await requestHintJudgment({fetch:gatewayFetch,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),model,job,onDiagnostic:data=>auditHint(job,'decision_response',{reason:data.reason,text:JSON.stringify(data)}),limit:config.hintContextTokens,signal,targetLang:config.translateTargetLang,preferWeb:config.hintWebSearch!==false,sensitivity:config.hintSensitivity});
        auditHint(job,'decision',{reason:result.repairAttempts?'judge_format_repaired':result.hint?'hint_selected':result.reason || 'llm_skip',topic:result.plan?.topic || result.topic || '',kind:result.plan?.kind || '',source:result.plan?.source || '',evidence:result.focus || ''});
        if(result.hint && !hintGate.relevant(hintGate.withEvidence(job,result),result.plan.topic)){auditHint(job,'skipped',{reason:'selection_evidence_changed'});return {hint:false,reason:'past_topic'};}
        if(result.hint && result.plan.source!=='conversation' && await hintDuplicate(`${result.plan.topic}: ${result.plan.question}`,'plan',signal)){auditHint(job,'skipped',{reason:'duplicate_plan',topic:result.plan.topic});return {...result,hint:false,reason:'duplicate_plan'};}
        els.hintJudgeStatus.textContent=`${result.hint?`정보 과제: ${result.plan.question}`:'보충할 정보 없음'} · 문맥 약 ${result.contextEstimatedTokens}/${result.contextBudget}토큰`;
        return result;
      }catch(error){
        if(signal.aborted)throw error;
        aiStatusError=String(error.message || error);
        auditHint(job,'skipped',{reason:error.code || 'judge_request_error'});
        els.hintJudgeStatus.textContent=`판별 LLM 지연/오류 · 전사 계속 · ${error.message}`;
        return {hint:false,reason:error.code || 'judge_request_error'};
      }
    },
    onHint:job=>{
      if(!isRecording || config.glassesMode!=='ai' || previewWakeId || pendingTrigger || (activeAIJob && !activeAIJob.hint) || pendingAI.some(job=>!job.hint))return false;
      if(activeAIJob?.hint && !hintGate.relevant(activeAIJob,activeAIJob.hintPlan?.topic)){activeAIJob.cancelReason=hintGate.rejectionReason(activeAIJob);aiAbortController?.abort();}
      job.auditSession=sessionId;job.auditJobID=`${sessionId}-${job.epoch}-${job.sequence}`;
      auditHint(job,'selected',{reason:'queued'});
      enqueueAI(`${job.id}-hint-${job.sequence}`,job.text,job.context,'ai',config.translateTargetLang,{...job,hint:true});
      return true;
    },
    onTopicChange:()=>{
      if(activeAIJob?.hint && activeAIJob.generationStartedAt===undefined){activeAIJob.cancelledByTopic=true;activeAIJob.cancelReason='topic_changed';aiAbortController?.abort();}
      for(let i=pendingAI.length-1;i>=0;i--)if(pendingAI[i].hint && pendingAI[i].topicVersion!==hintGate.topicVersion){hintGate.complete(pendingAI[i]);pendingAI.splice(i,1);}
      els.hintStatus.textContent=activeAIJob?.hint && activeAIJob.generationStartedAt!==undefined?'새 주제 확인 · 생성 중인 힌트 완료 후 반영':'새 주제 확인 · 대기 후보 갱신';
    },
    onAudit:(job,details)=>auditHint(job,details.stage,details),
    onStatus:text=>{els.hintStatus.textContent=text;if(iconOnlyHeader())Promise.resolve().then(()=>queueHudIcons());}
  });

  function ensureHintComparisonScope(){
    const scope=JSON.stringify([config.serverUrl,config.username,getAuthHeaders().Authorization || '']);
    if(scope!==hintComparisonScope){hintComparisonScope=scope;hintGate.recent=[];}
  }
  function hintDuplicate(text,kind,signal){
    ensureHintComparisonScope();
    return compareHintDuplicate({fetch:gatewayFetch,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),text,kind,signal,history:hintGate.duplicateHistory(kind)});
  }

  function localEngine(){return config.translationEngine || (config.chromeTranslation===false?'llm':'chrome');}
  function localLabel(){return ({apple:'Apple',chrome:'Chrome',llm:'LLM'})[localEngine()] || '번역';}
  function engineSpec(kind,overrides={}){
    return {kind,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),
      engine:kind==='stt'?config.sttEngine:localEngine(),
      language:resolveSTTLanguage({mode:normalizeGlassesMode(config.glassesMode),engine:overrides.engine || config.sttEngine,sourceLang:config.translateSourceLang,targetLang:config.translateTargetLang}),
      source:resolveTranslateSourceLang({sourceLang:config.translateSourceLang,targetLang:config.translateTargetLang}),target:config.translateTargetLang || 'ko',...overrides};
  }
  function canonicalSpec(spec){
    if(spec.kind==='stt'){const {kind,server,headers,engine,language}=spec;return {kind,server,headers,engine,language:engine==='whisper'?'ko':language};}
    const {kind,server,headers,engine,source,target}=spec;return {kind,server,headers,engine,source,target};
  }
  async function runEnginePreparation(spec,signal){
    const {kind,server,headers,engine}=spec;
    if(kind==='stt' && engine==='whisper'){
      const response=await gatewayFetch(`${server}/api/stt/status`,{headers,signal});
      const status=await response.json();
      if(!response.ok || !status.active)throw Error(status.message || '서버에서 STT 모델과 런타임을 설치하세요.');
      const warmup=await gatewayFetch(`${server}/api/stt/warmup`,{method:'POST',headers,signal});
      if(!warmup.ok)throw Error(`음성 엔진 준비 실패: HTTP ${warmup.status}`);
      return;
    }
    const path=kind==='stt'?`/api/stt/${engine==='apple'?'apple':'chrome'}/prepare`:`/api/translate/${engine}/prepare`;
    const body=kind==='stt'?{engine,language:spec.language}:{source:spec.source,target:spec.target};
    const response=await gatewayFetch(server+path,{method:'POST',headers,signal,body:JSON.stringify(body)});
    const result=await response.json();
    if(!response.ok || result.status!=='ready')throw Error(result.error || `HTTP ${response.status}`);
  }
  function preparationStatus(kind,state,spec,error){
    const stt=kind==='stt',el=stt?els.sttEngineStatus:els.chromeTranslationStatus;
    const label=spec.engine==='apple'?'Apple':spec.engine==='whisper'?'Whisper':'Chrome';
    const detail=stt?spec.language:`${spec.source} → ${spec.target}`;
    el.textContent=state==='preparing'?`${label} ${stt?'STT':'번역'} 자동 준비 중 · ${detail}${spec.engine==='apple'?' · 처음 사용할 때 서버 컴퓨터에서 언어팩 설치를 확인하세요':''}`:
      state==='ready'?`${label} ${stt?'STT':'로컬 번역'} 준비 완료 · ${detail}`:
      `${stt?'STT':'번역'} 준비 실패 · ${error?.name==='AbortError'?'준비 시간 초과':error?.message}${stt?'':' · LLM 번역 사용'}`;
    (stt?els.btnPrepareStt:els.btnPrepareChrome).disabled=state==='preparing';
  }
  const sttPreparation=new EnginePreparation(runEnginePreparation,(...args)=>preparationStatus('stt',...args));
  const translationPreparation=new EnginePreparation(runEnginePreparation,(...args)=>preparationStatus('translation',...args));
  function prepareSelectedStt(signal,force=false,engine=config.sttEngine){return waitForPreparation(sttPreparation.ensure(canonicalSpec(engineSpec('stt',{engine})),{force}),signal);}
  function prepareSelectedTranslation(signal,force=false){
    if(localEngine()==='llm'){translationPreparation.clear();els.btnPrepareChrome.disabled=false;els.chromeTranslationStatus.textContent='LLM 번역은 별도 언어팩 준비가 필요 없습니다';return Promise.resolve();}
    return waitForPreparation(translationPreparation.ensure(canonicalSpec(engineSpec('translation')),{force}),signal);
  }
  function autoPrepareEngines(){
    if(!config.serverUrl)return;
    prepareSelectedStt().catch(()=>{});
    prepareSelectedTranslation().catch(()=>{});
  }
  async function translateLocal(request,signal,engine=localEngine()){
    const spec=canonicalSpec(engineSpec('translation',{engine,source:request.source,target:request.target}));
    try{
      await waitForPreparation(translationPreparation.ensure(spec),signal);
      const started=Date.now();
      let result=await localTranslate(gatewayFetch,spec.server,spec.headers,request,signal,8000,engine);
      if(!result.ok && result.needsPrepare){
        // Server restart or removed language assets invalidate client readiness.
        await waitForPreparation(translationPreparation.ensure(spec,{force:true}),signal);
        result=await localTranslate(gatewayFetch,spec.server,spec.headers,request,signal,8000,engine);
      }
      if(request.traceId)translationMetrics.record(request.traceId,{translation_http_ms:Date.now()-started,translation_server_ms:result.timings?.server_ms});
      return result;
    }catch(error){if(signal?.aborted)throw error;return {ok:false,reason:error.message};}
  }
  const translationHistory=new TranslationHistory();
  const translationDirection=new TranslationDirection();
  const translationSentence=new TranslationSentence();
  const stableTranslation=new StableTranslation((job,signal)=>translateLocal({text:job.prompt,source:job.source,target:job.target,traceId:job.id},signal,job.engine));
  const llmStableTranslation=new StableTranslation(async(job,signal)=>{
    const key=JSON.stringify([chatScope(),resolveRoleModel('translation',config),job.source,job.target,job.prompt]);
    if(llmTranslationCache.has(key))return llmTranslationCache.get(key);
    const result=await llmTranslation({fetch:gatewayFetch,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),model:resolveRoleModel('translation',config),job,signal,onPartial:job.onChunk});
    if(result.ok && !signal.aborted){llmTranslationCache.set(key,result);while(llmTranslationCache.size>8)llmTranslationCache.delete(llmTranslationCache.keys().next().value);}
    return result;
  });
  const audioUpload=new AudioUpload();
  let translationPaint=null;
  const translationMetrics=new TranslationMetrics({onUpdate:entry=>{
    const el=document.getElementById('translation-latency');if(!el)return;
    const labels={stt_first_result_ms:'STT 인식 도착',stt_http_ms:'STT 왕복',stt_server_ms:'STT 서버',stt_lock_ms:'STT 잠금 대기',stt_engine_ms:'STT 엔진',save_ms:'저장',translation_queue_ms:'번역 대기',translation_http_ms:'번역 왕복',translation_server_ms:'번역 서버',first_text_ms:'첫 번역',sdk_ms:'안경 최근 성공'};
    el.textContent=Object.entries(labels).filter(([key])=>entry[key]!==undefined).map(([key,label])=>`${label} ${entry[key]}ms`).join(' · ')+(entry.upload_bytes!==undefined?` · 오디오 ${entry.upload_bytes}/${entry.audio_bytes ?? '?'}B`:'')+(entry.sdk_failed?' · 안경 전송 실패':'');
  }});
  const llmTranslationCache=new Map();
  let previewRetryTimer=null;
  const liveTranslations=new LiveTranslationQueue({
    translate:async(job,signal,onPartial)=>{
      if(job.engine!=='llm')return stableTranslation.run(job,signal);
      const started=Date.now();
      const result=await llmStableTranslation.run({...job,onPartial,cacheScope:JSON.stringify([chatScope(),resolveRoleModel('translation',config)])},signal);
      translationMetrics.record(job.id,{translation_http_ms:Date.now()-started});return result;
    },
    onToken:(token,info)=>{
      translationToken=token;
      const boundary=!info || info.final || info.pairChanged;
      if(boundary)translationReplyVersion++;
      const cancelReply=boundary || localEngine()==='llm';
      if(activeAIJob?.translationToken && (!activeAIJob.replyOnly || cancelReply))aiAbortController?.abort();
      pendingAI.splice(0,pendingAI.length,...pendingAI.filter(job=>!job.translationToken || job.replyOnly && !cancelReply));
    },
    onText:(job,text)=>{
      if(normalizeGlassesMode(config.glassesMode)!=='translate')return;
      translationPaint={id:job.id,at:Date.now(),acked:false,content:''};
      liveTranslationDraft=job.draft;
      translationFailure='';translationDisplay={...translationDisplay,translation:text,pronunciation:''};answerText=text;
      translationHistory.update(job.historyId || job.id,text,{final:!job.draft,revision:job.id});
      els.chatOutput.textContent=translationPhoneText(translationDisplay);
      els.chromeTranslationStatus.textContent=job.draft?`${localLabel()} 실시간 번역 · 인식에 따라 수정됩니다`:`${localLabel()} 확정 번역 · 답변 예시 생성 중`;renderLiveHUD();
    },
    onFinal:(job,result)=>{
      liveTranslationDraft=false;
      if(normalizeGlassesMode(config.glassesMode)!=='translate')return;
      if(!result.ok && job.engine==='llm'){translationFailure='번역 오류';els.chromeTranslationStatus.textContent=`LLM 번역 실패 · 이전 번역 유지 (${result.reason})`;renderLiveHUD();return;}
      enqueueAI(job.id+'-translation-'+job.token,job.prompt,job.context,'translate',job.target,{replyOnly:result.ok,replyVersion:translationReplyVersion,translatedBy:job.engine==='llm'?'LLM':localLabel(),chromeTranslationText:result.ok?result.text:'',chromeAttempted:true,translationSource:job.source,translationToken:job.token,translationHistoryId:job.historyId || job.id,translationRevisionId:job.id});
    },
    onFailure:(job,reason)=>{
      if(job.engine==='llm')els.chromeTranslationStatus.textContent=`LLM 번역 대기 · ${reason}`;
      else els.chromeTranslationStatus.textContent=job.draft?`실시간 번역 대기 · ${reason}`:`LLM 번역 사용 · ${reason}`;
    },
    onMeasure:(job,stage,ms)=>translationMetrics.record(job.id,{[stage]:ms}),
    onBusy:busy=>{translating=busy || liveTranslationDraft;renderLiveHUD();}
  });

  const hudIcons=new HudIconStore(),lastHudIconKeys=new Map();
  let hudImagesDisabled=false,lastFallbackHeader='';
  function hudMicrophoneState(){
    const active=isRecording || capturePreparing || micMayBeOpen || pendingMicOpen || micCloseUncertain;
    const source=active?(activeAudioInputSource===AudioInputSource.Phone?'phone':'glasses'):config.audioInputSource;
    const on=isRecording && !capturePreparing && !pendingMicOpen && !micCloseUncertain;
    return {source,on,key:microphoneIcon(source,on)};
  }
  function syncAIHeaderControls(resetPosition=false){
    config.aiHeaderLayout=normalizeAIHeaderLayout(config.aiHeaderLayout);config.aiIconPosition=normalizeAIIconPosition(config.aiIconPosition);
    if(resetPosition)aiIconPositionApplied=config.aiIconPosition;
    els.aiHeaderLayout.value=config.aiHeaderLayout;els.aiIconPosition.value=String(config.aiIconPosition);
    els.aiIconPosition.disabled=config.aiHeaderLayout!=='status-icon';
    const label=aiIconPositionLabel(config.aiIconPosition);els.aiIconPositionValue.textContent=label;
    els.aiIconPosition.setAttribute?.('aria-valuetext',label);
  }
  function updateAIIconPosition(commit=false){
    settingsEdited=true;aiIconPositionApplied ??= normalizeAIIconPosition(config.aiIconPosition);
    config.aiIconPosition=normalizeAIIconPosition(els.aiIconPosition.value);syncAIHeaderControls();queueHudIcons();
    clearTimeout(aiIconPositionTimer);
    const apply=()=>{aiIconPositionTimer=null;aiIconPositionApplied=config.aiIconPosition;renderLiveHUD();persistSettings().catch(console.warn);};
    if(commit)apply();else aiIconPositionTimer=setTimeout(apply,120);
  }
  function iconOnlyHeader(){return normalizeGlassesMode(config.glassesMode)==='ai' && normalizeAIHeaderLayout(config.aiHeaderLayout)==='status-icon';}
  function compactAIStatus(){
    const match=String(aiPhaseTitle).match(/\((\d+)%\)/);
    return aiStatusIcon({error:!!captureFailure || micCloseUncertain || !!aiStatusError || hudImagesDisabled || !hudTransport.connected,
      preparing:capturePreparing || !!pendingMicOpen || !!micStopPromise,listening:captureListening(),audio:receivedCaptureAudio,
      transcribing:draining || pendingClips.length>0 || previewBusy,
      working:processingAI || isThinking || !!hintGate.inflight,phase:aiPhaseTitle+' '+aiPhaseDetail,progress:match?Number(match[1]):null});
  }
  function hudCaptureIcon(){return captureStatusIcon({listening:captureListening() && receivedCaptureAudio,preparing:capturePreparing || !!pendingMicOpen || !!micStopPromise,transcribing:draining || pendingClips.length>0,error:!!captureFailure || micCloseUncertain});}
  function visibleHeaderMessage(){return hudImagesDisabled?currentHUDHeader(true):iconHeaderMessage(currentHUDHeader(true));}
  function currentHeaderLayout(translate=normalizeGlassesMode(config.glassesMode)==='translate'){
    const layout=headerLayout(translate,{iconOnly:!translate && iconOnlyHeader(),iconPosition:aiIconPositionApplied ?? config.aiIconPosition,trigger:triggerLabels()[0] || '',source:translationDirection.pair.split('>')[0] || config.translateSourceLang || 'auto',target:translationDirection.pair.split('>')[1] || config.translateTargetLang || 'ko'});
    // Recreate the header once when capture becomes ready, even on firmware
    // that ignores blank partial updates. PCM/STT capture keeps running.
    return {...layout,key:JSON.stringify([layout.key,!layout.iconOnly && captureListening() && receivedCaptureAudio,normalizeHUDDimming(config),iconOnlyHeader() && hudImagesDisabled])};
  }
  function nativeHUDNotice(){
    if(iconOnlyHeader())return hudImagesDisabled?'!':' ';
    const {source,on}=hudMicrophoneState(),translate=normalizeGlassesMode(config.glassesMode)==='translate';
    const fallback=hudImagesDisabled?`[${translate?'번역':'AI'} ${source==='phone'?'폰':'안경'} ${on?'녹음':'꺼짐'}] `:'';
    // A space explicitly clears the previous firmware text. An empty string
    // can be interpreted by the host as an omitted update field.
    return hudNoticeText(fallback+visibleHeaderMessage(),currentHeaderLayout(translate).width) || ' ';
  }
  function queueHudIcons(force=false){
    const mic=hudMicrophoneState(),translate=normalizeGlassesMode(config.glassesMode)==='translate';
    if(els.hudMicIcon){els.hudMicIcon.src=hudIconPath(mic.key);els.hudMicIcon.alt=`${mic.source==='phone'?'휴대폰':'안경'} 마이크 ${mic.on?'켜짐':'꺼짐'}`;}
    const layout=currentHeaderLayout(translate);
    els.hudHeader?.classList.toggle('ai-icon-header',!!layout.iconOnly);
    const ratio=normalizeAIIconPosition(config.aiIconPosition)/552;
    els.hudHeader?.style.setProperty('--ai-icon-percent',ratio*100+'%');
    els.hudHeader?.style.setProperty('--ai-icon-compensation',ratio*24+'px');
    els.hudHeader?.setAttribute?.('aria-label',layout.iconOnly?compactAIStatus().label:'현재 시각과 안경 배터리');
    els.hudModeLabel.textContent=layout.prefix.content;
    els.hudTriggerKeyword.textContent=layout.wake?.content || '';els.hudTriggerKeyword.hidden=false;
    const brightness=wakeBrightness();els.hudTriggerKeyword.style.opacity=String(hudLuminance(brightness));
    els.hudHeaderMessage.textContent=visibleHeaderMessage();
    els.hudHeaderMessage.style.opacity='1';
    els.hudHeaderMessage.style.color=hudPreviewColor(hudTextBrightness(3));
    if(els.hudMicIcon)els.hudMicIcon.style.opacity=String(hudLuminance(hudTextBrightness(5)));
    const compact=layout.iconOnly?compactAIStatus():null;
    const state=compact?.key || hudCaptureIcon(),modeIcon=translate?'mode-translate':'mode-ai';
    for(const [element,key,label] of [[els.hudModeIcon,modeIcon,translate?'번역':'AI'],[els.hudStateIcon,state,compact?.label || ({'status-idle':'대기중','status-listening':'듣는중','status-transcribing':'전사 처리중'})[state]]])if(element){element.src=hudIconPath(key);element.alt=label;element.style.opacity=String(hudLuminance(hudTextBrightness(6)));}
    if(!displayReady || headPaused)return;
    if(!layout.iconOnly && (force || lastWakeBrightness!==brightness)){lastWakeBrightness=brightness;pendingHUD.set(8,{containerID:8,containerName:'wake-word',textColor:brightness});}
    if(typeof bridge.updateImageRawData!=='function')hudImagesDisabled=true;
    if(hudImagesDisabled){
      const content=nativeHUDNotice();
      if(content!==lastFallbackHeader){lastFallbackHeader=content;pendingHUD.set(3,{containerID:3,containerName:'notice',content});}
      if(pendingHUD.size)flushHUD();
      return;
    }
    const icons=layout.iconOnly?[[11,'capture-state',state]]:[[5,'mic-state',mic.key],[6,'mode-state',modeIcon],[11,'capture-state',state]];
    for(const [id,name,key] of icons)if(force || lastHudIconKeys.get(id)!==`${key}:${hudTextBrightness(id)}`){
      lastHudIconKeys.set(id,`${key}:${hudTextBrightness(id)}`);pendingHUD.set(id,{containerID:id,containerName:name,icon:key});
    }
    if(pendingHUD.size)flushHUD();
  }

  // --- Lifecycle & Initialization ---
  async function init() {
    loadSettings();
    bindEvents();
    initEvenRealitiesBridge();
    if (config.serverUrl) {
      checkLoginStatus();
    }
  }

  // --- Settings Persistence ---
  function loadSettings(nativeValue) {
    try {
      const saved = nativeValue === undefined ? localStorage.getItem(STORAGE_KEY) : nativeValue;
      if (saved) {
        const incoming=JSON.parse(saved);
        config = { ...config, ...incoming };
        config.roleModels=normalizeRoleModels(incoming.roleModels);
        if(!Object.prototype.hasOwnProperty.call(incoming.roleModels || {},'hintDecision'))config.roleModels.hintDecision=JEV_ONLY;
        config.audioInputSource=incoming.audioInputSource==='phone'?'phone':'glasses';
        config.glassesInput=normalizeGlassesInput(incoming.glassesInput);
        if(!Object.prototype.hasOwnProperty.call(incoming,'translationEngine'))config.translationEngine=incoming.chromeTranslation===false?'llm':'chrome';
      }
      if (/^https?:\/\/(localhost|127\.0\.0\.1)(:|\/|$)/.test(config.serverUrl) &&
          !['localhost', '127.0.0.1'].includes(window.location.hostname)) {
        config.serverUrl = DKST_DEFAULT_SERVER_URL || window.location.origin;
      }
      if (config.connectionMode === 'websocket') config.connectionMode = 'sse';
    } catch (e) {
      console.warn('Failed to load settings:', e);
    }

    if (els.serverUrl) els.serverUrl.value = config.serverUrl || '';
    if (els.authUsername) els.authUsername.value = config.username || '';
    if (els.apiKey) els.apiKey.value = config.apiKey || '';
    if (els.connectionMode) els.connectionMode.value = config.connectionMode || 'sse';
    config.aiHintMode=true;config.voiceMode='continuous';
    config.hintContextTokens=normalizeHintBudget(config.hintContextTokens);els.hintContextTokens.value=String(config.hintContextTokens);
    els.hintSensitivity.value=config.hintSensitivity || 'balanced';
    if(els.hintWebSearch)els.hintWebSearch.checked=config.hintWebSearch!==false;
    syncAIHeaderControls(true);
    syncHUDDimmingControls();
    config.hideTranscript=config.hideTranscript===true;els.hideTranscript.checked=config.hideTranscript;
    config.triggerTranscriptOnly=config.triggerTranscriptOnly===true;els.triggerTranscriptOnly.checked=config.triggerTranscriptOnly;
    config.hintCardCount=hintCardCount(config.hintCardCount);els.hintCardCount.value=String(config.hintCardCount);
    config.hintAutoClose=config.hintAutoClose===true;els.hintAutoClose.checked=config.hintAutoClose;
    config.hintSecondsPerLine=secondsPerHintLine(config.hintSecondsPerLine);els.hintSecondsPerLine.value=String(config.hintSecondsPerLine);els.hintSecondsPerLine.disabled=!config.hintAutoClose;
    if (els.optConciseMode) els.optConciseMode.checked = config.conciseMode !== false;
    if (els.optEnableTts) els.optEnableTts.checked = config.enableTTS !== false;
    config.glassesMode = normalizeGlassesMode(config.glassesMode);
    config.translateSourceLang = config.translateSourceLang || 'auto';
    config.translateTargetLang = config.translateTargetLang || 'ko';
    if (els.glassesMode) els.glassesMode.value = config.glassesMode;
    if (els.translateSourceLang) els.translateSourceLang.value = config.translateSourceLang;
    if (els.translateTargetLang) els.translateTargetLang.value = config.translateTargetLang;
    els.voiceMode.value = config.voiceMode;
    els.triggerWord.value = config.triggerWord;
    els.speechFilter.value = config.speechFilter;
    config.roleModels=normalizeRoleModels(config.roleModels);renderRoleModelSelectors();
    config.audioInputSource=config.audioInputSource==='phone'?'phone':'glasses';
    els.audioInputSource.value=config.audioInputSource;
    config.glassesInput=normalizeGlassesInput(config.glassesInput);syncGlassesInputUI();
    els.sttEngine.value=config.sttEngine || 'whisper';els.sttFallback.checked=config.sttFallback!==false;
    els.speakerScope.value = config.speakerScope;
    if(els.translateExcludeSelf)els.translateExcludeSelf.checked=config.translateExcludeSelf!==false;
    config.translationEngine=['llm','chrome','apple'].includes(config.translationEngine)?config.translationEngine:(config.chromeTranslation===false?'llm':'chrome');
    config.chromeTranslation=config.translationEngine!=='llm';
    if(els.translationEngine)els.translationEngine.value=config.translationEngine;
    if(els.chromeTranslation)els.chromeTranslation.checked=config.chromeTranslation;
    const now = new Date();
    els.transcriptDate.value = `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,'0')}-${String(now.getDate()).padStart(2,'0')}`;
    config.headGestureEnabled=config.headGestureEnabled===true;els.headGestureEnabled.checked=config.headGestureEnabled;
    headDetector.configure(config.headGestureCalibration);updateHeadControls();
    if(displayReady)syncHeadIMU().catch(headGestureError);
    syncModeUI();
  }

  function isContinuousCaptureMode(mode = config.glassesMode) {
    const normalized = normalizeGlassesMode(mode);
    if (normalized === 'ai' || normalized === 'translate') return true;
    return config.voiceMode === 'continuous';
  }

  function abortPendingPreview() {
    clearTimeout(previewRetryTimer);previewRetryTimer=null;
    pendingPreview = null;
    if (previewAbortController) {
      previewAbortController.abort();
      previewAbortController = null;
    }
  }

  function stopChromeSegment(id){
    if(activeSttEngine==='whisper' || !id)return;
    gatewayFetch(`${config.serverUrl}/api/stt/${activeSttEngine==='apple'?'apple':'chrome'}/stop`,{method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify({sessionId,segmentId:id})}).catch(()=>{});
  }
  function resetQueuesForModeSwitch(nextMode, preserveAudio=false, preserveDisplay=false) {
    previewIntent.reset();previewWakeId='';previewWakePrompt='';audioUpload.reset();llmTranslationCache.clear();clearTimeout(hintDrainTimer);hintDrainTimer=null;
    hintLifetime.clear();answerAutoCloseEligible=false;voiceWakeJobs.clear();
    chatEpoch++;translationFailure='';translationDirection.reset();translationSentence.reset();if(!preserveDisplay)translationHistory.reset();stableTranslation.reset();llmStableTranslation.reset();
    stopChromeSegment(activeUtterance?.id);
    liveTranslationDraft=false;liveTranslations.reset();
    if(!preserveDisplay)translationDisplay={translation:'',positive:'',negative:''};
    pendingTrigger=false;
    hintGate.reset({forgetHistory:true});
    abortPendingPreview();
    activeUtterance = null;
    draftUtteranceId = '';
    captionStable = '';
    captionDraft = '';
    if(!preserveAudio)pendingClips.length = 0;
    if (clipAbortController && !preserveAudio) {
      clipAbortController.abort();
      clipAbortController = null;
    }
    pendingAI.length = 0;
    if (aiAbortController) {
      aiAbortedByModeSwitch = true;
      aiAbortController.abort();
      aiAbortController = null;
    }
    clearTimeout(answerTimer);
    answerHoldUntil = 0;
    stopAIAutoScroll();
    stopThinkingAnimation();
    if(!preserveDisplay){clearHintStack();answerText = '';answerTitle='';answerVoicePrompt='';aiScrollOffset = 0;}
    aiPhaseTitle = '';
    aiPhaseDetail = '';
    aiReasoningPreview = '';
    if (isRecording) {
      audioBuffer = [];
      recordedBytes = 0;
      recordingStarted = Date.now();
      segmenter = new SpeechSegmenter(inputSegmentOptions(activeSttEngine,nextMode,activeAudioInputSource,activeGlassesInput));
      clearTimeout(manualTimer);
      manualTimer = null;
      if (!isContinuousCaptureMode(nextMode)) {
        manualTimer = setTimeout(() => stopVoiceRecording().catch(console.warn), 25000);
      }
    }
  }

  function syncModeUI() {
    const mode = normalizeGlassesMode(config.glassesMode);
    const sourceLang = config.translateSourceLang || 'auto';
    const targetLang = config.translateTargetLang || 'ko';
    if (els.glassesMode && els.glassesMode.value !== mode) els.glassesMode.value = mode;
    if (els.translateSourceLang && els.translateSourceLang.value !== sourceLang) els.translateSourceLang.value = sourceLang;
    if (els.translateTargetLang && els.translateTargetLang.value !== targetLang) els.translateTargetLang.value = targetLang;
    if (els.translateLangRow) els.translateLangRow.hidden = false;
    for (const [btn, key] of [
      [els.modeBtnAi, 'ai'],
      [els.modeBtnTranslate, 'translate'],
    ]) {
      if (!btn) continue;
      const active = mode === key;
      btn.classList?.toggle?.('active', active);
      btn.setAttribute?.('aria-selected', active ? 'true' : 'false');
    }
    if (els.modeDescription) {
      if (mode === 'ai') {
        els.modeDescription.textContent = '대화 중 필요한 정보를 표시합니다. 호출어로 AI에게 질문하세요.';
      } else if (mode === 'translate') {
        els.modeDescription.textContent = `번역 모드 (→${targetLanguageLabel(targetLang, true)}) · 실시간 번역 + 긍정/부정 답변 표시`;
      }
    }
    renderLiveHUD();
  }

  function setGlassesMode(nextMode, persist = false) {
    if(persist)return applyLiveSettings({glassesMode:normalizeGlassesMode(nextMode)});
    const prevMode = normalizeGlassesMode(config.glassesMode);
    const mode = normalizeGlassesMode(nextMode);
    const modeChanged = prevMode !== mode;
    config.glassesMode = mode;
    pendingTrigger = false;
    captureFailure = '';
    if (modeChanged) {
      resetQueuesForModeSwitch(mode);
    }
    syncModeUI();
    setHUDNotice();
    if (persist) {
      autoPrepareEngines();
      persistSettings().catch(console.warn);
    }
  }

  async function saveSettings(showMessage = true) {
    if(inputSwitchPromise)await inputSwitchPromise;
    settingsEdited = true;
    config.serverUrl = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    config.username = (els.authUsername.value || '').trim();
    config.apiKey = (els.apiKey.value || '').trim();
    config.model = els.modelSelect.value || '';
    config.roleModels=Object.fromEntries(ROLE_MODEL_KEYS.map(role=>[role,els.roleModels[role].value || '']));
    config.connectionMode = els.connectionMode.value;
    config.conciseMode = els.optConciseMode.checked;
    config.aiHintMode=true;
    config.hintContextTokens=normalizeHintBudget(els.hintContextTokens.value);
    config.hintSensitivity=els.hintSensitivity.value || 'balanced';
    config.hintWebSearch=els.hintWebSearch?.checked!==false;
    config.aiHeaderLayout=normalizeAIHeaderLayout(els.aiHeaderLayout.value);config.aiIconPosition=normalizeAIIconPosition(els.aiIconPosition.value);
    config.hideTranscript=els.hideTranscript.checked===true;
    config.triggerTranscriptOnly=els.triggerTranscriptOnly.checked===true;
    config.hintCardCount=hintCardCount(els.hintCardCount.value);config.hintAutoClose=els.hintAutoClose.checked===true;config.hintSecondsPerLine=secondsPerHintLine(els.hintSecondsPerLine.value);
    hintGate.reset();
    config.enableTTS = els.optEnableTts.checked;
    if (els.glassesMode?.value) config.glassesMode = normalizeGlassesMode(els.glassesMode.value);
    if (els.translateSourceLang?.value) config.translateSourceLang = els.translateSourceLang.value;
    if (els.translateTargetLang?.value) config.translateTargetLang = els.translateTargetLang.value;
    config.voiceMode = 'continuous';
    config.triggerWord = els.triggerWord.value.trim();
    config.speechFilter = els.speechFilter.value === 'standard' ? 'standard' : 'strict';
    config.audioInputSource=els.audioInputSource.value==='phone'?'phone':'glasses';
    config.glassesInput=readGlassesInputControls();
    config.sttEngine=['chrome-online','apple'].includes(els.sttEngine.value)?els.sttEngine.value:'whisper';config.sttFallback=els.sttFallback.checked!==false;
    config.speakerScope = els.speakerScope.value === 'self' ? 'self' : 'conversation';
    config.translateExcludeSelf=els.translateExcludeSelf?.checked!==false;
    config.chromeTranslation=els.chromeTranslation?.checked!==false;
    syncModeUI();

    try {
      await persistSettings();
      prewarmConfiguredModels();
      autoPrepareEngines();
      if (showMessage !== false) alert('설정이 저장되었습니다.');
    } catch (err) {
      if (showMessage !== false) alert(`설정 저장 실패: ${err.message}`);
      else throw err;
    }
  }

  async function storageCall(promise) {
    let timer;
    try {
      return await Promise.race([promise, new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error('Even 앱 저장소 응답 시간 초과')), 8000);
      })]);
    } finally { clearTimeout(timer); }
  }

  async function persistSettings() {
    delete config.password;
    const value = JSON.stringify(config);
    try { localStorage.setItem(STORAGE_KEY, value); } catch (_) {}
    if (bridge && typeof bridge.setLocalStorage === 'function') {
      const success = await storageCall(bridge.setLocalStorage(STORAGE_KEY, value));
      if (!success) throw new Error('Even 앱 저장소가 저장을 거절했습니다.');
      pendingNativeSave = false;
    } else {
      pendingNativeSave = true;
    }
  }

  async function restoreNativeSettings() {
    if (typeof bridge.getLocalStorage !== 'function') return;
    try {
      if (pendingNativeSave) { await persistSettings(); await checkLoginStatus(); return; }
      const saved = await storageCall(bridge.getLocalStorage(STORAGE_KEY));
      if (saved && !settingsEdited) loadSettings(saved);
      else if (!saved && !settingsEdited) await persistSettings();
      await checkLoginStatus();
    } catch (err) {
      console.warn('Native settings restore failed:', err);
      setVoiceHint('설정 복원 실패 · 주소와 로그인 상태를 확인하세요');
    }
  }

  function headGestureError(error){els.headStatus.textContent=`헤드 제스처 오류 · ${error.message}`;}
  function updateHeadControls(){
    els.headCalibrate.disabled=!config.headGestureEnabled || !headImuActive || !!headCapture;
    els.headCalibrate.textContent=headCalibrationPhase==='readyRaised'?'올린 자세 저장':'정면 자세 저장';
    els.headResume.hidden=!headPaused;
    els.voiceBtn?.setAttribute?.('aria-label',headPaused?'화면/STT 재개':capturePreparing?'녹음 준비 취소':isRecording?'녹음 정지':'녹음 시작');
  }
  function cancelHeadCalibration(){clearTimeout(headCalibrationTimer);headCalibrationTimer=null;headCapture=null;headNeutral=null;headCalibrationPhase='idle';headDetector.reset();updateHeadControls();}
  async function syncHeadIMU(){
    headImuWanted=displayReady && config.headGestureEnabled;
    if(headImuPromise)return headImuPromise;
    const work=(async()=>{
      await Promise.resolve();
      if(typeof bridge?.imuControl!=='function'){if(headImuWanted)throw Error('현재 Even 앱이 IMU를 지원하지 않습니다');return;}
      while(headImuActive!==headImuWanted){
        const wanted=headImuWanted;
        if(!await bridge.imuControl(wanted,100))throw Error('안경 IMU 보고 설정을 확인하지 못했습니다');
        headImuActive=wanted;
      }
      updateHeadControls();
      if(headImuActive && headCalibrationPhase==='idle')els.headStatus.textContent=headPaused?'화면/STT 일시정지 · 같은 제스처로 재개':validHeadCalibration(config.headGestureCalibration)?'준비됨 · 고개를 올렸다 정면으로 돌아오면 토글':'정면과 고개를 올린 자세를 차례로 저장하세요';
    })();headImuPromise=work;
    try{await work;}finally{if(headImuPromise===work)headImuPromise=null;updateHeadControls();}
  }
  async function setHeadGestureEnabled(){
    settingsEdited=true;config.headGestureEnabled=els.headGestureEnabled.checked===true;cancelHeadCalibration();
    headDetector.configure(config.headGestureCalibration);
    try{await syncHeadIMU();await persistSettings();if(!config.headGestureEnabled){els.headStatus.textContent='헤드 제스처 꺼짐';if(headTransition)await headTransition.catch(()=>{});if(!config.headGestureEnabled && headPaused)await resumeHeadSession();}}
    catch(error){headGestureError(error);}
  }
  async function startHeadCalibration(){
    try{
      if(!config.headGestureEnabled)return;
      await syncHeadIMU();if(!headImuActive)throw Error('안경 IMU 연결을 확인하세요');
      const raised=headCalibrationPhase==='readyRaised';
      headCapture={raised,values:[],started:null};headCalibrationPhase=raised?'raised':'neutral';
      els.headStatus.textContent=raised?'고개를 올린 자세를 잠시 유지하세요':'정면을 보고 잠시 자세를 유지하세요';updateHeadControls();
      clearTimeout(headCalibrationTimer);headCalibrationTimer=setTimeout(()=>{cancelHeadCalibration();els.headStatus.textContent='보정하지 못했습니다. 안경 연결을 확인하고 다시 시도하세요';},10000);
    }catch(error){headGestureError(error);}
  }
  function handleHeadIMU(data,at=Date.now()){
    if(!config.headGestureEnabled || !headImuActive || !displayReady)return;
    const vector=imuVector(data);if(!vector)return;
    if(headCapture){
      const sample=headCapture;if(sample.started===null)sample.started=at;sample.values.push(vector);if(sample.values.length>256)sample.values.shift();
      if(sample.values.length<8 || at-sample.started<600)return;
      clearTimeout(headCalibrationTimer);headCalibrationTimer=null;headCapture=null;
      try{
        const pose=poseSample(sample.values);
        if(!sample.raised){headNeutral=pose;headCalibrationPhase='readyRaised';els.headStatus.textContent='고개를 위로 올려 유지한 뒤 ‘올린 자세 저장’을 누르세요';}
        else{config.headGestureCalibration=makeHeadCalibration(headNeutral,pose);headDetector.configure(config.headGestureCalibration);headCalibrationPhase='idle';headNeutral=null;settingsEdited=true;persistSettings().catch(console.warn);els.headStatus.textContent='보정 완료 · 정면으로 돌아온 뒤 새 제스처부터 동작합니다';}
      }catch(error){cancelHeadCalibration();headGestureError(error);}
      updateHeadControls();return;
    }
    if(headCalibrationPhase!=='idle' || headTransition)return;
    if(headDetector.consume(data,at))toggleHeadSession();
  }
  async function toggleHeadSession(){if(headTransition)return headTransition.catch(()=>{});return headPaused?resumeHeadSession():pauseHeadSession();}
  async function pauseHeadSession(){
    if(headTransition)return headTransition.catch(()=>{});
    headResumeWanted=isRecording || capturePreparing || resumeAfterSettings;
    headPaused=true;settingsPicker=null;recognitionEpoch++;hintGate.reset();hintLifetime.clear();
    aiAbortedByModeSwitch=true;aiAbortController?.abort();pendingAI.length=0;voiceWakeJobs.clear();pendingTrigger=false;
    updateHeadControls();renderLiveHUD();flushHUD();
    const work=(async()=>{
      await Promise.resolve();const starting=captureTask;
      await stopVoiceRecording(true);if(starting)await starting.catch(()=>{});
      if(micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)throw Error('마이크 종료 확인이 필요합니다. 재개 버튼으로 다시 확인하세요');
      resetQueuesForModeSwitch(config.glassesMode,true);els.headStatus.textContent='화면/STT 일시정지 · 같은 제스처로 재개';renderLiveHUD();
    })();headTransition=work;
    try{await work;}catch(error){headGestureError(error);}finally{if(headTransition===work)headTransition=null;updateHeadControls();}
  }
  async function resumeHeadSession(){
    if(headTransition)return headTransition.catch(()=>{});if(!headPaused)return;
    const work=(async()=>{
      await Promise.resolve();
      if(inputSwitchPromise)await inputSwitchPromise;
      if(micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)await stopVoiceRecording(false);
      if(micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)throw Error('기존 마이크 종료를 확인하지 못했습니다');
      recognitionEpoch++;headPaused=false;updateHeadControls();renderLiveHUD();flushHUD();
      const resumeRecording=headResumeWanted;
      if(resumeRecording)await toggleVoiceRecording({fromHeadResume:true});
      headResumeWanted=false;els.headStatus.textContent=resumeRecording && !isRecording?'화면 재개 · 마이크 시작 실패, 권한과 연결 상태를 확인하세요':config.headGestureEnabled?'재개됨 · 같은 제스처로 다시 일시정지':'헤드 제스처 꺼짐';
    })();headTransition=work;
    try{await work;}catch(error){headGestureError(error);}finally{if(headTransition===work)headTransition=null;updateHeadControls();}
  }

  // --- Even Realities SDK Bridge ---
  function nativeAITitles(translate,body){
    if(translate || !body.show || (!body.cards && !body.titleHeight))return [];
    if(body.cards)return body.cards.map((card,i)=>({containerID:i?10:9,containerName:i?'hint-title-older':'ai-title',xPosition:0,yPosition:card.titleY,width:576,height:card.titleHeight,borderWidth:1,borderColor:4,borderRadius:0,paddingLength:body.cards.length>1?0:2,content:body.titles[i],isEventCapture:0}));
    return [{containerID:9,containerName:'ai-title',xPosition:0,yPosition:27,width:576,height:body.titleHeight,borderWidth:1,borderColor:4,borderRadius:0,paddingLength:2,content:lastAITitleContent || aiTitleLine(body.title,hintLifetime.remaining),isEventCapture:0}];
  }
  function hudPage(translate,layout=currentHeaderLayout(translate),body=currentBodyLayout()){const page={
        containerTotalNum: 0,
        imageObject:[{containerID:5,containerName:'mic-state',xPosition:layout.micIconX,yPosition:1,width:24,height:24},{containerID:6,containerName:'mode-state',xPosition:layout.modeIconX,yPosition:1,width:24,height:24},{containerID:11,containerName:'capture-state',xPosition:layout.stateIconX,yPosition:1,width:24,height:24}],
        textObject: [...nativeAITitles(translate,body),...(body.show?[{
          xPosition: 0, yPosition: 27+body.titleHeight, width: 576, height: body.bodyHeight,
          borderWidth: 1, borderColor: 4, borderRadius: translate?4:0, paddingLength:body.cards?.length>1?0:2,
          containerID: 1, containerName: 'gateway',
          content: lastAIHUDContent || '',
          isEventCapture: 1,
        }]:[]), {
          xPosition: 400, yPosition: 0, width: 176, height: 27,
          paddingLength: 0, borderWidth: 0,
          containerID: 2, containerName: 'status', content: rightAlignedStatusText(glassesBattery), isEventCapture: 0,
        }, {
          xPosition: layout.x, yPosition: 0, width: layout.width, height: 27,
          paddingLength: 0, borderWidth: 0,
          containerID: 3, containerName: 'notice', content: nativeHUDNotice(), isEventCapture:!body.show && !body.showTranscript?1:0,
        }, ...(body.showTranscript?[{
          xPosition: 0, yPosition: body.transcriptY, width: 576, height: body.transcriptHeight,
          paddingLength: 0, borderWidth: 0,
          containerID: 4, containerName: 'transcript',
          content: lastTranscriptHUDContent || '',
          isEventCapture: body.show?0:1,
        }]:[]),{containerID:7,containerName:'mode-label',xPosition:layout.prefix.x,yPosition:0,width:layout.prefix.width,height:27,paddingLength:0,borderWidth:0,content:layout.prefix.content,isEventCapture:0},
        {containerID:8,containerName:'wake-word',xPosition:layout.wake.x,yPosition:0,width:layout.wake.width,height:27,paddingLength:0,borderWidth:0,content:layout.wake.content,textColor:wakeBrightness(),isEventCapture:0}],
        menuObject: {
          menuItems: [
            { itemID: 1, itemName: 'AI 모드' },
            { itemID: 2, itemName: '번역 모드' },
            { itemID: 3, itemName: '입력 소스 전환' },
            ...settingsMenuItems(config.audioInputSource==='glasses'),
          ],
        },
      };
      if(layout.iconOnly){
        page.imageObject=hudImagesDisabled?[]:page.imageObject.filter(c=>c.containerID===11);
        page.textObject=page.textObject.filter(c=>![2,7,8].includes(c.containerID));
        const notice=page.textObject.find(c=>c.containerID===3);
        if(hudImagesDisabled){notice.xPosition=layout.stateIconX;notice.width=24;}
      }
      if(body.cards?.length>1){
        const titles=page.textObject.filter(c=>c.containerID===9 || c.containerID===10);
        page.textObject=[...page.textObject.filter(c=>c.containerID!==9 && c.containerID!==10),...titles];
        const layerOrder=[...page.textObject.filter(c=>c.containerID!==9 && c.containerID!==10),...page.imageObject,...titles];
        layerOrder.forEach((container,i)=>{container.zOrderIndex=i;});
      }
      page.containerTotalNum=page.textObject.length+page.imageObject.length;
      for(const container of page.textObject){
        container.textColor=hudTextBrightness(container.containerID);
        if(container.borderWidth)container.borderColor=hudBorderBrightness(config);
      }
      return page;
  }

  function openSettingsPicker(id){
    const menu=SETTINGS_MENUS[id];if(!menu)return;
    if(id===9 && config.audioInputSource!=='glasses'){setVoiceHint('안경 입력 감지 민감도는 안경 입력에서만 적용됩니다.');return;}
    // Keep engines discoverable even before the asynchronous capability check.
    const options=menu.options;
    settingsPicker={...menu,options,value:id===9?config.glassesInput.sensitivity:config[menu.key],index:0,pageKey:String(id)};
    flushHUD();
  }
  function closeSettingsPicker(){settingsPicker=null;flushHUD();}
  function selectSettingsPicker(index){
    const picker=settingsPicker;
    if(!picker || !Number.isInteger(index) || !picker.options[index])return;
    const value=picker.options[index][0];
    if(picker.key==='hideTranscript'){settingsPicker=null;setTranscriptHidden(value);return;}
    closeSettingsPicker();
    applyLiveSettings({[picker.key]:picker.key==='glassesInput'?{...config.glassesInput,sensitivity:value}:value});
  }

  async function initEvenRealitiesBridge() {
    try {
      updateConnectionState(false, 'Even 앱 연결 대기 · 브라우저 미리보기');
      bridge = await waitForEvenAppBridge();
      restoreNativeSettings();
      nativeTranslateLayout=normalizeGlassesMode(config.glassesMode)==='translate';
      const initialHeaderLayout=currentHeaderLayout(nativeTranslateLayout);
      const initialBody=currentBodyLayout();
      const result = await bridge.createStartUpPageContainer(hudPage(nativeTranslateLayout,initialHeaderLayout,initialBody));
      if (result !== 0) throw new Error(`안경 화면 생성 실패: ${result}`);
      displayReady = true;nativeHeaderLayoutKey=initialHeaderLayout.key;nativeMenuSource=config.audioInputSource;nativeBodyLayoutKey=initialBody.key;
      queueHudIcons(true);
      updateConnectionState(true, 'G2 화면 연결됨');
      setupBridgeEvents();
      startHUDStatus();
      syncHeadIMU().catch(headGestureError);
    } catch (err) {
      console.warn('Even Realities Bridge init notice:', err);
      updateConnectionState(false, `G2 연결 실패: ${err.message}`);
    }
  }

  function setupBridgeEvents() {
    if (!bridge) return;

    // Listen to hardware events (tap, gesture, menu, mic audio)
    if (typeof bridge.onEvenHubEvent === 'function') {
      bridge.onEvenHubEvent((event) => {
        if (!event) return;
        if(event.sysEvent?.imuData || event.sysEvent?.eventType===8){handleHeadIMU(event.sysEvent?.imuData);return;}
        if(headPaused){
          if(event.menuItemClickEvent?.itemID===91 || (event.sysEvent || event.textEvent) && (event.sysEvent?.eventType ?? event.textEvent?.eventType ?? 0)===0)resumeHeadSession();
          return;
        }

        // Handle Audio Events
        if (event.audioEvent && isRecording) {
          const source=event.audioEvent.source;
          if([AudioInputSource.Glasses,AudioInputSource.Phone].includes(source) && source!==activeAudioInputSource)return;
          const pcm = event.audioEvent.audioPcm;
          if (pcm && pcm.length > 0) {
            receivePCM(new Uint8Array(pcm), event.audioEvent.speakerRole);
          }
        }
        if (event.audioEvent) return;

        // Handle Glasses Contextual Menu (전사 / AI / 번역 모드 선택)
        if (event.menuItemClickEvent) {
          if(event.menuItemClickEvent.itemID===90){closeSettingsPicker();return;}
          if(SETTINGS_MENUS[event.menuItemClickEvent.itemID]){openSettingsPicker(event.menuItemClickEvent.itemID);return;}
          if(event.menuItemClickEvent.itemID===3){switchAudioInputSource();return;}
          const selectedMode = glassesModeMenuIdToMode(event.menuItemClickEvent.itemID);
          if (selectedMode) {
            setGlassesMode(selectedMode, true);
          }
          return;
        }

        if(settingsPicker){
          const list=event.listEvent;
          if(list){
            if(list.containerID!==30 && list.containerName!=='settings-list')return;
            if(Number.isInteger(list.currentSelectItemIndex))settingsPicker.index=list.currentSelectItemIndex;
            if((list.eventType ?? 0)===0)selectSettingsPicker(settingsPicker.index);
          }else{
            const type=event.sysEvent?.eventType ?? event.textEvent?.eventType ?? null;
            if(type===0)selectSettingsPicker(settingsPicker.index);
            else if(type===3 || type===9)closeSettingsPicker();
          }
          return;
        }
        // Handle Touch / Gesture Event
        const sysType = event.sysEvent
          ? (event.sysEvent.eventType ?? OsEventTypeList.CLICK_EVENT) : null;
        const textType = event.textEvent ? (event.textEvent.eventType ?? OsEventTypeList.CLICK_EVENT) : null;
        const evType = sysType ?? textType;

        if (evType === OsEventTypeList.DOUBLE_CLICK_EVENT) {
          stopVoiceRecording().catch(console.warn).finally(() => bridge.shutDownPageContainer(1).catch(console.warn));
        } else if (evType === (OsEventTypeList.LONG_PRESS_EVENT ?? 9)) {
          setGlassesMode(nextGlassesMode(config.glassesMode), true);
        } else if (evType === (OsEventTypeList.SCROLL_TOP_EVENT ?? 1)) {
          if (answerText) {
            stopAIAutoScroll();
            if(normalizeGlassesMode(config.glassesMode)==='translate')translationHistory.scroll(-1);
            else if(hintStackActive())hintStack.scroll(-1);
            else aiScrollOffset = Math.max(0, aiScrollOffset - 1);
            renderLiveHUD();
          }
        } else if (evType === (OsEventTypeList.SCROLL_BOTTOM_EVENT ?? 2)) {
          if (answerText) {
            stopAIAutoScroll();
            const win = sliceHUDWindow(aiPresentationText(), aiScrollOffset, aiVisibleLines(), AI_BOX_WIDTH_PX);
            if(normalizeGlassesMode(config.glassesMode)==='translate')translationHistory.scroll(1);
            else if(hintStackActive())hintStack.scroll(1);
            else aiScrollOffset = Math.min(win.maxOffset, aiScrollOffset + 1);
            renderLiveHUD();
          }
        } else if (evType === OsEventTypeList.CLICK_EVENT) {
          if(normalizeGlassesMode(config.glassesMode)==='ai' && answerText){dismissAIWindow();return;}
          toggleVoiceRecording();
        }
      });
    }
  }

  function updateConnectionState(connected, label) {
    if (!els.connectionStatus || !els.connectionText) return;
    els.connectionStatus.classList.toggle('disconnected', !connected);
    els.connectionText.textContent = label || (connected ? '연결됨' : '연결 안 됨');
  }

  // --- Display / HUD Rendering (576x288 Micro-LED) ---
  function formatLiveAIStatus() {
    const headline = withAnimatedDots(aiPhaseTitle || '생각 중', aiAnimStep);
    const lines = [headline];
    if (aiPhaseDetail) lines.push(aiPhaseDetail);
    if (aiReasoningPreview) {
      const compact = aiReasoningPreview.replace(/\s+/g, ' ').trim().slice(-42);
      if (compact) lines.push(`💭 ${compact}`);
    }
    return lines.join('\n');
  }

  function startThinkingAnimation() {
    stopThinkingAnimation();
    aiAnimStep = 0;
    function tick() {
      if (!isThinking && !processingAI) {
        aiAnimTimer = null;
        return;
      }
      aiAnimStep = (aiAnimStep + 1) % 3;
      renderLiveHUD();
      aiAnimTimer = setTimeout(tick, 450);
    }
    aiAnimTimer = setTimeout(tick, 450);
  }

  function stopThinkingAnimation() {
    if (aiAnimTimer) {
      clearTimeout(aiAnimTimer);
      aiAnimTimer = null;
    }
  }

  function stopAIAutoScroll() {
    if (aiScrollTimer) {
      clearTimeout(aiScrollTimer);
      aiScrollTimer = null;
    }
  }

  function startAIAutoScroll(maxOffset) {
    if(hintStackActive())return;
    stopAIAutoScroll();
    aiScrollOffset = 0;
    if (maxOffset <= 0) return;
    function stepScroll() {
      if (processingAI || isThinking || !answerText) {
        aiScrollTimer = null;
        return;
      }
      const win = sliceHUDWindow(aiPresentationText(), aiScrollOffset, aiVisibleLines(), AI_BOX_WIDTH_PX);
      if (aiScrollOffset < win.maxOffset) {
        aiScrollOffset += 1;
        renderLiveHUD();
        aiScrollTimer = setTimeout(stepScroll, hintLifetime.active?Math.min(AI_SCROLL_LINE_MS,hintLifetime.seconds*1000):AI_SCROLL_LINE_MS);
      } else {
        aiScrollTimer = null;
      }
    }
    aiScrollTimer = setTimeout(stepScroll, hintLifetime.active?Math.min(AI_SCROLL_START_MS,aiVisibleLines()*hintLifetime.seconds*1000):AI_SCROLL_START_MS);
  }

  function buildTranscriptSectionText() {
    if (captureFailure) return captureFailure;
    const draft = captionStable + captionDraft;
    const transcriptLines = [
      captionFinal,
      draft ? draft + (captionDraft ? ' …' : '') : ''
    ].filter(Boolean);
    if (transcriptLines.length) {
      return transcriptLines.join('\n');
    }
    if (transcriptStatusMessage) return transcriptStatusMessage;
    const status=captureStatus();
    if(status && (capturePreparing || pendingMicOpen || isRecording))return status;
    return captureListening()
      ? (config.voiceMode === 'continuous' ? '듣는 중... 말하는 동안 자막이 갱신됩니다.' : '듣는 중... 다시 탭하면 전송합니다.')
      : '녹음 정지';
  }

  function renderHUDContainers(aiFullText, aiWindowText, aiMetaLabel, transcriptFullText, pageInfo = 'P 1/1', scrollRatio = 0) {
    const badgeLabel = glassesModeBadgeText(config.glassesMode, config.translateTargetLang);
    const translateMode=normalizeGlassesMode(config.glassesMode)==='translate';
    const body=currentBodyLayout();
    const bottomPhoneText=translateMode?translationReplyText(translationDisplay):transcriptFullText;
    if (els.hudScreen) {
      els.hudScreen.classList.toggle('hud-ai-mode',!translateMode);
      els.hudScreen.classList.toggle('hud-hint-stack',(body.cards?.length || 0)>1);
      const aiCursor = !body.hint && !hintLifetime.active && (isThinking || processingAI) && !(translateMode && config.chromeTranslation!==false) ? '<span class="hud-cursor"></span>' : '';
      const txCursor = (!translateMode && !isThinking && !processingAI && isRecording) ? '<span class="hud-cursor"></span>' : '';
      const previewBody=escapeHtml(aiFullText);
      const stackedMarkup=body.cards?.map((card,i)=>`<div class="hud-ai-box hud-hint-box" data-hint-card="${card.id}" style="flex:0 0 auto;--hud-content-lines:${card.rows}"><div class="hud-ai-header"><span id="${i?'hud-ai-badge-older':'hud-ai-badge'}" class="hud-ai-badge">${escapeHtml(card.title)}</span><span id="${i?'hud-ai-meta-older':'hud-ai-meta'}" class="hud-ai-meta">${card.remaining===null?'':`${card.remaining}초`}</span></div><div id="${i?'hud-ai-body-older':'hud-ai-body'}" class="hud-ai-body">${escapeHtml(card.text)}</div></div>`).join('');
      els.hudScreen.innerHTML =
        (stackedMarkup || (body.show?`<div class="hud-ai-box${body.hint?' hud-hint-box':''}"${!translateMode?` style="flex:0 0 auto;--hud-content-lines:${body.contentLines}"`:''}>` +
          (translateMode || body.titleHeight?`<div class="hud-ai-header">` +
            `<span id="hud-ai-badge" class="hud-ai-badge">${escapeHtml(body.hint?body.title:badgeLabel)}</span>` +
            `<span id="hud-ai-meta" class="hud-ai-meta">${escapeHtml(!translateMode?(hintLifetime.active?`${hintLifetime.remaining}초`:''):config.chromeTranslation!==false?'':aiMetaLabel)}</span>` +
          `</div>`:'') +
          `<div id="hud-ai-body" class="hud-ai-body">${previewBody}${aiCursor}</div>` +
        `</div>`:'')) +
        (body.showTranscript?`<div class="hud-transcript-box${translateMode?' hud-translation-replies':''}">` +
          `<div class="hud-transcript-label">${translateMode?'예상 답변':'전사'}</div>` +
          `<div id="hud-transcript-body" class="hud-transcript-body">${escapeHtml(bottomPhoneText)}${txCursor}</div>` +
        `</div>`:'');
      const aiBodyEl = typeof document.getElementById === 'function' ? document.getElementById('hud-ai-body') : null;
      if (!body.cards && aiBodyEl && typeof aiBodyEl.scrollHeight === 'number' && typeof aiBodyEl.clientHeight === 'number') {
        const maxScroll = Math.max(0, aiBodyEl.scrollHeight - aiBodyEl.clientHeight);
        aiBodyEl.scrollTop = Math.round(maxScroll * Math.max(0, Math.min(1, scrollRatio)));
      }
    }
    if(body.cards)for(let i=0;i<body.cards.length;i++){
      const card=body.cards[i],el=document.getElementById(i?'hud-ai-body-older':'hud-ai-body');
      if(el && typeof el.scrollHeight==='number')el.scrollTop=Math.round(Math.max(0,el.scrollHeight-el.clientHeight)*(card.maxOffset?card.offsetLine/card.maxOffset:0));
    }
    if (els.hudPageIndicator) {
      els.hudPageIndicator.textContent = pageInfo;
    }

    applyHUDPreviewBrightness();
    const nextTitle=body.show && !translateMode && (body.cards || body.titleHeight)?(body.titles?.[0] || aiTitleLine(body.title,hintLifetime.remaining)):'';
    const titleChanged=nextTitle!==lastAITitleContent;lastAITitleContent=nextTitle;
    const nextSecondTitle=body.titles?.[1] || '',secondTitleChanged=nextSecondTitle!==lastSecondAITitleContent;lastSecondAITitleContent=nextSecondTitle;
    const nextAI=body.show?String(aiWindowText || '').slice(0,2000):'';
    if(translateMode && translationPaint && translationPaint.content!==nextAI)translationPaint={...translationPaint,content:nextAI,acked:false};
    const nextTx=body.showTranscript?String(transcriptFullText || '').slice(0,2000):'';
    const txBrightness=hudTextBrightness(4),txBrightnessChanged=txBrightness!==lastTranscriptBrightness;
    lastTranscriptBrightness=txBrightness;
    const txPreview=document.getElementById('hud-transcript-body');
    if(txPreview){
      txPreview.style.opacity='1';
      txPreview.style.color=hudPreviewColor(txBrightness);
      txPreview.style.textShadow=txBrightness===4?'':'none';
    }
    const aiChanged=nextAI!==lastAIHUDContent,txChanged=nextTx!==lastTranscriptHUDContent;
    lastAIHUDContent=nextAI;lastTranscriptHUDContent=nextTx;
    if(displayReady){
      if(!translateMode && body.show && (body.cards || body.titleHeight) && titleChanged)pendingHUD.set(9,{containerID:9,containerName:'ai-title',content:nextTitle});
      if(translateMode || !body.show || (!body.cards && !body.titleHeight))pendingHUD.delete(9);
      if(body.cards?.length>1 && secondTitleChanged)pendingHUD.set(10,{containerID:10,containerName:'hint-title-older',content:nextSecondTitle});
      if(!(body.cards?.length>1))pendingHUD.delete(10);
      if(body.show && aiChanged)pendingHUD.set(1,{containerID:1,containerName:'gateway',content:nextAI,auditHints:visibleHintAudits(),translationPaint:translateMode?translationPaint:null});
      if(!body.show)pendingHUD.delete(1);
      if(!body.showTranscript)pendingHUD.delete(4);
      if(body.showTranscript && (txChanged || txBrightnessChanged))pendingHUD.set(4,{containerID:4,containerName:'transcript',content:nextTx});
      if(pendingHUD.size || nativeTranslateLayout!==translateMode || nativeBodyLayoutKey!==body.key || nativeHeaderLayoutKey!==currentHeaderLayout(translateMode).key)flushHUD();
    }
  }

  function updateHUD(text, pageInfo = 'P 1/1') {
    transcriptStatusMessage = String(text || '');
    renderLiveHUD(pageInfo);
  }

  function acknowledgeTranslationPaint(paint,content){
    if(!paint || paint.content!==content)return;
    const values={sdk_failed:0};
    if(!paint.acked){paint.acked=true;values.sdk_ms=Date.now()-paint.at;}
    translationMetrics.record(paint.id,values);
  }

  // Serialize and coalesce updates so streamed tokens do not flood Bluetooth.
  async function flushHUD() {
    if (sendingHUD || !hudTransport.ready) return;
    sendingHUD = true;let sendingHintMeta=[],sendingPaint=null;
    try {
      while (displayReady && hudTransport.ready) {
        sendingHintMeta=[];sendingPaint=null;
        if(headPaused){
          if(!nativeHeadBlank){
            if(!await hudTransport.call(()=>bridge.rebuildPageContainer(blankHeadPage())))throw Error('빈 화면 전환 실패');
            nativeHeadBlank=true;nativePickerKey='';nativeBodyLayoutKey='';nativeHeaderLayoutKey='';lastHudIconKeys.clear();
          }
          pendingHUD.clear();if(!headPaused)continue;break;
        }
        if(nativeHeadBlank){nativeHeadBlank=false;nativeBodyLayoutKey='';nativeHeaderLayoutKey='';}
        if(settingsPicker){
          const picker=settingsPicker;
          if(nativePickerKey!==picker.pageKey){
            if(!await hudTransport.call(()=>bridge.rebuildPageContainer(settingsPickerPage(picker))))throw Error('설정 선택 화면 생성 실패');
            nativePickerKey=picker.pageKey;
            continue;
          }
          break; // Retain coalesced HUD updates until the picker closes.
        }
        const translate=normalizeGlassesMode(config.glassesMode)==='translate';
        const header=currentHeaderLayout(translate);
        const body=currentBodyLayout();
        if(nativeBodyLayoutKey!==body.key || nativePickerKey || nativeMenuSource!==config.audioInputSource || nativeTranslateLayout!==translate || nativeHeaderLayoutKey!==header.key){
          const paint=translationPaint;const shownHints=visibleHintAudits();sendingHintMeta=shownHints;
          const page=hudPage(translate,header,body);
          sendingPaint=translate && body.show && paint && page.textObject.some(c=>c.containerID===1 && c.content===paint.content)?paint:null;
          const success=await hudTransport.call(()=>bridge.rebuildPageContainer(page));
          if(!success)throw new Error('안경 화면 배치 변경 실패');
          if(translate && body.show)acknowledgeTranslationPaint(paint,page.textObject.find(c=>c.containerID===1)?.content);
          if(!translate && body.show)shownHints.forEach(hintDeviceDisplayed);
          const allowed=new Set([...page.textObject,...page.imageObject].map(c=>c.containerID));
          for(const id of pendingHUD.keys())if(!allowed.has(id))pendingHUD.delete(id);
          nativePickerKey='';nativeMenuSource=config.audioInputSource;nativeTranslateLayout=translate;nativeHeaderLayoutKey=header.key;nativeBodyLayoutKey=body.key;
          if(!body.show)pendingHUD.delete(1);
          if(translate || !body.show || (!body.cards && !body.titleHeight))pendingHUD.delete(9);
          if(!(body.cards?.length>1))pendingHUD.delete(10);
          if(!body.showTranscript)pendingHUD.delete(4);
          lastHudIconKeys.clear();pendingHUD.delete(5);pendingHUD.delete(6);pendingHUD.delete(11);queueHudIcons(true);
          continue;
        }
        if(!pendingHUD.size)break;
        const [id, update] = pendingHUD.has(1)?[1,pendingHUD.get(1)]:pendingHUD.entries().next().value;
        pendingHUD.delete(id);
        if(update.icon){
          try{
            const imageData=await hudIcons.load(update.icon,hudLuminance(hudTextBrightness(update.containerID)));
            const result=await hudTransport.call(()=>bridge.updateImageRawData({containerID:update.containerID,containerName:update.containerName,imageData}));
            if(result!=='success')throw Error(`아이콘 전송 실패: ${result}`);
          }catch(error){
            if(!hudTransport.ready)throw error;
            console.warn('HUD image unavailable; keeping text updates:',error);
            hudImagesDisabled=true;pendingHUD.delete(5);pendingHUD.delete(6);pendingHUD.delete(11);queueHudIcons();
          }
        }else{
          const {auditHints:shownHints=[],translationPaint:paint,...payload}=update;sendingHintMeta=shownHints;sendingPaint=id===1 && translate && paint?.content===payload.content?paint:null;
          const success = await hudTransport.call(()=>bridge.textContainerUpgrade({...payload,textColor:hudTextBrightness(id)}));
          if (!success) throw new Error('안경 화면 갱신 실패');
          if(id===1 && translate)acknowledgeTranslationPaint(paint,payload.content);
          shownHints.forEach(hintDeviceDisplayed);
        }

      }
    } catch (err) {
      if(sendingPaint)translationMetrics.record(sendingPaint.id,{sdk_failed:1});
      sendingHintMeta.forEach(meta=>auditHint(meta,'device_error',{reason:'sdk_transport_error',text:JSON.stringify({message:String(err.message || err).slice(0,240),connected:hudTransport.connected})}));
      console.warn('Bridge display push error:', err);
      updateConnectionState(false, err.message);
    } finally {
      sendingHUD = false;
    }
  }

  function renderHUDStatus() {
    const date=new Date();
    const text = hudStatusText(glassesBattery,date);
    els.hudStatus.textContent = text;
    if (displayReady && !iconOnlyHeader() && text !== lastStatus) {
      pendingHUD.set(2, {containerID: 2, containerName: 'status', content: rightAlignedStatusText(glassesBattery,date)});
      flushHUD();
    }
    lastStatus = text;
  }

  function currentTranslationHeader(messageOnly=false){
    return translationHeaderText({messageOnly,imageIcons:true,source:translationDirection.pair.split('>')[0] || config.translateSourceLang || 'auto',target:translationDirection.pair.split('>')[1] || config.translateTargetLang,
      running:isRecording || capturePreparing || processingAI || isThinking || translating,
      listening:captureListening(),detail:captureStatus(),
      phase:(isThinking || processingAI)?aiPhaseTitle:'',phaseDetail:(isThinking || processingAI)?aiPhaseDetail:'',translating,
      error:captureFailure || micCloseUncertain?'오류':translationFailure});
  }
  function captureListening(){return (config.audioInputSource!=='glasses' || hudTransport.connected) && isRecording && !capturePreparing && !pendingMicOpen && !micCloseUncertain && !micStopPromise;}
  function captureStatus(){
    if(config.audioInputSource==='glasses' && !hudTransport.connected)return '안경 연결 대기';
    if(micStopPromise || pendingMicCloses.size)return '마이크 정지중';
    if(pendingMicOpen)return '마이크 연결중';
    if(capturePreparing)return '음성 엔진 준비중';
    if(!captureListening())return '대기중';
    if(!receivedCaptureAudio)return '오디오 수신 대기';
    if(activeFinalClip?.busyAttempts)return '전사 처리 대기';
    return '';
  }
  function setHUDNotice(message) {
    queueHudIcons();
    message=currentHUDHeader();
    if(els.hudNotice.textContent===message)return;
    els.hudNotice.textContent = message;
    if(iconOnlyHeader())return;
    // The left header is bounded so a long wake word cannot wrap into the body.
    const content = nativeHUDNotice();
    if (displayReady) {
      pendingHUD.set(3, {containerID:3,containerName:'notice',content});
      flushHUD();
    }
  }
  function currentHUDHeader(messageOnly=false){
    const mode=normalizeGlassesMode(config.glassesMode);
    if(mode==='translate')return currentTranslationHeader(messageOnly);
    const error=micCloseUncertain?'마이크 종료 확인 실패':captureFailure?'전사 오류':'';
    const detail=captureStatus() || (pendingTrigger?'질문 대기':draining || pendingClips.length?'전사 처리':'');
    return modeHeaderText({mode,messageOnly,imageIcons:true,hint:config.aiHintMode!==false,listening:captureListening(),
      running:isRecording || capturePreparing || processingAI || isThinking || draining || pendingClips.length>0,
      phase:(isThinking || processingAI)?aiPhaseTitle:'',phaseDetail:(isThinking || processingAI)?aiPhaseDetail:'',error,detail,trigger:triggerLabels()[0] || ''});
  }
  function triggerLabels() {
    const seen = new Set();
    return config.triggerWord.split(/[,，;\n]/u).map(value=>value.trim()).filter(value=>{
      const key=value.normalize('NFC').replace(/\s/gu,'').toLowerCase();
      if(!key || seen.has(key))return false;
      seen.add(key);return true;
    });
  }
  function transcriptNotice() { setHUDNotice(); }
  function renderLiveHUD(overridePageInfo) {
    if(headPaused){
      els.hudScreen.innerHTML='<div class="hud-head-paused">화면/STT 일시정지 · 고개 제스처 또는 재개 버튼으로 다시 켜기</div>';
      els.headResume.hidden=false;flushHUD();return;
    }
    queueHudIcons();
    const chromeTranslateMode=normalizeGlassesMode(config.glassesMode)==='translate' && config.chromeTranslation!==false;
    const notice=currentHUDHeader();if(els.hudNotice.textContent!==notice)setHUDNotice(notice);
    const triggerQuestion=normalizeGlassesMode(config.glassesMode)==='ai' && config.triggerTranscriptOnly?triggerTranscriptText():'';
    const transcriptRaw=triggerQuestion || buildTranscriptSectionText();
    const body=currentBodyLayout();
    const transcriptWin = sliceHUDWindow(transcriptRaw, Infinity, Math.max(1,body.transcriptLines), TRANSCRIPT_WIDTH_PX);
    const transcriptGlassesText = normalizeGlassesMode(config.glassesMode)==='translate'
      ? translationReplyText(translationDisplay,TRANSCRIPT_WIDTH_PX) : body.showTranscript?(transcriptWin.visibleText || transcriptRaw):'';


    if(normalizeGlassesMode(config.glassesMode)==='translate' && translationHistory.entries.length){
      const win=translationHistory.window();
      const ratio=win.maxOffset>0?win.offsetLine/win.maxOffset:0;
      renderHUDContainers(win.fullText,win.visibleText,'',transcriptGlassesText,overridePageInfo || `P ${win.page}/${win.totalPages}`,ratio);
      return;
    }

    if(body.cards){renderHUDContainers(answerText,body.nativeBody,'',transcriptGlassesText,overridePageInfo || `카드 ${body.cards.length}/${config.hintCardCount}`);return;}
    if (answerText) {
      const win = !chromeTranslateMode && normalizeGlassesMode(config.glassesMode)==='ai' && hintLifetime.active?hintCountdownWindow(answerText,answerTitle,aiScrollOffset,transcriptDisplayHidden(),hintLifetime.remaining):body.hint?hintWindow(answerText,answerTitle,aiScrollOffset,transcriptDisplayHidden()):sliceHUDWindow(aiPresentationText(), chromeTranslateMode && liveTranslationDraft?Infinity:aiScrollOffset, aiVisibleLines(), AI_BOX_WIDTH_PX);
      const pageInfo = overridePageInfo || `P ${win.page}/${win.totalPages}`;
      const ratio = win.maxOffset > 0 ? win.offsetLine / win.maxOffset : 0;
      const meta = win.maxOffset > 0
        ? `자동 스크롤 ${win.offsetLine + 1}/${win.maxOffset + 1}`
        : '응답 완료';
      renderHUDContainers(answerText, win.visibleText, meta, transcriptGlassesText, pageInfo, ratio);
      return;
    }

    const mode = normalizeGlassesMode(config.glassesMode);
    let idleAI = 'AI 대기 중 · 호출어로 부르거나 텍스트로 질문하세요.';
    if (mode === 'ai') {
      idleAI = 'AI 모드 · 평소에는 힌트, 호출어 뒤 질문에는 AI 채팅으로 답합니다.';
    } else if (mode === 'translate') {
      const langShort = targetLanguageLabel(config.translateTargetLang, true);
      idleAI = `번역 모드 (→${langShort}) · 말하면 번역과 긍정/부정 답변을 표시합니다.`;
    }
    if(chromeTranslateMode)idleAI='';
    renderHUDContainers(idleAI, idleAI, '대기', transcriptGlassesText, overridePageInfo || 'P 1/1', 0);
  }

  function applyGlassesStatus(status) {
    if (!glassesSN || status?.sn !== glassesSN) return;
    if(status.connectType){hudTransport.setConnected(status.connectType==='connected');if(!hudTransport.connected)receivedCaptureAudio=false;}
    if (status.connectType === 'disconnected' || status.connectType === 'none'){glassesBattery = null;headImuActive=false;headDetector.reset();}
    else{if (typeof status.batteryLevel === 'number') glassesBattery = status.batteryLevel;if(config.headGestureEnabled && displayReady)syncHeadIMU().catch(headGestureError);}
    renderHUDStatus();renderLiveHUD();
  }
  async function refreshGlassesBattery() {
    if (typeof bridge?.getDeviceInfo !== 'function') return;
    try {
      const info = await storageCall(bridge.getDeviceInfo());
      if (!info) { glassesBattery = null; renderHUDStatus(); return; }
      if (!['g1', 'g2'].includes(info.model)) return;
      if (glassesSN !== info.sn) glassesBattery = null;
      glassesSN = info.sn;
      applyGlassesStatus(info.status);
    } catch (err) { console.warn('Glasses battery unavailable:', err.message); }
  }
  function startHUDStatus() {
    renderHUDStatus();
    if (typeof bridge.onDeviceStatusChanged === 'function') {
      unsubscribeDeviceStatus = bridge.onDeviceStatusChanged(status => {
        if (!glassesSN) refreshGlassesBattery();
        else applyGlassesStatus(status);
      });
    }
    refreshGlassesBattery();
    function tickStatus() {
      renderHUDStatus();
      refreshGlassesBattery();
      statusTimer = setTimeout(tickStatus, 60000 - Date.now()%60000);
    }
    statusTimer = setTimeout(tickStatus, 60000 - Date.now()%60000);
    window.addEventListener?.('pagehide', () => {
      clearTimeout(statusTimer);
      unsubscribeDeviceStatus?.();
      displayReady = false;cancelHeadCalibration();syncHeadIMU().catch(()=>{});
      pendingHUD.clear();
    }, {once:true});
  }

  // --- Voice capture, continuous VAD and durable transcript pipeline ---
  function newID() { return window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`; }
  function resetCaptureTimers() {
    clearTimeout(captureTimer); clearTimeout(manualTimer);
    captureTimer = manualTimer = null;
  }
  function setVoiceHint(text) { if (els.actionHint) els.actionHint.textContent = text; }

  async function microphoneCall(promise) {
    let timer;
    try {
      return await Promise.race([promise,new Promise((_,reject)=>{
        timer=setTimeout(()=>reject(new Error('마이크 응답 시간 초과')),5000);
      })]);
    } finally {clearTimeout(timer);}
  }
  function showMicStopped() {
    els.voiceBtn?.classList.remove('active');
    els.voiceBtn?.setAttribute?.('aria-pressed','false');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 시작');
    els.voiceMode.disabled=false;els.triggerWord.disabled=false;els.sttEngine.disabled=false;els.audioInputSource.disabled=false;syncGlassesInputUI();
    if (!captureFailure && !micCloseUncertain) transcriptStatusMessage = '';
    setHUDNotice(pendingClips.length || draining ? '녹음 정지 · 남은 전사 처리 중' : '녹음 정지');
    setVoiceHint(pendingClips.length || draining ? '마이크 꺼짐 · 남은 음성만 처리 중' : '마이크 꺼짐 · 탭하면 녹음 시작');
    if(!captureFailure && !pendingClips.length && !draining)renderLiveHUD();
  }
  async function confirmMicrophoneOff() {
    const generation=captureGeneration;
    const closing=Promise.resolve().then(()=>bridge.audioControl(false));
    pendingMicCloses.add(closing);
    closing.then(()=>pendingMicCloses.delete(closing),()=>pendingMicCloses.delete(closing));
    const closed = await microphoneCall(closing);
    if(generation!==captureGeneration)return;
    if (!closed) throw new Error('Even 앱이 마이크 종료를 확인하지 않았습니다.');
    if (pendingMicOpen) {
      micCloseUncertain=true;
      setHUDNotice('마이크 정지 확인 중');
      setVoiceHint('마이크 시작 응답을 기다린 뒤 종료를 다시 확인합니다.');
    } else {
      micMayBeOpen=false;micCloseUncertain=false;
      showMicStopped();
    }
  }

  async function toggleVoiceRecording({fromSourceSwitch=false,fromHeadResume=false}={}) {
    if(headPaused && !fromHeadResume){if(!fromSourceSwitch)return resumeHeadSession();return;}
    if(inputSwitchPromise && !fromSourceSwitch){resumeAfterSettings=false;await stopVoiceRecording();return;}
    // Stop has priority even while warmup or microphone-open is awaiting a reply.
    if (isRecording || capturePreparing || pendingMicOpen || micCloseUncertain || pendingMicCloses.size) {
      await stopVoiceRecording();return;
    }
    if (micBusy || micStopPromise || (inputSwitchPromise && !fromSourceSwitch)) return;
    if (!displayReady) { updateHUD('Even 앱에서 G2에 연결한 뒤 시작하세요.'); return; }
    micBusy=true;capturePreparing=true;
    const generation=++captureGeneration;
    const controller=new AbortController();captureAbort=controller;
    els.voiceBtn?.classList.add('active');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 준비 취소');
    setHUDNotice('녹음 준비 중 · 탭하면 취소');
    let startingTask;
    try {
      startingTask=startVoiceRecording(generation,controller.signal);
      captureTask=startingTask;await startingTask;
    } catch (err) {
      if(generation!==captureGeneration || controller.signal.aborted){
        if(pendingMicOpen){micCloseUncertain=true;setHUDNotice('마이크 상태 확인 필요');setVoiceHint('마이크 시작 응답이 없습니다. 앱을 완전히 닫아주세요.');}
        return;
      }
      isRecording=false;resetCaptureTimers();
      if(micMayBeOpen || pendingMicOpen) await stopVoiceRecording(false);
      else showMicStopped();
      setVoiceHint('녹음 실패 · 마이크 권한과 G2 연결을 확인하세요');
      updateHUD(`마이크 오류: ${err.message}`);
    } finally {
      if(captureAbort===controller)captureAbort=null;
      capturePreparing=false;micBusy=false;
      if(captureTask===startingTask)captureTask=null;
    }
  }

  function setTranscriptHidden(hidden){
    settingsEdited=true;config.hideTranscript=!!hidden;els.hideTranscript.checked=config.hideTranscript;
    renderLiveHUD();persistSettings().catch(console.warn);
  }
  function setTriggerTranscriptOnly(){
    settingsEdited=true;config.triggerTranscriptOnly=els.triggerTranscriptOnly.checked===true;
    renderLiveHUD();persistSettings().catch(console.warn);
  }
  function dismissAIWindow(cardID){
    if(normalizeGlassesMode(config.glassesMode)!=='ai' || !answerText)return false;
    const stacked=hintStackActive();
    chatEpoch++;pendingTrigger=false;voiceWakeJobs.clear();
    for(const job of pendingAI)if(job.hint)hintGate.complete(job);
    pendingAI.length=0;hintGate.pending=null;clearTimeout(hintGate.timer);hintGate.timer=null;hintGate.inflight?.controller.abort();
    aiAbortedByModeSwitch=true;if(activeAIJob?.hint)activeAIJob.cancelReason='user_dismissed';aiAbortController?.abort();hintLifetime.clear();
    clearTimeout(answerTimer);stopAIAutoScroll();
    if(stacked){hintStack.remove(typeof cardID==='string'?cardID:undefined);return true;}
    clearHintStack();answerText=answerTitle=answerVoicePrompt='';answerAutoCloseEligible=false;aiScrollOffset=0;answerHoldUntil=0;
    renderLiveHUD();return true;
  }
  // The card scrolls. Fitting a whole sentence into the first viewport used
  // to return an empty string for long sentences and silently drop the card.
  function fitHintContent(text){return limitHUDResponse(text);}
  function syncCaptureSettings(){
    syncModeUI();syncGlassesInputUI();
    els.audioInputSource.value=config.audioInputSource;
    els.sttEngine.value=config.sttEngine;
    els.translationEngine.value=config.translationEngine;
    els.chromeTranslation.checked=config.translationEngine!=='llm';
    els.translateExcludeSelf.checked=config.translateExcludeSelf!==false;
  }
  async function applyLiveSettings(patch){
    if(!Object.entries(patch).some(([key,value])=>JSON.stringify(value)!==JSON.stringify(pendingSettings[key] ?? config[key])))return inputSwitchPromise;
    // These controls affect only future PCM frames or translation requests.
    // Reopening STT here flushes a segment and competes with its finalization.
    if(!inputSwitchPromise && Object.keys(patch).every(key=>['glassesInput','translationEngine'].includes(key))){
      Object.assign(config,patch);settingsEdited=true;config.chromeTranslation=config.translationEngine!=='llm';
      if(patch.glassesInput){
        activeGlassesInput=normalizeGlassesInput(config.glassesInput);
        activeInputProcessor.configure(activeGlassesInput);
        if(segmenter)segmenter.threshold=inputSegmentOptions(activeSttEngine,config.glassesMode,activeAudioInputSource,activeGlassesInput).threshold;
      }
      if(patch.translationEngine){
        liveTranslationDraft=false;liveTranslations.reset();translationSentence.reset();stableTranslation.reset();llmStableTranslation.reset();llmTranslationCache.clear();
        translationReplyVersion++;translationFailure='';
        pendingAI.splice(0,pendingAI.length,...pendingAI.filter(job=>!job.translationToken));
        if(activeAIJob?.translationToken)aiAbortController?.abort();
        // Preparation has its own latest-settings queue and must not stop audio.
        prepareSelectedTranslation().catch(()=>{});
      }
      syncCaptureSettings();await persistSettings();renderLiveHUD();return;
    }
    pendingSettings={...pendingSettings,...patch};settingsEdited=true;
    if(inputSwitchPromise){
      // Cancel a superseded engine warmup; the transaction will open only the latest choice.
      if(capturePreparing)stopVoiceRecording(false).catch(console.warn);
      return inputSwitchPromise;
    }
    resumeAfterSettings=isRecording || capturePreparing;
    const work=(async()=>{
      // Let inputSwitchPromise be assigned before any asynchronous work can finish.
      await Promise.resolve();
      try{
        do{
          const starting=captureTask;
          if(isRecording || capturePreparing || micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)await stopVoiceRecording(true);
          if(starting)await starting.catch(()=>{});
          if(micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)throw Error('기존 마이크 종료를 확인한 뒤 다시 변경해 주세요.');
          const next=pendingSettings;pendingSettings={};
          const preserveDisplay=config.glassesMode==='translate' && (!next.glassesMode || next.glassesMode==='translate');
          Object.assign(config,next);config.chromeTranslation=config.translationEngine!=='llm';
          recognitionEpoch++;
          resetQueuesForModeSwitch(config.glassesMode,true,preserveDisplay);
          syncCaptureSettings();await persistSettings();
          // Translation preparation may fall back to LLM; STT readiness is checked before opening.
          await prepareSelectedTranslation().catch(()=>{});
          if(Object.keys(pendingSettings).length)continue;
          if(resumeAfterSettings)await toggleVoiceRecording({fromSourceSwitch:true});
          else autoPrepareEngines();
        }while(Object.keys(pendingSettings).length);
      }catch(error){pendingSettings={};syncCaptureSettings();els.hintStatus.textContent=`설정 변경 실패 · ${error.message}`;setVoiceHint(`설정 변경 실패 · ${error.message}`);}
    })();
    inputSwitchPromise=work;
    try{await work;}finally{if(inputSwitchPromise===work){inputSwitchPromise=null;resumeAfterSettings=false;syncGlassesInputUI();}}
  }
  function switchAudioInputSource(){
    if(inputSwitchPromise)return inputSwitchPromise;
    return applyLiveSettings({audioInputSource:config.audioInputSource==='phone'?'glasses':'phone'});
  }


  async function startVoiceRecording(generation, signal) {
    receivedCaptureAudio=false;
    translationSentence.reset();
    abortPendingPreview();
    hintGate.reset();
    config.aiHintMode=true;
    config.hintContextTokens=normalizeHintBudget(els.hintContextTokens.value);
    config.hintSensitivity=els.hintSensitivity.value || 'balanced';
    config.hintWebSearch=els.hintWebSearch?.checked!==false;
    config.aiHeaderLayout=normalizeAIHeaderLayout(els.aiHeaderLayout.value);config.aiIconPosition=normalizeAIIconPosition(els.aiIconPosition.value);
    config.hideTranscript=els.hideTranscript.checked===true;
    config.triggerTranscriptOnly=els.triggerTranscriptOnly.checked===true;
    config.hintCardCount=hintCardCount(els.hintCardCount.value);config.hintAutoClose=els.hintAutoClose.checked===true;config.hintSecondsPerLine=secondsPerHintLine(els.hintSecondsPerLine.value);
    if (els.glassesMode?.value) config.glassesMode = normalizeGlassesMode(els.glassesMode.value);
    if (els.translateSourceLang?.value) config.translateSourceLang = els.translateSourceLang.value;
    if (els.translateTargetLang?.value) config.translateTargetLang = els.translateTargetLang.value;
    config.voiceMode = 'continuous';
    config.triggerWord = els.triggerWord.value.trim();
    config.speechFilter = els.speechFilter.value === 'standard' ? 'standard' : 'strict';
    config.audioInputSource=els.audioInputSource.value==='phone'?'phone':'glasses';
    config.glassesInput=readGlassesInputControls();
    config.sttEngine=['chrome-online','apple'].includes(els.sttEngine.value)?els.sttEngine.value:'whisper';config.sttFallback=els.sttFallback.checked!==false;
    config.speakerScope = els.speakerScope.value === 'self' ? 'self' : 'conversation';
    config.translateExcludeSelf=els.translateExcludeSelf?.checked!==false;
    config.chromeTranslation=els.chromeTranslation?.checked!==false;
    const mode = normalizeGlassesMode(config.glassesMode);
    const continuousCapture = isContinuousCaptureMode(mode);
    const labels = triggerLabels();
    if (mode === 'ai' && (labels.length>8 || labels.some(label=>Array.from(label.replace(/\s/gu,'')).length>40))) throw new Error('AI 호출어는 최대 8개, 각각 40자 이하로 입력하세요.');
    activeSttEngine=config.sttEngine;els.sttEngine.disabled=false;
    activeAudioInputSource=config.audioInputSource==='phone'?AudioInputSource.Phone:AudioInputSource.Glasses;
    activeGlassesInput=normalizeGlassesInput(config.glassesInput);activeInputProcessor=new GlassesInputProcessor(activeGlassesInput);lastInputMeterAt=0;
    els.glassesInputMeter.value=-72;els.glassesInputLevel.textContent='오디오 수신 대기';
    const captureAudioSource=activeAudioInputSource;els.audioInputSource.disabled=false;syncGlassesInputUI();
    if(activeSttEngine!=='whisper'){
      try{await prepareSelectedStt(signal,true);}
      catch(error){if(signal.aborted)throw error;if(!config.sttFallback)throw error;const failedEngine=activeSttEngine;activeSttEngine='whisper';els.sttEngineStatus.textContent=`${failedEngine==='apple'?'Apple':'Chrome'} 준비 실패 · Whisper 사용 (${error.message})`;}
    }
    if(activeSttEngine==='whisper'){
    setHUDNotice('음성 엔진 준비 중');
    await prepareSelectedStt(signal,true,'whisper');
    }else els.sttEngineStatus.textContent=activeSttEngine==='chrome-online'?'Chrome 온라인 STT · 음성은 Google로 전송됩니다':'Apple 로컬 STT · 서버에서 음성 처리 중';
    if(generation!==captureGeneration || signal.aborted)return;
    captureFailure = '';
    for(const clip of pendingClips)clip.attempts=0;
    sessionId = newID(); pendingTrigger = false;
    activeUtterance = null; pendingPreview = null; draftUtteranceId = '';
    captionStable = captionDraft = ''; captionFinal = '';
    transcriptNotice(continuousCapture ? '실시간 전사 중' : '녹음 중');
    audioBuffer = []; recordedBytes = 0; recordingStarted = Date.now();
    segmenter = new SpeechSegmenter(inputSegmentOptions(activeSttEngine,mode,activeAudioInputSource,activeGlassesInput));
    isRecording = true;
    els.voiceMode.disabled = true; els.triggerWord.disabled = true;els.sttEngine.disabled=false;
    els.voiceBtn?.classList.add('active');
    updateHUD(continuousCapture
      ? '말씀하세요. 말하는 동안 자막이 갱신됩니다.' : '말씀하세요. 다시 탭하면 전송합니다.');
    micMayBeOpen=true;
    setHUDNotice('마이크 여는 중 · 탭하면 취소');
    const opening = Promise.resolve().then(()=>bridge.audioControl(true,captureAudioSource));
    pendingMicOpen=opening;
    // A delayed successful open must never undo a newer stop request.
    opening.then(async opened=>{
      if(pendingMicOpen===opening)pendingMicOpen=null;
      if(!opened)micMayBeOpen=false;
      if(generation!==captureGeneration && opened){
        try {
          if(micStopPromise)await micStopPromise;
          await confirmMicrophoneOff();
        } catch(err) {
          micCloseUncertain=true;setHUDNotice('마이크 종료 확인 실패');setVoiceHint('다시 탭해 종료를 재시도하거나 앱을 완전히 닫아주세요.');
        }
      }
    },()=>{if(pendingMicOpen===opening)pendingMicOpen=null;});
    const opened = await microphoneCall(opening);
    if(generation!==captureGeneration || signal.aborted)return;
    if (!opened) { els.voiceMode.disabled = false; els.triggerWord.disabled = false; throw new Error(`${captureAudioSource===AudioInputSource.Phone?'휴대폰':'안경'} 마이크를 켤 수 없습니다. Even 앱의 마이크 권한을 확인하세요.`); }
    capturePreparing=false;micCloseUncertain=false;
    els.voiceBtn?.setAttribute?.('aria-pressed','true');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 정지');
    transcriptNotice(continuousCapture?'실시간 전사 중':'녹음 중');
    renderLiveHUD();
    captureTimer = setTimeout(() => {
      if (isRecording && !receivedCaptureAudio) {
        captureFailure=captureAudioSource===AudioInputSource.Phone?'휴대폰 마이크 입력이 없습니다.\n휴대폰 설정에서 Even 앱의 마이크 권한을 확인한 뒤 다시 시작해 주세요.':'마이크는 열렸지만 오디오가 오지 않습니다.\nG2 연결과 마이크 권한을 확인하고 앱을 다시 여세요.';
        stopVoiceRecording().then(()=>setVoiceHint(captureFailure)).catch(console.warn);
        updateHUD(captureFailure);
      }
    }, 6000);
    if (!continuousCapture) manualTimer = setTimeout(() => stopVoiceRecording().catch(console.warn), 25000);
    setVoiceHint(`${captureAudioSource===AudioInputSource.Phone?'휴대폰':'안경'} 마이크 켜짐 · ${receivedCaptureAudio?'오디오 수신 중':'오디오 수신 대기'}`);
    if (pendingClips.length) drainClips();
  }

  function readGlassesInputControls(){return normalizeGlassesInput({sensitivity:els.glassesSensitivity.value,gain:els.glassesGain.value});}
  function syncGlassesInputUI(){
    const settings=normalizeGlassesInput(config.glassesInput);
    els.glassesInputSettings.hidden=config.audioInputSource!=='glasses';
    els.glassesSensitivity.value=settings.sensitivity;els.glassesGain.value=String(settings.gain);
    els.glassesSensitivity.disabled=config.audioInputSource!=='glasses';els.glassesGain.disabled=config.audioInputSource!=='glasses';
  }
  function updateGlassesInputMeter(signal,excluded){
    if(activeAudioInputSource!==AudioInputSource.Glasses || Date.now()-lastInputMeterAt<150)return;
    lastInputMeterAt=Date.now();
    const db=value=>Number.isFinite(levelDb(value))?`${Math.round(levelDb(value))} dBFS`:'무음';
    els.glassesInputMeter.value=Math.max(-72,Math.min(0,levelDb(signal.outputLevel)));
    const detected=signal.rawLevel>=signal.threshold;
    els.glassesInputLevel.textContent=`수신 ${db(signal.rawLevel)} → 적용 후 ${db(signal.outputLevel)} · ${excluded?'화자 필터로 제외':detected?'입력 감지':'감지 기준 미만'}${activeGlassesInput.sensitivity==='auto'?` · 자동 기준 ${db(signal.threshold)}`:''}${activeGlassesInput.gain==='auto'?` · 자동 증폭 ${signal.effectiveGain.toFixed(1)}배`:''}${signal.limited?' · 큰 소리 증폭 제한':''}`;
  }

  function receivePCM(bytes, speakerRole = 'unknown') {
    // Preserve the audio clock and silence boundaries when excluding a speaker.
    const signal=activeInputProcessor.process(bytes,activeAudioInputSource);
    bytes=filterSpeakerPCM(signal.bytes,speakerRole,config);
    const excluded=bytes!==signal.bytes;
    // Gain changes STT input volume, not which background frames count as speech.
    const level=excluded?0:signal.rawLevel;
    segmenter.threshold=signal.threshold;
    updateGlassesInputMeter(signal,excluded);
    recordedBytes += bytes.length;
    if(!receivedCaptureAudio && bytes.length){receivedCaptureAudio=true;renderLiveHUD();}
    if(level>=segmenter.threshold)lastVoiceFrameAt=recordingStarted+recordedBytes/32;
    const continuousCapture = isContinuousCaptureMode();
    setVoiceHint(`${continuousCapture ? '연속 감지 중' : '녹음 중'} · ${(recordedBytes/32000).toFixed(1)}초 · 음량 ${Math.round(level*1000)} · 대기 ${pendingClips.length}`);
    if (!continuousCapture) {
      audioBuffer.push(bytes);
      if(activeSttEngine!=='whisper'){
        if(!activeUtterance)activeUtterance={id:newID(),revision:0,lastPreviewBytes:0,agreement:new LocalAgreement()};
        if(recordedBytes-activeUtterance.lastPreviewBytes>=8000){
          activeUtterance.lastPreviewBytes=recordedBytes;
          pendingPreview={bytes:mergePCM(audioBuffer),sessionId,segmentId:activeUtterance.id,engine:activeSttEngine,revision:++activeUtterance.revision,startedAt:new Date(recordingStarted).toISOString(),endedAt:new Date(recordingStarted+recordedBytes/32).toISOString(),mode:normalizeGlassesMode(config.glassesMode),sourceLang:config.translateSourceLang,targetLang:config.translateTargetLang,speechFilter:config.speechFilter};
          drainPreview();
        }
      }
      return;
    }
    if(normalizeGlassesMode(config.glassesMode)==='ai'){
      segmenter.silenceMs=(pendingTrigger || (previewWakeId && previewWakePrompt))?Math.min(segmenter.baseSilenceMs,500):previewWakeId?Math.min(segmenter.baseSilenceMs,650):segmenter.baseSilenceMs;
    }
    const clip = segmenter.push(bytes,level);
    if (clip) {
      const id = activeUtterance?.id || newID();
      activeUtterance = null; abortPendingPreview();
      enqueueClip(clip, true, recordingStarted + recordedBytes/32, id);
      return;
    }
    if (!segmenter.parts.length) { activeUtterance = null; pendingPreview = null; return; }
    if (!activeUtterance) activeUtterance = {id:newID(),revision:0,lastPreviewBytes:0,agreement:new LocalAgreement()};
    if (recordedBytes-activeUtterance.lastPreviewBytes < (activeSttEngine==='whisper'?32000:8000)) return;
    const snapshot = segmenter.snapshot();
    if (!snapshot) return;
    activeUtterance.lastPreviewBytes = recordedBytes;
    const endedAt = recordingStarted + recordedBytes/32;
    pendingPreview = {bytes:snapshot,sessionId,segmentId:activeUtterance.id,engine:activeSttEngine,
      revision:++activeUtterance.revision,startedAt:new Date(endedAt-snapshot.length/32).toISOString(),
      endedAt:new Date(endedAt).toISOString(),speechFilter:config.speechFilter,
      mode:normalizeGlassesMode(config.glassesMode),
      sourceLang:config.translateSourceLang || 'auto',
      targetLang:config.translateTargetLang || 'ko'};
    drainPreview();
  }

  function fallbackTranslateSourceLang(clip) {
    const target = String(clip?.targetLang || config.translateTargetLang || 'ko').toLowerCase();
    return target === 'ko' ? 'en' : 'ko';
  }

  function resolveTranslateSourceLang(clip) {
    const explicit = String(clip?.sourceLang || config.translateSourceLang || 'auto').trim().toLowerCase();
    if (/^[a-z]{2}$/.test(explicit)) return explicit;
    return fallbackTranslateSourceLang(clip);
  }

  function resolveSTTLanguage(clip) {
    const engine=clip.engine || activeSttEngine;
    const source=String(clip.sourceLang || config.translateSourceLang || 'auto').trim().toLowerCase();
    const target=clip.targetLang || config.translateTargetLang || 'ko';
    // Native recognizers require a locale; Whisper supports automatic detection.
    if(engine==='whisper') {
      if(clip.mode==='translate' && target==='ko')return 'auto';
      return /^[a-z]{2}$/.test(source)?source:'auto';
    }
    return resolveTranslateSourceLang({...clip,sourceLang:source,targetLang:target});
  }

  function sttPayload(clip, overrideLang = '') {
    const lang=overrideLang || resolveSTTLanguage(clip);
    return {engine:clip.engine || activeSttEngine,fallback:config.sttFallback!==false,audio:uint8ArrayToBase64(clip.bytes),format:'pcm',sampleRate:16000,language:lang,
      realtime:clip.mode==='ai',sessionId:clip.sessionId,segmentId:clip.segmentId,revision:clip.revision || 0,
      startedAt:clip.startedAt,endedAt:clip.endedAt,triggerWord:clip.triggerWord || '',speechFilter:clip.speechFilter};
  }

  async function drainPreview() {
    if (previewBusy || previewRetryTimer || !pendingPreview || !isRecording) return;
    // Each engine has one recognizer. Wait for final inference, but allow a
    // newer preview after stt.recognized while the old result is being archived.
    if((activeFinalClip && !activeFinalClip.inferenceDone && activeFinalClip.engine===pendingPreview.engine) || pendingClips.some(clip=>!clip.inferenceDone && clip.engine===pendingPreview.engine))return;
    const snapshot = pendingPreview; pendingPreview = null; previewBusy = true;
    const controller = new AbortController();
    previewAbortController = controller;
    const timeout = setTimeout(() => controller.abort(),15000);
    try {
      const upload=audioUpload.prepare(snapshot);let uploadedBytes=upload.bytes.length;
      let payload = {...sttPayload({...snapshot,bytes:upload.bytes}),audioDelta:upload.audioDelta,audioOffset:upload.audioOffset};
      const sttSentAt=Date.now();
      let res = await gatewayFetch(`${config.serverUrl}/api/stt/preview`, {
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),
        body:JSON.stringify(payload),signal:controller.signal,
      });
      if(res.status===409){
        audioUpload.resync(snapshot);payload=sttPayload(snapshot);uploadedBytes+=snapshot.bytes.length;
        res=await gatewayFetch(`${config.serverUrl}/api/stt/preview`,{method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify(payload),signal:controller.signal});
      }
      if (res.status===429) {
        // The native engine may still be finalizing audio. Keep only the newest
        // snapshot and retry independently of transcript persistence/backlog.
        if(isRecording && sessionId===snapshot.sessionId && activeUtterance?.id===snapshot.segmentId){
          if(!pendingPreview)pendingPreview=snapshot;
          previewRetryTimer=setTimeout(()=>{previewRetryTimer=null;drainPreview();},300);
        }
        return;
      }
      if (!res.ok) throw new Error(`임시 전사 HTTP ${res.status}`);
      const data = await res.json();
      if (!isRecording || sessionId!==snapshot.sessionId || activeUtterance?.id!==snapshot.segmentId ||
          data.sessionId!==snapshot.sessionId || data.segmentId!==snapshot.segmentId || data.revision!==snapshot.revision) return;
      audioUpload.acknowledge(snapshot,data);
      if(snapshot.mode==='translate')translationMetrics.record(snapshot.segmentId,{stt_http_ms:Date.now()-sttSentAt,stt_server_ms:data.timings?.server_ms ?? data.timings?.stt_ms,stt_lock_ms:data.timings?.lock_ms,stt_engine_ms:data.timings?.engine_ms,upload_bytes:uploadedBytes,audio_bytes:snapshot.bytes.length});
      if(data.rejected){reconcileHintTranscript(snapshot.segmentId,'');hintGate.withdraw(snapshot.segmentId);previewIntent.reset();if(previewWakeId===snapshot.segmentId){previewWakeId='';previewWakePrompt='';}return;}
      if (!data.text?.trim()) return;
      const agreed = (data.engine==='apple' || data.engine?.startsWith('chrome-'))
        ? typeof data.stableText==='string' && typeof data.draftText==='string'
          ? {stable:data.stableText+(data.stableText && data.draftText?' ':''),draft:data.draftText}
          : {stable:'',draft:data.text}
        : activeUtterance.agreement.update(data.text);
      observeAIPreview(snapshot,data,agreed);
      draftUtteranceId = snapshot.segmentId;
      captionStable = agreed.stable; captionDraft = agreed.draft;
      if(snapshot.mode==='translate'){
        const translationText=localEngine()==='llm'?stableTranslationPreview(data.text,agreed.stable.trim()):data.text;
        if(translationText)enqueueTranslation(snapshot.segmentId,translationText,'',snapshot.sourceLang || 'auto',snapshot.targetLang,{sessionId:snapshot.sessionId,draft:true,stableText:agreed.stable.trim(),revision:snapshot.revision,detectedLanguage:data.language,sttEngine:data.engine || snapshot.engine,joinSentence:String(snapshot.engine||'').startsWith('chrome-') && isContinuousCaptureMode(),startedAt:snapshot.startedAt,endedAt:snapshot.endedAt});
      }
      transcriptStatusMessage = '';
      transcriptNotice('실시간 전사 중');
      renderLiveHUD();
    } catch (err) {
      if (isRecording && !controller.signal.aborted) setVoiceHint(`임시 자막 지연 · 최종 전사는 계속합니다 (${err.message})`);
    } finally {
      clearTimeout(timeout);
      if (previewAbortController === controller) previewAbortController = null;
      previewBusy = false;
      if (pendingClips.length) drainClips();
      if (pendingPreview) drainPreview();
    }
  }

  function enqueueClip(bytes, continuous, endedAt = Date.now(), segmentId = newID()) {
    abortPendingPreview();
    const mode = normalizeGlassesMode(config.glassesMode);
    const sourceLang = config.translateSourceLang || 'auto';
    const targetLang = config.translateTargetLang || 'ko';
    const triggerWord = mode==='ai' ? config.triggerWord : '';
    if(mode==='ai') {
      if(lastVoiceFrameAt>=endedAt-bytes.length/32 && lastVoiceFrameAt<=endedAt)voiceLatency.mark(segmentId,'last_voice',lastVoiceFrameAt);
      voiceLatency.mark(segmentId,'clip_end',endedAt);
      // Keep one active inference and only the newest waiting clip.
      pendingClips=pendingClips.filter(clip=>clip.mode!=='ai' || clip===activeFinalClip);
    }
    pendingClips.push({ epoch:recognitionEpoch, bytes, continuous, sessionId, segmentId, mode, sourceLang, targetLang,engine:activeSttEngine,
      startedAt:new Date(endedAt-bytes.length/32).toISOString(),endedAt:new Date(endedAt).toISOString(),
      triggerWord, speechFilter:config.speechFilter });
    if(mode!=='ai' && isRecording && (pendingClips.length>=MAX_PENDING_CLIPS || pendingClips.reduce((sum,clip)=>sum+clip.bytes.length,0)>=MAX_PENDING_AUDIO_BYTES)) {
      // Preserve every queued segment; bound growth by stopping capture, not deleting audio.
      stopVoiceRecording().then(()=>{
        if(!captureFailure && !isRecording) {
          setHUDNotice('처리 지연 · 녹음 일시정지');
          setVoiceHint('대기 음성을 전사·저장하고 있습니다. 처리 후 탭하면 다시 녹음합니다.');
        }
      }).catch(console.warn);
    }
    drainClips();
  }

  async function stopVoiceRecording(flush = true) {
    if(micStopPromise)return micStopPromise;
    if(!isRecording && !capturePreparing && !pendingMicOpen && !micMayBeOpen && !micCloseUncertain && !pendingMicCloses.size)return;
    hintGate.reset();
    previewIntent.reset();previewWakeId='';previewWakePrompt='';
    const hadCapture=isRecording;
    const continuousCapture = isContinuousCaptureMode();
    ++captureGeneration;
    captureAbort?.abort();capturePreparing=false;
    isRecording=false;resetCaptureTimers();
    const segmentId=activeUtterance?.id || newID();
    if(!flush)stopChromeSegment(segmentId);
    activeUtterance=null;abortPendingPreview();
    const clip=hadCapture ? (continuousCapture?segmenter?.flush():mergePCM(audioBuffer)) : null;
    if(pendingTrigger && (!flush || !clip || clip.length<3200))pendingTrigger=false;
    audioBuffer=[];
    if((!flush || !clip) && liveTranslations.hasDraft()){liveTranslationDraft=false;liveTranslations.reset();}
    setHUDNotice('마이크 정지 중');
    setVoiceHint('마이크를 끄고 있습니다. 남은 전사는 계속 처리합니다.');
    els.voiceBtn?.setAttribute?.('aria-label','마이크 정지 중');
    const stopping=(async()=>{
      try {
        if(micMayBeOpen || pendingMicOpen || micCloseUncertain)await confirmMicrophoneOff();
        else showMicStopped();
      } catch(err) {
        micCloseUncertain=true;
        setHUDNotice('마이크 종료 확인 실패');
        setVoiceHint('다시 탭해 종료를 재시도하거나 앱을 완전히 닫아주세요.');
        updateHUD(`마이크 종료를 확인하지 못했습니다.\n${err.message}\n다시 탭하거나 앱을 완전히 닫아주세요.`);
      }
    })();
    micStopPromise=stopping;
    try {await stopping;} finally {if(micStopPromise===stopping)micStopPromise=null;}
    if(flush && clip?.length>=3200)enqueueClip(clip,continuousCapture,recordingStarted+recordedBytes/32,segmentId);
    if(!micCloseUncertain && !pendingMicOpen && !pendingMicCloses.size)showMicStopped();
  }

  async function drainClips() {
    if (previewBusy) abortPendingPreview();
    if (draining) return;
    draining = true;
    try {
      while (pendingClips.length) {
        const clip = pendingClips[0];
        activeFinalClip=clip;
        if(clip.mode==='ai' && Date.now()-Date.parse(clip.endedAt)>20000){hintGate.withdraw(clip.segmentId);if(previewWakeId===clip.segmentId)previewWakeId='';pendingClips.shift();continue;}
        let data;
        try {
          data = await processAudioTranscription(clip);
          if(clip.mode==='ai')voiceLatency.mark(clip.segmentId,'stt_final');
        } catch (err) {
          // A mode switch can clear the old request while new audio is queued.
          // Never shift a new segment in response to an old cancellation/failure.
          if(!pendingClips.includes(clip))continue;
          if(err.busy){
            clip.busyAttempts=(clip.busyAttempts || 0)+1;
            if(clip.busyAttempts<=8){
              if(clip.epoch===recognitionEpoch){setVoiceHint('음성 엔진 처리 대기 · 자동 재시도 중');setHUDNotice('전사 처리 대기');}
              await new Promise(resolve=>setTimeout(resolve,Math.min(1000,250*clip.busyAttempts)));continue;
            }
          }
          if(clip.mode==='ai') {
            hintGate.withdraw(clip.segmentId);if(previewWakeId===clip.segmentId)previewWakeId='';
            pendingClips.splice(pendingClips.indexOf(clip),1);
            if(clip.epoch===undefined || clip.epoch===recognitionEpoch){
              els.hintStatus.textContent=`음성 처리 지연 · 해당 구간 생략, 새 대화 계속 (${err.message})`;
              setHUDNotice('지난 음성 생략 · 계속 듣는 중');
              setVoiceHint('실시간 AI · 처리하지 못한 구간은 버리고 최신 대화를 듣습니다.');
            }
            // Avoid a hot loop on repeated server failures without stopping capture.
            await new Promise(resolve=>setTimeout(resolve,200));
            continue;
          }
          clip.attempts=(clip.attempts || 0)+1;
          if(err.retryable!==false && clip.attempts<=3) {
            setHUDNotice(`전사 재시도 ${clip.attempts}/3 · 음성 보관 중`);
            setVoiceHint(`일시적인 전사 지연 · ${pendingClips.length}개 보관 · 자동 재시도합니다.`);
            await new Promise(resolve=>setTimeout(resolve,300 * 2**(clip.attempts-1)));
            continue;
          }
          throw err;
        }
        if(!pendingClips.includes(clip))continue;
        pendingClips.splice(pendingClips.indexOf(clip),1);
        if (draftUtteranceId === clip.segmentId) { captionStable = captionDraft = ''; draftUtteranceId = ''; }
        if(previewWakeId===clip.segmentId)previewWakeId='';
        if(data.rejected || !data.text?.trim()){if(clip.mode==='ai')reconcileHintTranscript(clip.segmentId,'');hintGate.withdraw(clip.segmentId);}
        if (data.rejected) { setVoiceHint('불확실하거나 반복되는 인식 결과 제외 · 다시 말씀하세요'); renderLiveHUD(); continue; }
        if (!data.text?.trim()) { setVoiceHint('음성 필터 · 인식할 말소리 없음'); renderLiveHUD(); continue; }
        const text = data.text.trim();

        const time = new Date(clip.startedAt).toLocaleTimeString();
        transcriptText += `[${time}] ${text}\n`;
        els.transcriptLog.textContent = transcriptText;
        if(headPaused || clip.epoch!==undefined && clip.epoch!==recognitionEpoch)continue;
        if(clip.mode!=='translate' || !(Date.parse(clip.startedAt)<translationDirection.latest)){captionFinal=text;transcriptStatusMessage='';}
        transcriptContext.push(text);if(transcriptContext.length>8)transcriptContext.shift();
        const clipMode = normalizeGlassesMode(clip.mode || config.glassesMode);
        const clipTargetLang = clip.targetLang || config.translateTargetLang || 'ko';
        let prompt = '';
        let directIntent={direct:false,search:false};
        let chatSession=false,wakeToken;
        if(clipMode==='ai') {
          reconcileHintTranscript(clip.segmentId,text);
          const wasWaitingForQuestion=pendingTrigger;
          const route=routeAIUtterance(data,{waiting:pendingTrigger,name:triggerLabels()[0] || data.matchedTrigger || 'AI'});
          pendingTrigger=route.waiting;
          if(route.kind==='ack'){
            voiceWakeJobs.clear();previewWakeId='';previewWakePrompt='';
                  const wakeName=triggerLabels()[0] || data.matchedTrigger || 'AI';
            chatEpoch++;hintGate.reset();aiAbortController?.abort();pendingAI.length=0;
            clearHintStack();hintLifetime.clear();answerAutoCloseEligible=false;answerVoicePrompt='';answerIsWakeInput=true;answerTitle=`${wakeName} 질문`;answerText=route.ack;els.chatOutput.textContent=route.ack;
            els.hintStatus.textContent='호출어 확인 · 다음 문장을 AI 채팅 질문으로 받습니다';
            setHUDNotice('질문 대기');renderLiveHUD();continue;
          }
          if(route.kind==='chat'){
            const wakeName=triggerLabels()[0] || data.matchedTrigger || 'AI';
            previewWakeId='';previewWakePrompt='';
            hintLifetime.clear();clearHintStack();answerAutoCloseEligible=false;
            answerIsWakeInput=true;answerTitle=`${wakeName} 질문`;answerVoicePrompt='';answerText=`${wakeName}: ${route.prompt}`;els.chatOutput.textContent=answerText;
            chatSession=true;prompt=route.prompt;wakeToken=++nextWakeJob;voiceWakeJobs.set(wakeToken,prompt);
            directIntent={...directQuestionIntent(prompt),direct:true};
            hintGate.reset();if(activeAIJob?.hint){activeAIJob.cancelReason='chat_priority';aiAbortController?.abort();}
            els.hintStatus.textContent='AI 채팅 · 이전 질문·답변 참조';
          }else if(isRecording && normalizeGlassesMode(config.glassesMode)==='ai'){
            // Ambient questions are observed conversation too. Only the wake
            // route (or typed input) authorizes a conversational AI response.
            hintGate.observe(text,{id:clip.segmentId,at:Date.parse(clip.endedAt)});
          }
        }else if(clipMode==='translate'){prompt=text;pendingTrigger=false;}
        if (prompt) {
          const context=chatSession?'':clip.continuous?transcriptContext.slice(0,-1).slice(-2).join('\n').slice(-800):'';
          if(clipMode==='translate')enqueueTranslation(clip.segmentId,prompt,context,clip.sourceLang || 'auto',clipTargetLang,{sessionId:clip.sessionId,detectedLanguage:data.language,sttEngine:data.engine || clip.engine,joinSentence:String(clip.engine||'').startsWith('chrome-') && clip.continuous,startedAt:clip.startedAt,endedAt:clip.endedAt});
          else enqueueAI(clip.segmentId,prompt,context,clipMode,clipTargetLang,directIntent.direct?{directQuestion:true,forceSearch:directIntent.search,chatSession,wakeToken,traceId:clip.segmentId,chatEpoch:chatSession?chatEpoch:undefined}:{});
        } else {
          if (pendingTrigger) setHUDNotice('AI 호출됨 · 질문을 말씀하세요');
          else transcriptNotice('전사 저장됨');
          renderLiveHUD();
        }
      }
    } catch (err) {
      await stopVoiceRecording(false).catch(console.warn);
      captureFailure = `전사/저장 실패: ${err.message}\n처리하지 못한 음성 ${pendingClips.length}개를 앱 안에 보관 중입니다. 다시 탭하면 재시도합니다. 앱을 닫으면 미저장 음성은 사라집니다.`;
      setHUDNotice('전사 오류 · 미저장 음성 대기'); renderLiveHUD();
      setVoiceHint('오류로 녹음 중지 · 서버 확인 후 다시 시작');
    } finally {
      activeFinalClip=null;
      draining = false;
      if(!isRecording && !pendingClips.length)pendingTrigger=false;
      renderLiveHUD();
      if (!isRecording && !pendingClips.length && !micStopPromise && !pendingMicOpen && !micCloseUncertain && !pendingMicCloses.size) setVoiceHint('마이크 꺼짐 · 전사 저장 완료');
      if (pendingPreview) drainPreview();
    }
  }

  async function processAudioTranscription(clip) {
    const controller = new AbortController();
    clipAbortController = controller;
    let timedOut=false;
    // Worker inference allows 90s plus cold-start time; a 20s client limit caused
    // abandoned requests and repeated inference under load.
    const timeout = setTimeout(() => {timedOut=true;controller.abort();}, clip.mode==='ai'?12000:120000);
    try {
      if(clip.mode==='ai')voiceLatency.mark(clip.segmentId,'stt_sent');
      const sttSentAt=Date.now();
      clip.inferenceDone=false;
      const payload = {...sttPayload(clip),stream:clip.mode==='translate'};
      let res = await gatewayFetch(`${config.serverUrl}/api/stt/transcribe`, {
        method: 'POST', headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify(payload), signal: controller.signal,
      });
      if (!res.ok) {
        const err=new Error(`HTTP ${res.status}: ${(await res.text()).slice(0,300)}`);
        err.busy=res.status===429;
        err.retryable=[408,425,429].includes(res.status) || res.status>=500;
        throw err;
      }
      const data = await readFinalTranscription(res,{signal:controller.signal,onRecognized:recognized=>{
        if(!recognized || recognized.sessionId!==clip.sessionId || recognized.segmentId!==clip.segmentId)return;
        clip.inferenceDone=true;
        translationMetrics.record(clip.segmentId,{stt_first_result_ms:Date.now()-sttSentAt});
        if(clip.epoch===recognitionEpoch && sessionId===clip.sessionId && normalizeGlassesMode(config.glassesMode)==='translate' && recognized.text?.trim() && !recognized.rejected){
          enqueueTranslation(clip.segmentId,recognized.text.trim(),transcriptContext.slice(-2).join('\n').slice(-800),clip.sourceLang || 'auto',clip.targetLang || 'ko',
            {sessionId:clip.sessionId,detectedLanguage:recognized.language,sttEngine:recognized.engine || clip.engine,joinSentence:String(clip.engine || '').startsWith('chrome-') && clip.continuous,startedAt:clip.startedAt,endedAt:clip.endedAt});
        }
        if(pendingPreview)drainPreview();
      }});
      if(data.warning)els.sttEngineStatus.textContent=data.warning;
      if(clip.mode==='translate')translationMetrics.record(clip.segmentId,{stt_http_ms:Date.now()-sttSentAt,stt_server_ms:data.timings?.server_ms ?? data.timings?.stt_ms,stt_lock_ms:data.timings?.lock_ms,stt_engine_ms:data.timings?.engine_ms,save_ms:data.timings?.save_ms,upload_bytes:clip.bytes.length,audio_bytes:clip.bytes.length});
      if (data.text?.trim() && !data.saved) {
        const err=new Error('서버에서 전사 저장을 확인하지 못했습니다. 서버를 업데이트하세요.');
        err.retryable=false;throw err;
      }
      return data;
    } catch(err) {
      if(timedOut){const timeoutError=new Error('전사 서버 응답 시간 초과 · 음성 보관 중');timeoutError.retryable=true;throw timeoutError;}
      throw err;
    } finally {
      clearTimeout(timeout);
      if (clipAbortController === controller) clipAbortController = null;
    }
  }

  function openTranscriptModal() {
    recordReturnFocus = document.activeElement;
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    els.transcriptModal.hidden = false;
    els.btnCloseTranscripts.focus();
  }
  function closeTranscriptModal() {
    els.transcriptModal.hidden = true;
    document.body.style.overflow = previousBodyOverflow;
    recordReturnFocus?.focus?.();
  }

  async function loadTranscripts() {
    const date = els.transcriptDate.value;
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const res = await gatewayFetch(`${config.serverUrl}/api/stt/transcripts?date=${encodeURIComponent(date)}&tz=${encodeURIComponent(tz)}`, {headers:getAuthHeaders()});
    if (!res.ok) throw new Error(`전사 조회 실패: HTTP ${res.status}`);
    const data = await res.json();
    transcriptText = data.entries.map(entry => `[${new Date(entry.startedAt).toLocaleTimeString()} – ${new Date(entry.endedAt).toLocaleTimeString()}] ${entry.text}`).join('\n');
    els.transcriptLog.textContent = transcriptText || '저장된 전사 기록이 없습니다.';
  }

  function enqueueTranslation(id,prompt,context,source,target,options={}){
    if(options.sessionId && options.sessionId!==sessionId)return;
    if(options.sttEngine!=='whisper')source=resolveTranslateSourceLang({sourceLang:source,targetLang:target});
    const direction=translationDirection.route({source,target,detected:options.detectedLanguage,engine:options.sttEngine,text:prompt,startedAt:options.startedAt});
    if(!direction){els.chromeTranslationStatus.textContent='상대 언어를 먼저 인식하거나 출발 언어를 선택하세요';return;}
    if(direction.changed){
      liveTranslations.reset();translationSentence.reset();translationHistory.reset();
      stableTranslation.reset();llmStableTranslation.reset();
      translationDisplay={translation:'',positive:'',negative:''};answerText='';
      els.chatOutput.textContent='';renderLiveHUD();
    }
    const job={id,prompt,context:direction.changed?'':context,source:direction.source,target:direction.target,engine:localEngine(),...options};
    const sentence=options.joinSentence?translationSentence.update(job):job;
    if(sentence)liveTranslations.submit(sentence);
  }

  function translationStale(job){return job.replyOnly && job.replyVersion!==undefined?job.replyVersion!==translationReplyVersion:job.translationToken && job.translationToken!==translationToken;}

  function enqueueAI(id, prompt, context, mode = config.glassesMode, targetLang = config.translateTargetLang, options = {}) {
    if(options.replyOnly && pendingAI.some(job=>job.mode==='translate' && !job.replyOnly))return;
    if (knownAIJobs.has(id)){finishWakeJob(options.wakeToken);return;}
    knownAIJobs.add(id);
    if (knownAIJobs.size>1000) knownAIJobs.delete(knownAIJobs.values().next().value);
    const normMode = normalizeGlassesMode(mode);
    const normLang = targetLang || 'ko';
    if(options.chatSession){
      for(const dropped of pendingAI)if(dropped.hint){auditHint(dropped,'skipped',{reason:'pending_replaced'});hintGate.complete(dropped);}
      pendingAI.splice(0,pendingAI.length,...pendingAI.filter(job=>job.chatSession));
      if(pendingAI.length>=8){finishWakeJob(options.wakeToken);els.hintStatus.textContent='AI 채팅 질문 8개 대기 중 · 답변 후 다시 말씀하세요';return;}
    }else if(pendingAI.some(job=>job.chatSession)){
      if(options.hint)hintGate.complete(options);
      return;
    }
    if (pendingAI.length > 0 && !options.chatSession) {
      const last = pendingAI[pendingAI.length - 1];
      if(options.hint){for(const dropped of pendingAI)if(dropped.hint){auditHint(dropped,'skipped',{reason:'pending_replaced'});hintGate.complete(dropped);}pendingAI.length=0;}
      else if (last.mode === normMode && last.targetLang === normLang && !last.hint && !last.replyOnly && !options.replyOnly && !options.translationToken && !last.translationToken) {
        last.prompt = `${last.prompt}\n${prompt}`.trim();
        if (context) last.context = context;
        return;
      }
      pendingAI.length = 0;
    }
    if(options.traceId)voiceLatency.mark(options.traceId,'chat_queued');
    pendingAI.push({...options,chatEpoch:options.chatEpoch || (normMode==='ai'?chatEpoch:undefined),translationHistoryId:options.translationHistoryId || (normMode==='translate'?id:undefined),prompt, context, mode: normMode, targetLang: normLang});
    drainAI();
  }
  async function drainAI() {
    if (processingAI || !pendingAI.length) return;
    clearTimeout(hintDrainTimer);hintDrainTimer=null;
    const wait=pendingAI[0].hint?hintGate.displayDelay():0;
    if(wait>0){
      els.hintStatus.textContent='새 힌트 대기 · 표시 간격 조절 중';
      hintDrainTimer=setTimeout(()=>{hintDrainTimer=null;drainAI();},wait);
      return;
    }
    processingAI = true; clearTimeout(answerTimer); stopAIAutoScroll();
    aiAbortedByModeSwitch = false;
    try {
      while (pendingAI.length) {
        if(pendingAI[0].hint && hintGate.displayDelay()>0)break;
        const job = pendingAI.shift();
        if(job.hint && !hintGate.relevant(job,job.hintPlan?.topic)){auditHint(job,'skipped',{reason:hintGate.rejectionReason(job)});hintGate.complete(job);continue;}
        if(job.hint && !hintGate.beginGeneration(job)){auditHint(job,'skipped',{reason:hintGate.rejectionReason(job)});hintGate.complete(job);continue;}
        activeAIJob=job;
        if(job.traceId)voiceLatency.mark(job.traceId,'chat_start');
        if(translationStale(job) || job.chatEpoch && job.chatEpoch!==chatEpoch){finishWakeJob(job.wakeToken);continue;}
        if(job.mode==='translate')translationFailure='';
        aiScrollOffset = 0;
        aiPhaseTitle = job.mode === 'translate' ? '번역 및 답변 생성 중' : '생각 중';
        aiPhaseDetail = '';
        aiReasoningPreview = '';
        if(!job.hint && !captureFailure)setHUDNotice(isRecording ? 'AI 답변 중 · 전사 계속' : '녹음 정지 · AI 답변 중');
        renderLiveHUD();
        try{await queryDKSTChat(job.prompt, job.context, job.mode, job.targetLang,job);}
        finally{finishWakeJob(job.wakeToken);activeAIJob=null;}
      }
    } finally {
      activeAIJob=null;
      processingAI = false;
      stopThinkingAnimation();
      if (!aiAbortedByModeSwitch) {
        const win = sliceHUDWindow(aiPresentationText(), 0, aiVisibleLines(), AI_BOX_WIDTH_PX);
        aiScrollOffset = 0;
        const holdMs = Math.max(8000, AI_SCROLL_START_MS + win.maxOffset * AI_SCROLL_LINE_MS + 4000);
        answerHoldUntil = Date.now() + holdMs;
        transcriptNotice(isRecording ? (isContinuousCaptureMode() ? '실시간 전사 중' : '녹음 중') : '전사 저장됨');
        renderLiveHUD();
        if (win.maxOffset > 0 && normalizeGlassesMode(config.glassesMode)!=='translate') {
          startAIAutoScroll(win.maxOffset);
        }
        answerTimer = setTimeout(() => { answerHoldUntil=0; if(isRecording)renderLiveHUD(); }, holdMs);
      }
      aiAbortedByModeSwitch = false;
      if (pendingAI.length) drainAI();
    }
  }

  function finalChatText(data) {
    if(typeof data.final_assistant_content==='string')return data.final_assistant_content;
    const output=data.result?.output || data.output;
    if(!Array.isArray(output))return null;
    const messages=output.filter(item=>item.type==='message' && typeof item.content==='string');
    return messages.length ? messages.map(item=>item.content).join('') : null;
  }

  // --- LLM Chat Execution ---
  async function queryDKSTChat(prompt, conversationContext = '', mode = config.glassesMode, targetLang = config.translateTargetLang, options = {}) {
    const activeMode = normalizeGlassesMode(mode);
    const modelConfig={model:config.model,roleModels:normalizeRoleModels(config.roleModels)};
    const requestChatScope=chatScope();
    const chatContext=options.chatSession?chatMemory.context(requestChatScope):'';
    let enrichmentPlan=null;let hintProof=null;
    const recordHintProof=proof=>{hintProof=proof;if(!proof)return;const records=Array.isArray(proof.records)?proof.records:[];auditHint(options,'decision_response',{reason:'source_records',topic:proof.topic || '',text:JSON.stringify({mode:proof.mode,source:proof.source,task:String(proof.task || enrichmentPlan?.question || '').slice(0,300),model:proof.model || '',promptRevision:proof.prompt_revision || '',records:records.slice(0,4).map((record,index)=>({record:index+1,title:String(record.title || '').slice(0,100),sentences:record.spans?.length || 0}))})});};
    let repliesReplaced=false;
    let receivedVisibleAnswer=false;
    let chromeText=options.chromeTranslationText || '';let chromeSource=options.translationSource || resolveTranslateSourceLang({targetLang});
    let cueDisplayed=false;let cueTopic='';let hintSkipReason='not_completed';
    aiStatusError='';
    const controller = new AbortController();
    aiAbortController = controller;
    const deadline=chatDeadline(controller,{chatSession:options.chatSession===true,hint:options.hint===true,setTimer:setTimeout,clearTimer:clearTimeout});
    if (!options.hint) {
      isThinking = true;
      stopAIAutoScroll();
      aiPhaseTitle = activeMode === 'translate' ? '번역 및 답변 생성 중' : '생각 중';
      aiPhaseDetail = '대화 설정 확인 중';
      aiReasoningPreview = '';
      aiReasoningStartMs = 0;
      els.chatQuestion.textContent=prompt;
      if(!chromeText)els.chatOutput.textContent=answerText;
      startThinkingAnimation();
      renderLiveHUD();
    }
    try {
      if(activeMode==='translate' && config.chromeTranslation!==false && !chromeText && !options.chromeAttempted){
        aiPhaseTitle=localLabel()+' 로컬 번역 중';renderLiveHUD();
        const local=await translateLocal({text:prompt,source:chromeSource,target:targetLang || config.translateTargetLang},controller.signal);
        if(local.ok){
          chromeText=local.text;translationDisplay={...translationDisplay,translation:chromeText};answerText=chromeText;
          translationHistory.update(options.translationHistoryId,chromeText,{final:true,revision:options.translationRevisionId || options.translationHistoryId});
          els.chatOutput.textContent=translationPhoneText(translationDisplay);
          els.chromeTranslationStatus.textContent=localLabel()+' 로컬 번역 완료 · 답변 예시 생성 중';
          aiPhaseTitle='긍정·부정 답변 생성 중';renderLiveHUD();
        }else els.chromeTranslationStatus.textContent=`LLM 번역 사용 · ${local.reason}`;
      }
      const chatPayload=request=>({
        model:resolveRoleModel(llmRequestRole({mode:activeMode,chatSession:options.chatSession,translated:!!chromeText}),modelConfig),
        stream:true,glasses_request:{version:1,...request}
      });
      if(options.hint) {
        aiPhaseTitle='정보 조회 중';aiPhaseDetail='';setHUDNotice();
        els.hintStatus.textContent='보충할 정보 찾는 중';
        let cue=options.hintPlan;
        if(!cue){
          const planRes=await gatewayFetch(`${config.serverUrl}/api/chat`,{
            method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),signal:controller.signal,
            body:JSON.stringify(chatPayload({task:'plan',text:prompt,context:conversationContext,category:options.category,preferWeb:config.hintWebSearch!==false,keyword:options.keyword || ''}))
          });
          cue=parseCueResponse(await abortableWait(readHintPlanResponse(planRes),controller.signal),[conversationContext,prompt].join('\n'));
        }
        if(options.chatEpoch && options.chatEpoch!==chatEpoch)return;
        if(cue)els.hintStatus.textContent=`주제: ${cue.topic} · ${cue.question || '내부 지식 힌트'}`;
        if(cue?.fact)cue={...cue,fact:undefined,question:cue.question || `${cue.topic}의 확인 가능한 설명`,source:cue.source || 'internal',knownFacts:cue.knownFacts || [prompt]};
        if(cue && !isInformationHintPlan(cue)){hintSkipReason='conversation_summary_disabled';els.hintStatus.textContent='보탤 정보 없음 · 계속 듣는 중';return;}
        enrichmentPlan=cue;
        if(!enrichmentPlan || !hintGate.relevant(options,enrichmentPlan.topic)){
          hintSkipReason=enrichmentPlan?hintGate.rejectionReason(options):'no_research_plan';
          els.hintStatus.textContent='새로 보탤 도움 없음 · 계속 듣는 중';return;
        }
        aiPhaseTitle=enrichmentPlan.source==='web'?'웹에서 근거 찾는 중':enrichmentPlan.source==='memory'?'기억에서 근거 찾는 중':'내부 지식에서 정보 찾는 중';
        aiPhaseDetail=enrichmentPlan.topic || '';setHUDNotice();
        els.hintStatus.textContent=enrichmentPlan.source==='web'?'웹에서 근거 찾는 중':enrichmentPlan.source==='memory'?'기억에서 근거 찾는 중':'내부 지식에서 정보 찾는 중';
      }
      if(options.hint && enrichmentPlan?.source==='web'){els.hintStatus.textContent='선별한 내용 웹검색 중';}
      const separateLLMReplies=activeMode==='translate' && !chromeText && resolveRoleModel('translation',modelConfig)!==resolveRoleModel('translationReply',modelConfig);
      const request={
        task:options.hint?'answer':separateLLMReplies?'translation_only':chromeText?'translation_replies':activeMode==='translate'?'translation':'chat',
        text:prompt,source:chromeSource,target:targetLang || config.translateTargetLang || 'ko',translated:chromeText,
        concise:config.conciseMode,chatSession:options.chatSession===true,
        context:options.chatSession?chatContext:conversationContext,
        directQuestion:options.directQuestion===true,forceSearch:options.forceSearch===true,
      };
      if(options.hint)Object.assign(request,{plan:enrichmentPlan,observed:options.observedText || prompt,
        anchor:Array.from(enrichmentPlan.evidence || options.focusText || prompt).slice(0,160).join(''),preferWeb:config.hintWebSearch!==false});
      const payload=chatPayload(request);
      if(!payload.model)throw Error('설정에서 기본 모델 또는 해당 역할의 LLM 모델을 선택하세요.');
      if (!options.hint) {
        aiPhaseTitle = activeMode === 'translate' ? '번역 및 답변 생성 중' : '생각 중';
        aiPhaseDetail = '';
        if(!chromeText)els.chatOutput.textContent = answerText;
        renderLiveHUD();
      }
      if(options.traceId)voiceLatency.mark(options.traceId,'chat_sent');
      if(options.hint && !hintGate.beginGeneration(options)){hintSkipReason='canceled_before_generation';els.hintStatus.textContent='힌트 요청 전 취소 · 새 발화 대기';return;}
      if(options.hint)auditHint(options,'generation',{reason:'request_sent',text:JSON.stringify({model:payload.model,task:request.task}),topic:enrichmentPlan.topic,kind:enrichmentPlan.kind || options.hintPlan?.kind || 'key',source:enrichmentPlan.source});
      let res = await gatewayFetch(`${config.serverUrl}/api/chat`, {
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify(payload), signal: controller.signal
      });
      if(options.hint && res.status===422){const detail=await res.text();if(detail.includes('hint_task_not_actionable')){hintSkipReason='unresolved_hint_task';return;}throw Error(detail);}
      if(!res.ok && res.status!==401 && res.status!==403 && res.status!==424){
        const detail=(await res.text()).trim().slice(0,500);
        if(isModelUnloadedError(res.status,detail)){
          aiPhaseTitle='모델 로딩 중';aiPhaseDetail=payload.model;renderLiveHUD();
          const loaded=await ensureModelLoaded(payload.model,controller.signal,true);
          if(loaded){
            aiPhaseTitle=activeMode==='translate'?'번역 및 답변 생성 중':'생각 중';aiPhaseDetail='';renderLiveHUD();
            res=await gatewayFetch(`${config.serverUrl}/api/chat`,{
              method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify(payload),signal:controller.signal
            });
          }else{
            throw new Error('모델 자동 로딩 실패 · '+(modelLoadErrors.get(modelLoadKey(payload.model)) || detail));
          }
        }else{
          throw new Error(`대화 생성 실패: HTTP ${res.status}${detail ? ' · '+detail : ''}`);
        }
      }
      if(options.hint && res.status===424){hintSkipReason='source_lookup_failed';
        const lookupDetail=(await res.text()).trim().slice(0,300);
        auditHint(options,'decision_response',{reason:'source_lookup_detail',topic:enrichmentPlan?.topic || '',text:lookupDetail});
        if(options.topicVersion===hintGate.topicVersion)hintGate.currentTopic='';
        els.hintStatus.textContent='명칭·출처를 확인하지 못해 힌트 생략 · 계속 듣는 중';
        return;
      }
      if(!res.ok) {
        const detail=(await res.text()).trim().slice(0,500);
        throw new Error(`대화 생성 실패: HTTP ${res.status}${detail ? ' · '+detail : ''}`);
      }

      let rawAnswer = '';
      let rawReasoning = '';
      function setVisibleAnswer(raw,commitReplies=false) {
        if(translationStale(options) || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
        if(activeMode==='translate') {
          const previousTranslation=translationDisplay.translation;
          const parsedTranslation=parseTranslationDisplay(raw);
          if(options.replyOnly)parsedTranslation.translation='';
          if(commitReplies && options.replyOnly && targetLang!=='ko' && targetLang!=='en' && parsedTranslation.pronunciation){
            translationDisplay.pronunciation=parsedTranslation.pronunciation;
            const spoken=pronunciationText(translationDisplay.translation,parsedTranslation.pronunciation,targetLang);
            translationHistory.update(options.translationHistoryId,spoken,{final:true,revision:options.translationRevisionId || options.translationHistoryId});
          }
          if(separateLLMReplies){parsedTranslation.positive='';parsedTranslation.negative='';}
          if(commitReplies && !chromeText && (!parsedTranslation.translation || ['…','...'].includes(parsedTranslation.translation)))throw Error('LLM에서 번역문을 받지 못했습니다.');
          translationDisplay=mergeTranslationDisplay(translationDisplay,parsedTranslation,{commitReplies});
          if(commitReplies && hasCompleteTranslationReplies(parsedTranslation))repliesReplaced=true;
          if(!translationDisplay.translation && config.chromeTranslation!==false)translationDisplay.translation=previousTranslation;
          if(chromeText && !options.replyOnly)translationDisplay.translation=chromeText;
          answerText=translationDisplay.translation;
          if(answerText!==previousTranslation)translationPaint={id:options.translationRevisionId || options.translationHistoryId || String(options.translationToken || 'fallback'),at:Date.now(),acked:false,content:''};
          if(!options.replyOnly && (chromeText || parsedTranslation.translation))translationHistory.update(options.translationHistoryId,answerText,{final:commitReplies || !!chromeText,revision:options.translationRevisionId || options.translationHistoryId});
          els.chatOutput.textContent=translationPhoneText(translationDisplay);
        } else {
          const visible=limitHUDResponse(stripSourcesForGlasses(raw));
          if(visible){receivedVisibleAnswer=true;if(options.traceId)voiceLatency.mark(options.traceId,'first_text');}
          if(visible || commitReplies){clearHintStack();hintLifetime.clear();answerAutoCloseEligible=false;answerText=visible;answerTitle=prompt.replace(/\s+/gu,' ').trim() || 'AI 답변';answerVoicePrompt=options.wakeToken!==undefined?prompt:'';}
          els.chatOutput.textContent=answerText;
        }
      }

      function applyStreamStatusEvent(type, parsed) {
        if(type==='hint.grounding' && options.hint){if(hintProof===null)recordHintProof(parsed.grounding);return true;}
        if (type === 'model_load.start') {
          aiPhaseTitle = '모델 로딩 중';
          aiPhaseDetail = '모델을 메모리에 불러오는 중';
          return true;
        }
        if (type === 'model_load.progress') {
          const pct = typeof parsed.progress === 'number' ? Math.round(parsed.progress * 100) : null;
          aiPhaseTitle = pct !== null ? `모델 로딩 중 (${pct}%)` : '모델 로딩 중';
          aiPhaseDetail = '모델을 메모리에 불러오는 중';
          return true;
        }
        if (type === 'model_load.end') {
          aiPhaseTitle = '모델 로드 완료';
          aiPhaseDetail = '프롬프트 처리 중';
          return true;
        }
        if (type === 'prompt_processing.progress') {
          const pct = typeof parsed.progress === 'number' ? Math.round(parsed.progress * 100) : null;
          aiPhaseTitle = pct !== null ? `프롬프트 처리 중 (${pct}%)` : '프롬프트 처리 중';
          aiPhaseDetail = '';
          return true;
        }
        if (type === 'skill.applied') {
          const names = Array.isArray(parsed.skills)
            ? parsed.skills.map(s => String(s?.display_name || s?.name || s?.namespace || '').trim()).filter(Boolean)
            : [];
          aiPhaseTitle = '스킬 적용됨';
          aiPhaseDetail = names.join(', ');
          return true;
        }
        if (type === 'reasoning.start') {
          if (!aiReasoningStartMs) aiReasoningStartMs = Date.now();
          aiPhaseTitle = '생각 중';
          aiPhaseDetail = '';
          return true;
        }
        if (type === 'reasoning.delta') {
          if (!aiReasoningStartMs) aiReasoningStartMs = Date.now();
          const chunk = parsed.content || parsed.reasoning_content || parsed.text || '';
          rawReasoning += chunk;
          const elapsedMs = Number(parsed.total_elapsed_ms || parsed.elapsed_ms || (Date.now() - aiReasoningStartMs));
          const sec = Number.isFinite(elapsedMs) && elapsedMs >= 1000 ? Math.round(elapsedMs / 1000) : 0;
          aiPhaseTitle = sec > 0 ? `생각 중 (${sec}초)` : '생각 중';
          aiPhaseDetail = '';
          aiReasoningPreview = rawReasoning;
          return true;
        }
        if (type === 'reasoning.end') {
          const elapsedMs = Number(parsed.total_elapsed_ms || parsed.elapsed_ms || (aiReasoningStartMs ? Date.now() - aiReasoningStartMs : 0));
          const sec = Number.isFinite(elapsedMs) && elapsedMs >= 1000 ? Math.round(elapsedMs / 1000) : 0;
          aiPhaseTitle = sec > 0 ? `${sec}초 동안 생각함` : '생각 완료';
          aiPhaseDetail = '답변 작성 중';
          aiReasoningPreview = '';
          return true;
        }
        if (type.startsWith('tool_call.')) {
          const toolInfo = formatToolStatusText({ ...parsed, type });
          aiPhaseTitle = toolInfo.title;
          aiPhaseDetail = toolInfo.detail;
          aiReasoningPreview = '';
          return true;
        }
        if (type === 'generation.phase') {
          if (parsed.phase === 'thinking') {
            aiPhaseTitle = '생각 중';
            return true;
          }
          if (parsed.phase === 'tool_call' && parsed.tool) {
            const toolInfo = formatToolStatusText({ ...parsed, type: 'tool_call.start' });
            aiPhaseTitle = toolInfo.title;
            aiPhaseDetail = toolInfo.detail;
            return true;
          }
          if (parsed.phase === 'answering') {
            aiPhaseTitle = '답변 작성 중';
            aiPhaseDetail = '';
            aiReasoningPreview = '';
            return true;
          }
        }
        return false;
      }

      if ((res.headers?.get('content-type')?.includes('text/event-stream') || config.connectionMode === 'sse') && res.body) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffered = '';
        let eventName='';
        let streamComplete=false;
        function consumeLine(line) {
          if(streamComplete)return;
          if(options.chatEpoch && options.chatEpoch!==chatEpoch)return;
          line=line.replace(/\r$/,'');
          if(!line){eventName='';return;}
          if(line.startsWith('event:')){eventName=line.slice(6).trim();return;}
          if (!line.startsWith('data:')) return;
          const dataStr = line.slice(5).trim();
          if (!dataStr) return;
          if(dataStr==='[DONE]'){streamComplete=true;return;}
          if(eventName==='error')throw new Error(dataStr);
          const parsed = JSON.parse(dataStr);
          if (parsed.error) throw new Error(typeof parsed.error === 'string' ? parsed.error : parsed.error.message || JSON.stringify(parsed.error));
          const type=parsed.type || eventName;
          if(type==='error')throw new Error(parsed.message || 'LLM 응답 오류');

          if (parsed.choices?.[0]?.delta?.reasoning_content) {
            if (!aiReasoningStartMs) aiReasoningStartMs = Date.now();
            rawReasoning += parsed.choices[0].delta.reasoning_content;
            aiPhaseTitle = '생각 중';
            aiReasoningPreview = rawReasoning;
          }

          const statusChanged = applyStreamStatusEvent(type, parsed);

          const final=finalChatText(parsed);
          if(final!==null)rawAnswer=final;
          else if(type==='message.delta')rawAnswer+=parsed.content || '';
          else rawAnswer+=parsed.choices?.[0]?.delta?.content || parsed.message?.content || '';
          if(type==='request.complete')streamComplete=true;

          const extracted = extractVisibleAndReasoning(rawAnswer);
          if (extracted.isInlineReasoning) {
            aiPhaseTitle = '생각 중';
            aiReasoningPreview = extracted.reasoningText;
          } else if (extracted.visibleText) {
            aiPhaseTitle = '답변 작성 중';
            aiPhaseDetail = '';
            aiReasoningPreview = '';
          }

          if(translationStale(options) || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
          if(options.hint){if(statusChanged || extracted.isInlineReasoning || extracted.visibleText)setHUDNotice();return;} // Optional hints only update header status until complete.
          setVisibleAnswer(rawAnswer);
          if(config.connectionMode==='sse' || statusChanged)renderLiveHUD();
        }
        while (true) {
          const { done, value } = await abortableWait(reader.read(),controller.signal,()=>{Promise.resolve(reader.cancel?.()).catch(()=>{});});
          if(value?.length)deadline.touch();
          buffered += decoder.decode(value, { stream: !done });
          const lines = buffered.split('\n');
          buffered = lines.pop();
          for (const line of lines) consumeLine(line);
          if(streamComplete){Promise.resolve(reader.cancel?.()).catch(()=>{});break;}
          if (done) {
            if (buffered.trim()) consumeLine(buffered);
            break;
          }
        }
      } else {
        const data = await abortableWait(res.json(),controller.signal);
        if(data.error)throw new Error(typeof data.error==='string'?data.error:data.error.message || JSON.stringify(data.error));
        if(options.hint && hintProof===null)recordHintProof(data.hint_grounding || null);
        rawAnswer = finalChatText(data) ?? data.choices?.[0]?.message?.content ?? data.response ?? '';
        if(!options.hint){
          setVisibleAnswer(rawAnswer);
          renderLiveHUD();
        }
      }

      if(translationStale(options) || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
      let verifiedHint=null;
      if(options.hint){
        auditHint(options,'generated',{reason:'model_completed',topic:enrichmentPlan?.topic || '',text:extractVisibleAndReasoning(rawAnswer).visibleText});
        const conversationTopic=enrichmentPlan?.topic;
        const verified=validateHintGrounding(rawAnswer,hintProof);
        auditHint(options,'decision_response',{reason:'validation_detail',topic:enrichmentPlan?.topic || '',text:JSON.stringify({ok:verified.ok,reason:verified.reason || '',normalized:verified.normalized || '',selection:verified.selection || null,model:hintProof?.model || payload.model,promptRevision:hintProof?.prompt_revision || ''})});
        if(!verified.ok){hintSkipReason=verified.reason;
          if(options.topicVersion===hintGate.topicVersion)hintGate.currentTopic='';
          els.hintStatus.textContent=({missing_proof:'출처 검증 정보 없음 · 게이트웨이 업데이트 필요',no_information_value:'구체적인 추가 정보 없음 · 새 대화 계속',incomplete_or_empty_source:'출처 설명이 잘렸거나 내용 부족 · 새 대화 계속',source_does_not_answer_task:'출처에 요청한 정보 없음 · 새 대화 계속'})[verified.reason] || '명칭·본문의 출처 근거 불일치 · 힌트 생략';return;
        }
        if(verified.source==='conversation' || verified.kind==='recap'){hintSkipReason='conversation_summary_disabled';return;}
        enrichmentPlan={...enrichmentPlan,topic:verified.topic,source:verified.source,reference:verified.reference};
        verifiedHint=verified;
        auditHint(options,'validated',{reason:hintProof?.task_oriented && hintProof.mode==='known'?'internal_knowledge':'source_validated',topic:verified.topic,kind:verified.kind || enrichmentPlan.kind,source:verified.source});
        rawAnswer=JSON.stringify({fact:verified.fact,topic:verified.topic,kind:verified.kind || enrichmentPlan.kind,adds_information:true});
        if(options.topicVersion===hintGate.topicVersion && (!enrichmentPlan.taskOriented || (options.sequence ?? 0)>=hintGate.topicDecisionSequence))hintGate.currentTopic=enrichmentPlan.taskOriented?conversationTopic:verified.topic;
      }
      let enrichedFact=options.hint?(['quote','conversation'].includes(hintProof?.mode)?verifiedHint.fact:extractHintFact(rawAnswer,[conversationContext,prompt].join('\n'),enrichmentPlan)):'';
      if(options.hint && ['quote','conversation'].includes(hintProof?.mode)){
        const target=targetLang || config.translateTargetLang || 'ko';
        if(hintNeedsLocalization(enrichedFact,target)){
          aiPhaseTitle='힌트 번역 중';setHUDNotice();
          const localized=await localizeVerifiedHint({text:enrichedFact,target,signal:controller.signal,translate:async(text,target)=>{
            if(['chrome','apple'].includes(config.translationEngine)){
              const local=await localTranslate(gatewayFetch,config.serverUrl,getAuthHeaders({'Content-Type':'application/json'}),{text,source:'auto',target},controller.signal,4000,config.translationEngine);
              if(local.ok)return local;
            }
            return llmTranslation({fetch:gatewayFetch,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),model:resolveRoleModel('translation',modelConfig),job:{prompt:text,source:'auto',target,draft:true},signal:controller.signal});
          }});
          if(!localized.ok){hintSkipReason=localized.reason;els.hintStatus.textContent='힌트 번역 실패 · 새 대화 계속';return;}
          enrichedFact=localized.text;
        }
      }
      if(options.hint){
        const reason=hintGate.rejectionReason(options);
        const labels={topic_changed:'주제 전환 확인',transcript_changed:'근거 전사 수정',expired_or_canceled:'유효 시간 경과 또는 취소'};
        if(reason){hintSkipReason=reason;els.hintStatus.textContent=`힌트 생략 · ${labels[reason] || reason}`;return;}
        if(!fitHintContent(enrichedFact)){hintSkipReason='empty_or_echo';els.hintStatus.textContent='힌트 생략 · 본문 형식 또는 보충 정보 없음';return;}
        const observedFacts=(enrichmentPlan.knownFacts || []).flatMap(text=>String(text).split(/\n|(?<=[.!?。！？])\s+/u)).filter(Boolean).slice(-4);
        if(await compareHintDuplicate({fetch:gatewayFetch,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),text:enrichedFact,history:observedFacts,kind:'transcript',signal:controller.signal,timeoutMs:1000})){hintSkipReason='transcript_echo';els.hintStatus.textContent='이미 설명된 내용 · 새 정보 대기';return;}
        if(controller.signal.aborted)return;
        const visibleHintTexts=answerIsHint?(hintStackActive()?hintStack.cards.map(card=>card.text):[answerText]):[];
        if(hintGate.identicalCue(fitHintContent(enrichedFact),visibleHintTexts)){hintSkipReason='already_visible';els.hintStatus.textContent='동일한 힌트가 이미 표시됨 · 현재 카드 유지';return;}
        if(controller.signal.aborted)return;
      }
      if(options.hint){const note={...enrichmentPlan,kind:hintResultKind(rawAnswer,enrichmentPlan.kind),evidence:enrichmentPlan.evidence || options.focusText,auditMeta:{...options,hintPlan:enrichmentPlan}};rememberHint(note);displayHintCard(note,fitHintContent(enrichedFact));cueDisplayed=true;cueTopic=enrichmentPlan.topic;auditHint(options,'phone_display',{reason:'card_rendered',topic:enrichmentPlan.topic,kind:enrichmentPlan.kind,text:fitHintContent(enrichedFact),source:enrichmentPlan.source});els.chatQuestion.textContent=cueTopic?`💡 자동 힌트 · ${cueTopic}`:'💡 자동 힌트';}else setVisibleAnswer(rawAnswer,true);
      if(options.hint)els.hintStatus.textContent=enrichmentPlan.source==='web'?'웹검색 근거로 보충한 정보':enrichmentPlan.source==='memory'?'저장된 기억으로 보충한 정보':enrichmentPlan.source==='conversation'?'최근 대화에서 요점·일정 정리':'내부 지식으로 보충한 정보';
      els.chatOutput.textContent = activeMode==='translate'?translationPhoneText(translationDisplay):answerText;
      if(chromeText)els.chromeTranslationStatus.textContent=repliesReplaced?`${options.translatedBy || localLabel()} 번역 · 답변 예시 완료`:hasCompleteTranslationReplies(translationDisplay)?`${options.translatedBy || localLabel()} 번역 완료 · 이전 답변 예시 유지`:`${options.translatedBy || localLabel()} 번역 완료 · 답변 예시를 받지 못했습니다`;
      renderLiveHUD();
      if(!answerText.trim())throw new Error('LLM에서 답변 본문을 받지 못했습니다. 서버의 모델 상태를 확인하세요.');

      if(separateLLMReplies){
        els.chromeTranslationStatus.textContent='LLM 번역 완료 · 지정한 번역 답변 모델로 생성 중';
        enqueueAI(newID(),prompt,conversationContext,'translate',targetLang || config.translateTargetLang,{replyOnly:true,replyVersion:options.replyVersion ?? translationReplyVersion,chromeTranslationText:answerText,chromeAttempted:true,translationSource:chromeSource,translationToken:options.translationToken,translationHistoryId:options.translationHistoryId,translationRevisionId:options.translationRevisionId,translatedBy:'LLM'});
      }
      if(options.chatSession && requestChatScope===chatScope()){
        chatMemory.remember(requestChatScope,prompt,answerText);
        answerAutoCloseEligible=true;startHintLifetime();
        els.hintStatus.textContent='직접 질문 답변 완료 · 계속 듣는 중';
      }

      // TTS Playback if enabled
      if (!options.hint && config.enableTTS && answerText && config.voiceMode !== 'continuous') {
        playTTS(answerText);
      }
    } catch (err) {
      if(!controller.signal.aborted)aiStatusError=String(err.message || err);
      if(options.hint)hintSkipReason=controller.signal.aborted?(options.cancelReason || (deadline.reason()?'writer_timeout':'canceled')):'writer_error';
      if(translationStale(options) || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
      if (controller.signal.aborted && aiAbortedByModeSwitch) {
        return;
      }
      if(options.hint && options.cancelledByTopic)return;
      if(options.replyOnly && controller.signal.aborted)return;
      const failure=chatFailureMessage(err,deadline);
      if(options.hint && controller.signal.aborted && pendingAI.some(job=>job.directQuestion))return;
      if(options.hint){els.hintStatus.textContent=`힌트 생성 실패 · 전사 계속 (${failure})`;return;}
      if(chromeText){els.chromeTranslationStatus.textContent=`번역 유지 · 답변 예시 생성 실패 (${failure})`;els.chatOutput.textContent=translationPhoneText(translationDisplay);renderLiveHUD();return;}
      if(activeMode==='translate'){
        translationFailure='번역 오류';answerText=translationDisplay.translation;
        els.chromeTranslationStatus.textContent=`LLM 번역 실패 · ${answerText?'이전 번역 유지':'새 번역 대기'} (${failure})`;
        els.chatOutput.textContent=translationPhoneText(translationDisplay);renderLiveHUD();return;
      }
      console.error('Chat error:', err);
      els.hintStatus.textContent=`AI 채팅 중단 · ${failure}`;
      if(!receivedVisibleAnswer){clearHintStack();hintLifetime.clear();answerAutoCloseEligible=false;answerTitle='AI 오류';answerVoicePrompt=options.wakeToken!==undefined?prompt:'';answerText=`AI 오류: ${failure}`;}
      els.chatOutput.textContent=receivedVisibleAnswer?`${answerText}\n\n응답 중단: ${failure}`:answerText;renderLiveHUD();
    } finally {
      if(options.hint && !cueDisplayed)auditHint(options,controller.signal.aborted?'canceled':'skipped',{reason:hintSkipReason,topic:enrichmentPlan?.topic || ''});
      if(options.hint)hintGate.complete(options,{displayed:cueDisplayed,text:cueDisplayed?answerText:'',topic:cueTopic});
      deadline.stop();
      if (aiAbortController === controller) aiAbortController = null;
      isThinking = false;
      stopThinkingAnimation();
      aiPhaseTitle = '';
      aiPhaseDetail = '';
      aiReasoningPreview = '';
      if (!isRecording && !captureFailure && !micCloseUncertain && !pendingMicOpen) setVoiceHint('마이크 꺼짐 · 탭하면 녹음 시작');
    }
  }

  document.getElementById('btn-hint-history')?.addEventListener('click',async()=>{
    const el=document.getElementById('hint-history-log');const status=document.getElementById('hint-history-status');
    const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),5000),scope=chatScope();
    try{const res=await gatewayFetch(`${config.serverUrl}/api/glasses/hint-events?limit=100`,{headers:getAuthHeaders(),signal:controller.signal});if(!res.ok)throw Error(`HTTP ${res.status}`);const data=await res.json();if(scope!==chatScope())return;
      const labels={decision_response:'판별 응답·소요 시간',decision:'판별',selected:'선택',generation:'생성 요청',generated:'본문 생성',validated:'응답 검사 완료',skipped:'생략',phone_display:'폰 표시',device_display:'안경 전송 완료',device_error:'안경 전송 오류',canceled:'취소'};
      const reasons={validation_detail:'힌트 검증 상세',transcript_revised:'원문 전사 수정으로 철회',unresolved_hint_task:'대상이 불명확한 조사 과제',mixed_source_records:'서로 다른 출처 혼합',invalid_hint_response:'힌트 응답 형식 오류',no_additional_information:'모델이 추가 정보 없음으로 판단',heading_only:'제목만 반복하는 본문',navigation_only:'사이트 안내만 있는 본문',insufficient_context:'정보 부족 안내만 있는 본문',internal_knowledge:'내부 지식 응답 · 외부 출처 검증 없음',unresolved_hint_subject:'대상을 특정할 수 없는 표현 · 검색 생략',source_does_not_answer_task:'출처에 요청한 정보 없음',user_dismissed:'사용자가 창 닫음',chat_priority:'AI 채팅 호출로 전환',writer_timeout:'힌트 처리 시간 초과',pending_replaced:'대기 후보를 최신 힌트로 교체',source_records:'출처·문장 후보 확인',invalid_source_selection:'출처 문장 번호 오류',unsupported_quote_or_name:'생성 문장과 출처가 일치하지 않음',transcript_echo:'전사에 이미 설명된 내용',source_lookup_detail:'출처 조회 상세',no_information_value:'구체적인 추가 정보 없음',incomplete_or_empty_source:'잘린 출처·추가 정보 없음',conversation_summary_disabled:'대화 요약·재진술 제외',invalid_summary:'요약 항목 형식 오류',no_summary_value:'원문 반복·정리할 정보 없음',hint_translation_failed:'검증된 힌트의 표시 언어 번역 실패',judge_response:'판별 응답 수신',judge_timeout:'판별 시간 초과',judge_invalid_response:'판별 응답 형식·근거 오류',judge_http_error:'판별 HTTP 오류',judge_request_error:'판별 통신·응답 처리 오류',judge_canceled:'판별 취소',llm_skip:'보충할 도움 없음',hint_selected:'힌트 필요',judge_format_repaired:'판별 형식 재검사 완료',selection_evidence_changed:'선택한 원문이 수정됨',transcript_changed:'근거 전사 수정·철회',topic_changed:'주제 전환',source_lookup_failed:'출처 조회 실패',no_fact:'표시할 구조화된 본문 없음',unsupported_summary:'원문에 없는 요약 근거',unverified_name:'검증한 명칭과 불일치',already_visible:'같은 카드가 표시 중',sdk_ack:'안경 SDK 전송 확인',model_completed:'모델 생성 완료',source_validated:'원문 근거 확인',card_rendered:'폰 화면에 카드 표시',duplicate_candidate:'최근과 같은 조사 후보',duplicate_plan:'의미가 같은 조사 계획',candidate_expired:'대기 중 유효 시간 초과',writer_error:'본문 생성 오류',canceled:'작업 취소',judge_format_or_timeout:'판별 형식 오류 또는 시간 초과'};
      el.textContent=data.events.map(e=>`${new Date(e.occurredAt || e.createdAt).toLocaleString()} [${labels[e.stage] || e.stage}] ${e.kind?hintPresentation({kind:e.kind,topic:e.topic}).title:e.topic || ''}\n${reasons[e.reason] || e.reason || ''}${e.text?'\n'+e.text:''}`).join('\n\n') || '저장된 이력이 없습니다.';status.textContent='최근 100개 처리 이력';
    }catch(error){if(scope===chatScope())status.textContent=`이력 조회 실패 · ${error.message}`;}finally{clearTimeout(timer);}
  });

  // --- TTS Support ---
  async function playTTS(text) {
    try {
      const res = await gatewayFetch(`${config.serverUrl}/api/tts`, {
        method: 'POST',
        headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
          text: text.slice(0, 300), // Limit length for glasses glance
          lang: 'ko',
          format: 'wav'
        })
      });
      if (!res.ok) return;

      const blob = await res.blob();
      const audioUrl = URL.createObjectURL(blob);
      const audio = new Audio(audioUrl);
      audio.addEventListener('ended', () => URL.revokeObjectURL(audioUrl), { once: true });
      try { await audio.play(); } catch (err) { URL.revokeObjectURL(audioUrl); throw err; }
    } catch (e) {
      console.warn('TTS playback notice:', e);
    }
  }

  // --- Server Connection & Auth ---
  async function gatewayFetch(url, options = {}) {
    // Every LLM role (including preview translation and hint judgment) resolves
    // its real loaded instance before inference. Preparation calls use another
    // URL, so this does not recurse. Never send WebView cookies.
    if(url===`${config.serverUrl}/api/chat` && typeof options.body==='string'){
      const body=JSON.parse(options.body),key=modelLoadKey(body.model);
      if(body.model){
        const ready=await abortableWait(ensureModelLoaded(body.model,options.signal),options.signal);
        if(ready){
          body.model=loadedModelInstances.get(key) || body.model;
          options={...options,body:JSON.stringify(body)};
        }else{
          const reason=modelLoadErrors.get(key) || '모델 준비를 확인하지 못했습니다';
          // Other providers may not expose LM Studio's management API; their
          // inference endpoint can still work. Actual loading/auth failures
          // must remain visible rather than issuing another doomed request.
          if(!/HTTP 404|HTTP 405|404 page not found/i.test(reason))throw Error('모델 준비 실패 · '+reason);
        }
      }
    }
    return abortableWait(fetch(url, { ...options, credentials: 'omit' }),options.signal);
  }

  function getAuthHeaders(baseHeaders = {}) {
    const headers = { ...baseHeaders };
    if (config.authToken) {
      headers['Authorization'] = `Bearer ${config.authToken}`;
    } else if (config.apiKey) {
      headers['Authorization'] = `Bearer ${config.apiKey}`;
    }
    return headers;
  }

  async function testServerConnection() {
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    if (!url) {
      alert('서버 주소를 입력하세요.');
      return;
    }

    els.btnTestConnection.textContent = '연결 중...';
    try {
      const res = await gatewayFetch(`${url}/api/health`, { method: 'GET' });
      if (res.ok) {
        alert('서버 연결 성공! 정상 통신 가능합니다.');
      } else {
        alert(`서버 응답 오류: ${res.status}`);
      }
    } catch (err) {
      alert(`서버 연결 실패:\n${err.message}`);
    } finally {
      els.btnTestConnection.textContent = '테스트';
    }
  }

  async function login() {
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    const username = (els.authUsername.value || '').trim();
    const password = (els.authPassword.value || '').trim();

    if (!username || !password) {
      alert('아이디와 비밀번호를 입력하세요.');
      return;
    }

    els.btnLogin.textContent = '로그인 중...';
    try {
      const res = await gatewayFetch(`${url}/api/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: username, password, remember_me: true })
      });

      if (!res.ok) {
        throw new Error(res.status === 401
          ? '아이디 또는 비밀번호가 올바르지 않습니다. (HTTP 401)'
          : `서버 응답 오류: HTTP ${res.status}`);
      }

      const data = await res.json();
      if (!data.token) throw new Error('서버가 인증 토큰을 반환하지 않았습니다.');
      config.authToken = data.token;
      config.username = username;
      await saveSettings(false);

      if (els.loginStatusLabel) {
        els.loginStatusLabel.textContent = `인증됨 (${username})`;
        els.loginStatusLabel.style.color = 'var(--accent-green)';
      }
      alert('로그인 성공!');
      await fetchModelList();
      autoPrepareEngines();
    } catch (err) {
      const networkFailure = err instanceof TypeError || /load failed|failed to fetch/i.test(err.message);
      alert(networkFailure
        ? `서버 요청이 차단되었거나 연결되지 않았습니다.\n주소: ${url}\n서버 CORS 설정, 네트워크 권한, HTTP/HTTPS 주소를 확인하세요.`
        : `로그인 오류: ${err.message}`);
      if (els.loginStatusLabel) {
        els.loginStatusLabel.textContent = networkFailure ? '서버 연결 실패' : '로그인 실패';
        els.loginStatusLabel.style.color = 'var(--accent-red)';
      }
    } finally {
      els.btnLogin.innerHTML = '<span class="material-icons-round" style="font-size: 16px;">→</span> 로그인';
    }
  }

  async function checkLoginStatus() {
    const requestedToken = config.authToken;
    const requestedURL = config.serverUrl;
    try {
      const res = await gatewayFetch(`${config.serverUrl}/api/auth/check`, {
        headers: getAuthHeaders()
      });
      const status = res.ok ? await res.json() : {};
      if (requestedToken !== config.authToken || requestedURL !== config.serverUrl) return;
      if (status.authenticated && els.loginStatusLabel) {
        els.loginStatusLabel.textContent = `인증됨 (${config.username || 'User'})`;
        els.loginStatusLabel.style.color = 'var(--accent-green)';
        fetchModelList();
        autoPrepareEngines();
      } else if (requestedToken && status.authenticated === false) {
        config.authToken = '';
        await persistSettings();
        els.loginStatusLabel.textContent = '세션 만료 · 다시 로그인';
      }
    } catch (_) {}
  }

  async function fetchAppleCapabilities(){
    try{
      const response=await gatewayFetch(`${config.serverUrl}/api/apple/status`,{headers:getAuthHeaders()});
      const status=await response.json();
      document.getElementById('stt-option-apple').disabled=!response.ok || !status.available || !status.speechAvailable;
      document.getElementById('translation-option-apple').disabled=!response.ok || !status.available;
    }catch{}
  }
  function renderRoleModelSelectors(){
    for(const role of ROLE_MODEL_KEYS){
      const select=els.roleModels[role],selected=config.roleModels?.[role] || '';
      if(!select)continue;
      select.innerHTML='';
      const options=roleModelOptions(availableModels,config.model,selected).filter(option=>role!=='hintDecision' || option.id!==JEV_ONLY);
      if(role==='hintDecision' && !options.some(option=>option.id===JEV_ONLY))options.splice(1,0,{id:JEV_ONLY,label:'사전·JEV만 사용 (판별 LLM 끔)'});
      for(const model of options){
        const option=document.createElement('option');option.value=model.id;option.textContent=model.label;select.appendChild(option);
      }
      select.value=selected;
    }
  }
  async function fetchModelList() {
    fetchAppleCapabilities();
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    try {
      const res = await gatewayFetch(`${url}/api/models`, {
        headers: getAuthHeaders()
      });
      if (!res.ok) return;

      const data = await res.json();
      const models = normalizeModels(data);
      availableModels=models;
      if(config.hintDecisionAuto!==false){
        const suggested=suggestedHintJudge(models);
        if(suggested && config.roleModels.hintDecision===JEV_ONLY){config.roleModels.hintDecision=suggested;persistSettings().catch(console.warn);}
      }
      const previousModel = els.modelSelect.value || config.model;

      els.modelSelect.innerHTML = '';
      if (models.length > 0) {
        models.forEach((m) => {
          const opt = document.createElement('option');
          opt.value = m.id;
          opt.textContent = m.label;
          els.modelSelect.appendChild(opt);
        });
        els.modelSelect.value = models.some(m => m.id === previousModel)
          ? previousModel : models[0].id;
        config.model = els.modelSelect.value;
      } else {
        els.modelSelect.innerHTML = '<option value="">등록된 모델 없음</option>';
        config.model = '';
      }
      // Keep the user's provider choice if that connection is temporarily offline.
      if(previousModel.startsWith('@llm/') && !models.some(model=>model.id===previousModel)){
        const option=document.createElement('option');option.value=previousModel;option.textContent='선택한 연결 모델 · 목록 확인 불가';els.modelSelect.appendChild(option);els.modelSelect.value=previousModel;config.model=previousModel;
      }
      renderRoleModelSelectors();
      prewarmConfiguredModels();
    } catch (e) {
      console.warn('Failed to fetch models:', e);
    }
  }

  // --- Helpers ---
  function bindEvents() {
    els.chatForm.addEventListener('submit',event=>{
      event.preventDefault();
      if(headPaused){els.chatOutput.textContent='화면/STT 일시정지 중입니다. 재개 후 질문하세요.';return;}
      const prompt=els.chatInput.value.trim();
      if(!prompt)return;
      const role=llmRequestRole({mode:normalizeGlassesMode(config.glassesMode),chatSession:true,translated:normalizeGlassesMode(config.glassesMode)==='translate' && localEngine()!=='llm'});
      if(!resolveRoleModel(role,config)){els.chatOutput.textContent='설정에서 기본 모델 또는 해당 역할의 LLM 모델을 선택하세요.';return;}
      if(pendingAI.length>=8){els.chatOutput.textContent='대기 중인 질문이 많습니다. 답변 후 다시 보내주세요.';return;}
      els.chatInput.value='';
      if(processingAI)els.chatOutput.textContent='질문을 대기열에 추가했습니다.';
      const chatSession=normalizeGlassesMode(config.glassesMode)==='ai';
      if(chatSession){pendingTrigger=false;hintGate.reset();if(activeAIJob?.hint){activeAIJob.cancelReason='chat_priority';aiAbortController?.abort();}}
      enqueueAI(newID(),prompt,'',config.glassesMode,config.translateTargetLang,chatSession?{directQuestion:true,chatSession:true,chatEpoch,forceSearch:directQuestionIntent(prompt).search}:{});
    });
    const viewport=window.visualViewport;
    if(viewport) {
      const updateViewport=()=>{
        const gap=viewport.scale===1 ? Math.max(0,window.innerHeight-viewport.height-viewport.offsetTop) : 0;
        document.body.style.setProperty('--keyboard-gap',`${gap}px`);
        if(gap>0 && /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement?.tagName))document.activeElement.scrollIntoView({block:'nearest'});
      };
      viewport.addEventListener('resize',updateViewport);
      viewport.addEventListener('scroll',updateViewport);
      updateViewport();
    }
    if (els.btnTestConnection) els.btnTestConnection.addEventListener('click', testServerConnection);
    if (els.btnLogin) els.btnLogin.addEventListener('click', login);
    if (els.btnRefreshModels) els.btnRefreshModels.addEventListener('click', fetchModelList);
    els.modelSelect.addEventListener('change',()=>{settingsEdited=true;config.model=els.modelSelect.value;renderRoleModelSelectors();persistSettings().catch(console.warn);});
    for(const role of ROLE_MODEL_KEYS)els.roleModels[role]?.addEventListener('change',()=>{
      settingsEdited=true;config.roleModels={...normalizeRoleModels(config.roleModels),[role]:els.roleModels[role].value};
      if(role==='hintDecision'){config.hintDecisionAuto=false;hintGate.reset();els.hintJudgeStatus.textContent='새 판별 모델 적용 · 다음 전사 대기';}
      persistSettings().catch(console.warn);
    });

    if (els.btnSaveSettings) els.btnSaveSettings.addEventListener('click', saveSettings);
    if (els.voiceBtn) els.voiceBtn.addEventListener('click', toggleVoiceRecording);
    if (els.modeBtnAi) els.modeBtnAi.addEventListener('click', () => setGlassesMode('ai', true));
    if (els.modeBtnTranslate) els.modeBtnTranslate.addEventListener('click', () => setGlassesMode('translate', true));
    if (els.glassesMode) {
      els.glassesMode.addEventListener('change', () => {
        settingsEdited = true;
        setGlassesMode(els.glassesMode.value, true);
      });
    }
    if (els.translateSourceLang) {
      els.translateSourceLang.addEventListener('change', () => {
        settingsEdited = true;
        return applyLiveSettings({translateSourceLang:els.translateSourceLang.value || 'auto'});
      });
    }
    if (els.translateTargetLang) {
      els.translateTargetLang.addEventListener('change', () => {
        settingsEdited = true;
        return applyLiveSettings({translateTargetLang:els.translateTargetLang.value || 'ko'});
      });
    }
    els.btnTranscripts.addEventListener('click', () => { openTranscriptModal(); loadTranscripts().catch(err => { els.transcriptLog.textContent = err.message; }); });
    els.btnCloseTranscripts.addEventListener('click', closeTranscriptModal);
    els.transcriptModal.addEventListener('click', event => { if (event.target === els.transcriptModal) closeTranscriptModal(); });
    document.addEventListener('keydown', event => {
      if (els.transcriptModal.hidden) return;
      if (event.key === 'Escape') closeTranscriptModal();
      if (event.key === 'Tab') {
        const first = els.btnCloseTranscripts, last = els.btnExportTranscripts;
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    });
    els.glassesSensitivity.addEventListener('change',()=>applyLiveSettings({glassesInput:readGlassesInputControls()}));
    els.glassesGain.addEventListener('change',()=>applyLiveSettings({glassesInput:readGlassesInputControls()}));
    els.audioInputSource.addEventListener('change',()=>applyLiveSettings({audioInputSource:els.audioInputSource.value==='phone'?'phone':'glasses'}));
    els.sttEngine.addEventListener('change',()=>applyLiveSettings({sttEngine:els.sttEngine.value}));
    els.sttFallback.addEventListener('change',()=>{settingsEdited=true;config.sttFallback=els.sttFallback.checked;persistSettings().catch(console.warn);});
    els.btnPrepareStt.addEventListener('click',()=>prepareSelectedStt(undefined,true).catch(()=>{}));
    els.chromeTranslation?.addEventListener('change',()=>applyLiveSettings({translationEngine:els.chromeTranslation.checked?'chrome':'llm'}));
    els.translationEngine?.addEventListener('change',()=>applyLiveSettings({translationEngine:els.translationEngine.value}));
    els.btnPrepareChrome?.addEventListener('click',()=>{
      if(localEngine()==='llm'){els.chromeTranslationStatus.textContent='LLM 번역은 별도 언어팩 준비가 필요 없습니다';return;}
      return prepareSelectedTranslation(undefined,true).catch(()=>{});
    });
    els.hintContextTokens.addEventListener('change',()=>{settingsEdited=true;config.hintContextTokens=normalizeHintBudget(els.hintContextTokens.value);els.hintContextTokens.value=String(config.hintContextTokens);hintGate.reset();persistSettings().catch(console.warn);});
    els.headGestureEnabled.addEventListener('change',setHeadGestureEnabled);
    els.headCalibrate.addEventListener('click',startHeadCalibration);
    els.headResume.addEventListener('click',resumeHeadSession);
    els.hintCardCount.addEventListener('change',updateHintCardCount);
    els.hintAutoClose.addEventListener('change',updateHintLifetimeSettings);
    els.hintSecondsPerLine.addEventListener('change',updateHintLifetimeSettings);
    els.triggerTranscriptOnly.addEventListener('change',setTriggerTranscriptOnly);
    els.hudScreen.addEventListener('click',event=>{const card=event.target?.closest?.('.hud-ai-box');if(card)dismissAIWindow(card.dataset?.hintCard);});
    els.aiHeaderLayout.addEventListener('change',()=>{settingsEdited=true;config.aiHeaderLayout=normalizeAIHeaderLayout(els.aiHeaderLayout.value);syncAIHeaderControls();renderLiveHUD();persistSettings().catch(console.warn);});
    els.aiIconPosition.addEventListener('input',()=>updateAIIconPosition());
    els.aiIconPosition.addEventListener('change',()=>updateAIIconPosition(true));
    els.hudDimLevel.addEventListener('input',updateHUDDimming);
    for(const el of [els.hudDimEnabled,els.hudFullTranscript,els.hudFullAI])el.addEventListener('change',updateHUDDimming);
    els.hideTranscript.addEventListener('change',()=>setTranscriptHidden(els.hideTranscript.checked));
    els.hintWebSearch?.addEventListener('change',()=>{settingsEdited=true;config.hintWebSearch=els.hintWebSearch.checked;persistSettings().catch(console.warn);});
    els.translateExcludeSelf?.addEventListener('change',()=>{
      return applyLiveSettings({translateExcludeSelf:els.translateExcludeSelf.checked});
    });
    for (const el of [els.hintSensitivity, els.serverUrl, els.authUsername, els.apiKey, els.modelSelect, els.connectionMode, els.voiceMode, els.triggerWord, els.speechFilter, els.speakerScope, els.optEnableTts, els.optConciseMode]) {
      if (!el) continue;
      el.addEventListener('input', () => { settingsEdited = true; });
      el.addEventListener('change', () => { settingsEdited = true; });
    }
    els.btnExportTranscripts.addEventListener('click', async () => {
      try {
        await loadTranscripts();
        const url = URL.createObjectURL(new Blob([transcriptText], {type:'text/plain;charset=utf-8'}));
        const link = document.createElement('a'); link.href=url; link.download=`transcript-${els.transcriptDate.value}.txt`; link.click();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
      } catch(err) { alert(err.message); }
    });
  }

  function uint8ArrayToBase64(bytes) {
    let binary = '';
    const len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return window.btoa(binary);
  }

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Run on DOM Ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

