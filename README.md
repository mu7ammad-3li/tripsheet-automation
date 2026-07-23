# POC: Trucking Trip Sheet Automation

Automated extraction of handwritten trucking trip sheets using Vision Language Models (VLMs), validated by a deterministic Go backend, and persisted to Postgres.

## Project Structure

```
├── docs/                    # Architecture & planning docs
│   └── phase_1_planning.md  # Phase 1 engineering decisions
├── scripts/
│   └── benchmark_vlm.py     # VLM benchmarking script (Python)
├── test_data/
│   ├── images/              # Sample trip sheet images
│   └── ground_truth.json    # Human-labeled ground truth for scoring
├── requirements.txt         # Python dependencies
└── MVP Implementation Plan.md
```

## Quick Start

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
export GEMINI_API_KEY="your_key_here"
python scripts/benchmark_vlm.py
```

## MVP Phases

1. **Phase 1** – AI Extraction Core (VLM → JSON) ← *current*
2. **Phase 2** – Go Ingestion & Validation API
3. **Phase 3** – Persistence (Postgres)
4. **Phase 4** – TMS/Accounting Hand-off
