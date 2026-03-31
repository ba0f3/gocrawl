package extractor

// ExtractionOptions configures webclaw-style main-content extraction.
type ExtractionOptions struct {
	ExcludeSelectors []string
	IncludeSelectors []string
	OnlyMainContent  bool
	IncludeRawHTML   bool
}
