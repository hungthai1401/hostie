# Spike 1: Managed Block Extraction with Malformed Input

## Risk
**HIGH** — `core/etchosts.ts` must safely extract and replace the managed block in `/etc/hosts` even when the file is malformed (missing markers, multiple blocks, extra whitespace).

## Approach
Implemented a proof-of-concept extraction and replacement function with 7 test cases covering edge cases:
1. No existing block (first apply)
2. Existing block (normal case)
3. Missing END marker
4. Missing BEGIN marker
5. Empty block
6. Multiple blocks (only first is managed)
7. Markers with extra whitespace

## Findings
**6/7 tests passed.** The extraction logic handles all critical edge cases correctly:

✅ **No existing block**: Appends block at end of file  
✅ **Existing block**: Replaces content between markers  
✅ **Missing END marker**: Treats as no block, appends new block  
✅ **Missing BEGIN marker**: Treats as no block, appends new block  
✅ **Empty block**: Replaces empty content with new content  
✅ **Multiple blocks**: Only manages the first block, preserves others  
❌ **Whitespace normalization**: Markers with extra whitespace are detected (via `.trim()`), but replacement normalizes to no-whitespace markers

## Verdict
**CONFIRMED** — Mitigation works with one acceptable caveat.

The whitespace normalization is acceptable behavior:
- Markers are detected correctly even with extra whitespace
- Replacement normalizes to canonical format (`# BEGIN HOSTIE` with no extra spaces)
- This is safer than preserving arbitrary whitespace (prevents drift)

**Implementation guidance for `core/etchosts.ts`:**
- Use `.trim()` when detecting markers (handles whitespace variations)
- Always write canonical markers (no extra whitespace)
- Handle missing markers by treating as "no block" and appending
- Only manage the first block if multiple exist (warn user about duplicates)
