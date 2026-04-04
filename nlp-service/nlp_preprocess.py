import pandas as pd
import re
import spacy

from rapidfuzz import fuzz
from data.famous_quotes import FAMOUS_QUOTES

    
# load the spacy English model
nlp = spacy.load("en_core_web_sm")

# Helper functions for cleaning and extracting quotes

def clean_raw_text(text):

    if pd.isna(text): 
        return ""

    text = str(text).lower()  # Convert to lowercase
    text = text.replace('\xa0', ' ') # Remove non-breaking space characters
    text = re.sub(r'[^ -~]+', '', text) # Remove any other non-ASCII characters
    text = re.sub(r'[\W_]+', ' ', text) # Remove punctuation and replace with space, keep alphanumeric
    text = re.sub(r'\s+', ' ', text).strip() # Normalize whitespace
    return text

def extract_keywords_spacy(text):

    doc = nlp(text)

    # Extract lemmatized words that are not stop words and are nouns, proper nouns, adjectives, or verbs
    keywords = [token.lemma_ for token in doc if not token.is_stop and token.is_alpha and token.pos_ in ("NOUN", "PROPN", "ADJ", "VERB")]

    return sorted(list(set(keywords)))

# Define a list of common character names for entity linking heuristic
character_names = ['frodo', 'sam', 'gandalf', 'aragorn', 'legolas', 'gimli', 'merry', 'pippin', 'smeagol', 'gollum', 'bilbo', 'elrond', 'galadriel', 'boromir', 'eowyn', 'theoden', 'deagol']

def extract_entities_from_keywords(keywords, character):
    lotr_places = ['mordor', 'shire', 'rivendell', 'gondor', 'rohan', 
                   'isengard', 'moria', 'lothlorien', 'minas', 'tirith']
    lotr_artifacts = ['ring', 'sting', 'palantir', 'lembas', 'mithril']
    
    entities = []
    
    # Check keywords against known LOTR terms
    for keyword in keywords:
        if keyword.lower() in lotr_places:
            entities.append(keyword)
        if keyword.lower() in lotr_artifacts:
            entities.append(keyword)
        if keyword.lower() in [c.lower() for c in character_names]:
            entities.append(keyword)
    
    # Always include the speaker as an entity
    if character:
        entities.append(character.lower())
    
    return sorted(list(set(entities)))

def score_quote(cleaned_quote, keywords, entities):
    score = 0
    
    word_count = len(cleaned_quote.split())
    
    # Reward meaningful length — sweet spot is 8-20 words
    if 8 <= word_count <= 20:
        score += 3
    elif 4 <= word_count < 8:
        score += 1
        
    # Reward keyword richness
    score += min(len(keywords), 4)  # Cap at 4 so long quotes don't dominate
    
    # Reward named entities — places and artifacts make better slugs
    score += min(len(entities), 3)
    
    return score

def is_famous(cleaned_quote, threshold=85):

    for famous in FAMOUS_QUOTES:

        # Case 1 — full quote similarity (handles minor wording differences)
        full_similarity = fuzz.ratio(cleaned_quote, famous)
        if full_similarity >= threshold:
            return True
            
        # Case 2 — famous quote is a snippet of a larger CSV quote
        partial_similarity = fuzz.partial_ratio(famous, cleaned_quote)
        if partial_similarity >= threshold:
            return True
            
        # Case 3 — CSV quote is a snippet of your famous quote
        partial_similarity_inverse = fuzz.partial_ratio(cleaned_quote, famous)
        if partial_similarity_inverse >= threshold:
            return True
            
    return False

def generate_processed_quotes(file_path = './data/lotr_scripts.csv'):

    # load the csv file
    df = pd.read_csv(file_path)

    processed_quotes = []

    for _, row in df.iterrows():

        raw_quote_text = row['dialog']
        character_name = row['char']
        movie_source = row['movie']
        
        cleaned_quote = clean_raw_text(raw_quote_text)
        famous = is_famous(cleaned_quote) if len(cleaned_quote.split()) >= 3 else False 

        if not famous:
            if len(cleaned_quote.split()) < 4:
                continue

            keywords = extract_keywords_spacy(cleaned_quote)

            if len(keywords) < 2:
                continue


            if character_name.lower() not in [c.lower() for c in character_names]:
                continue
        else:
            keywords = extract_keywords_spacy(cleaned_quote)
        
        entities = extract_entities_from_keywords(keywords, character_name)
        score = score_quote(cleaned_quote, keywords, entities)

        structured_quote = {
            "quote": raw_quote_text, # Original raw quote
            "character": character_name,
            "keywords": keywords,
            "entities": entities,
            "source": movie_source,
            "score": score,
            "famous": famous
        }

        processed_quotes.append(structured_quote)

    famous_quotes = [q for q in processed_quotes if q['famous']]
    regular_quotes = [q for q in processed_quotes if not q['famous']]

    regular_quotes.sort(key=lambda x: x['score'], reverse=True)
    top_regular = regular_quotes[:300]

    final_quotes = famous_quotes + top_regular

    return final_quotes

# TEST 

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


if __name__ == "__main__":
    test_generate_processed_quotes()
