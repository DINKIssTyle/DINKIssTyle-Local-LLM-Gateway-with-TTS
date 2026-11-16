import {hudNoticeText,sliceHUDWindow} from './hud-status.mjs';

// G2 SDK canvas: 576×288. The AI title is its own fixed 33px frame;
// its bottom border separates it from the scrolling body immediately below.
export function adaptiveHUD({mode='ai',title='',text='',hideTranscript=false,hideTitle=false}={}){
  if(mode==='translate')return {key:'translate',show:true,showTranscript:true,hint:false,height:141,transcriptY:168,transcriptHeight:120,transcriptLines:4,contentLines:5,title:'',titleHeight:0,bodyHeight:141};
  const show=!!String(text).trim(),showTranscript=!hideTranscript,transcriptHeight=showTranscript?81:0;
  const caption=show && !hideTitle?hudNoticeText(String(title || 'AI 답변').replace(/\s+/gu,' ').trim(),570):'';
  const titleHeight=show && !hideTitle?33:0,maxContentLines=Math.floor((261-transcriptHeight-titleHeight-6)/27);
  const contentLines=show?Math.max(1,Math.min(maxContentLines,sliceHUDWindow(text,0,maxContentLines,570).totalLines)):0;
  const bodyHeight=show?contentLines*27+6:0,height=titleHeight+bodyHeight;
  return {key:`ai-${height}-${showTranscript}`,show,showTranscript,hint:show,height,transcriptY:27+height,transcriptHeight,transcriptLines:showTranscript?3:0,contentLines,title:caption,titleHeight,bodyHeight};
}
export function hintWindow(text,title,offset=0,hideTranscript=false){
  const layout=adaptiveHUD({title,text,hideTranscript});
  const window=sliceHUDWindow(text,offset,layout.contentLines || 5,570);
  return {...window,layout,visibleText:window.visibleText};
}
