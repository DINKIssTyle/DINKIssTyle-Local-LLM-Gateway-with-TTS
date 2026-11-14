export const HUD_ICONS={
  'glasses-mic-on':{width:24,height:24},'glasses-mic-off':{width:24,height:24},
  'phone-mic-on':{width:24,height:24},'phone-mic-off':{width:24,height:24},
  'ai-trigger':{width:24,height:24}
};
export const hudIconPath=key=>`assets/hud-icons/${key}.png`;
export function microphoneIcon(source,on){return `${source==='phone'?'phone':'glasses'}-mic-${on?'on':'off'}`;}
// SDK 0.0.16: row-major packed gray4, left pixel in the high nibble.
export function rgbaToGray4(rgba,width,height){
  if(width%2 || rgba.length!==width*height*4)throw Error('아이콘 크기 또는 RGBA 데이터가 올바르지 않습니다.');
  const bytes=new Uint8Array(width*height/2);
  for(let pixel=0;pixel<width*height;pixel++){
    const i=pixel*4,gray=Math.round((.299*rgba[i]+.587*rgba[i+1]+.114*rgba[i+2])*(rgba[i+3]/255)/17);
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
  constructor(render=rasterizeIcon){this.render=render;this.cache=new Map();}
  load(key){
    const spec=HUD_ICONS[key];if(!spec)throw Error('알 수 없는 HUD 아이콘');
    if(!this.cache.has(key))this.cache.set(key,Promise.resolve().then(()=>this.render(key,spec)).then(rgba=>rgbaToGray4(rgba,spec.width,spec.height)).catch(error=>{this.cache.delete(key);throw error;}));
    return this.cache.get(key);
  }
}
