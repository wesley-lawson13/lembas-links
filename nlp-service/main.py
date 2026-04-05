from fastapi import FastAPI

app = FastAPI()

@app.get("/health")
def health():
    return {"status": "ok", "service": "nlp-service", "note": "Preprocessing tool for slug generation - run generate_slugs.py to regenerate slug pool."}
