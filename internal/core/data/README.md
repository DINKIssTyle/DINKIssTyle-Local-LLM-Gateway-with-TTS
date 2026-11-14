# Local hint keyword index

`hint-keywords.tsv.gz` is a sorted UTF-8 title index derived from the Korean Wikipedia namespace-0 public title dump, with a small set of project/technology names. It contains names, not article bodies or a new model. The companion JSON records the source URL, input SHA-256, row count and byte size.

Source: https://dumps.wikimedia.org/kowiki/latest/kowiki-latest-all-titles-in-ns0.gz
Wikimedia dump information/terms: https://dumps.wikimedia.org/legal.html

Rebuild with `python3 scripts/build-hint-lexicon.py /path/to/all-titles-in-ns0.gz`. Ordinary application builds do not download data. The index is embedded in the gateway, decompressed once, and queried by binary search over compact row offsets. Full input transcriptions never leave the device for dictionary matching.

Korean particles are removed only if the remaining span matches a known name. Original surface spans and sentence context are retained. This is dictionary-assisted extraction, not a full morphological parser; unknown terms still use the existing JEV sentence route. JEV chooses useful keywords using observed conversation and recently shown information; dictionary membership alone never generates a hint.
