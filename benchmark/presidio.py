#!/usr/bin/env python3
"""Run the benchmark questions through Microsoft Presidio and write
predictions in the format `benchmark score` reads.

Presidio is an open-source PII detector (regex + spaCy NER). It finds
entities; it has no notion of keys, markings, health, finance, legal or
code, so entity types are mapped to the nearest Sluss class below and
everything else counts as "flagged, class unknown".

    pip install presidio-analyzer && python -m spacy download en_core_web_lg
    python3 benchmark/presidio.py benchmark/corpus.jsonl > presidio.jsonl
    go run ./cmd/benchmark score presidio.jsonl
"""
import json
import sys
import time

from presidio_analyzer import AnalyzerEngine

MAP = {
    "PERSON": "pii", "EMAIL_ADDRESS": "pii", "PHONE_NUMBER": "pii",
    "LOCATION": "pii", "CREDIT_CARD": "pii", "IBAN_CODE": "pii",
    "IP_ADDRESS": "pii", "NRP": "pii", "US_SSN": "pii", "UK_NHS": "health",
    "MEDICAL_LICENSE": "health", "CRYPTO": "financial", "US_BANK_NUMBER": "financial",
    "URL": None, "DATE_TIME": None,
}
# Presidio's own recommendation is to ignore low-confidence hits; 0.5 is its
# documented default threshold in most examples.
THRESHOLD = 0.5
# Entities that alone do not make a question sensitive for routing purposes.
IGNORE = {"DATE_TIME", "URL"}


def main(path):
    engine = AnalyzerEngine()
    with open(path, encoding="utf-8") as f:
        cases = [json.loads(l) for l in f if l.strip()]
    for c in cases:
        start = time.perf_counter()
        hits = engine.analyze(text=c["text"], language="en", score_threshold=THRESHOLD)
        ms = (time.perf_counter() - start) * 1000
        types = sorted({h.entity_type for h in hits} - IGNORE)
        classes = sorted({MAP.get(t) or "pii" for t in types})
        print(json.dumps({"id": c["id"], "flagged": bool(types), "classes": classes,
                          "ms": round(ms, 3), "tool": "presidio", "entities": types},
                         ensure_ascii=False))


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "benchmark/corpus.jsonl")
