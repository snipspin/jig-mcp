# web_search

Unified web search tool that works with any search API provider. Configure your preferred backend via environment variables - no code changes needed.

## Overview

The `web_search` tool provides a single, provider-agnostic interface for web searches. It adapts to different backends based on your configuration:

| Provider | Type | Auth | Cost |
|----------|------|------|------|
| **SearXNG** | Self-hosted metasearch | None | Free |
| **Ollama** | Cloud API | API Key | Paid |
| **Custom** | Any JSON search API | Optional | Varies |

## Configuration

Add these environment variables to your `.env` file:

### Required Settings

```bash
# Search endpoint URL
# SearXNG: http://your-instance/search
# Ollama: https://ollama.com/api/web_search
SEARCH_ENDPOINT=http://localhost:8080/search

# HTTP method
# SearXNG: GET
# Ollama: POST
SEARCH_METHOD=GET
```

### Optional Settings

```bash
# API key for authenticated endpoints (leave empty for no auth)
# Ollama: Get from https://ollama.com/settings/keys
SEARCH_API_KEY=your_api_key_here

# Allowed URL prefixes for SSRF protection (recommended for remote instances)
SEARCH_ALLOWED_PREFIXES=http://localhost:8080

# Response format hint (SearXNG: json)
SEARCH_FORMAT=json

# Content-Type header (defaults to application/json)
SEARCH_CONTENT_TYPE=application/json
```

## Usage

### Basic Search

```json
{
  "name": "web_search",
  "arguments": {
    "q": "your search query"
  }
}
```

### Advanced Options

```json
{
  "name": "web_search",
  "arguments": {
    "q": "machine learning tutorials",
    "max_results": 10,
    "categories": "general,tech",
    "language": "en",
    "time_range": "week",
    "page": 1,
    "safesearch": 1
  }
}
```

### Input Parameters

| Parameter | Type | Description | Provider Support |
|-----------|------|-------------|------------------|
| `q` | string | Search query | All |
| `query` | string | Alias for `q` | All |
| `max_results` | integer | Max results to return | Ollama |
| `categories` | string | Comma-separated categories | SearXNG |
| `language` | string | Language code (e.g., `en`, `de`) | SearXNG |
| `time_range` | string | Time filter: `day`, `week`, `month`, `year` | SearXNG |
| `page` | integer | Pagination page number | SearXNG |
| `safesearch` | integer | Level: 0=off, 1=moderate, 2=strict | SearXNG |

## Response Format

### SearXNG Response

```json
{
  "query": "search query",
  "number_of_results": 42,
  "results": [
    {
      "title": "Result Title",
      "url": "https://example.com",
      "content": "Snippet text...",
      "engine": "google",
      "score": 0.95,
      "category": "general"
    }
  ],
  "suggestions": ["related query 1", "related query 2"]
}
```

### Ollama Response

```json
{
  "results": [
    {
      "title": "Result Title",
      "url": "https://example.com",
      "content": "Full page content..."
    }
  ]
}
```

## Provider Setup

### SearXNG (Recommended for Self-Hosting)

1. Deploy SearXNG (Docker example):
```bash
docker run -d -p 8080:8080 searxng/searxng
```

2. Configure `.env`:
```bash
SEARCH_ENDPOINT=http://localhost:8080/search
SEARCH_METHOD=GET
SEARCH_ALLOWED_PREFIXES=http://localhost:8080
SEARCH_FORMAT=json
```

### Ollama Cloud API

1. Get API key from https://ollama.com/settings/keys

2. Configure `.env`:
```bash
SEARCH_ENDPOINT=https://ollama.com/api/web_search
SEARCH_METHOD=POST
SEARCH_API_KEY=your_api_key_here
SEARCH_ALLOWED_PREFIXES=https://ollama.com
```

### Custom Provider

Any search API that returns JSON can be used:

```bash
SEARCH_ENDPOINT=https://your-search-api.com/search
SEARCH_METHOD=POST
SEARCH_API_KEY=your_key
SEARCH_ALLOWED_PREFIXES=https://your-search-api.com
SEARCH_CONTENT_TYPE=application/json
```

## Examples

### Example 1: Simple Search

**Request:**
```json
{"name": "web_search", "arguments": {"q": "latest Go release"}}
```

**Response (SearXNG):**
```json
{
  "query": "latest Go release",
  "number_of_results": 150,
  "results": [
    {
      "title": "Go 1.22 Release Notes",
      "url": "https://go.dev/doc/devel/release/1.22",
      "content": "Go 1.22 was released on February 6, 2024..."
    }
  ]
}
```

### Example 2: Time-Filtered News Search

**Request:**
```json
{
  "name": "web_search",
  "arguments": {
    "q": "AI regulations",
    "categories": "news",
    "time_range": "week",
    "language": "en"
  }
}
```

### Example 3: Switching Providers

To switch from SearXNG to Ollama, just change `.env`:

```bash
# Before (SearXNG)
SEARCH_ENDPOINT=http://localhost:8080/search
SEARCH_METHOD=GET

# After (Ollama)
SEARCH_ENDPOINT=https://ollama.com/api/web_search
SEARCH_METHOD=POST
SEARCH_API_KEY=sk-xxx
```

Restart jig-mcp and the same tool calls now use Ollama.

## Troubleshooting

### "URL does not match any allowed prefix"

**Cause:** SSRF protection is blocking the request.

**Fix:** Add your endpoint to `SEARCH_ALLOWED_PREFIXES`:
```bash
SEARCH_ALLOWED_PREFIXES=http://your-instance:8080
```

### Empty Results

**Cause:** Provider returned no matches or wrong format.

**Fix:** 
1. Check `SEARCH_FORMAT=json` for SearXNG
2. Verify endpoint URL is correct
3. Test directly: `curl "http://your-instance/search?q=test&format=json"`

### Authentication Errors (Ollama)

**Cause:** Invalid or missing API key.

**Fix:**
1. Verify key at https://ollama.com/account/api-keys
2. Ensure `SEARCH_API_KEY` is set in `.env`
3. Check for typos (no extra spaces)

### Connection Refused

**Cause:** Endpoint unreachable.

**Fix:**
1. Verify the service is running
2. Check firewall rules
3. For Docker: ensure network access

## Security Notes

1. **SSRF Protection:** Always set `SEARCH_ALLOWED_PREFIXES` when running on a network with internal services.

2. **API Key Security:** Never commit `.env` files with API keys. The template in `example.env` uses placeholders.

3. **Rate Limiting:** Both SearXNG and Ollama may rate-limit requests. Consider adding delays between searches in automated workflows.

## Files

```
tools/web_search/
├── manifest.yaml    # Tool configuration
├── README.md        # This documentation
└── scripts/         # (Optional) Custom wrapper scripts
```

## See Also

- [Tool Developer Guide](../../docs/TOOL_DEVELOPER_GUIDE.md) - Complete guide for building tools
- [Configuration Reference](../../README.md#configuration-reference) - Environment variables
- [SearXNG Documentation](https://docs.searxng.org/) - Self-hosted search engine
- [Ollama API Docs](https://docs.ollama.com/capabilities/web-search#web-search) - Cloud search API
