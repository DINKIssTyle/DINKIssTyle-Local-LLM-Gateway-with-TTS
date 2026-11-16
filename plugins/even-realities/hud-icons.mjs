export const HUD_ICONS={
  'glasses-mic-on':{width:24,height:24},'glasses-mic-off':{width:24,height:24},
  'phone-mic-on':{width:24,height:24},'phone-mic-off':{width:24,height:24},
  'mode-ai':{width:24,height:24},'mode-translate':{width:24,height:24},
  'status-idle':{width:24,height:24},'status-listening':{width:24,height:24},'status-transcribing':{width:24,height:24}
};
export const hudIconPath=key=>`assets/hud-icons/${key}.png`;
export function microphoneIcon(source,on){return `${source==='phone'?'phone':'glasses'}-mic-${on?'on':'off'}`;}
export function captureStatusIcon({listening=false,preparing=false,transcribing=false,error=false}={}){
  if(preparing || error)return 'status-idle';
  if(transcribing)return 'status-transcribing';
  return listening?'status-listening':'status-idle';
}
export function iconHeaderMessage(text){
  const detail=String(text || '').replace(/^\s*\|\s*/u,'').trim();
  if(/^(?:대기중|듣는중|전사처리(?:중|대기)?|-)$/u.test(detail.replace(/\s+/gu,'')))return '';
  return detail?'| '+detail:'';
}
// SDK 0.0.16: row-major packed gray4, left pixel in the high nibble.
export function rgbaToGray4(rgba,width,height,brightness=1){
  if(width%2 || rgba.length!==width*height*4)throw Error('아이콘 크기 또는 RGBA 데이터가 올바르지 않습니다.');
  const bytes=new Uint8Array(width*height/2);
  for(let pixel=0;pixel<width*height;pixel++){
    const i=pixel*4,gray=Math.round((.299*rgba[i]+.587*rgba[i+1]+.114*rgba[i+2])*(rgba[i+3]/255)/17*Math.max(0,Math.min(1,brightness)));
    bytes[pixel>>1]|=gray<<(pixel%2?0:4);
  }
  return bytes;
}
export async function rasterizeIcon(key,{width,height},baseURL=document.baseURI){
  const image=new Image();image.src=new URL(hudIconPath(key),baseURL).href;
  await new Promise((resolve,reject)=>{image.onload=resolve;image.onerror=()=>reject(Error(`아이콘을 불러오지 못했습니다: ${key}`));});
  if(image.naturalWidth!==width || image.naturalHeight!==height)throw Error(`아이콘은 ${width}×${height} 원본 크기여야 합니다: ${key}`);
  const canvas=document.createElement('canvas');canvas.width=width;canvas.height=height;
  const ctx=canvas.getContext('2d',{willReadFrequently:true});
  ctx.imageSmoothingEnabled=false;ctx.drawImage(image,0,0);
  return ctx.getImageData(0,0,width,height).data;
}
export class HudIconStore {
  constructor(render=rasterizeIcon){this.render=render;this.cache=new Map();this.rasters=new Map();}
  load(key,brightness=key.endsWith('-off') || key.startsWith('mode-') || key.startsWith('status-')?0.5:1){
    const spec=HUD_ICONS[key];if(!spec)throw Error('알 수 없는 HUD 아이콘');
    const level=Math.max(0,Math.min(4,Math.round((Number(brightness) || 1/15)*4))),cacheKey=`${key}:${level}`;
    if(!this.rasters.has(key))this.rasters.set(key,Promise.resolve().then(()=>this.render(key,spec)).catch(error=>{this.rasters.delete(key);throw error;}));
    if(!this.cache.has(cacheKey))this.cache.set(cacheKey,this.rasters.get(key).then(rgba=>rgbaToGray4(rgba,spec.width,spec.height,level===0?1/15:level/4)).catch(error=>{this.cache.delete(cacheKey);throw error;}));
    return this.cache.get(cacheKey);
  }
}
