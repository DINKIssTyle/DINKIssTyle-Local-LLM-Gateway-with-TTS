// Keep unknown frames: unsupported host versions must not silently mute everyone.
export function filterSpeakerPCM(bytes,role,{glassesMode,speakerScope,translateExcludeSelf=true}) {
  const exclude=glassesMode==='translate' && translateExcludeSelf
    ? role==='self'
    : speakerScope==='self' && role==='other';
  // Preserve PCM timing and VAD silence boundaries rather than dropping time.
  return exclude?new Uint8Array(bytes.length):bytes;
}
