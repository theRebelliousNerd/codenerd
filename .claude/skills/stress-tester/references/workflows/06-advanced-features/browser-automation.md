# Browser Automation Stress Test

Stress test for rod-based browser control.

## Overview

Tests browser automation with:

- 50+ concurrent page fetches
- DOM projection at scale
- CDP event handling
- Session management

**Expected Duration:** 25-40 minutes

## Quick Reference

### Test Commands

```bash
# Multiple page fetches
./nerd.exe browse "fetch documentation from 10 different URLs"
./nerd.exe browse "scrape all links from a large website"

# Concurrent operations
./nerd.exe browse "open 20 tabs and extract content"

# Check browser state
./nerd.exe query "browser_session"
./nerd.exe query "page_fetch"
```

### Expected Behavior

- All pages should load
- DOM should be projected correctly
- Sessions should be managed
- Memory should stay bounded

---

## Severity Levels

### Conservative
- 10 page fetches, sequential

### Aggressive
- 30 page fetches, concurrent

### Chaos
- 50+ fetches, malformed URLs mixed in

### Hybrid
- Browser + code analysis combined

