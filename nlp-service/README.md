# Lembas Links — NLP Pipeline

An offline, one-time pipeline that generates the LOTR-themed slug pool consumed
by the API's `quotes` table (see `../db/seeds/quotes.sql`). It is not part of
the running application — there's no `nlp-service` entry in
`docker-compose.yml`, and nothing in the API calls out to it at runtime.

### Structure

- `nlp_preprocess.py` — cleans raw quote text, extracts keywords and LOTR
  entities (characters/places/artifacts) via spaCy, and scores each quote by
  length, keyword richness, and entity density
- `slug_generator.py` — calls the Claude Haiku API per quote to generate
  2-3 word hyphenated slugs, then sanitizes and resolves collisions against an
  in-memory set
- `generate_slugs.py` — orchestrates the two steps above and writes
  `data/quotes.sql`
- `main.py` — a minimal FastAPI app exposing only a `/health` stub; it isn't
  wired into Docker Compose or run in normal operation
- `data/` — `famous_quotes.py` (curated list always kept regardless of
  score), `lotr_scripts.csv` (source dataset), `quotes.sql` (pipeline output)
- `tests/` — pytest suite covering preprocessing and slug generation

--- 

## How It Works

1. **Data loading** — reads the [LOTR movie script dataset](https://www.kaggle.com/datasets/paultimothymooney/lord-of-the-rings-data?select=lotr_scripts.csv) (~2,000 quotes) from a csv file
2. **Preprocessing** — cleans text, filters by character relevance and quote quality using spaCy
3. **Scoring** — ranks quotes by keyword richness and named entity density
4. **Famous quote detection** — fuzzy matches against a curated list of 'famous' quotes using rapidfuzz, ensuring they always make it into the pool regardless of score
5. **Slug generation** — sends enriched quote data to Claude Haiku API with extracted keywords and named entities, generating memorable 2-3 word hyphenated slugs
6. **Collision handling** — avoids duplicate slugs using an in-memory set
7. **Output** — writes `db/seeds/quotes.sql` with LOTR themed slugs ready to seed!

## Running the pipeline

```bash
cd nlp-service
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python -m spacy download en_core_web_sm
cp .env.example .env   # add your ANTHROPIC_API_KEY
cd ..
make generate           # cd nlp-service && python generate_slugs.py
cp nlp-service/data/quotes.sql db/seeds/quotes.sql
```

This reads `data/lotr_scripts.csv`, scores and filters quotes, calls the
Claude Haiku API to generate slugs, and writes `data/quotes.sql`.

## Environment Variables

This directory has its own `.env`, separate from the root one used by the API
and Docker Compose. Copy `.env.example` and fill in:

| Variable | Description | Default |
|---|---|---|
| `ANTHROPIC_API_KEY` | Anthropic API key used by `slug_generator.py` to call Claude Haiku; only needed when regenerating the slug pool | required |

## Running the tests

```bash
make test-nlp   # from repo root, venv activated — cd nlp-service && pytest
```

Or directly:
```bash
cd nlp-service
pytest   # picks up tests/, per pytest.ini
```

