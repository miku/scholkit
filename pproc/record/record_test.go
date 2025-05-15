package record

import (
	"bufio"
	"strings"
	"testing"
)

func TestFindFirstCompleteTag(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		tagName   string
		wantStart int
		wantEnd   int
	}{
		{
			name:      "Single element",
			input:     `<div>content</div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   18,
		},
		{
			name:      "Multiple elements - finds first",
			input:     `<div>first</div><div>second</div><div>third</div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   16, // Only the first <div>first</div>
		},
		{
			name:      "Nested elements - finds outermost",
			input:     `<div>outer<div>inner</div></div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   32, // The complete outer div
		},
		{
			name:      "With attributes",
			input:     `<div class="test" id="main">content</div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   41,
		},
		{
			name:      "Self-closing tag",
			input:     `<div/>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   6,
		},
		{
			name:      "Mixed self-closing and normal - finds first",
			input:     `<div/><div>content</div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   6, // Only the self-closing div
		},
		{
			name:      "No matching elements",
			input:     `<span>not div</span>`,
			tagName:   "div",
			wantStart: -1,
			wantEnd:   -1,
		},
		{
			name:      "Invalid tag (no closing)",
			input:     `<div>unclosed`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   -1, // Found start but no end
		},
		{
			name:      "Tag in text content",
			input:     `<p>text with <div> inside</p><div>real</div>`,
			tagName:   "div",
			wantStart: 29,
			wantEnd:   44, // The real div element
		},
		{
			name:      "Similar tag names",
			input:     `<divx>wrong</divx><div>correct</div>`,
			tagName:   "div",
			wantStart: 18,
			wantEnd:   36,
		},
		{
			name:      "Multiple nested levels - finds outermost",
			input:     `<div><div><div>deep</div></div></div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   37, // The complete outer div
		},
		{
			name:      "Empty elements - finds first",
			input:     `<div></div><div></div>`,
			tagName:   "div",
			wantStart: 0,
			wantEnd:   11, // Only the first empty div
		},
		{
			name:      "With newlines and spaces",
			input:     "<div>\n  <div>inner</div>\n</div>",
			tagName:   "div",
			wantStart: 0,
			wantEnd:   31, // The complete outer div with formatting
		},
		{
			name:      "Malformed nested - missing closing",
			input:     `<div><div>inner</div>`,
			tagName:   "div",
			wantStart: 5, // Should find the inner div since outer is incomplete
			wantEnd:   21,
		},
		{
			name:      "Find inner element when outer is malformed",
			input:     `<div><span>inner</span>`,
			tagName:   "span",
			wantStart: 5,
			wantEnd:   23, // Can find the complete inner span
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := findFirstCompleteTag(tt.input, tt.tagName)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("findFirstCompleteTag() = (%v, %v), want (%v, %v)",
					gotStart, gotEnd, tt.wantStart, tt.wantEnd)
				if gotStart != -1 && gotEnd != -1 && gotEnd <= len(tt.input) {
					t.Errorf("Found content: %q", tt.input[gotStart:gotEnd])
				}
			}
		})
	}
}

// Test with real-world HTML-like content
func TestFindFirstCompleteTagRealWorld(t *testing.T) {
	input := `
																																																																																																																																																																																																																																																																																																																																																	    <html>
																																																																																																																																																																																																																																																																																																																																																		        <body>
																																																																																																																																																																																																																																																																																																																																																				            <div class="header">
																																																																																																																																																																																																																																																																																																																																																							                <h1>Title</h1>
																																																																																																																																																																																																																																																																																																																																																											            </div>
																																																																																																																																																																																																																																																																																																																																																														            <div class="content">
																																																																																																																																																																																																																																																																																																																																																																	                <p>Some text</p>
																																																																																																																																																																																																																																																																																																																																																																					                <div>Nested content</div>
																																																																																																																																																																																																																																																																																																																																																																									            </div>
																																																																																																																																																																																																																																																																																																																																																																												            <div class="footer">
																																																																																																																																																																																																																																																																																																																																																																															                Footer
																																																																																																																																																																																																																																																																																																																																																																																			            </div>
																																																																																																																																																																																																																																																																																																																																																																																						        </body>
																																																																																																																																																																																																																																																																																																																																																																																								    </html>`

	start, end := findFirstCompleteTag(input, "div")
	if start == -1 || end == -1 {
		t.Error("Should find first div element")
	}
	// Verify the content between start and end
	content := input[start:end]
	if len(content) == 0 {
		t.Error("Content should not be empty")
	}
	// Should find only the first div with class="header"
	if !strings.Contains(content, `class="header"`) {
		t.Error("Should find the header div")
	}
	if strings.Contains(content, `class="content"`) || strings.Contains(content, `class="footer"`) {
		t.Error("Should not include other divs")
	}
	t.Logf("Found element starts at: %d", start)
	t.Logf("Found element ends at: %d", end)
	t.Logf("Content: %q", content)
}

func TestXMLTagSplitterMultipleElements(t *testing.T) {
	input := `<root><item>first</item><item>second</item><item>third</item></root>`
	splitterFunc := TagSplitter("item", 10, 1000)
	// Create a scanner with our split function
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(splitterFunc)
	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}
	expectedTokens := []string{
		"<item>first</item>",
		"<item>second</item>",
		"<item>third</item>",
	}
	if len(tokens) != len(expectedTokens) {
		t.Errorf("Expected %d tokens, got %d", len(expectedTokens), len(tokens))
		t.Errorf("Tokens: %v", tokens)
	}
	for i, token := range tokens {
		if i < len(expectedTokens) && token != expectedTokens[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expectedTokens[i], token)
		}
	}
}
