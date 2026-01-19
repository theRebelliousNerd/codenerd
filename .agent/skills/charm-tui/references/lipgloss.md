# Lipgloss Styling Reference

Complete reference for terminal styling with Lipgloss.

## Table of Contents

- [Style Basics](#style-basics)
- [Colors](#colors)
- [Text Formatting](#text-formatting)
- [Spacing](#spacing)
- [Dimensions](#dimensions)
- [Alignment](#alignment)
- [Borders](#borders)
- [Style Composition](#style-composition)
- [Layout Utilities](#layout-utilities)
- [Tables](#tables)
- [Lists](#lists)
- [Trees](#trees)
- [Renderers](#renderers)

---

## Style Basics

### Creating Styles

```go
import "github.com/charmbracelet/lipgloss"

// Create a new style
style := lipgloss.NewStyle()

// Chain style methods
style := lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("212")).
    Padding(1, 2)

// Render text with style
output := style.Render("Hello, World!")
```

### Style Immutability

Styles are immutable - methods return new copies:

```go
baseStyle := lipgloss.NewStyle().Bold(true)

// This creates a NEW style, doesn't modify baseStyle
derivedStyle := baseStyle.Foreground(lipgloss.Color("212"))

// baseStyle is still just Bold(true)
// derivedStyle is Bold(true) + Foreground("212")
```

### Getting Style Properties

```go
style := lipgloss.NewStyle().
    Bold(true).
    Width(40).
    Padding(1, 2)

// Check if property is set
if style.GetBold() {
    // ...
}

// Get dimensions
width := style.GetWidth()
padding := style.GetPadding() // returns [top, right, bottom, left]

// Get horizontal/vertical padding
hPad := style.GetHorizontalPadding()
vPad := style.GetVerticalPadding()
```

---

## Colors

### Color Types

```go
// ANSI 16 colors (0-15)
// 0=black, 1=red, 2=green, 3=yellow, 4=blue, 5=magenta, 6=cyan, 7=white
// 8-15 = bright variants
lipgloss.Color("5")  // Magenta
lipgloss.Color("12") // Bright blue

// ANSI 256 colors (0-255)
lipgloss.Color("201") // Hot pink
lipgloss.Color("63")  // Purple

// True Color (24-bit hex)
lipgloss.Color("#FF00FF")
lipgloss.Color("#7D56F4")
lipgloss.Color("#FAFAFA")

// RGB format also works
lipgloss.Color("rgb(255, 0, 255)")
```

### Adaptive Colors

Automatically choose color based on terminal background:

```go
// Simple adaptive
adaptiveColor := lipgloss.AdaptiveColor{
    Light: "236",  // Dark gray for light backgrounds
    Dark:  "252",  // Light gray for dark backgrounds
}

style := lipgloss.NewStyle().Foreground(adaptiveColor)
```

### Complete Colors

Specify exact colors for each color profile:

```go
// Full control over all color profiles
completeColor := lipgloss.CompleteColor{
    TrueColor: "#FF00FF",  // 24-bit terminals
    ANSI256:   "201",      // 256-color terminals
    ANSI:      "5",        // 16-color terminals
}

style := lipgloss.NewStyle().Foreground(completeColor)
```

### Complete Adaptive Colors

Both adaptive AND complete:

```go
complexColor := lipgloss.CompleteAdaptiveColor{
    Light: lipgloss.CompleteColor{
        TrueColor: "#000000",
        ANSI256:   "16",
        ANSI:      "0",
    },
    Dark: lipgloss.CompleteColor{
        TrueColor: "#FFFFFF",
        ANSI256:   "231",
        ANSI:      "15",
    },
}
```

### No Color

```go
// Explicitly no color
lipgloss.NoColor{}

// Unset a color
style = style.UnsetForeground()
style = style.UnsetBackground()
```

### Applying Colors

```go
style := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#FAFAFA")).  // Text color
    Background(lipgloss.Color("#7D56F4"))   // Background color
```

---

## Text Formatting

### Inline Formatting

```go
style := lipgloss.NewStyle().
    Bold(true).
    Italic(true).
    Underline(true).
    Strikethrough(true).
    Faint(true).         // Dim text
    Blink(true).         // Blinking (limited support)
    Reverse(true)        // Swap foreground/background
```

### Text Transform

```go
style := lipgloss.NewStyle().
    Transform(strings.ToUpper)  // Apply string transform

// Custom transform
style := lipgloss.NewStyle().
    Transform(func(s string) string {
        return ">> " + s + " <<"
    })
```

### Inline Styles

Embed string in style:

```go
// Set string on style
style := lipgloss.NewStyle().
    SetString("Hello").
    Bold(true).
    Foreground(lipgloss.Color("212"))

// Use Stringer interface
fmt.Println(style)  // Prints styled "Hello"

// Or render explicitly
fmt.Println(style.String())
```

---

## Spacing

### Padding

```go
// All sides same
style := lipgloss.NewStyle().Padding(2)  // 2 on all sides

// Vertical and horizontal
style := lipgloss.NewStyle().Padding(1, 2)  // v=1, h=2

// Top, horizontal, bottom
style := lipgloss.NewStyle().Padding(1, 2, 3)  // t=1, h=2, b=3

// All four sides (clockwise from top)
style := lipgloss.NewStyle().Padding(1, 2, 3, 4)  // t=1, r=2, b=3, l=4

// Individual sides
style := lipgloss.NewStyle().
    PaddingTop(1).
    PaddingRight(2).
    PaddingBottom(1).
    PaddingLeft(2)
```

### Margin

```go
// Same patterns as padding
style := lipgloss.NewStyle().Margin(2)
style := lipgloss.NewStyle().Margin(1, 2)
style := lipgloss.NewStyle().Margin(1, 2, 3, 4)

// Individual sides
style := lipgloss.NewStyle().
    MarginTop(1).
    MarginRight(2).
    MarginBottom(1).
    MarginLeft(2)

// Margin background color
style := lipgloss.NewStyle().
    Margin(1).
    MarginBackground(lipgloss.Color("63"))
```

---

## Dimensions

### Width and Height

```go
style := lipgloss.NewStyle().
    Width(40).           // Exact width
    Height(10).          // Exact height
    MaxWidth(80).        // Maximum width
    MaxHeight(20)        // Maximum height
```

### Measuring Content

```go
rendered := style.Render("Hello, World!")

// Get dimensions
width := lipgloss.Width(rendered)
height := lipgloss.Height(rendered)

// Or both at once
w, h := lipgloss.Size(rendered)
```

---

## Alignment

### Horizontal Alignment

```go
style := lipgloss.NewStyle().
    Width(40).
    Align(lipgloss.Left)    // Left align (default)

style := lipgloss.NewStyle().
    Width(40).
    Align(lipgloss.Center)  // Center align

style := lipgloss.NewStyle().
    Width(40).
    Align(lipgloss.Right)   // Right align
```

### Vertical Alignment

```go
style := lipgloss.NewStyle().
    Height(10).
    AlignVertical(lipgloss.Top)     // Top align (default)

style := lipgloss.NewStyle().
    Height(10).
    AlignVertical(lipgloss.Center)  // Middle align

style := lipgloss.NewStyle().
    Height(10).
    AlignVertical(lipgloss.Bottom)  // Bottom align
```

### Both Alignments

```go
style := lipgloss.NewStyle().
    Width(40).
    Height(10).
    Align(lipgloss.Center).
    AlignVertical(lipgloss.Center)
```

### Position Constants

```go
lipgloss.Left    // 0.0
lipgloss.Center  // 0.5
lipgloss.Right   // 1.0
lipgloss.Top     // 0.0
lipgloss.Bottom  // 1.0

// Custom positions (0.0 to 1.0)
style := lipgloss.NewStyle().
    Width(40).
    Align(0.25)  // 25% from left
```

---

## Borders

### Border Styles

```go
// Built-in border styles
lipgloss.NormalBorder()     // ┌─┐│ │└─┘
lipgloss.RoundedBorder()    // ╭─╮│ │╰─╯
lipgloss.ThickBorder()      // ┏━┓┃ ┃┗━┛
lipgloss.DoubleBorder()     // ╔═╗║ ║╚═╝
lipgloss.BlockBorder()      // █▀██ ██▄█
lipgloss.HiddenBorder()     // (invisible, but takes space)
lipgloss.ASCIIBorder()      // +-+| |+-+
lipgloss.MarkdownBorder()   // Markdown table style
```

### Applying Borders

```go
// Full border
style := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("63"))

// Selective edges
style := lipgloss.NewStyle().
    Border(lipgloss.NormalBorder(), true, false, true, false)
    // top, right, bottom, left

// Individual edge control
style := lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderTop(true).
    BorderBottom(true).
    BorderLeft(false).
    BorderRight(false)
```

### Border Colors

```go
style := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("212")).   // Border line color
    BorderBackground(lipgloss.Color("63"))     // Border background

// Individual edge colors
style := lipgloss.NewStyle().
    Border(lipgloss.NormalBorder()).
    BorderTopForeground(lipgloss.Color("212")).
    BorderBottomForeground(lipgloss.Color("212")).
    BorderLeftForeground(lipgloss.Color("63")).
    BorderRightForeground(lipgloss.Color("63"))
```

### Custom Borders

```go
customBorder := lipgloss.Border{
    Top:          "._.:*:",
    Bottom:       "._.:*:",
    Left:         "|*",
    Right:        "|*",
    TopLeft:      "*",
    TopRight:     "*",
    BottomLeft:   "*",
    BottomRight:  "*",
    MiddleTop:    ".:.",
    MiddleBottom: ".:.",
    MiddleLeft:   "| ",
    MiddleRight:  " |",
    Middle:       " + ",
}

style := lipgloss.NewStyle().
    Border(customBorder).
    BorderForeground(lipgloss.Color("205"))
```

---

## Style Composition

### Inheritance

```go
// Only unset properties are inherited
baseStyle := lipgloss.NewStyle().
    Background(lipgloss.Color("63")).
    Padding(1, 2)

childStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("212")).  // Child's own foreground
    Inherit(baseStyle)                  // Gets background and padding

// childStyle has:
// - Foreground: 212 (own)
// - Background: 63 (inherited)
// - Padding: 1, 2 (inherited)
```

### Copying

```go
// Copy via assignment (styles are value types)
styleCopy := originalStyle

// Modify copy without affecting original
modified := styleCopy.Bold(true)
```

### Unsetting Properties

```go
style := lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("212")).
    UnsetBold().           // Remove bold
    UnsetForeground()      // Remove foreground color

// Unset methods for all properties:
// UnsetBold(), UnsetItalic(), UnsetUnderline(), etc.
// UnsetForeground(), UnsetBackground()
// UnsetPadding(), UnsetPaddingTop(), etc.
// UnsetMargin(), UnsetMarginTop(), etc.
// UnsetWidth(), UnsetHeight(), etc.
// UnsetBorder(), UnsetBorderTop(), etc.
```

---

## Layout Utilities

### Joining Strings

```go
// Join horizontally (side by side)
result := lipgloss.JoinHorizontal(
    lipgloss.Top,      // Vertical alignment
    block1,
    block2,
    block3,
)

// Alignment options: Top, Center, Bottom, or 0.0-1.0

// Join vertically (stacked)
result := lipgloss.JoinVertical(
    lipgloss.Left,     // Horizontal alignment
    block1,
    block2,
    block3,
)

// Alignment options: Left, Center, Right, or 0.0-1.0
```

### Placing in Whitespace

```go
// Place horizontally in space
result := lipgloss.PlaceHorizontal(
    80,                // Width
    lipgloss.Center,   // Position
    content,
)

// Place vertically in space
result := lipgloss.PlaceVertical(
    24,                // Height
    lipgloss.Bottom,   // Position
    content,
)

// Place in 2D space
result := lipgloss.Place(
    80,                // Width
    24,                // Height
    lipgloss.Center,   // Horizontal position
    lipgloss.Center,   // Vertical position
    content,
)
```

### Whitespace Options

```go
// Style the whitespace fill
result := lipgloss.Place(
    80, 24,
    lipgloss.Center, lipgloss.Center,
    content,
    lipgloss.WithWhitespaceChars("."),              // Fill character
    lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),  // Background
    lipgloss.WithWhitespaceForeground(lipgloss.Color("240")),  // Foreground
)
```

---

## Tables

Import: `github.com/charmbracelet/lipgloss/table`

### Basic Table

```go
import "github.com/charmbracelet/lipgloss/table"

t := table.New().
    Headers("Name", "Age", "City").
    Row("Alice", "30", "New York").
    Row("Bob", "25", "Los Angeles").
    Row("Charlie", "35", "Chicago")

fmt.Println(t)
```

### Table Styling

```go
purple := lipgloss.Color("99")
gray := lipgloss.Color("245")

headerStyle := lipgloss.NewStyle().
    Foreground(purple).
    Bold(true).
    Align(lipgloss.Center)

cellStyle := lipgloss.NewStyle().
    Padding(0, 1)

t := table.New().
    Border(lipgloss.RoundedBorder()).
    BorderStyle(lipgloss.NewStyle().Foreground(purple)).
    StyleFunc(func(row, col int) lipgloss.Style {
        if row == table.HeaderRow {
            return headerStyle
        }
        if row%2 == 0 {
            return cellStyle.Foreground(gray)
        }
        return cellStyle
    }).
    Headers("Name", "Age", "City").
    Rows(rows...)
```

### Table Options

```go
t := table.New().
    Border(lipgloss.RoundedBorder()).
    BorderTop(true).
    BorderBottom(true).
    BorderLeft(true).
    BorderRight(true).
    BorderRow(false).           // Row separators
    BorderColumn(false).        // Column separators
    BorderHeader(true).         // Header separator
    Width(80).                  // Total width
    Headers("Col1", "Col2").
    Rows(data...)
```

---

## Lists

Import: `github.com/charmbracelet/lipgloss/list`

### Basic List

```go
import "github.com/charmbracelet/lipgloss/list"

// Simple list
l := list.New("Item 1", "Item 2", "Item 3")
fmt.Println(l)

// Nested list
l := list.New(
    "Fruits",
    list.New("Apple", "Banana", "Cherry"),
    "Vegetables",
    list.New("Carrot", "Broccoli", "Spinach"),
)
```

### List Enumerators

```go
// Built-in enumerators
l := list.New(items...).Enumerator(list.Bullet)      // • item
l := list.New(items...).Enumerator(list.Dash)        // - item
l := list.New(items...).Enumerator(list.Arabic)      // 1. item
l := list.New(items...).Enumerator(list.Roman)       // I. item
l := list.New(items...).Enumerator(list.Alphabet)    // A. item

// Custom enumerator
l := list.New("Duck", "Duck", "Goose", "Duck").
    Enumerator(func(items list.Items, i int) string {
        if items.At(i).Value() == "Goose" {
            return ">>> "
        }
        return "    "
    })
```

### List Styling

```go
l := list.New(items...).
    Enumerator(list.Arabic).
    EnumeratorStyle(lipgloss.NewStyle().
        Foreground(lipgloss.Color("99")).
        MarginRight(1)).
    ItemStyle(lipgloss.NewStyle().
        Foreground(lipgloss.Color("212"))).
    ItemStyleFunc(func(items list.Items, i int) lipgloss.Style {
        if i == selectedIndex {
            return lipgloss.NewStyle().Bold(true)
        }
        return lipgloss.NewStyle()
    })
```

---

## Trees

Import: `github.com/charmbracelet/lipgloss/tree`

### Basic Tree

```go
import "github.com/charmbracelet/lipgloss/tree"

// Simple tree
t := tree.Root(".").
    Child("README.md").
    Child("main.go").
    Child("go.mod")

// Nested tree
t := tree.Root("project").
    Child(
        tree.Root("src").
            Child("main.go", "utils.go"),
    ).
    Child(
        tree.Root("tests").
            Child("main_test.go"),
    ).
    Child("README.md")
```

### Tree Enumerators

```go
// Built-in styles
tree.DefaultEnumerator   // ├── and └──
tree.RoundedEnumerator   // ├── and ╰──

t := tree.Root("root").
    Child("a", "b", "c").
    Enumerator(tree.RoundedEnumerator)
```

### Tree Styling

```go
t := tree.Root("Project").
    Child("src", "tests", "docs").
    Enumerator(tree.RoundedEnumerator).
    EnumeratorStyle(lipgloss.NewStyle().
        Foreground(lipgloss.Color("63"))).
    RootStyle(lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("212"))).
    ItemStyle(lipgloss.NewStyle().
        Foreground(lipgloss.Color("252"))).
    ItemStyleFunc(func(children tree.Children, i int) lipgloss.Style {
        // Custom per-item styling
        return lipgloss.NewStyle()
    })
```

---

## Renderers

### Default Renderer

```go
// Get default renderer (uses stdout)
r := lipgloss.DefaultRenderer()

// Create style on renderer
style := r.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "15"})

output := style.Render("Hello")
```

### Custom Renderers

For SSH servers or multiple outputs:

```go
// Create renderer for specific output
r := lipgloss.NewRenderer(os.Stdout)

// Or with custom writer
r := lipgloss.NewRenderer(myWriter)

// Create styles bound to this renderer
style := r.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("212"))

// Check renderer properties
hasDark := r.HasDarkBackground()
profile := r.ColorProfile()
```

### SSH Server Example

```go
func handleSSHSession(sess ssh.Session) {
    // Each session gets its own renderer
    renderer := lipgloss.NewRenderer(sess)

    // Styles adapt to client terminal
    style := renderer.NewStyle().
        Foreground(lipgloss.AdaptiveColor{
            Light: "0",
            Dark: "15",
        })

    io.WriteString(sess, style.Render("Welcome!"))
}
```

### Color Profiles

```go
// Get current color profile
profile := lipgloss.ColorProfile()

// Available profiles
lipgloss.TrueColor  // 24-bit (16 million colors)
lipgloss.ANSI256    // 8-bit (256 colors)
lipgloss.ANSI       // 4-bit (16 colors)
lipgloss.Ascii      // No color support

// Force specific profile (for testing)
r := lipgloss.NewRenderer(os.Stdout)
r.SetColorProfile(lipgloss.ANSI256)
```

---

## Complete Example

```go
package main

import (
    "fmt"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/lipgloss/table"
)

func main() {
    // Define colors
    purple := lipgloss.Color("99")
    pink := lipgloss.Color("212")
    gray := lipgloss.Color("245")

    // Define styles
    titleStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FAFAFA")).
        Background(purple).
        Padding(0, 2).
        MarginBottom(1)

    boxStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(purple).
        Padding(1, 2)

    // Create title
    title := titleStyle.Render("Dashboard")

    // Create stats
    stat1 := boxStyle.Render(fmt.Sprintf("%s\n%s",
        lipgloss.NewStyle().Bold(true).Render("Users"),
        lipgloss.NewStyle().Foreground(pink).Render("1,234")))

    stat2 := boxStyle.Render(fmt.Sprintf("%s\n%s",
        lipgloss.NewStyle().Bold(true).Render("Revenue"),
        lipgloss.NewStyle().Foreground(pink).Render("$12,345")))

    stats := lipgloss.JoinHorizontal(lipgloss.Top, stat1, "  ", stat2)

    // Create table
    t := table.New().
        Border(lipgloss.RoundedBorder()).
        BorderStyle(lipgloss.NewStyle().Foreground(gray)).
        Headers("Name", "Status", "Score").
        Row("Alice", "Active", "95").
        Row("Bob", "Inactive", "82").
        Row("Charlie", "Active", "91")

    // Combine everything
    output := lipgloss.JoinVertical(
        lipgloss.Left,
        title,
        stats,
        "",
        t.String(),
    )

    fmt.Println(output)
}
```
