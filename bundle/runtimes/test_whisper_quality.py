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


if __name__ == '__main__':
    unittest.main()
