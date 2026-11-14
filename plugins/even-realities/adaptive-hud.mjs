import {hudNoticeText,sliceHUDWindow} from './hud-status.mjs';

// G2 SDK canvas: 576×288. Keep the 27px clock/status row unchanged.
export function adaptiveHUD({mode='ai',title='',text='',hideTranscript=false}={}){
  if(mode==='translate')return {key:'translate',show:true,showTranscript:true,hint:false,height:141,transcriptY:168,transcriptHeight:120,transcriptLines:4,contentLines:5,title:''};
  // Transcript is capped at three rows. Empty space below the content stays blank.
  const show=!!String(text).trim(),hint=show && !!String(title).trim();
  const caption=hint?hudNoticeText(String(title).replace(/\s+/gu,' ').trim(),570):'';
  const showTranscript=!hideTranscript,transcriptHeight=showTranscript?81:0;
  const maxRows=Math.floor((261-transcriptHeight-6)/27);
  const maxContentLines=maxRows-(hint?1:0);
  const contentLines=show?Math.max(1,Math.min(maxContentLines,sliceHUDWindow(text,0,maxContentLines,570).totalLines)):0;
  const height=show?(contentLines+(hint?1:0))*27+6:0;
  return {key:`ai-${height}-${showTranscript}`,show,showTranscript,hint,height,transcriptY:27+height,transcriptHeight,transcriptLines:showTranscript?3:0,contentLines,title:caption};
}
export function hintWindow(text,title,offset=0,hideTranscript=false){
  const layout=adaptiveHUD({title,text,hideTranscript});
  const window=sliceHUDWindow(text,offset,layout.contentLines || 6,570);
  return {...window,layout,visibleText:layout.title?`${layout.title}\n${window.visibleText}`:window.visibleText};
}
