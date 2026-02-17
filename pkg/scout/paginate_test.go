package scout

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(paginateTestRoutes)
}

func paginateTestRoutes(mux *http.ServeMux) {
	// Paginated products with next button
	for i := 1; i <= 3; i++ {
		page := i
		mux.HandleFunc(fmt.Sprintf("/products-page%d", page), func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")

			nextLink := ""
			if page < 3 {
				nextLink = fmt.Sprintf(`<a id="next" href="/products-page%d">Next</a>`, page+1)
			}

			_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Products Page %d</title></head>
<body>
<div class="item"><span class="name">Item %dA</span><span class="price">%d0</span></div>
<div class="item"><span class="name">Item %dB</span><span class="price">%d5</span></div>
%s
</body></html>`, page, page, page, page, page, nextLink)
		})
	}

	// URL-pattern pagination
	mux.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		page := r.URL.Query().Get("page")
		if page == "" {
			page = "1"
		}

		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Products</title></head>
<body>
<div class="item"><span class="name">P%s-A</span><span class="price">10</span></div>
<div class="item"><span class="name">P%s-B</span><span class="price">20</span></div>
</body></html>`, page, page)
	})

	// Infinite scroll simulation
	mux.HandleFunc("/infinite", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Infinite</title></head>
<body>
<div id="container">
<div class="item"><span class="name">Init1</span><span class="price">10</span></div>
<div class="item"><span class="name">Init2</span><span class="price">20</span></div>
</div>
<script>
let count = 0;
window.addEventListener('scroll', () => {
  if (count < 2 && (window.innerHeight + window.scrollY) >= document.body.offsetHeight - 10) {
    count++;
    const container = document.getElementById('container');
    const div = document.createElement('div');
    div.className = 'item';
    const nameSpan = document.createElement('span');
    nameSpan.className = 'name';
    nameSpan.textContent = 'Scroll' + count;
    const priceSpan = document.createElement('span');
    priceSpan.className = 'price';
    priceSpan.textContent = String(count * 10);
    div.appendChild(nameSpan);
    div.appendChild(priceSpan);
    container.appendChild(div);
  }
});
</script>
</body></html>`)
	})

	// Load more button
	mux.HandleFunc("/load-more", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Load More</title></head>
<body>
<div id="container">
<div class="item"><span class="name">Start1</span><span class="price">10</span></div>
</div>
<button id="load-more" onclick="
  var c = document.getElementById('container');
  var items = c.querySelectorAll('.item').length;
  if (items < 3) {
    var d = document.createElement('div');
    d.className = 'item';
    var ns = document.createElement('span');
    ns.className = 'name';
    ns.textContent = 'More' + items;
    var ps = document.createElement('span');
    ps.className = 'price';
    ps.textContent = String(items * 10);
    d.appendChild(ns);
    d.appendChild(ps);
    c.appendChild(d);
  } else {
    document.getElementById('load-more').style.display = 'none';
  }
">Load More</button>
</body></html>`)
	})
}

type testItem struct {
	Name  string `scout:"span.name"`
	Price int    `scout:"span.price"`
}

func TestPaginateByURL(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	items, err := PaginateByURL[testItem](b, func(page int) string {
		return fmt.Sprintf("%s/api/products?page=%d", srv.URL, page)
	}, WithPaginateMaxPages(3), WithPaginateDelay(100*time.Millisecond))
	if err != nil {
		t.Fatalf("PaginateByURL() error: %v", err)
	}

	if len(items) == 0 {
		t.Error("PaginateByURL() returned no items")
	}

	// Should have items from multiple pages
	t.Logf("PaginateByURL() collected %d items", len(items))

	for i, item := range items {
		t.Logf("  [%d] Name=%q Price=%d", i, item.Name, item.Price)
	}
}

func TestPaginateByClick(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/products-page1")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	items, err := PaginateByClick[testItem](page, "#next",
		WithPaginateMaxPages(5),
		WithPaginateDelay(200*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("PaginateByClick() error: %v", err)
	}

	if len(items) == 0 {
		t.Error("PaginateByClick() returned no items")
	}

	t.Logf("PaginateByClick() collected %d items", len(items))
}

func TestPaginateDedup(t *testing.T) {
	items := []testItem{
		{Name: "A", Price: 10},
		{Name: "B", Price: 20},
		{Name: "A", Price: 10},
		{Name: "C", Price: 30},
	}

	seen := make(map[string]bool)
	result := dedup(items, seen, "Name")

	if len(result) != 3 {
		t.Errorf("dedup returned %d items, want 3", len(result))
	}
}

func TestPaginateOptions(t *testing.T) {
	o := paginateDefaults()

	if o.maxPages != 10 {
		t.Errorf("default maxPages = %d, want 10", o.maxPages)
	}

	if o.delay != 500*time.Millisecond {
		t.Errorf("default delay = %v, want 500ms", o.delay)
	}

	WithPaginateMaxPages(5)(o)

	if o.maxPages != 5 {
		t.Errorf("maxPages = %d, want 5", o.maxPages)
	}

	WithPaginateDelay(1 * time.Second)(o)

	if o.delay != 1*time.Second {
		t.Errorf("delay = %v, want 1s", o.delay)
	}

	WithPaginateDedup("Name")(o)

	if o.dedupField != "Name" {
		t.Errorf("dedupField = %q, want Name", o.dedupField)
	}

	WithPaginateStopOnEmpty()(o)

	if !o.stopOnEmpty {
		t.Error("stopOnEmpty should be true")
	}
}

func TestPaginateByScroll(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/infinite")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	items, err := PaginateByScroll[testItem](page, ".item",
		WithPaginateMaxPages(5),
		WithPaginateDelay(500*time.Millisecond),
		WithPaginateStopOnEmpty(),
	)
	if err != nil {
		t.Fatalf("PaginateByScroll() error: %v", err)
	}

	if len(items) == 0 {
		t.Error("PaginateByScroll() returned no items")
	}

	t.Logf("PaginateByScroll() collected %d items", len(items))
}

func TestPaginateByLoadMore(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/load-more")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	items, err := PaginateByLoadMore[testItem](page, "#load-more",
		WithPaginateMaxPages(5),
		WithPaginateDelay(300*time.Millisecond),
		WithPaginateStopOnEmpty(),
	)
	if err != nil {
		t.Fatalf("PaginateByLoadMore() error: %v", err)
	}

	if len(items) == 0 {
		t.Error("PaginateByLoadMore() returned no items")
	}

	t.Logf("PaginateByLoadMore() collected %d items", len(items))
}
