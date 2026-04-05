import os
import re
from anthropic import Anthropic
from dotenv import load_dotenv

load_dotenv()

client = Anthropic(api_key = os.getenv("ANTHROPIC_API_KEY"))
generated_slugs = set()

def sanitize_slug(slug: str) -> str:

    slug = slug.lower()
    slug = slug.strip()
    slug = slug.replace(' ', '-')
    slug = re.sub(r'[^a-z0-9-]', '', slug)
    slug = re.sub(r'-+', '-', slug)
    
    slug = slug.strip('-')
    
    return slug

def generate_slug_with_claude(quote_data: dict, num_slugs: int = 1) -> list[str]:

    prompt = f"""You are generating URL slugs for a Lord of the Rings themed URL shortener called Lembas Links.

    Given this quote:
    Character: {quote_data['character'].title()}
    Quote: {quote_data['quote']}
    Key Terms Extracted: {', '.join(quote_data['keywords'])}
    Named Entities: {', '.join(quote_data['entities'])}

    Generate {num_slugs} memorable 2-3 word hyphenated URL slug(s) that capture different aspects of this quote.

    Rules:
    - Lowercase only
    - Hyphens between words
    - No special characters or numbers
    - Must be URL safe
    - Each slug should capture a DIFFERENT aspect of the quote
    - Prefer using the character name or a key entity as one of the words
    - Return only the slug(s), one per line, no explanation, no numbering
    """

    response = client.messages.create(
        model="claude-haiku-4-5-20251001",
        max_tokens=60,
        messages=[{"role": "user", "content": prompt}] 
    )

    slugs = [line.strip() for line in response.content[0].text.strip().split('\n') if line.strip()]
    return slugs[:num_slugs]

def handle_collision(slug: str, quote_data: dict) -> str:

    if slug not in generated_slugs:
        return slug
    
    # Try prepending the character name if not already there
    character = quote_data['character'].split()[0].lower()
    character_slug = f"{character}-{slug}"
    if character_slug not in generated_slugs:
        return character_slug
    
    # Try appending a keyword that isn't already in the slug
    for keyword in quote_data['keywords']:
        if keyword not in slug:
            keyword_slug = f"{slug}-{keyword}"
            if keyword_slug not in generated_slugs:
                return keyword_slug
    
    # Otherwise, append incrementing number
    counter = 1
    while True:
        numbered_slug = f"{slug}-{counter}"
        if numbered_slug not in generated_slugs:
            return numbered_slug
        counter += 1


def generate_slugs(quote_data: dict) -> list[str]:
    
    word_count = len(quote_data['quote'].split())

    if quote_data['famous'] and word_count >= 20:
        num_slugs = 3
    elif quote_data['famous'] and word_count >= 10:
        num_slugs = 2
    else:
        num_slugs = 1
    
    raw_slugs = generate_slug_with_claude(quote_data, num_slugs)
    
    final_slugs = []
    for raw_slug in raw_slugs:
        sanitized = sanitize_slug(raw_slug)
        resolved = handle_collision(sanitized, quote_data)
        generated_slugs.add(resolved)
        final_slugs.append(resolved)
    
    return final_slugs

