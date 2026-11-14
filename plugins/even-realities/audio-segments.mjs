export function pcmLevel(bytes) {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let sum = 0;
  for (let i = 0; i + 1 < bytes.length; i += 2) {
    const sample = view.getInt16(i, true) / 32768;
    sum += sample * sample;
  }
  return Math.sqrt(sum / Math.max(1, Math.floor(bytes.length / 2)));
}

export function mergePCM(parts) {
  const merged = new Uint8Array(parts.reduce((size, part) => size + part.length, 0));
  let offset = 0;
  for (const part of parts) { merged.set(part, offset); offset += part.length; }
  return merged;
}

// Segment by audio sample count, so delayed Bluetooth batches do not change VAD.
export function speechSegmentOptions(engine,mode){
  if(engine==='whisper')return {};
  if(engine==='apple'){
    return {snapshotMinMs:250,snapshotVoicedMs:150,...(mode==='translate'?{silenceMs:900,maxMs:8000}:{})};
  }
  return {snapshotMinMs:250,snapshotVoicedMs:150,...(mode==='translate'?{silenceMs:1600,maxMs:20000}:{})};
}

export class SpeechSegmenter {
  constructor({ threshold = 0.008, silenceMs = 900, maxMs = 12000, snapshotMinMs = 1200, snapshotVoicedMs = 400 } = {}) {
    Object.assign(this, { threshold, silenceMs, maxMs, snapshotMinMs, snapshotVoicedMs });
    this.reset();
  }
  reset() { this.parts = []; this.preRoll = []; this.preMs = 0; this.ms = 0; this.silence = 0; this.voiced = 0; }
  push(bytes) {
    if (!bytes.length || bytes.length % 2) throw new Error('잘못된 PCM16 오디오 프레임');
    const ms = bytes.length / 32;
    const speech = pcmLevel(bytes) >= this.threshold;
    if (!this.parts.length && !speech) {
      this.preRoll.push(bytes); this.preMs += ms;
      while (this.preMs > 250 && this.preRoll.length > 1) this.preMs -= this.preRoll.shift().length / 32;
      return null;
    }
    if (!this.parts.length) { this.parts = this.preRoll; this.ms = this.preMs; this.preRoll = []; this.preMs = 0; }
    this.parts.push(bytes); this.ms += ms;
    this.voiced += speech ? ms : 0;
    this.silence = speech ? 0 : this.silence + ms;
    if (this.silence >= this.silenceMs || this.ms >= this.maxMs) return this.flush();
    return null;
  }
  flush() {
    const clip = this.voiced >= 250 ? mergePCM(this.parts) : null;
    this.reset();
    return clip;
  }
  snapshot() {
    return this.ms >= this.snapshotMinMs && this.voiced >= this.snapshotVoicedMs ? mergePCM(this.parts) : null;
  }
}
