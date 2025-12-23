# Rod Builder Assets

Test HTML pages for practicing Rod browser automation.

## Available Test Pages

### 1. test-form.html

Complete form with various input types for testing form automation.

**Features:**
- Text inputs (username, email, password)
- Select dropdown (country)
- Checkboxes (interests)
- Radio buttons (subscription type)
- Textarea (bio)
- Client-side validation
- Success/error messages
- Data attributes (`data-testid`) for stable selectors

**Practice Scenarios:**
```go
// Fill form
page.MustElement("#username").MustInput("testuser")
page.MustElement("#email").MustInput("test@example.com")
page.MustElement("#password").MustInput("password123")
page.MustElement("#country").MustSelect("us")

// Check boxes
page.MustElement("[data-testid='interest-coding']").MustClick()
page.MustElement("[data-testid='interest-testing']").MustClick()

// Submit
page.MustElement("[data-testid='submit-button']").MustClick()

// Verify success
page.MustElement("[data-testid='success-message']").MustWaitVisible()
```

### 2. test-table.html

Interactive data table with sorting, filtering, and pagination.

**Features:**
- Searchable table
- Sortable columns
- Pagination (5 items per page)
- Row actions (edit, delete)
- Status badges
- 10 sample user records

**Practice Scenarios:**
```go
// Search
page.MustElement("[data-testid='search-input']").MustInput("John")
page.MustElement("[data-testid='search-button']").MustClick()

// Extract table data
rows := page.MustElements("#table-body tr")
for _, row := range rows {
    cells := row.MustElements("td")
    id := cells[0].MustText()
    name := cells[1].MustText()
    email := cells[2].MustText()
    // ...
}

// Pagination
page.MustElement("[data-testid='next-page']").MustClick()

// Sort by name
page.MustElement("[data-testid='header-name']").MustClick()

// Edit action
page.MustElement("[data-testid='edit-1']").MustClick()
```

### 3. test-react-app.html

Simple React application for testing React component interaction.

**Features:**
- Counter component with state
- Todo list (add, toggle, delete)
- User profile with async loading
- React Fiber tree for extraction
- Data attributes for testing

**Practice Scenarios:**
```go
// Wait for React to load
page.MustWaitLoad()

// Counter interaction
page.MustElement("[data-testid='increment-button']").MustClick()
value := page.MustElement("[data-testid='counter-value']").MustText()

// Add todo
page.MustElement("[data-testid='todo-input']").MustInput("New task")
page.MustElement("[data-testid='add-todo-button']").MustClick()

// Toggle todo
page.MustElement("[data-testid='todo-checkbox-1']").MustClick()

// Extract React Fiber tree
result := page.MustEval(`
    (function() {
        const root = document.querySelector('[data-testid="app"]');
        const fiberKey = Object.keys(root).find(k => k.startsWith('__reactFiber'));
        // ... fiber extraction logic
    })()
`)
```

## Usage

### Local File Access

```go
import "path/filepath"

// Get absolute path
assetsDir := filepath.Join("path", "to", "rod-builder", "assets")
formPath := filepath.Join(assetsDir, "test-form.html")

// Navigate
page.MustNavigate("file://" + formPath)
```

### HTTP Server (Recommended)

```bash
# Serve assets directory
cd assets
python -m http.server 8080

# Or with Go
go run -m http.server 8080
```

```go
// Navigate to served page
page.MustNavigate("http://localhost:8080/test-form.html")
```

## Best Practices

1. **Use data-testid attributes** - More stable than CSS classes
2. **Wait for elements** - Use `MustWaitVisible()` or `MustWaitLoad()`
3. **Verify state changes** - Check for success messages, updated text
4. **Test error cases** - Submit invalid data, test validation
5. **Extract all data** - Practice comprehensive scraping

## Testing Checklist

### Form Testing
- [ ] Fill all input types
- [ ] Test validation (invalid data)
- [ ] Verify error messages
- [ ] Submit successfully
- [ ] Reset form
- [ ] Handle file uploads (if applicable)

### Table Testing
- [ ] Extract all rows
- [ ] Search/filter data
- [ ] Navigate pagination
- [ ] Sort columns
- [ ] Click row actions
- [ ] Handle empty states

### React Testing
- [ ] Wait for component mount
- [ ] Interact with state (counter)
- [ ] Manipulate list (add/delete todos)
- [ ] Extract React Fiber tree
- [ ] Verify async loading
- [ ] Test re-renders

## Customization

Feel free to modify these HTML files for your specific testing needs:

- Add more form fields
- Increase table size
- Add more React components
- Create custom test scenarios
- Add AJAX requests
- Implement authentication flows

## Integration with Scripts

Use these assets with the provided scripts:

```bash
# Scrape table data
go run scripts/scraper_template.go \
    --url "file://$(pwd)/assets/test-table.html" \
    --selector "[data-testid^='row-']"

# Test form submission
# (modify scraper_template.go to fill form)
```

## Advanced Scenarios

### Combining Multiple Pages

```go
// Navigate through workflow
page.MustNavigate("file://" + formPath)
// Fill form...
page.MustElement("button[type='submit']").MustClick()

// Navigate to table
page.MustNavigate("file://" + tablePath)
// Verify data...
```

### Screenshot Comparison

```go
// Take baseline
page.MustNavigate("file://" + reactPath)
baseline, _ := page.Screenshot(true, nil)
os.WriteFile("baseline.png", baseline, 0644)

// Make changes
page.MustElement("[data-testid='increment-button']").MustClick()

// Compare
current, _ := page.Screenshot(true, nil)
diff := compareImages(baseline, current)
```

These test assets provide a complete environment for learning and testing Rod browser automation patterns.
