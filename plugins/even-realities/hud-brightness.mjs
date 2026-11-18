// SDK textColor 0 is the lowest level; hardware may render it nearly invisible.
export function normalizeHUDDimming(config={}){
  const value=Number(config.hudDimLevel);
  return {hudDimEnabled:config.hudDimEnabled!==false,hudDimLevel:Number.isFinite(value)?Math.max(0,Math.min(4,Math.round(value))):1,hudFullTranscript:config.hudFullTranscript===true,hudFullAI:config.hudFullAI!==false};
}
export function hudBrightness(id,config={}, {translate=false,wakeActive=false}={}){
  const c=normalizeHUDDimming(config),dim=c.hudDimEnabled?c.hudDimLevel:4;
  if(id===8 && !translate && wakeActive)return 4;
  if(id===1 || id===9 || id===10 || id===12)return translate || c.hudFullAI?4:dim;
  if(id===4)return translate || c.hudFullTranscript?4:dim;
  return dim;
}
export function hudLuminance(level){return level===0?1/15:level/4;}
export function hudPreviewColor(level){return level===4?'var(--accent-green)':`rgb(0, ${Math.round(255*hudLuminance(level))}, ${Math.round(102*hudLuminance(level))})`;}

// Borders use Gray4 (0–15), not textColor (0–4). Keep a visible frame even
// at the lowest auxiliary brightness so content does not lose its boundary.
export function hudBorderBrightness(config={}){return Math.max(4,Math.round(15*hudLuminance(hudBrightness(7,config))));}
export function hudBorderPreviewColor(config={}){
 const ratio=hudBorderBrightness(config)/15;
 return `rgb(0, ${Math.round(255*ratio)}, ${Math.round(102*ratio)})`;
}
