import math
import types
import unittest

import jev_worker


class FakeVector(list):
    def __getitem__(self, key):
        value = super().__getitem__(key)
        return FakeVector(value) if isinstance(key, slice) else value

    def tolist(self):
        return list(self)


class LengthLimitedEmbedder:
    def __init__(self):
        self.attempts = []
        self.parts = []

    def embed_text(self, text):
        self.attempts.append(text)
        prefix, content = text.split(" | ", 1)
        part = content.split(": ", 1)[1]
        # Simulate token counts that differ from Python character counts.
        tokens = len(part.encode("utf-8")) + 32
        if tokens > 128:
            raise ValueError(
                f"Input sequence length ({tokens}) exceeds maximum supported signature length (128).; Failed to compute text embedding."
            )
        self.parts.append(part)
        vector = FakeVector([1.0, float(part.count("끝")) + 1.0] + [0.0] * 766)
        return types.SimpleNamespace(embeddings=[types.SimpleNamespace(embedding=vector)])


class WorkerLengthTests(unittest.TestCase):
    def test_short_input_uses_one_inference(self):
        embedder = LengthLimitedEmbedder()
        result = jev_worker.embed_text(embedder, "짧은 질문", "query")
        self.assertEqual(len(embedder.attempts), 1)
        self.assertTrue(embedder.attempts[0].startswith("task: search result | query: "))
        self.assertAlmostEqual(sum(v * v for v in result), 1.0)

    def test_recursive_split_preserves_all_unicode_and_tail(self):
        for usage in ["document", "query"]:
            with self.subTest(usage=usage):
                embedder = LengthLimitedEmbedder()
                text = "한글🙂e\u0301" * 200 + "끝끝끝"
                result = jev_worker.embed_text(embedder, text, usage)
                self.assertEqual("".join(embedder.parts), text)
                self.assertGreater(len(embedder.parts), 2)
                self.assertEqual(len(result), 256)
                self.assertTrue(all(math.isfinite(v) for v in result))
                self.assertAlmostEqual(sum(v * v for v in result), 1.0)
                self.assertGreater(result[1], result[0])  # Tail affects the pooled result.

    def test_other_runtime_errors_are_not_retried(self):
        class BrokenEmbedder:
            calls = 0

            def embed_text(self, text):
                self.calls += 1
                raise RuntimeError("model runtime failure")

        embedder = BrokenEmbedder()
        with self.assertRaisesRegex(RuntimeError, "model runtime failure"):
            jev_worker.embed_text(embedder, "x" * 200, "document")
        self.assertEqual(embedder.calls, 1)


if __name__ == "__main__":
    unittest.main()
