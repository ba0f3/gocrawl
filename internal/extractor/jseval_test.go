package extractor

import (
	"strings"
	"testing"
)

func TestExtractJsDataFromHTML_Preloaded(t *testing.T) {
	html := `<html><body><script>
window.__preloadedData = {
  "page": {
    "title": "Hello",
    "body": "This is a longer paragraph of text that should be extracted from the preloaded data blob successfully and it needs to be long enough to pass the threshold for JSON size."
  }
};
</script></body></html>`
	blobs := ExtractJsDataFromHTML(html)
	if len(blobs) == 0 {
		t.Fatal("expected blobs")
	}
	found := false
	for _, b := range blobs {
		if b.Name == "__preloadedData" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected __preloadedData, got %#v", blobs)
	}
	txt := ExtractReadableTextFromBlobs(blobs)
	if !strings.Contains(txt, "longer paragraph") {
		t.Fatalf("expected readable text: %q", txt)
	}
}

func TestFilterReadable_Prose(t *testing.T) {
	s := filterReadable("This is a normal sentence with enough words in it to pass the minimum length checks for prose extraction here.")
	if s == "" {
		t.Fatal("expected prose")
	}
}

func TestFilterReadable_RejectURL(t *testing.T) {
	if filterReadable("https://example.com/some/long/path/that/has/many/segments/to/test") != "" {
		t.Fatal("expected reject")
	}
}
