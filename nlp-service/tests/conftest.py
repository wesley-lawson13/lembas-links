import os

# Set a dummy API key so slug_generator.py's module-level Anthropic client
# initialisation doesn't raise AuthenticationError when tests are collected.
# The actual API is never called in tests — generate_slug_with_claude is mocked.
os.environ.setdefault("ANTHROPIC_API_KEY", "test-key-for-testing")
