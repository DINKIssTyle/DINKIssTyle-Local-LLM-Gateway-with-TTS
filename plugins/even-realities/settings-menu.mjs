// Use the same labels and values as the phone settings. SDK menus have no submenus.
export const SETTINGS_MENUS={
  10:{title:'전사문 숨기기',key:'hideTranscript',options:[[false,'표시 · 최대 3줄'],[true,'숨김 · 전사와 저장은 계속']]},
  4:{title:'인식 언어',key:'translateSourceLang',options:[['auto','자동 (상대 언어)'],['en','영어 (English)'],['ko','한국어 (Korean)'],['ja','일본어 (Japanese)'],['zh','중국어 (Chinese)'],['es','스페인어 (Spanish)'],['fr','프랑스어 (French)'],['de','독일어 (German)'],['vi','베트남어 (Vietnamese)'],['th','태국어 (Thai)']]},
  5:{title:'번역 언어',key:'translateTargetLang',options:[['ko','한국어 (Korean)'],['en','영어 (English)'],['ja','일본어 (Japanese)'],['zh','중국어 (Chinese)'],['es','스페인어 (Spanish)'],['fr','프랑스어 (French)'],['de','독일어 (German)'],['vi','베트남어 (Vietnamese)'],['th','태국어 (Thai)']]},
  6:{title:'번역 엔진',key:'translationEngine',options:[['llm','LLM 번역'],['chrome','Chrome 로컬 번역'],['apple','Apple 로컬 번역 · macOS 26+']]},
  7:{title:'음성 인식 엔진',key:'sttEngine',options:[['whisper','Whisper'],['qwen3-asr','Qwen3-ASR · Apple Silicon'],['chrome-online','Chrome 온라인 STT · Google로 음성 전송'],['apple','Apple 로컬 STT · macOS 26+']]},
  8:{title:'내 목소리 제외',key:'translateExcludeSelf',options:[[true,'켬 · 번역 중 내 목소리 제외'],[false,'끔 · 모든 목소리 번역']]},
  9:{title:'안경 입력 감지 민감도',key:'glassesInput',options:[['standard','표준 · 기존 감지 기준'],['high','높음 · 작은 목소리 감지'],['very_high','매우 높음 · 조용한 환경'],['auto','자동 · 주변 음량에 맞춤']]},
};
export function settingsMenuItems(glasses=true){return Object.entries(SETTINGS_MENUS).filter(([id])=>glasses || id!=='9').map(([id,menu])=>({itemID:Number(id),itemName:menu.title}));}
export function settingsPickerPage(picker){
  const names=picker.options.map(([value,label])=>`${value===picker.value?'✓ ':''}${label}`);
  return {containerTotalNum:3,listObject:[{containerID:30,containerName:'settings-list',xPosition:0,yPosition:30,width:576,height:228,isEventCapture:1,itemContainer:{itemCount:names.length,itemWidth:0,isItemSelectBorderEn:1,itemName:names}}],
    textObject:[{containerID:31,containerName:'settings-title',xPosition:0,yPosition:0,width:576,height:27,content:picker.title,isEventCapture:0},
      {containerID:32,containerName:'settings-help',xPosition:0,yPosition:261,width:576,height:27,content:'탭: 선택 · 길게: 돌아가기',isEventCapture:0}],
    menuObject:{menuItems:[{itemID:90,itemName:'돌아가기'}]}};
}
