package scout

import "net/url"

// Wikipedia is the Wikipedia search engine.
const Wikipedia SearchEngine = 3

var wikipediaParser = serpParser{
	resultSelector:  ".mw-search-results li, .mw-search-result",
	titleSelector:   ".mw-search-result-heading a",
	linkSelector:    ".mw-search-result-heading a",
	snippetSelector: ".searchresult",
	nextSelector:    ".mw-nextlink",
	buildURL: func(query string, opts *searchOptions) string {
		u := "https://en.wikipedia.org/w/index.php?search=" + url.QueryEscape(query) + "&ns0=1"
		if opts.language != "" {
			// Use language subdomain instead of en
			u = "https://" + url.QueryEscape(opts.language) + ".wikipedia.org/w/index.php?search=" + url.QueryEscape(query) + "&ns0=1"
		}
		return u
	},
}
