# TEST Functions for slug_generator.py -- Eventually needs to be updated to be able to test

import re
from slug_generator import generate_slugs, sanitize_slug, generated_slugs

def test_slug_generator():

    # Sample quote data mimicking what preprocess.py outputs
    test_quotes = [
        {
            "quote": "You shall not pass!",
            "character": "GANDALF",
            "keywords": ["pass"],
            "entities": ["gandalf"],
            "source": "The Fellowship of the Ring",
            "famous": True
        },
        {
            "quote": "One does not simply walk into Mordor.",
            "character": "BOROMIR",
            "keywords": ["mordor", "simply", "walk"],
            "entities": ["boromir", "mordor"],
            "source": "The Fellowship of the Ring",
            "famous": True
        },
        {
            "quote": "Even the smallest person can change the course of the future.",
            "character": "GALADRIEL",
            "keywords": ["change", "course", "future", "person", "small"],
            "entities": ["galadriel"],
            "source": "The Fellowship of the Ring",
            "famous": True
        }
    ]

    print("--- Testing Slug Generator ---\n")

    # Test 1 — basic slug generation
    print("Test 1: Basic slug generation")
    for quote_data in test_quotes:
        slugs = generate_slugs(quote_data)
        print(f"  {quote_data['character']}: {slugs}")
    print()

    # Test 2 — sanitize_slug edge cases
    print("Test 2: Sanitize slug edge cases")
    edge_cases = [
        ("  Gandalf--Shadow! ", "gandalf-shadow"),
        ("frodo's burden", "frodos-burden"),
        ("one-ring-to-rule", "one-ring-to-rule"),
        ("MORDOR darkness", "mordor-darkness"),
        ("---leading-hyphens---", "leading-hyphens"),
    ]
    all_passed = True
    for input_slug, expected in edge_cases:
        result = sanitize_slug(input_slug)
        status = "✓" if result == expected else "✗"
        if result != expected:
            all_passed = False
        print(f"  {status} sanitize_slug('{input_slug}') → '{result}' (expected: '{expected}')")
    print(f"  {'All sanitize tests passed!' if all_passed else 'Some tests failed'}")
    print()

    # Test 3 — collision handling
    print("Test 3: Collision handling")
    generated_slugs.clear()  # Reset set for clean test
    
    collision_quote = {
        "quote": "My precious.",
        "character": "GOLLUM",
        "keywords": ["precious"],
        "entities": ["gollum"],
        "source": "The Two Towers",
        "famous": True
    }
    
    # Generate same quote twice to force collision
    slug1 = generate_slugs(collision_quote)
    generated_slugs.update(slug1)  # Manually add to force collision
    slug2 = generate_slugs(collision_quote)
    
    are_different = slug1 != slug2
    print(f"  {'✓' if are_different else '✗'} Collision handled: '{slug1}' vs '{slug2}'")
    print()

    # Test 4 — verify all slugs are URL safe
    print("Test 4: URL safety check")
    generated_slugs.clear()
    all_safe = True
    for quote_data in test_quotes:
        slugs = generate_slugs(quote_data)
        for slug in slugs:
            is_safe = bool(re.match(r'^[a-z0-9-]+$', slug))
            if not is_safe:
                all_safe = False
            print(f"  {'✓' if is_safe else '✗'} '{slug}' is {'safe' if is_safe else 'NOT safe'}")
    print(f"  {'All slugs are URL safe!' if all_safe else 'Some slugs are not URL safe!'}")

if __name__ == "__main__":
    test_slug_generator()
