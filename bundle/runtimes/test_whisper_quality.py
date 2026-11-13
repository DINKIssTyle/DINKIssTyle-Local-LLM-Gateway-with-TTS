import unittest
from whisper_worker import transcript_rejection


class TranscriptQualityTests(unittest.TestCase):
    def test_repeated_word_from_reported_failure(self):
        self.assertEqual(transcript_rejection('여기 ' * 100, -0.1), 'repetition')

    def test_compressed_loop_without_spaces(self):
        self.assertEqual(transcript_rejection('여기' * 100, -0.1), 'repetition')

    def test_low_confidence_is_not_accepted_as_speech(self):
        self.assertEqual(transcript_rejection('오늘 회의는 세 시입니다.', -2.0), 'low-confidence')

    def test_normal_korean_sentence(self):
        self.assertEqual(transcript_rejection('안녕하세요. 오늘 회의는 오후 3시에 시작합니다.', -0.12), '')

    def test_reported_video_closing_with_weak_audio(self):
        text = '시청해주셔서 감사합니다. 다음 영상에서 또 만나요.'
        self.assertEqual(transcript_rejection(text, -0.10, 0.55, 0.1, 0.1), 'unsupported-video-caption')

    def test_reported_closing_with_uncertain_decode(self):
        self.assertEqual(transcript_rejection('시청해주셔서 감사합니다.', -0.6, 3.0, 0.9, 0.1), 'unsupported-video-caption')

    def test_actual_clear_video_speech_is_not_censored(self):
        text = '시청해주셔서 감사합니다. 다음 영상에서 또 만나요.'
        self.assertEqual(transcript_rejection(text, -0.1, 4.0, 0.9, 0.05), '')

    def test_normal_thanks_and_topic_are_preserved(self):
        self.assertEqual(transcript_rejection('감사합니다.', -0.1, 0.5, 0.8, 0.05), '')
        self.assertEqual(transcript_rejection('청동기 시대', -0.1, 0.5, 0.8, 0.05), '')

    def test_excessive_text_on_tiny_speech_fragment(self):
        self.assertEqual(transcript_rejection('정말로 이런 말은 전혀 하지 않았는데 문장이 길게 생성됩니다.', -0.1, 0.45, 0.3), 'text-exceeds-speech')

    def test_boilerplate_phrase_inside_regular_conversation_is_not_blocked(self):
        self.assertEqual(transcript_rejection('방송 끝에 시청해주셔서 감사합니다라고 말했어.', -0.15, 4.0, 0.8, 0.05), '')


if __name__ == '__main__':
    unittest.main()
