# Lembas Links

A Lord of the Rings themed URL shortener built in Go (Gin) with Redis caching, API key authentication middleware, and rate limiting.

**Live Demo:** *(coming soon)*

---

## Table of Contents
- [Features](#features)
- [Description, Project Outcomes, and Future Plans](#description)
- [How it Works](#how-it-works)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [NLP Pipeline](#nlp-pipeline)
- [Deployment](#deployment)

---

## Features

- LOTR themed slugs generated from quote keyphrases
- Redis caching on all redirects
- API key authentication middleware on protected routes
- Redis-based rate limiting per IP and per API key
- Click analytics: timestamp, referrer, user agent, and IP on every redirect asynchronously
- Fully containerized with Docker Compose
- Automatic database migrations on startup

---

## Description, Project Outcomes, and Future Plans

As a huge Lord of the Rings fan, I've always been looking for ways to incorporate my love for the fantasy franchise into different aspects of my life (seriously, I talk about it way too much). Thus, when I thought of Lembas Links, it felt like the perfect opportunity; Not only to make a LOTR themed project, but also to learn important backend programming principles and apply skills I've learned from my Natural Language Processing coursework at Boston College.

In building this Lembas Links, I gained hands-on, end-to-end experience designing and implementing a REST API in Go using Gin, a relevant backend framework. This experience really helped me understand how each component of the backend architecture interacts with one another, such as how authentication middleware, rate limiting, and the models layer interact, deepening my understanding of backend security and clean API design. Additionally, I gained valuable insight into important caching principles and practices through my use of Redis and how such technologies can improve performance. Lastly, I also developed practical skills in containerization with Docker Compose, building upon my previous experience using the technology.

Currently, I'm working on creating documentation for the API endpoints using Go's OpenAPI library. Next, I plan on deploying the API on Railway, before building a frontend in React.js.

Thanks for checking out my Lembas Links repo! If you have any questions please feel free to get in touch.

---

## How It Works

Incoming requests hit the Go API which is built with Gin. The API first checks Redis and redirects on a cache hit, recording the click asynchronously. On a cache miss it queries Postgres, caches the result with a TTL, and redirects. Rate limiting is handled by Redis middleware that runs before every request, incrementing a count associated with both the user's API key and IP address. All persistent data lives in Postgres.

The NLP preprocessing pipeline is a separate tool that runs once to generate the slug pool. It reads a [LOTR movie script CSV](https://www.kaggle.com/datasets/paultimothymooney/lord-of-the-rings-data?select=lotr_scripts.csv), processes quotes through spaCy for keyword extraction and entity recognition, then calls the Claude Haiku API to generate slugs. The output is a SQL seed file that gets loaded into Postgres at setup time.

---

## Tech Stack

| Layer | Technology |
|---|---|
| API | Go 1.25, Gin |
| Database | Postgres 15 |
| Cache + Rate Limiting | Redis 7 |
| NLP Pipeline | Python 3.11, spaCy, pandas, RAKE, rapidfuzz, Claude Haiku API |
| Containerization | Docker, Docker Compose |
| Migrations | golang-migrate |

---

## Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.25+
- Python 3.11+ *(for NLP pipeline only)*

### 1. Clone the repo
```bash
git clone https://github.com/wesley-lawson13/lembas-links.git
cd lembas-links
```

### 2. Set up environment variables
```bash
cp .env.example .env
```
Fill in the required values — see [Environment Variables](#environment-variables) for details.

### 3. Start the services
```bash
make run
```

### 4. Seed the database
In a new terminal window:
```bash
make seed
```
Loads the pre-generated LOTR themed slug pool (~340 slugs) into Postgres.

### 5. Create a dev API key
```bash
make seed-dev
```
Inserts a test API key for local development.

### 6. Verify everything is running
```bash
curl http://localhost:8080/health
```

---

## API Reference

### Authentication

Protected endpoints require an API key passed in the `Authorization` header:
    `Authorization: your-api-key`

### Endpoints

#### `POST /links` — Protected
Create a new Lord of the Rings link.

**Request:**
```json
{
    "url": "https://your-long-url.com"
}
```

**Response `201`:**
```json
{
    "slug": "gandalf-shadow-flame",
    "short_url": "http://localhost:8080/gandalf-shadow-flame",
    "original": "https://your-long-url.com"
}
```

---

#### `GET /:slug` — Public
Redirect to the original URL. Checks Redis cache first, falls back to Postgres. Records click analytics asynchronously.

**Response:** `302` redirects to original URL

| Status | Meaning |
|---|---|
| `302` | Redirect successful |
| `404` | Slug not found or link inactive |
| `410` | Link has expired |

---

#### `GET /links/:slug/stats` — Protected
Get analytics for a Lord of the Rings link.

**Response `200`:**
```json
{
    "slug": "gandalf-shadow-flame",
    "original": "https://your-long-url.com",
    "click_count": 42,
    "created_at": "2026-04-06T16:00:00Z",
    "expires_at": "2026-05-06T16:00:00Z",
    "is_active": true,
    "recent_clicks": [
        {
            "id": "abc123",
            "slug": "gandalf-shadow-flame",
            "clicked_at": "2026-04-09T18:10:03Z",
            "referrer": "https://twitter.com",
            "user_agent": "Mozilla/5.0...",
            "ip_address": "192.168.1.1"
        }
    ]
}
```

---

#### `DELETE /links/:slug` — Protected
Soft delete a Lord of the Rings link. Immediately invalidates Redis cache.

**Response:** `204 No Content`

---

### Rate Limits

| Type | Limit |
|---|---|
| Per IP | 60 requests/minute |
| Per API Key | 120 requests/minute |

Exceeding limits returns `429 Too Many Requests`.

---

## NLP Pipeline

The slug generation pipeline runs offline as a one-time preprocessing step and is not part of the running application.

### How It Works

1. **Data loading** — reads the [LOTR movie script dataset](https://www.kaggle.com/datasets/paultimothymooney/lord-of-the-rings-data?select=lotr_scripts.csv) (~2,000 quotes) from a csv file
2. **Preprocessing** — cleans text, filters by character relevance and quote quality using spaCy
3. **Scoring** — ranks quotes by keyword richness and named entity density
4. **Famous quote detection** — fuzzy matches against a curated list of 'famous' quotes using rapidfuzz, ensuring they always make it into the pool regardless of score
5. **Slug generation** — sends enriched quote data to Claude Haiku API with extracted keywords and named entities, generating memorable 2-3 word hyphenated slugs
6. **Collision handling** — avoids duplicate slugs using an in-memory set
7. **Output** — writes `db/seeds/quotes.sql` with LOTR themed slugs ready to seed!

### Regenerating the Slug Pool 

```bash
cd nlp-service
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python -m spacy download en_core_web_sm
cp .env.example .env  # add your ANTHROPIC_API_KEY
cd ..
make generate
cp nlp-service/data/quotes.sql db/seeds/quotes.sql
```

---

## Deployment

This project will be deployed to Railway sometime in the near future.

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `DATABASE_URL` | Postgres connection string | required |
| `REDIS_URL` | Redis connection string | required |
| `API_PORT` | Port to run the API on | `8080` |
| `API_KEY_SECRET` | Secret for API key signing | required |
| `BASE_URL` | Base URL for short links | required |
| `IP_RATE_LIMIT` | Requests per minute per IP | `60` |
| `KEY_RATE_LIMIT` | Requests per minute per API key | `120` |
| `DEFAULT_TTL_DAYS` | Default link expiry in days | `30` |

Generate a secure `API_KEY_SECRET`:
```bash
openssl rand -hex 32
```

---

## License

MIT
