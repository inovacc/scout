// This file contains all query related code for Page and Element to separate the concerns.

package engine

import (
	"errors"
	"regexp"

	"github.com/inovacc/scout/internal/engine/lib/cdp"
	"github.com/inovacc/scout/internal/engine/lib/js"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/internal/engine/lib/utils"
)

// SelectorType enum.
type SelectorType string

const (
	// SelectorTypeRegex type.
	SelectorTypeRegex SelectorType = "regex"
	// SelectorTypeCSSSector type.
	SelectorTypeCSSSector SelectorType = "css-selector"
	// SelectorTypeText type.
	SelectorTypeText SelectorType = "text"
)

// Elements provides some helpers to deal with element list.
type Elements []*rodElement

// First returns the first element, if the list is empty returns nil.
func (els Elements) First() *rodElement {
	if els.Empty() {
		return nil
	}

	return els[0]
}

// Last returns the last element, if the list is empty returns nil.
func (els Elements) Last() *rodElement {
	if els.Empty() {
		return nil
	}

	return els[len(els)-1]
}

// Empty returns true if the list is empty.
func (els Elements) Empty() bool {
	return len(els) == 0
}

// Pages provides some helpers to deal with page list.
type Pages []*rodPage

// First returns the first page, if the list is empty returns nil.
func (ps Pages) First() *rodPage {
	if ps.Empty() {
		return nil
	}

	return ps[0]
}

// Last returns the last page, if the list is empty returns nil.
func (ps Pages) Last() *rodPage {
	if ps.Empty() {
		return nil
	}

	return ps[len(ps)-1]
}

// Empty returns true if the list is empty.
func (ps Pages) Empty() bool {
	return len(ps) == 0
}

// Find the page that has the specified element with the css selector.
func (ps Pages) Find(selector string) (*rodPage, error) {
	for _, page := range ps {
		has, _, err := page.Has(selector)
		if err != nil {
			return nil, err
		}

		if has {
			return page, nil
		}
	}

	return nil, &PageNotFoundError{}
}

// FindByURL returns the page that has the url that matches the jsRegex.
func (ps Pages) FindByURL(jsRegex string) (*rodPage, error) {
	for _, page := range ps {
		res, err := page.Eval(`() => location.href`)
		if err != nil {
			return nil, err
		}

		url := res.Value.String()
		if regexp.MustCompile(jsRegex).MatchString(url) {
			return page, nil
		}
	}

	return nil, &PageNotFoundError{}
}

// Has an element that matches the css selector.
func (p *rodPage) Has(selector string) (bool, *rodElement, error) {
	el, err := p.Sleeper(NotFoundSleeper).Element(selector)
	if errors.Is(err, &ElementNotFoundError{}) {
		return false, nil, nil
	}

	if err != nil {
		return false, nil, err
	}

	return true, el.Sleeper(p.sleeper), nil
}

// HasX an element that matches the XPath selector.
func (p *rodPage) HasX(selector string) (bool, *rodElement, error) {
	el, err := p.Sleeper(NotFoundSleeper).ElementX(selector)
	if errors.Is(err, &ElementNotFoundError{}) {
		return false, nil, nil
	}

	if err != nil {
		return false, nil, err
	}

	return true, el.Sleeper(p.sleeper), nil
}

// HasR an element that matches the css selector and its display text matches the jsRegex.
func (p *rodPage) HasR(selector, jsRegex string) (bool, *rodElement, error) {
	el, err := p.Sleeper(NotFoundSleeper).ElementR(selector, jsRegex)
	if errors.Is(err, &ElementNotFoundError{}) {
		return false, nil, nil
	}

	if err != nil {
		return false, nil, err
	}

	return true, el.Sleeper(p.sleeper), nil
}

// Element retries until an element in the page that matches the CSS selector, then returns
// the matched element.
func (p *rodPage) Element(selector string) (*rodElement, error) {
	return p.ElementByJS(evalHelper(js.Element, selector))
}

// ElementR retries until an element in the page that matches the css selector and it's text matches the jsRegex,
// then returns the matched element.
func (p *rodPage) ElementR(selector, jsRegex string) (*rodElement, error) {
	return p.ElementByJS(evalHelper(js.ElementR, selector, jsRegex))
}

// ElementX retries until an element in the page that matches one of the XPath selectors, then returns
// the matched element.
func (p *rodPage) ElementX(xPath string) (*rodElement, error) {
	return p.ElementByJS(evalHelper(js.ElementX, xPath))
}

// ElementByJS returns the element from the return value of the js function.
// If sleeper is nil, no retry will be performed.
// By default, it will retry until the js function doesn't return null.
// To customize the retry logic, check the examples of Page.Sleeper.
func (p *rodPage) ElementByJS(opts *EvalOptions) (*rodElement, error) {
	var (
		res *proto2.RuntimeRemoteObject
		err error
	)

	removeTrace := func() {}
	err = utils.Retry(p.ctx, p.sleeper(), func() (bool, error) {
		remove := p.tryTraceQuery(opts)

		removeTrace()
		removeTrace = remove

		res, err = p.Evaluate(opts.ByObject())
		if err != nil {
			return true, err
		}

		if res.Type == proto2.RuntimeRemoteObjectTypeObject && res.Subtype == proto2.RuntimeRemoteObjectSubtypeNull {
			return false, nil
		}

		return true, nil
	})

	removeTrace()

	if err != nil {
		return nil, err
	}

	if res.Subtype != proto2.RuntimeRemoteObjectSubtypeNode {
		return nil, &ExpectElementError{res}
	}

	return p.ElementFromObject(res)
}

// Elements returns all elements that match the css selector.
func (p *rodPage) Elements(selector string) (Elements, error) {
	return p.ElementsByJS(evalHelper(js.Elements, selector))
}

// ElementsX returns all elements that match the XPath selector.
func (p *rodPage) ElementsX(xpath string) (Elements, error) {
	return p.ElementsByJS(evalHelper(js.ElementsX, xpath))
}

// ElementsByJS returns the elements from the return value of the js.
func (p *rodPage) ElementsByJS(opts *EvalOptions) (Elements, error) {
	res, err := p.Evaluate(opts.ByObject())
	if err != nil {
		return nil, err
	}

	if res.Subtype != proto2.RuntimeRemoteObjectSubtypeArray {
		return nil, &ExpectElementsError{res}
	}

	defer func() { err = p.Release(res) }()

	list, err := proto2.RuntimeGetProperties{
		ObjectID:      res.ObjectID,
		OwnProperties: true,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	elemList := Elements{}

	for _, obj := range list.Result {
		if obj.Name == "__proto__" || obj.Name == "length" {
			continue
		}

		val := obj.Value

		if val.Subtype != proto2.RuntimeRemoteObjectSubtypeNode {
			return nil, &ExpectElementsError{val}
		}

		el, err := p.ElementFromObject(val)
		if err != nil {
			return nil, err
		}

		elemList = append(elemList, el)
	}

	return elemList, err
}

// Search for the given query in the DOM tree until the result count is not zero, before that it will keep retrying.
// The query can be plain text or css selector or xpath.
// It will search nested iframes and shadow doms too.
func (p *rodPage) Search(query string) (*rodSearchResult, error) {
	sr := &rodSearchResult{
		page:    p,
		restore: p.EnableDomain(proto2.DOMEnable{}),
	}

	err := utils.Retry(p.ctx, p.sleeper(), func() (bool, error) {
		if sr.DOMPerformSearchResult != nil {
			_ = proto2.DOMDiscardSearchResults{SearchID: sr.SearchID}.Call(p)
		}

		res, err := proto2.DOMPerformSearch{
			Query:                     query,
			IncludeUserAgentShadowDOM: true,
		}.Call(p)
		if err != nil {
			return true, err
		}

		sr.DOMPerformSearchResult = res

		if res.ResultCount == 0 {
			return false, nil
		}

		result, err := proto2.DOMGetSearchResults{
			SearchID:  res.SearchID,
			FromIndex: 0,
			ToIndex:   1,
		}.Call(p)
		if err != nil {
			// when the page is still loading the search result is not ready
			if errors.Is(err, cdp.ErrCtxNotFound) ||
				errors.Is(err, cdp.ErrSearchSessionNotFound) {
				return false, nil
			}

			return true, err
		}

		id := result.NodeIDs[0]

		// TODO: This is definitely a bad design of cdp, hope they can optimize it in the future.
		// It's unnecessary to ask the user to explicitly call it.
		//
		// When the id is zero, it means the proto.DOMDocumentUpdated has fired which will
		// invalidate all the existing NodeID. We have to call proto.DOMGetDocument
		// to reset the remote browser's tracker.
		if id == 0 {
			_, _ = proto2.DOMGetDocument{}.Call(p)
			return false, nil
		}

		el, err := p.ElementFromNode(&proto2.DOMNode{NodeID: id})
		if err != nil {
			return true, err
		}

		sr.First = el

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return sr, nil
}

// SearchResult handler.
type rodSearchResult struct {
	*proto2.DOMPerformSearchResult

	page    *rodPage
	restore func()

	// First element in the search result
	First *rodElement
}

// Get l elements at the index of i from the remote search result.
func (s *rodSearchResult) Get(i, l int) (Elements, error) {
	result, err := proto2.DOMGetSearchResults{
		SearchID:  s.SearchID,
		FromIndex: i,
		ToIndex:   i + l,
	}.Call(s.page)
	if err != nil {
		return nil, err
	}

	list := Elements{}

	for _, id := range result.NodeIDs {
		el, err := s.page.ElementFromNode(&proto2.DOMNode{NodeID: id})
		if err != nil {
			return nil, err
		}

		list = append(list, el)
	}

	return list, nil
}

// All returns all elements.
func (s *rodSearchResult) All() (Elements, error) {
	return s.Get(0, s.ResultCount)
}

// Release the remote search result.
func (s *rodSearchResult) Release() {
	s.restore()
	_ = proto2.DOMDiscardSearchResults{SearchID: s.SearchID}.Call(s.page)
}

type raceBranch struct {
	condition func(*rodPage) (*rodElement, error)
	callback  func(*rodElement) error
}

// RaceContext stores the branches to race.
type RaceContext struct {
	page     *rodPage
	branches []*raceBranch
}

// Race creates a context to race selectors.
func (p *rodPage) Race() *RaceContext {
	return &RaceContext{page: p}
}

// ElementFunc takes a custom function to determine race success.
func (rc *RaceContext) ElementFunc(fn func(*rodPage) (*rodElement, error)) *RaceContext {
	rc.branches = append(rc.branches, &raceBranch{
		condition: fn,
	})

	return rc
}

// Element is similar to [Page.Element].
func (rc *RaceContext) Element(selector string) *RaceContext {
	return rc.ElementFunc(func(p *rodPage) (*rodElement, error) {
		return p.Element(selector)
	})
}

// ElementX is similar to [Page.ElementX].
func (rc *RaceContext) ElementX(selector string) *RaceContext {
	return rc.ElementFunc(func(p *rodPage) (*rodElement, error) {
		return p.ElementX(selector)
	})
}

// ElementR is similar to [Page.ElementR].
func (rc *RaceContext) ElementR(selector, regex string) *RaceContext {
	return rc.ElementFunc(func(p *rodPage) (*rodElement, error) {
		return p.ElementR(selector, regex)
	})
}

// ElementByJS is similar to [Page.ElementByJS].
func (rc *RaceContext) ElementByJS(opts *EvalOptions) *RaceContext {
	return rc.ElementFunc(func(p *rodPage) (*rodElement, error) {
		return p.ElementByJS(opts)
	})
}

// Search is similar to [Page.Search].
func (rc *RaceContext) Search(query string) *RaceContext {
	return rc.ElementFunc(func(p *rodPage) (*rodElement, error) {
		res, err := p.Search(query)
		if err != nil {
			return nil, err
		}

		res.Release()

		return res.First, nil
	})
}

// Handle adds a callback function to the most recent chained selector.
// The callback function is run, if the corresponding selector is
// present first, in the Race condition.
func (rc *RaceContext) Handle(callback func(*rodElement) error) *RaceContext {
	rc.branches[len(rc.branches)-1].callback = callback
	return rc
}

// Do the race.
func (rc *RaceContext) Do() (*rodElement, error) {
	var el *rodElement

	err := utils.Retry(rc.page.ctx, rc.page.sleeper(), func() (stop bool, err error) {
		for _, branch := range rc.branches {
			bEl, err := branch.condition(rc.page.Sleeper(NotFoundSleeper))
			if err == nil {
				el = bEl.Sleeper(rc.page.sleeper)

				if branch.callback != nil {
					err = branch.callback(el)
				}

				return true, err
			} else if !errors.Is(err, &ElementNotFoundError{}) {
				return true, err
			}
		}

		return
	})

	return el, err
}

// Has an element that matches the css selector.
func (el *rodElement) Has(selector string) (bool, *rodElement, error) {
	el, err := el.Element(selector)
	if errors.Is(err, &ElementNotFoundError{}) {
		return false, nil, nil
	}

	return err == nil, el, err
}

// HasX an element that matches the XPath selector.
func (el *rodElement) HasX(selector string) (bool, *rodElement, error) {
	el, err := el.ElementX(selector)
	if errors.Is(err, &ElementNotFoundError{}) {
		return false, nil, nil
	}

	return err == nil, el, err
}

// HasR returns true if a child element that matches the css selector and its text matches the jsRegex.
func (el *rodElement) HasR(selector, jsRegex string) (bool, *rodElement, error) {
	el, err := el.ElementR(selector, jsRegex)
	if errors.Is(err, &ElementNotFoundError{}) {
		return false, nil, nil
	}

	return err == nil, el, err
}

// Element returns the first child that matches the css selector.
func (el *rodElement) Element(selector string) (*rodElement, error) {
	return el.ElementByJS(evalHelper(js.Element, selector))
}

// ElementR returns the first child element that matches the css selector and its text matches the jsRegex.
func (el *rodElement) ElementR(selector, jsRegex string) (*rodElement, error) {
	return el.ElementByJS(evalHelper(js.ElementR, selector, jsRegex))
}

// ElementX returns the first child that matches the XPath selector.
func (el *rodElement) ElementX(xPath string) (*rodElement, error) {
	return el.ElementByJS(evalHelper(js.ElementX, xPath))
}

// ElementByJS returns the element from the return value of the js.
func (el *rodElement) ElementByJS(opts *EvalOptions) (*rodElement, error) {
	e, err := el.page.Context(el.ctx).Sleeper(NotFoundSleeper).ElementByJS(opts.This(el.Object))
	if err != nil {
		return nil, err
	}

	return e.Sleeper(el.sleeper), nil
}

// Parent returns the parent element in the DOM tree.
func (el *rodElement) Parent() (*rodElement, error) {
	return el.ElementByJS(Eval(`() => this.parentElement`))
}

// Parents that match the selector.
func (el *rodElement) Parents(selector string) (Elements, error) {
	return el.ElementsByJS(evalHelper(js.Parents, selector))
}

// Next returns the next sibling element in the DOM tree.
func (el *rodElement) Next() (*rodElement, error) {
	return el.ElementByJS(Eval(`() => this.nextElementSibling`))
}

// Previous returns the previous sibling element in the DOM tree.
func (el *rodElement) Previous() (*rodElement, error) {
	return el.ElementByJS(Eval(`() => this.previousElementSibling`))
}

// Elements returns all elements that match the css selector.
func (el *rodElement) Elements(selector string) (Elements, error) {
	return el.ElementsByJS(evalHelper(js.Elements, selector))
}

// ElementsX returns all elements that match the XPath selector.
func (el *rodElement) ElementsX(xpath string) (Elements, error) {
	return el.ElementsByJS(evalHelper(js.ElementsX, xpath))
}

// ElementsByJS returns the elements from the return value of the js.
func (el *rodElement) ElementsByJS(opts *EvalOptions) (Elements, error) {
	return el.page.Context(el.ctx).ElementsByJS(opts.This(el.Object))
}
