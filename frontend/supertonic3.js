/*
 * Supertonic 3 Web Runtime (WebGPU / WASM SIMD Worker + Web Audio API Scheduler)
 * Based on DINKIssTyle-Wiki-Viewer Supertonic 3 implementation.
 */

(function attachSupertonic3(global) {
    const SUPERTONIC3_VOICES = [
        { id: 'M1', name: 'Alex' },
        { id: 'M2', name: 'James' },
        { id: 'M3', name: 'Robert' },
        { id: 'M4', name: 'Sam' },
        { id: 'M5', name: 'Daniel' },
        { id: 'F1', name: 'Sarah' },
        { id: 'F2', name: 'Lily' },
        { id: 'F3', name: 'Jessica' },
        { id: 'F4', name: 'Olivia' },
        { id: 'F5', name: 'Emily' },
    ];

    class Supertonic3WorkerClient {
        constructor() {
            const baseUrl = (typeof document !== 'undefined' && document.baseURI)
                ? document.baseURI
                : (typeof window !== 'undefined' ? window.location.href : 'http://localhost');
            const workerUrl = new URL('vendor/supertonic3-worker.js', baseUrl);
            this.worker = new Worker(workerUrl);
            this.pending = new Map();
            this.progressListeners = new Set();
            this.requestNumber = 0;

            this.handleMessage = (event) => {
                const message = event.data;
                if (!message) return;
                if (message.type === 'progress') {
                    for (const listener of this.progressListeners) {
                        listener(message);
                    }
                    return;
                }
                const request = this.pending.get(message.id);
                if (!request) return;
                this.pending.delete(message.id);
                if (message.type === 'error') {
                    request.reject(new Error(message.message || 'Synthesis error'));
                } else {
                    request.resolve(message);
                }
            };

            this.handleWorkerError = (event) => {
                const error = new Error(event?.message || 'Supertonic worker crashed.');
                for (const request of this.pending.values()) {
                    request.reject(error);
                }
                this.pending.clear();
            };

            this.worker.addEventListener('message', this.handleMessage);
            this.worker.addEventListener('error', this.handleWorkerError);
        }

        onProgress(listener) {
            this.progressListeners.add(listener);
            return () => this.progressListeners.delete(listener);
        }

        async preload(voice) {
            const response = await this.request({ type: 'preload', voice });
            if (response.type !== 'preloaded') throw new Error('Unexpected Supertonic preload response.');
            return response;
        }

        async synthesize(text, options = {}) {
            const response = await this.request({
                type: 'synthesize',
                text,
                language: options.language || 'ko',
                voice: options.voice || 'F1',
                speed: options.speed || 0.9,
                steps: options.steps || 5,
            });
            if (response.type !== 'result') throw new Error('Unexpected Supertonic synthesis response.');
            return response;
        }

        dispose() {
            this.worker.removeEventListener('message', this.handleMessage);
            this.worker.removeEventListener('error', this.handleWorkerError);
            this.worker.terminate();
            for (const request of this.pending.values()) {
                request.reject(new Error('Supertonic worker stopped.'));
            }
            this.pending.clear();
        }

        request(payload) {
            const id = `supertonic-${Date.now().toString(36)}-${++this.requestNumber}`;
            return new Promise((resolve, reject) => {
                this.pending.set(id, { resolve, reject });
                this.worker.postMessage({ ...payload, id });
            });
        }
    }

    class Supertonic3Runtime {
        constructor() {
            this.client = null;
            this.audioContext = null;
            this.sources = new Set();
            this.preparedVoices = new Set();
            this.run = 0;
            this.audioCursor = 0;
            this.synthesisQueue = Promise.resolve();
            this.activeBackend = 'WASM';
        }

        supported() {
            return typeof window !== 'undefined' &&
                typeof Worker !== 'undefined' &&
                typeof WebAssembly !== 'undefined' &&
                Boolean(window.AudioContext || window.webkitAudioContext);
        }

        async unlock() {
            if (!this.supported()) return;
            const context = this.getAudioContext();
            if (context && context.state === 'suspended') {
                await context.resume();
            }
        }

        cancel() {
            this.run += 1;
            for (const source of this.sources) {
                try {
                    source.stop();
                } catch {
                    // Ignore errors if source already stopped
                }
            }
            this.sources.clear();
            this.audioCursor = 0;
        }

        dispose() {
            this.cancel();
            this.client?.dispose();
            this.client = null;
            if (this.audioContext) {
                void this.audioContext.close();
            }
            this.audioContext = null;
            this.preparedVoices.clear();
        }

        async prepare(voice, onProgress) {
            if (this.preparedVoices.has(voice)) return;
            const client = this.getClient();
            const removeProgress = client.onProgress((progress) => {
                onProgress?.({
                    phase: progress.phase,
                    fraction: progress.fraction,
                });
            });
            onProgress?.({ phase: 'runtime', fraction: null });
            try {
                const result = await client.preload(voice);
                if (result?.backend) {
                    this.activeBackend = result.backend;
                }
                this.preparedVoices.add(voice);
            } finally {
                removeProgress();
            }
        }

        async speak(chunks, options = {}) {
            if (!this.supported()) throw new Error('Supertonic 3 is unavailable in this browser.');
            this.cancel();
            const run = this.run;
            await this.unlock();
            await this.prepare(options.voice || 'F1', options.onPreparationProgress);
            if (run !== this.run) throw new Error('Supertonic speech canceled.');
            options.onPreparationProgress?.(null);

            let finalPlayback = Promise.resolve();
            for (let index = 0; index < chunks.length; index += 1) {
                const chunk = String(chunks[index] || '').trim();
                if (!chunk) continue;
                const result = await this.synthesizeQueued(chunk, options);
                if (run !== this.run) throw new Error('Supertonic speech canceled.');
                if (result?.backend) {
                    this.activeBackend = result.backend;
                }
                const isFinal = index === chunks.length - 1;
                const pauseSeconds = isFinal ? 0 : boundaryPause(chunk);
                finalPlayback = this.schedule(result.samples, result.sampleRate, pauseSeconds);
            }
            await finalPlayback;
        }

        getClient() {
            if (!this.client) this.client = new Supertonic3WorkerClient();
            return this.client;
        }

        getAudioContext() {
            if (!this.audioContext) {
                const AudioCtx = (typeof window !== 'undefined') ? (window.AudioContext || window.webkitAudioContext) : null;
                if (AudioCtx) {
                    this.audioContext = new AudioCtx({ sampleRate: 44100 });
                }
            }
            return this.audioContext;
        }

        synthesizeQueued(text, options) {
            let resolveResult;
            let rejectResult;
            const result = new Promise((resolve, reject) => {
                resolveResult = resolve;
                rejectResult = reject;
            });

            this.synthesisQueue = this.synthesisQueue
                .catch(() => undefined)
                .then(async () => {
                    try {
                        resolveResult(await this.getClient().synthesize(text, options));
                    } catch (error) {
                        rejectResult(error);
                    }
                });
            return result;
        }

        schedule(samples, sampleRate, pauseSeconds = 0.08) {
            const context = this.getAudioContext();
            if (!context) return Promise.resolve();
            const buffer = context.createBuffer(1, samples.length, sampleRate);
            buffer.getChannelData(0).set(samples);

            const source = context.createBufferSource();
            source.buffer = buffer;
            source.connect(context.destination);

            const start = Math.max(this.audioCursor || context.currentTime + 0.06, context.currentTime + 0.04);
            this.audioCursor = start + (samples.length / sampleRate) + pauseSeconds;
            this.sources.add(source);
            source.start(start);

            return new Promise((resolve) => {
                source.addEventListener('ended', () => {
                    this.sources.delete(source);
                    resolve();
                }, { once: true });
            });
        }

        async scheduleAudioBuffer(audioBuffer, pauseSeconds = 0.08) {
            const context = this.getAudioContext();
            if (!context) return Promise.resolve();
            const source = context.createBufferSource();
            source.buffer = audioBuffer;
            source.connect(context.destination);

            const start = Math.max(this.audioCursor || context.currentTime + 0.06, context.currentTime + 0.04);
            this.audioCursor = start + audioBuffer.duration + pauseSeconds;
            this.sources.add(source);
            source.start(start);

            return new Promise((resolve) => {
                source.addEventListener('ended', () => {
                    this.sources.delete(source);
                    resolve();
                }, { once: true });
            });
        }
    }

    function boundaryPause(chunk) {
        const lastChar = String(chunk || '').trim().slice(-1);
        const pauses = {
            '?': 0.28,
            '!': 0.24,
            '.': 0.22,
            ';': 0.16,
            ':': 0.13,
            ',': 0.09,
        };
        return pauses[lastChar] ?? 0.08;
    }

    function cleanSpeechText(value) {
        return String(value || '')
            .replace(/(?:https?|ftp):\/\/[^\s<>()]+/giu, (match) => /[.,!?。！？]$/u.test(match) ? ` ${match[match.length - 1]} ` : ' ')
            .replace(/\bwww\.[^\s<>()]+/giu, (match) => /[.,!?。！？]$/u.test(match) ? ` ${match[match.length - 1]} ` : ' ')
            .replace(/[\[［【]\s*\d+(?:\s*[-–—,]\s*\d+)*\s*[\]］】]/gu, ' ')
            .replace(/\[(?:citation needed|출처 필요)\]/giu, ' ')
            .replace(/\s+/g, ' ')
            .replace(/\s+([,.;:!?。！？])/g, '$1')
            .trim();
    }

    function splitLongPart(value, maxLength) {
        if (value.length <= maxLength) return [value];
        const words = value.split(/\s+/);
        const parts = [];
        let current = '';

        for (const word of words) {
            if (!current) {
                current = word;
            } else if (`${current} ${word}`.length <= maxLength) {
                current += ` ${word}`;
            } else {
                parts.push(current);
                current = word;
            }
        }
        if (current) parts.push(current);
        return parts;
    }

    function chunkSpeechText(value, maxLength = 600) {
        const normalized = String(value || '').trim();
        if (!normalized) return [];

        const sentences = normalized
            .split(/\n+|(?<=[.!?。！？])\s+/u)
            .flatMap((part) => splitLongPart(part.trim(), maxLength))
            .filter(Boolean);
        const chunks = [];
        let current = '';

        for (const sentence of sentences) {
            if (!current) {
                current = sentence;
            } else if (`${current} ${sentence}`.length <= maxLength) {
                current += ` ${sentence}`;
            } else {
                chunks.push(current);
                current = sentence;
            }
        }
        if (current) chunks.push(current);
        return chunks;
    }

    const defaultRuntime = new Supertonic3Runtime();

    const DKSTSupertonic3 = {
        SUPERTONIC3_VOICES,
        Supertonic3WorkerClient,
        Supertonic3Runtime,
        boundaryPause,
        cleanSpeechText,
        chunkSpeechText,
        isSupertonic3Supported: () => defaultRuntime.supported(),
        unlockSupertonic3Audio: () => defaultRuntime.unlock(),
        cancelSupertonic3Speech: () => defaultRuntime.cancel(),
        disposeSupertonic3: () => defaultRuntime.dispose(),
        speakSupertonic3: (chunks, options) => defaultRuntime.speak(chunks, options),
        scheduleSupertonic3AudioBuffer: (buf, pause) => defaultRuntime.scheduleAudioBuffer(buf, pause),
        getSupertonic3AudioContext: () => defaultRuntime.getAudioContext(),
        getSupertonic3Backend: () => defaultRuntime.activeBackend,
    };

    global.DKSTSupertonic3 = DKSTSupertonic3;
})(typeof window !== 'undefined' ? window : globalThis);
