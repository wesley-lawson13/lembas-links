import re
import pytest
from unittest.mock import patch
from slug_generator import sanitize_slug, generate_slugs, generated_slugs


@pytest.fixture(autouse=True)
def reset_generated_slugs():
    """Clear the global slug set before and after each test for isolation."""
    generated_slugs.clear()
    yield
    generated_slugs.clear()


# --- sanitize_slug ---

def test_sanitize_slug_trims_and_lowercases():
    assert sanitize_slug("  Gandalf--Shadow! ") == "gandalf-shadow"

def test_sanitize_slug_removes_special_chars():
    assert sanitize_slug("frodo's burden") == "frodos-burden"

def test_sanitize_slug_preserves_valid_hyphens():
    assert sanitize_slug("one-ring-to-rule") == "one-ring-to-rule"

def test_sanitize_slug_lowercases_and_joins_spaces():
    assert sanitize_slug("MORDOR darkness") == "mordor-darkness"

def test_sanitize_slug_strips_leading_and_trailing_hyphens():
    assert sanitize_slug("---leading-hyphens---") == "leading-hyphens"


# --- generate_slugs ---

_SAMPLE_QUOTE = {
    "quote": "My precious.",
    "character": "GOLLUM",
    "keywords": ["precious"],
    "entities": ["gollum"],
    "source": "The Two Towers",
    "famous": True,
}


def test_generate_slugs_returns_nonempty_list():
    with patch("slug_generator.generate_slug_with_claude", return_value=["my-precious"]):
        slugs = generate_slugs(_SAMPLE_QUOTE)
    assert len(slugs) > 0


def test_generate_slugs_are_url_safe():
    with patch("slug_generator.generate_slug_with_claude", return_value=["my-precious"]):
        slugs = generate_slugs(_SAMPLE_QUOTE)
    for slug in slugs:
        assert re.match(r"^[a-z0-9-]+$", slug), f"'{slug}' is not URL-safe"


def test_generate_slugs_collision_produces_unique_slug():
    with patch("slug_generator.generate_slug_with_claude", return_value=["my-precious"]):
        slug1 = generate_slugs(_SAMPLE_QUOTE)
        # Second call sees slug1 already in generated_slugs, so must return something different
        slug2 = generate_slugs(_SAMPLE_QUOTE)
    assert slug1 != slug2, f"Expected different slugs on collision, got {slug1!r} both times"
