"""Offline Whisper ONNX worker. JSON lines on stdin/stdout, no network port."""
import argparse
import base64
import io
import json
import os
import sys
import wave
import hashlib
import re
import zlib

os.environ['HF_HUB_OFFLINE'] = '1'
os.environ['TRANSFORMERS_OFFLINE'] = '1'


def send(value):
    print(json.dumps(value, ensure_ascii=False), flush=True)


def transcript_rejection(text, average_logprob=None):
    """Reject decoding failures instead of saving them as something someone said."""
    data = text.strip().encode('utf-8')
    if not data:
        return ''
    words = re.findall(r'\w+', text.lower())
    if len(words) >= 6 and max(words.count(word) for word in set(words)) / len(words) >= 0.7:
        return 'repetition'
    if len(data) > 80 and len(data) / len(zlib.compress(data)) > 2.4:
        return 'repetition'
    if average_logprob is not None and average_logprob < -1.0:
        return 'low-confidence'
    return ''


def decode_audio(request):
    import numpy as np
    raw = base64.b64decode(request['audio'], validate=True)
    if request.get('format', 'pcm') == 'wav':
        with wave.open(io.BytesIO(raw), 'rb') as wav:
            if (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) != (1, 2, 16000):
                raise ValueError('WAV must be 16 kHz, mono, signed 16-bit PCM')
            raw = wav.readframes(wav.getnframes())
    if len(raw) % 2 or not 3200 <= len(raw) <= 960000:
        raise ValueError('Audio must contain 0.1–30 seconds of 16 kHz PCM16 mono')
    return np.frombuffer(raw, dtype='<i2').astype(np.float32) / 32768.0


class SpeechFilter:
    """Silero ONNX: 512 samples plus 64 samples of context at 16 kHz.

    Model contract: https://github.com/snakers4/silero-vad (MIT).
    This rejects non-speech; it does not isolate a voice from overlapping sounds.
    """
    def __init__(self, model_path):
        import onnxruntime as ort
        with open(model_path, 'rb') as model_file:
            digest = hashlib.sha256(model_file.read()).hexdigest()
        if digest != '1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3':
            raise ValueError('Silero VAD model checksum mismatch; download STT again')
        options = ort.SessionOptions()
        options.intra_op_num_threads = options.inter_op_num_threads = 1
        self.session = ort.InferenceSession(model_path, sess_options=options, providers=['CPUExecutionProvider'])

    def apply(self, audio, mode='strict'):
        import numpy as np
        state = np.zeros((2, 1, 128), dtype=np.float32)
        context = np.zeros((1, 64), dtype=np.float32)
        voiced = []
        for offset in range(0, len(audio), 512):
            frame = audio[offset:offset + 512]
            frame = np.pad(frame, (0, 512 - len(frame))).reshape(1, 512)
            value = np.concatenate((context, frame), axis=1)
            probability, state = self.session.run(None, {'input': value, 'state': state, 'sr': np.array(16000, dtype=np.int64)})
            voiced.append(float(probability.reshape(-1)[0]) >= (0.75 if mode == 'strict' else 0.5))
            context = frame[:, -64:]
        run = longest = 0
        for speech in voiced:
            run = run + 1 if speech else 0
            longest = max(longest, run)
        if sum(voiced) < 12 or longest < 5:
            return np.empty(0, dtype=np.float32)
        # Trim only the outside boundaries. Collapsing internal pauses changes
        # speech timing and makes short, noisy continuous clips harder to decode.
        indices = np.flatnonzero(voiced)
        start = max(0, (int(indices[0]) - 5) * 512)
        end = min(len(audio), (int(indices[-1]) + 6) * 512)
        return audio[start:end]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--model', required=True)
    parser.add_argument('--backend', choices=['onnx','mlx'], default='onnx')
    args = parser.parse_args()
    import numpy as np
    speech_filter = SpeechFilter(os.path.join(args.model, 'silero_vad.onnx'))
    if args.backend == 'mlx':
        import mlx.core as mx
        import mlx_whisper
        from mlx_whisper.transcribe import ModelHolder
        model = ModelHolder.get_model(args.model, mx.float16)
        mx.eval(model.parameters())
        mx.synchronize()
    else:
        import torch
        import onnxruntime as ort
        from transformers import WhisperProcessor
        from optimum.onnxruntime import ORTModelForSpeechSeq2Seq
        torch.set_num_threads(min(4, os.cpu_count() or 1))
        options = ort.SessionOptions()
        options.intra_op_num_threads = min(4, os.cpu_count() or 1)
        processor = WhisperProcessor.from_pretrained(args.model, local_files_only=True)
        model = ORTModelForSpeechSeq2Seq.from_pretrained(
            args.model, local_files_only=True, use_merged=True,
            provider='CPUExecutionProvider', session_options=options,
        )
    send({'ready': True})
    for line in sys.stdin:
        try:
            request = json.loads(line)
            audio = decode_audio(request)
            duration = len(audio) / 16000
            # Avoid Whisper's hallucinated captions on digital silence.
            if float(np.sqrt(np.mean(audio * audio))) < 0.001:
                send({'text': '', 'duration': len(audio) / 16000})
                continue
            audio = speech_filter.apply(audio, request.get('speechFilter') or 'strict')
            if not len(audio):
                send({'text': '', 'duration': duration, 'filtered': True})
                continue
            if args.backend == 'mlx':
                result = mlx_whisper.transcribe(
                    audio, path_or_hf_repo=args.model, language=request.get('language') or 'ko',
                    task='transcribe', verbose=None, temperature=0.0,
                    condition_on_previous_text=False, compression_ratio_threshold=2.4,
                    logprob_threshold=-1.0, no_speech_threshold=0.6,
                    sample_len=min(192, max(32, int(len(audio)/16000*16))),
                )
                text = result.get('text', '').strip()
                segments = result.get('segments') or []
                weights = [max(1,len(segment.get('tokens',[]))) for segment in segments]
                average = (sum(segment.get('avg_logprob',-10)*weight for segment,weight in zip(segments,weights))/sum(weights)) if weights else None
                rejected = transcript_rejection(text, average)
                if any(segment.get('no_speech_prob',0)>.6 and segment.get('avg_logprob',-10)<-.5 for segment in segments):
                    rejected = rejected or 'no-speech'
                send({'text': '' if rejected else text, 'duration': duration,
                      'rejected': bool(rejected), 'reason': rejected, 'averageLogprob': average})
                continue
            inputs = processor(audio, sampling_rate=16000, return_tensors='pt', return_attention_mask=True)
            with torch.inference_mode():
                generated = model.generate(
                    inputs.input_features, attention_mask=inputs.attention_mask,
                    language=request.get('language') or 'ko', task='transcribe',
                    max_new_tokens=min(192, max(32, int(len(audio) / 16000 * 16))),
                    do_sample=False, return_dict_in_generate=True, output_scores=True,
                )
            text = processor.batch_decode(generated.sequences, skip_special_tokens=True)[0].strip()
            probabilities = []
            scores = generated.scores or ()
            chosen = generated.sequences[0, -len(scores):] if scores else ()
            for score, token in zip(scores, chosen):
                probability = torch.log_softmax(score[0].float(), dim=-1)[token].item()
                if np.isfinite(probability):
                    probabilities.append(probability)
            average = sum(probabilities) / len(probabilities) if probabilities else None
            rejected = transcript_rejection(text, average)
            # An unfinished decode at the token ceiling is unsafe to treat as a
            # complete utterance, especially when it is already looping.
            eos = model.generation_config.eos_token_id
            if generated.sequences[0, -1].item() not in ([eos] if isinstance(eos, int) else eos):
                rejected = rejected or 'incomplete-decode'
            send({'text': '' if rejected else text, 'duration': duration,
                  'rejected': bool(rejected), 'reason': rejected, 'averageLogprob': average})
        except Exception as exc:
            send({'error': str(exc)})


if __name__ == '__main__':
    try:
        main()
    except Exception as exc:
        send({'error': str(exc)})
        sys.exit(1)
