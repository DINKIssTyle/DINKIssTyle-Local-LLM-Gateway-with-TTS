"""Private stdio worker: local MediaPipe embeddings and optional Jev decisions.

No listening port, credentials, or generative model. Stdout is JSONL only.
Pinned to the tested MediaPipe 1.1.0 Python API.
"""
import argparse
import json
import math
import re
import sys

MAX_INPUT_TOKENS = 2048
LENGTH_ERROR = re.compile(
    r"Input sequence length \(\d+\) exceeds maximum supported signature length \(\d+\)"
)


def normalized(vector):
    magnitude = math.sqrt(sum(v * v for v in vector))
    if not math.isfinite(magnitude) or not magnitude:
        raise ValueError("invalid embedding magnitude")
    return [v / magnitude for v in vector]


def embed_text(embedder, text, usage):
    """Keep the full input, splitting only on the SDK's actual token overflow.

    Character-sized DB chunks can exceed a token budget in Korean or code.
    Pool normalized MRL vectors by source length when splitting is needed.
    Python string slices preserve Unicode code points; no tail is discarded.
    """
    prefix = "task: search result | query: " if usage == "query" else "title: memory | text: "

    def embed_part(part):
        try:
            vector = embedder.embed_text(prefix + part).embeddings[0].embedding[:256].tolist()
            return normalized(vector)
        except Exception as exc:
            if not LENGTH_ERROR.search(str(exc)) or len(part) < 2:
                raise
            middle = len(part) // 2
            left, right = embed_part(part[:middle]), embed_part(part[middle:])
            weight = middle / len(part)
            return [a * weight + b * (1 - weight) for a, b in zip(left, right)]

    return normalized(embed_part(text))


def send(value):
    print(json.dumps(value, ensure_ascii=False, allow_nan=False), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    args = parser.parse_args()
    from mediapipe.tasks.python import BaseOptions, retrieval, decision

    embedder = retrieval.UniversalEmbedder.create_from_options(
        retrieval.UniversalEmbedderOptions(
            base_options=BaseOptions(model_asset_path=args.model),
            l2_normalize=True,
            max_input_length=MAX_INPUT_TOKENS,
        )
    )
    decider = None
    try:
        # Warm the graph before advertising readiness.
        embedder.embed_text("task: search result | query: memory")
        send({"ready": True, "dimension": 256})
        for line in sys.stdin:
            try:
                request = json.loads(line)
                if request["op"] == "embed":
                    send({"vector": embed_text(embedder, request["text"], request.get("usage"))})
                elif request["op"] == "choice":
                    # Python 1.1.0 does not expose shared-embedder creation.
                    # Create the decision session only when explicitly enabled.
                    if decider is None:
                        decider = decision.DecisionMaker.create_from_options(
                            decision.DecisionMakerOptions(
                                base_options=BaseOptions(model_asset_path=args.model),
                                max_num_tokens=MAX_INPUT_TOKENS,
                            )
                        )
                    question = decision.ChoiceQuestion(
                        criteria=request["criteria"],
                        instructions=request.get("instructions", ""),
                    )
                    result = decider.evaluate_choice(request["text"], question)
                    send({"selected": result.selected_key, "probabilities": result.probabilities})
                else:
                    raise ValueError("unknown operation")
            except Exception as exc:
                send({"error": str(exc)})
    finally:
        if decider is not None:
            decider.close()
        embedder.close()


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        send({"error": str(exc)})
        sys.exit(1)
