from nlp_preprocess import generate_processed_quotes
from slug_generator import generate_slugs

def format_sql_value(text: str) -> str:
    # Escape single quotes for SQL
    return text.replace("'", "''")

def generate_slugs_pipeline():

    print("Step 1: preprocessing quotes.")
    quotes = generate_processed_quotes()
    print(f"{len(quotes)} quotes ready for slug generation\n")

    print("Step 2: Generating slugs via Claude API call.")
    results = []

    for i, quote_data in enumerate(quotes):

        print(f"Creating slugs for quote {i+1}/{len(quotes)}: [{quote_data['character']}] - {quote_data['quote'][:20]} ... \n")

        slugs = generate_slugs(quote_data)

        for slug in slugs:
            results.append({
                "quote": quote_data['quote'],
                "character": quote_data['character'],
                "source": quote_data['source'],
                "slug": slug,
                "famous": quote_data['famous']
            })

    
    print(f"Created {len(results)} total slugs\n")

    print("Step 3: writing to slugs.sql file.")
    write_sql(results)

    print(f"Done! Create {len(results)} slugs and wrote them to slugs.sql.")
    return

def write_sql(results: list):
    lines = ["INSERT INTO quotes (quote, character, source, slug) VALUES"]
    
    values = []
    for r in results:
        quote = format_sql_value(r['quote'])
        character = format_sql_value(r['character'])
        source = format_sql_value(r['source'])
        slug = r['slug']
        values.append(f"  ('{quote}', '{character}', '{source}', '{slug}')")
    
    lines.append(',\n'.join(values) + ';')
    
    with open('data/quotes.sql', 'w') as f:
        f.write('\n'.join(lines))

    return

if __name__ == "__main__":
    generate_slugs_pipeline()
