# CSS and XPath Selector Patterns

Comprehensive guide to element selection in Rod browser automation.

## CSS Selectors

### Basic Selectors

```css
/* Element type */
button
div
input

/* ID */
#submit-btn
#user-profile

/* Class */
.btn
.btn-primary
.card-header

/* Attribute */
[type="text"]
[data-id="123"]
[href^="https://"]  /* starts with */
[href$=".pdf"]      /* ends with */
[href*="api"]       /* contains */
```

### Combinators

```css
/* Descendant (any depth) */
div.container button
.parent .child

/* Direct child */
div.container > button
ul > li

/* Adjacent sibling */
h2 + p

/* General sibling */
h2 ~ p
```

### Pseudo-classes

```css
/* State */
:hover
:focus
:active
:disabled
:enabled
:checked

/* Position */
:first-child
:last-child
:nth-child(2)
:nth-child(even)
:nth-child(odd)
:nth-of-type(3)

/* Content */
:empty
:not(.disabled)
```

### Pseudo-elements

```css
/* Note: Pseudo-elements don't work directly with Rod */
/* Use JavaScript evaluation instead */

/* Get ::before content */
page.MustEval(`
    window.getComputedStyle(document.querySelector('.element'), '::before').content
`)
```

### Complex Selectors

```go
// Multiple classes
page.MustElement(".btn.btn-primary.active")

// Attribute with value
page.MustElement("input[type='email'][required]")

// Nested with pseudo-class
page.MustElement("ul.menu > li:first-child a")

// Not selector
page.MustElement("button:not(.disabled)")

// Multiple selectors (OR)
page.MustElement("button, input[type='submit']")
```

## XPath Selectors

### Basic Paths

```xpath
<!-- Absolute path -->
/html/body/div/button

<!-- Relative path (recommended) -->
//button
//div[@id='container']
//input[@type='text']
```

### Predicates

```xpath
<!-- Attribute equals -->
//button[@id='submit']
//div[@class='card']

<!-- Attribute contains -->
//div[contains(@class, 'btn')]
//a[contains(@href, '/api/')]

<!-- Attribute starts with -->
//input[starts-with(@name, 'user')]

<!-- Text equals -->
//button[text()='Submit']
//span[text()='Success']

<!-- Text contains -->
//div[contains(text(), 'Error')]

<!-- Position -->
//li[1]          <!-- First li -->
//li[last()]     <!-- Last li -->
//li[position()>2] <!-- After 2nd li -->
```

### Axes

```xpath
<!-- Parent -->
//button[@id='submit']/parent::div

<!-- Ancestor -->
//span[@class='error']/ancestor::form

<!-- Child -->
//div[@id='container']/child::button

<!-- Descendant -->
//div[@id='container']//button

<!-- Following sibling -->
//h2/following-sibling::p

<!-- Preceding sibling -->
//p/preceding-sibling::h2
```

### Complex XPath

```go
// Multiple conditions (AND)
page.MustElementX("//button[@type='submit' and @class='btn-primary']")

// Multiple conditions (OR)
page.MustElementX("//button[@type='submit' or @type='button']")

// Not condition
page.MustElementX("//button[not(@disabled)]")

// Position and attribute
page.MustElementX("//div[@class='card'][position()<=3]")

// Parent-child relationship
page.MustElementX("//div[@class='form-group']//input[@type='text']")

// Text match
page.MustElementX("//button[contains(text(), 'Submit') and @type='submit']")
```

## CSS vs XPath

### When to Use CSS

**Advantages:**
- Simpler, more readable syntax
- Faster performance (native browser support)
- Better for class and ID selection
- More familiar to web developers

**Best For:**
- Simple selections: `#id`, `.class`, `element`
- Attribute matches: `[attr="value"]`
- Descendant selection: `parent child`
- Pseudo-classes: `:first-child`, `:not()`

### When to Use XPath

**Advantages:**
- Navigate up to parents
- More powerful text matching
- More flexible predicates
- Better for complex conditions

**Best For:**
- Parent/ancestor selection: `//child/parent::`
- Text content matching: `[text()='value']`
- Complex conditions: multiple AND/OR
- Position-based selection with conditions

## Practical Selection Patterns

### Finding by Visible Text

```go
// CSS (requires exact match + JavaScript)
element := page.MustElementByJS(`
    document.evaluate("//button[text()='Submit']", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue
`)

// XPath (native support)
element := page.MustElementX("//button[text()='Submit']")

// XPath with partial text match
element := page.MustElementX("//button[contains(text(), 'Submit')]")
```

### Dynamic Attributes

```go
// data-testid (common in React)
page.MustElement("[data-testid='submit-button']")

// Custom attributes
page.MustElement("[data-action='delete']")
page.MustElement("[aria-label='Close']")
```

### Tables

```go
// All rows
rows := page.MustElements("table tbody tr")

// Specific cell (row 2, column 3)
cell := page.MustElementX("//table/tbody/tr[2]/td[3]")

// Row with specific text in first column
row := page.MustElementX("//table/tbody/tr[td[1][contains(text(), 'John')]]")

// All cells in a column
cells := page.MustElementsX("//table/tbody/tr/td[2]")
```

### Forms

```go
// Input by name
page.MustElement("input[name='username']")

// Input by label text
page.MustElementX("//label[text()='Email']/following-sibling::input")

// Input in specific form
page.MustElement("form#login input[name='password']")

// Selected option in dropdown
page.MustElement("select[name='country'] option:checked")

// All checked checkboxes
checked := page.MustElements("input[type='checkbox']:checked")
```

### Lists

```go
// First item
page.MustElement("ul.menu > li:first-child")

// Last item
page.MustElementX("//ul[@class='menu']/li[last()]")

// Specific item by text
page.MustElementX("//ul[@class='menu']/li[text()='Settings']")

// All items except first
items := page.MustElements("ul.menu > li:not(:first-child)")
```

### Dynamic Content

```go
// Element that appears after action
page.MustElement("button#load-more").MustClick()
page.MustWaitElementsMoreThan(".item", 10) // Wait for items

// Element with dynamic class
page.MustElement(".alert.alert-success") // Success message
page.MustElement(".alert.alert-error")   // Error message

// Element that becomes visible
element := page.MustElement(".modal")
element.MustWaitVisible()
```

## Element Not Found Strategies

### Wait and Retry

```go
func FindElementWithRetry(page *rod.Page, selector string, maxAttempts int) (*rod.Element, error) {
    for i := 0; i < maxAttempts; i++ {
        element, err := page.Timeout(2 * time.Second).Element(selector)
        if err == nil {
            return element, nil
        }

        if i < maxAttempts-1 {
            time.Sleep(time.Second)
        }
    }

    return nil, fmt.Errorf("element not found after %d attempts: %s", maxAttempts, selector)
}
```

### Fallback Selectors

```go
func FindElementWithFallback(page *rod.Page, selectors []string) (*rod.Element, error) {
    for _, selector := range selectors {
        element, err := page.Element(selector)
        if err == nil {
            return element, nil
        }
    }

    return nil, errors.New("element not found with any selector")
}

// Usage
element, err := FindElementWithFallback(page, []string{
    "#submit-btn",
    ".btn-primary[type='submit']",
    "//button[@type='submit']",
})
```

### Dynamic Wait

```go
// Wait for element to exist
page.MustWaitElementsMoreThan(selector, 0)

// Wait for minimum count
page.MustWaitElementsMoreThan(".product-card", 5)

// Wait for element to disappear
page.MustWait(fmt.Sprintf(`() => document.querySelector('%s') === null`, selector))
```

## Performance Optimization

### Specific vs General Selectors

```go
// SLOW: General selector
elements := page.MustElements("div")

// FAST: Specific selector
elements := page.MustElements("div.product-card")

// FASTEST: ID selector
element := page.MustElement("#product-123")
```

### Scoped Searches

```go
// SLOW: Search entire document
element := page.MustElement("button.save")

// FAST: Search within container
container := page.MustElement("div.form-container")
element := container.MustElement("button.save")
```

### Reuse Elements

```go
// SLOW: Query multiple times
page.MustElement("#container").MustElement(".title").MustText()
page.MustElement("#container").MustElement(".price").MustText()

// FAST: Query once, reuse
container := page.MustElement("#container")
title := container.MustElement(".title").MustText()
price := container.MustElement(".price").MustText()
```

## Testing Selectors

### Browser DevTools

```javascript
// CSS selector
document.querySelector("#id")
document.querySelectorAll(".class")

// XPath
$x("//button[@type='submit']")
```

### Rod Helper

```go
func TestSelector(page *rod.Page, selector string) {
    elements, err := page.Elements(selector)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Found %d elements\n", len(elements))
    for i, el := range elements {
        fmt.Printf("  [%d] %s\n", i, el.MustText())
    }
}

// Usage
TestSelector(page, "button.btn-primary")
```

## Best Practices

1. **Prefer ID selectors when available** - Fastest and most reliable
2. **Use data-testid attributes** - Stable across UI changes
3. **Avoid overly specific paths** - Brittle to DOM changes
4. **Use semantic HTML** - Rely on `<button>`, `<input>`, etc.
5. **Test selectors in DevTools first** - Verify before coding
6. **Keep selectors readable** - Balance specificity with clarity
7. **Use XPath for text matching** - More powerful than CSS
8. **Scope searches when possible** - Search within containers
9. **Wait for dynamic elements** - Don't assume immediate presence
10. **Document complex selectors** - Explain why they're needed
