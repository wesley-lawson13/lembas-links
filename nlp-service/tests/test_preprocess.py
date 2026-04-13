import pandas as pd
from rapidfuzz import fuzz

from nlp_preprocess import generate_processed_quotes, clean_raw_text, character_names
from data.famous_quotes import FAMOUS_QUOTES

def test_generate_processed_quotes():
    results = generate_processed_quotes()
    
    # Test 1 — did we get any results back?
    assert len(results) > 0, "No quotes were generated"
    print(f"✓ Generated {len(results)} quotes total")
    
    # Test 2 — check famous quotes made it through
    famous_count = sum(1 for q in results if q['famous'])
    print(f"✓ Famous quotes in pool: {famous_count}")
    
    # Test 3 — check no quote is too short
    short_quotes = [q for q in results if not q['famous'] and len(q['quote'].split()) < 4]
    assert len(short_quotes) == 0, f"Found {len(short_quotes)} quotes that are too short"
    print(f"✓ No short quotes slipped through filters")
    
    # Test 4 — check all quotes have required fields
    required_fields = ['quote', 'character', 'keywords', 'entities', 'source', 'score', 'famous']
    for q in results:
        for field in required_fields:
            assert field in q, f"Quote missing field: {field}"
    print(f"✓ All quotes have required fields")
    
    # Test 5 — check all characters are in known list
    unknown_chars = [q['character'] for q in results if not q['famous'] and q['character'].lower() not in [c.lower() for c in character_names]]
    assert len(unknown_chars) == 0, f"Unknown characters slipped through: {set(unknown_chars)}"
    print(f"✓ All characters are in known list")
    
    # Test 6 — check pool size is reasonable
    assert len(results) <= 400, f"Pool too large: {len(results)} quotes"
    print(f"✓ Pool size is within expected range")

    # Print a sample of 5 quotes to eyeball
    print("\n--- Sample Quotes ---")
    for q in results[:5]:
        label = 'FAMOUS' if q['famous'] else f"score:{q['score']}"
        print(f"[{label}] {q['character']}: {q['quote']}")
        print(f"  keywords: {q['keywords'][:5]}")
        print(f"  entities: {q['entities']}")
        print()

def debug_famous_quotes(file_path='./data/lotr_scripts.csv'):
    df = pd.read_csv(file_path)
    
    for famous in FAMOUS_QUOTES:
        best_score = 0
        best_match = None
        
        for _, row in df.iterrows():
            cleaned = clean_raw_text(str(row['dialog']))
            
            score1 = fuzz.ratio(cleaned, famous)
            score2 = fuzz.partial_ratio(famous, cleaned)
            score3 = fuzz.partial_ratio(cleaned, famous)
            best = max(score1, score2, score3)
            
            if best > best_score:
                best_score = best
                best_match = cleaned
        
        status = "✓ MATCHED" if best_score >= 85 else "✗ MISSED"
        print(f"{status} (score:{best_score}) | Famous: '{famous}'")
        if best_score < 85:
            print(f"  Best CSV match: '{best_match if best_match else None}'")
        print()

    return

if __name__ == "__main__":
    test_generate_processed_quotes()
    debug_famous_quotes()
