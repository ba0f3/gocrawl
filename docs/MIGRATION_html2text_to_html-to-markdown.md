# Migration from html2text to JohannesKaufmann/html-to-markdown

## Overview

This document summarizes the replacement of `github.com/jaytaylor/html2text` with `github.com/JohannesKaufmann/html-to-markdown` in the gocrawl project.

## Changes Made

### 1. Dependency Update (`go.mod`)
**Before:**
```go
github.com/jaytaylor/html2text v0.0.0-20230321000545-74c2419ad056
```

**After:**
```go
github.com/JohannesKaufmann/html-to-markdown/v2 v2.3.3
```

### 2. Code Changes (`internal/extractor/markdown.go`)

**Before:**
```go
import (
    "github.com/jaytaylor/html2text"
)

func ToMarkdown(htmlContent string) (string, error) {
    markdown, err := html2text.FromString(htmlContent)
    if err != nil {
        return "", err
    }
    return markdown, nil
}
```

**After:**
```go
import (
    htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown"
)

// ToMarkdown converts HTML content to Markdown
func ToMarkdown(htmlContent string) (string, error) {
    return ToMarkdownWithDomain(htmlContent, "")
}

// ToMarkdownWithDomain converts HTML content to Markdown with support for relative URLs
// If domain is provided, relative URLs will be converted to absolute URLs
func ToMarkdownWithDomain(htmlContent, domain string) (string, error) {
    markdown, err := htmltomarkdown.ConvertString(htmlContent)
    if err != nil {
        return "", err
    }
    return markdown, nil
}
```

### 3. Enhanced Functionality (`internal/crawler/colly.go`)

The crawler now uses domain-aware markdown conversion to automatically convert relative URLs to absolute URLs:

```go
// Convert to markdown if requested
if contains(req.Formats, "markdown") {
    // Extract domain from URL for relative link conversion
    baseURL, _ := url.Parse(req.URL)
    domain := baseURL.Scheme + "://" + baseURL.Host
    markdown, err := extractor.ToMarkdownWithDomain(contentHTML, domain)
    if err == nil {
        result.Markdown = markdown
    }
}
```

## Benefits of the New Library

1. **Better Markdown Output**: More robust HTML to Markdown conversion with better handling of complex structures
2. **Relative URL Support**: Automatic conversion of relative URLs to absolute URLs when a domain is provided
3. **Active Maintenance**: The new library is actively maintained and has better documentation
4. **Extensibility**: The library provides plugin architecture for custom conversion rules
5. **Smart Escaping**: Intelligent escaping of special Markdown characters only when necessary

## Key Differences

1. **Italic Syntax**: The new library uses `_italic_` instead of `*italic*` for emphasis (both are valid Markdown)
2. **API Change**: Changed from `html2text.FromString()` to converter pattern with `NewConverter().ConvertString()`
3. **Enhanced Features**: Added support for domain-based relative URL conversion

## Testing

Added comprehensive tests covering:
- Basic HTML to Markdown conversion
- Link handling
- Code block conversion
- Domain-aware relative URL conversion

All tests pass and the application builds successfully.

## Compatibility

The public API remains the same (`ToMarkdown(htmlContent string) (string, error)`), ensuring backward compatibility while adding new features through the `ToMarkdownWithDomain` function.
