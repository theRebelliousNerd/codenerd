# Common Browser Automation Scenarios

Complete, production-ready examples for typical automation tasks using Rod.

## Web Scraping

### Basic Product Scraper

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/go-rod/rod"
)

type Product struct {
    Name        string  `json:"name"`
    Price       string  `json:"price"`
    Rating      float64 `json:"rating"`
    URL         string  `json:"url"`
    ImageURL    string  `json:"image_url"`
    InStock     bool    `json:"in_stock"`
}

func ScrapeProducts(url string) ([]Product, error) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)

    // Wait for products to load
    page.MustWaitElementsMoreThan(".product-card", 0)

    // Handle lazy-loaded images
    page.MustWaitLoad()

    var products []Product
    cards := page.MustElements(".product-card")

    for _, card := range cards {
        product := Product{
            Name:     card.MustElement(".product-name").MustText(),
            Price:    card.MustElement(".product-price").MustText(),
            URL:      card.MustElement("a").MustAttribute("href"),
            ImageURL: card.MustElement("img").MustAttribute("src"),
        }

        // Extract rating (may not exist)
        if ratingEl, err := card.Element(".rating"); err == nil {
            rating := ratingEl.MustAttribute("data-rating")
            fmt.Sscanf(rating, "%f", &product.Rating)
        }

        // Check stock status
        product.InStock = true
        if stockEl, err := card.Element(".out-of-stock"); err == nil {
            if stockEl.MustVisible() {
                product.InStock = false
            }
        }

        products = append(products, product)
    }

    return products, nil
}

func main() {
    products, err := ScrapeProducts("https://example.com/products")
    if err != nil {
        panic(err)
    }

    // Save to JSON
    data, _ := json.MarshalIndent(products, "", "  ")
    os.WriteFile("products.json", data, 0644)

    fmt.Printf("Scraped %d products\n", len(products))
}
```

### Pagination Scraper

```go
func ScrapeAllPages(baseURL string) ([]Product, error) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    var allProducts []Product
    page := browser.MustPage(baseURL)

    for pageNum := 1; ; pageNum++ {
        fmt.Printf("Scraping page %d...\n", pageNum)

        // Wait for products
        page.MustWaitElementsMoreThan(".product-card", 0)

        // Scrape current page
        products := extractProducts(page)
        allProducts = append(allProducts, products...)

        // Check for next page
        nextBtn, err := page.Element("button.next-page:not(.disabled)")
        if err != nil {
            break // No more pages
        }

        // Click next
        nextBtn.MustClick()
        page.MustWaitLoad()

        // Avoid rate limiting
        time.Sleep(2 * time.Second)
    }

    return allProducts, nil
}
```

### Infinite Scroll Scraper

```go
func ScrapeInfiniteScroll(url string) ([]Product, error) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)
    page.MustWaitElementsMoreThan(".product-card", 0)

    var products []Product
    previousCount := 0

    for {
        // Get current product count
        cards := page.MustElements(".product-card")
        currentCount := len(cards)

        if currentCount == previousCount {
            // No new products loaded, done
            break
        }

        // Extract new products
        for i := previousCount; i < currentCount; i++ {
            product := extractProduct(cards[i])
            products = append(products, product)
        }

        previousCount = currentCount

        // Scroll to bottom
        page.MustEval(`window.scrollTo(0, document.body.scrollHeight)`)

        // Wait for new content (or timeout)
        time.Sleep(2 * time.Second)
    }

    return products, nil
}
```

## Form Automation

### Login Flow

```go
func Login(username, password string) error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com/login")

    // Fill credentials
    page.MustElement("input[name='username']").MustInput(username)
    page.MustElement("input[name='password']").MustInput(password)

    // Submit form
    page.MustElement("button[type='submit']").MustClick()

    // Wait for navigation
    page.MustWaitNavigation()

    // Verify success
    currentURL := page.MustInfo().URL
    if !strings.Contains(currentURL, "/dashboard") {
        // Check for error message
        if errorEl, err := page.Element(".error-message"); err == nil {
            return fmt.Errorf("login failed: %s", errorEl.MustText())
        }
        return errors.New("login failed: unexpected redirect")
    }

    return nil
}
```

### Multi-Step Form

```go
func SubmitApplication(data ApplicationData) error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com/apply")

    // Step 1: Personal Info
    page.MustElement("#first-name").MustInput(data.FirstName)
    page.MustElement("#last-name").MustInput(data.LastName)
    page.MustElement("#email").MustInput(data.Email)
    page.MustElement("button#next-step-1").MustClick()

    // Wait for step 2
    page.MustWaitVisible("#step-2")

    // Step 2: Address
    page.MustElement("#address").MustInput(data.Address)
    page.MustElement("#city").MustInput(data.City)
    page.MustElement("#state").MustSelect(data.State)
    page.MustElement("#zip").MustInput(data.ZipCode)
    page.MustElement("button#next-step-2").MustClick()

    // Wait for step 3
    page.MustWaitVisible("#step-3")

    // Step 3: Documents
    page.MustElement("input[type='file']#resume").
        MustSetFiles(data.ResumePath)

    page.MustElement("input[type='file']#cover-letter").
        MustSetFiles(data.CoverLetterPath)

    // Submit
    page.MustElement("button[type='submit']").MustClick()

    // Wait for confirmation
    page.MustWaitVisible(".success-message")

    return nil
}
```

### CAPTCHA Handling

```go
func LoginWithCaptcha(username, password string) error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com/login")

    // Fill credentials
    page.MustElement("input[name='username']").MustInput(username)
    page.MustElement("input[name='password']").MustInput(password)

    // Check if CAPTCHA present
    if captchaEl, err := page.Element(".captcha-container"); err == nil {
        // CAPTCHA detected - pause for manual solving
        fmt.Println("CAPTCHA detected. Please solve manually...")
        fmt.Println("Press Enter when done...")

        // Keep browser open
        browser.MustIncognito().MustClose()

        // Wait for user input
        bufio.NewReader(os.Stdin).ReadString('\n')

        // Verify CAPTCHA solved
        if _, err := captchaEl.Element(".captcha-success"); err != nil {
            return errors.New("CAPTCHA not solved")
        }
    }

    // Submit
    page.MustElement("button[type='submit']").MustClick()
    page.MustWaitNavigation()

    return nil
}
```

## Testing

### E2E Test Suite

```go
package tests

import (
    "testing"

    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type TestSuite struct {
    browser *rod.Browser
    cleanup func()
}

func (s *TestSuite) Setup(t *testing.T) {
    launcher := launcher.New().Headless(true).NoSandbox(true)
    url := launcher.MustLaunch()

    s.browser = rod.New().ControlURL(url).MustConnect()
    s.cleanup = func() {
        s.browser.MustClose()
        launcher.Cleanup()
    }
}

func (s *TestSuite) Teardown() {
    s.cleanup()
}

func TestLogin(t *testing.T) {
    suite := &TestSuite{}
    suite.Setup(t)
    defer suite.Teardown()

    page := suite.browser.MustPage("https://example.com/login")

    // Test login
    page.MustElement("#username").MustInput("testuser")
    page.MustElement("#password").MustInput("testpass")
    page.MustElement("button[type='submit']").MustClick()

    page.MustWaitNavigation()

    // Assertions
    assert.Contains(t, page.MustInfo().URL, "/dashboard")
    assert.Contains(t, page.MustElement("h1").MustText(), "Welcome")
}

func TestProductSearch(t *testing.T) {
    suite := &TestSuite{}
    suite.Setup(t)
    defer suite.Teardown()

    page := suite.browser.MustPage("https://example.com")

    // Search
    page.MustElement("input.search").MustInput("laptop")
    page.MustElement("button.search-btn").MustClick()

    // Wait for results
    page.MustWaitElementsMoreThan(".product-card", 0)

    // Verify
    results := page.MustElements(".product-card")
    require.NotEmpty(t, results, "No search results")

    // Check first result
    firstResult := results[0].MustElement(".product-name").MustText()
    assert.Contains(t, strings.ToLower(firstResult), "laptop")
}
```

### Visual Regression Testing

```go
func TestVisualRegression(t *testing.T) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com")
    page.MustWaitLoad()

    // Take screenshot
    screenshot, err := page.Screenshot(true, nil)
    require.NoError(t, err)

    // Compare with baseline
    baseline, err := os.ReadFile("testdata/baseline.png")
    require.NoError(t, err)

    diff := compareImages(screenshot, baseline)
    assert.Less(t, diff, 0.01, "Visual regression detected (>1% difference)")
}

func compareImages(img1, img2 []byte) float64 {
    // Image comparison logic
    // Return percentage difference (0.0 = identical, 1.0 = completely different)
    // Use libraries like "github.com/disintegration/imaging"
    return 0.0
}
```

## Data Extraction

### Table Scraper

```go
type TableRow struct {
    Columns []string
}

func ScrapeTable(url, tableSelector string) ([]TableRow, error) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)
    page.MustWaitLoad()

    // Find table
    table := page.MustElement(tableSelector)

    // Get all rows
    rows := table.MustElements("tbody tr")

    var data []TableRow
    for _, row := range rows {
        cells := row.MustElements("td")

        var columns []string
        for _, cell := range cells {
            columns = append(columns, strings.TrimSpace(cell.MustText()))
        }

        data = append(data, TableRow{Columns: columns})
    }

    return data, nil
}
```

### PDF Generation

```go
func GeneratePDFReport(url string, outputPath string) error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)
    page.MustWaitLoad()

    // Wait for dynamic content
    page.MustWait(`() => document.querySelector('.report-ready') !== null`)

    // Generate PDF
    pdf, err := page.PDF(&proto.PagePrintToPDF{
        Landscape:           false,
        DisplayHeaderFooter: true,
        PrintBackground:     true,
        Scale:               1.0,
        PaperWidth:          8.5,
        PaperHeight:         11.0,
        MarginTop:           0.5,
        MarginBottom:        0.5,
        MarginLeft:          0.5,
        MarginRight:         0.5,
        HeaderTemplate:      "<div style='font-size:10px;text-align:center;width:100%'>Report Generated: <span class='date'></span></div>",
        FooterTemplate:      "<div style='font-size:10px;text-align:center;width:100%'>Page <span class='pageNumber'></span> of <span class='totalPages'></span></div>",
    })
    if err != nil {
        return err
    }

    return os.WriteFile(outputPath, pdf, 0644)
}
```

## Network Interception

### API Monitoring

```go
type APICall struct {
    Method   string
    URL      string
    Status   int
    Duration time.Duration
}

func MonitorAPICalls(url string) ([]APICall, error) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    var calls []APICall
    var mu sync.Mutex

    // Intercept requests
    router := browser.HijackRequests()
    defer router.MustStop()

    router.MustAdd("*/api/*", func(ctx *rod.Hijack) {
        start := time.Now()

        // Log request
        method := ctx.Request.Method()
        url := ctx.Request.URL().String()

        // Continue request
        ctx.MustLoadResponse()

        // Log response
        duration := time.Since(start)
        status := ctx.Response.Payload().ResponseCode

        mu.Lock()
        calls = append(calls, APICall{
            Method:   method,
            URL:      url,
            Status:   status,
            Duration: duration,
        })
        mu.Unlock()
    })

    go router.Run()

    // Navigate and trigger API calls
    page := browser.MustPage(url)
    page.MustWaitLoad()

    // Wait for all requests to complete
    time.Sleep(2 * time.Second)

    return calls, nil
}
```

### Request Modification

```go
func AddAuthHeader(url, token string) error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    router := browser.HijackRequests()
    defer router.MustStop()

    // Add auth header to all requests
    router.MustAdd("*", func(ctx *rod.Hijack) {
        ctx.Request.SetHeader("Authorization", "Bearer "+token)
        ctx.MustLoadResponse()
    })

    go router.Run()

    page := browser.MustPage(url)
    page.MustWaitLoad()

    return nil
}
```

## Session Management

### Cookie-Based Sessions

```go
type Session struct {
    browser *rod.Browser
    cookies []*proto.NetworkCookie
}

func NewSession() *Session {
    return &Session{
        browser: rod.New().MustConnect(),
    }
}

func (s *Session) Login(username, password string) error {
    page := s.browser.MustPage("https://example.com/login")

    page.MustElement("#username").MustInput(username)
    page.MustElement("#password").MustInput(password)
    page.MustElement("button[type='submit']").MustClick()

    page.MustWaitNavigation()

    // Save cookies
    s.cookies = page.MustCookies()

    page.MustClose()
    return nil
}

func (s *Session) NewPage(url string) *rod.Page {
    page := s.browser.MustPage(url)

    // Restore cookies
    if s.cookies != nil {
        page.MustSetCookies(s.cookies...)
    }

    return page
}

func (s *Session) Close() {
    s.browser.MustClose()
}

// Usage
func main() {
    session := NewSession()
    defer session.Close()

    session.Login("user", "pass")

    // Reuse session
    page1 := session.NewPage("https://example.com/dashboard")
    page2 := session.NewPage("https://example.com/profile")
}
```

### Session Pool

```go
type SessionPool struct {
    mu       sync.Mutex
    sessions []*rod.Browser
    maxSize  int
}

func NewSessionPool(maxSize int) *SessionPool {
    return &SessionPool{
        sessions: make([]*rod.Browser, 0, maxSize),
        maxSize:  maxSize,
    }
}

func (p *SessionPool) Acquire() *rod.Browser {
    p.mu.Lock()
    defer p.mu.Unlock()

    if len(p.sessions) > 0 {
        // Reuse existing
        browser := p.sessions[len(p.sessions)-1]
        p.sessions = p.sessions[:len(p.sessions)-1]
        return browser
    }

    // Create new
    return rod.New().MustConnect()
}

func (p *SessionPool) Release(browser *rod.Browser) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if len(p.sessions) < p.maxSize {
        p.sessions = append(p.sessions, browser)
    } else {
        browser.MustClose()
    }
}

func (p *SessionPool) Close() {
    p.mu.Lock()
    defer p.mu.Unlock()

    for _, browser := range p.sessions {
        browser.MustClose()
    }
    p.sessions = nil
}

// Usage
func main() {
    pool := NewSessionPool(5)
    defer pool.Close()

    browser := pool.Acquire()
    defer pool.Release(browser)

    page := browser.MustPage("https://example.com")
    // ... work with page
}
```

## Error Handling

### Retry Logic

```go
func RetryOperation(fn func() error, maxAttempts int) error {
    var lastErr error

    for i := 0; i < maxAttempts; i++ {
        err := fn()
        if err == nil {
            return nil
        }

        lastErr = err

        if i < maxAttempts-1 {
            backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
            time.Sleep(backoff)
        }
    }

    return fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}

// Usage
err := RetryOperation(func() error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com")
    // ... perform operations

    return nil
}, 3)
```

### Graceful Degradation

```go
func ScrapeWithFallback(url string) ([]Product, error) {
    // Try with JavaScript
    products, err := scrapeWithJS(url)
    if err == nil {
        return products, nil
    }

    log.Printf("JS scraping failed: %v, trying HTML parsing", err)

    // Fallback to HTML parsing
    products, err = scrapeWithHTML(url)
    if err == nil {
        return products, nil
    }

    return nil, fmt.Errorf("all scraping methods failed")
}
```

These examples provide production-ready patterns for common browser automation tasks. Adapt them to your specific needs and always handle errors appropriately.
