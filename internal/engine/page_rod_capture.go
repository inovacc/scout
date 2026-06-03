package engine

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/js"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	utils2 "github.com/inovacc/scout/internal/engine/lib/utils"
)

// TriggerFavicon supports when browser in headless mode
// to trigger favicon's request. Pay attention to this
// function only supported when browser in headless mode,
// if you call it in no-headless mode, it will raise an error
// with the message "browser is no-headless".
func (p *rodPage) TriggerFavicon() error {
	// check if browser whether in headless mode
	// if not in headless mode then raise error
	if !p.browser.Context(p.ctx).isHeadless() {
		return errors.New("browser is no-headless")
	}

	_, err := p.Evaluate(evalHelper(js.TriggerFavicon).ByPromise())
	if err != nil {
		return err
	}

	return nil
}

// HandleDialog accepts or dismisses next JavaScript initiated dialog (alert, confirm, prompt, or onbeforeunload).
// Because modal dialog will block js, usually you have to trigger the dialog in another goroutine.
// For example:
//
//	wait, handle := page.MustHandleDialog()
//	go page.MustElement("button").MustClick()
//	wait()
//	handle(true, "")
func (p *rodPage) HandleDialog() (
	wait func() *proto2.PageJavascriptDialogOpening,
	handle func(*proto2.PageHandleJavaScriptDialog) error,
) {
	restore := p.EnableDomain(&proto2.PageEnable{})

	var e proto2.PageJavascriptDialogOpening

	w := p.WaitEvent(&e)

	return func() *proto2.PageJavascriptDialogOpening {
			w()
			return &e
		}, func(h *proto2.PageHandleJavaScriptDialog) error {
			defer restore()
			return h.Call(p)
		}
}

// HandleFileDialog return a functions that waits for the next file chooser dialog pops up and returns the element
// for the event.
func (p *rodPage) HandleFileDialog() (func([]string) error, error) {
	err := proto2.PageSetInterceptFileChooserDialog{Enabled: true}.Call(p)
	if err != nil {
		return nil, err
	}

	var e proto2.PageFileChooserOpened

	w := p.WaitEvent(&e)

	return func(paths []string) error {
		w()

		err := proto2.PageSetInterceptFileChooserDialog{Enabled: false}.Call(p)
		if err != nil {
			return err
		}

		return proto2.DOMSetFileInputFiles{
			Files:         utils2.AbsolutePaths(paths),
			BackendNodeID: e.BackendNodeID,
		}.Call(p)
	}, nil
}

// Screenshot captures the screenshot of current page.
func (p *rodPage) Screenshot(fullPage bool, req *proto2.PageCaptureScreenshot) ([]byte, error) {
	if req == nil {
		req = &proto2.PageCaptureScreenshot{}
	}

	if fullPage {
		metrics, err := proto2.PageGetLayoutMetrics{}.Call(p)
		if err != nil {
			return nil, err
		}

		if metrics.CSSContentSize == nil {
			return nil, errors.New("failed to get css content size")
		}

		oldView := proto2.EmulationSetDeviceMetricsOverride{}
		set := p.LoadState(&oldView)
		view := oldView
		view.Width = int(metrics.CSSContentSize.Width)
		view.Height = int(metrics.CSSContentSize.Height)

		err = p.SetViewport(&view)
		if err != nil {
			return nil, err
		}

		defer func() { // try to recover the viewport
			if !set {
				_ = proto2.EmulationClearDeviceMetricsOverride{}.Call(p)
				return
			}

			_ = p.SetViewport(&oldView)
		}()
	}

	shot, err := req.Call(p)
	if err != nil {
		return nil, err
	}

	return shot.Data, nil
}

// ScrollScreenshotOptions is the options for the ScrollScreenshot.
type ScrollScreenshotOptions struct {
	// Format (optional) Image compression format (defaults to png).
	Format proto2.PageCaptureScreenshotFormat `json:"format,omitempty"`

	// Quality (optional) Compression quality from range [0..100] (jpeg only).
	Quality *int `json:"quality,omitempty"`

	// FixedTop (optional) The number of pixels to skip from the top.
	// It is suitable for optimizing the screenshot effect when there is a fixed
	// positioning element at the top of the page.
	FixedTop float64

	// FixedBottom (optional) The number of pixels to skip from the bottom.
	FixedBottom float64

	// WaitPerScroll until no animation (default is 300ms)
	WaitPerScroll time.Duration
}

// ScrollScreenshot Scroll screenshot does not adjust the size of the viewport,
// but achieves it by scrolling and capturing screenshots in a loop, and then stitching them together.
// Note that this method also has a flaw: when there are elements with fixed
// positioning on the page (usually header navigation components),
// these elements will appear repeatedly, you can set the FixedTop parameter to optimize it.
//
// Only support png and jpeg format yet, webP is not supported because no suitable processing
// library was found in golang.
func (p *rodPage) ScrollScreenshot(opt *ScrollScreenshotOptions) ([]byte, error) {
	if opt == nil {
		opt = &ScrollScreenshotOptions{}
	}

	if opt.WaitPerScroll == 0 {
		opt.WaitPerScroll = time.Millisecond * 300
	}

	metrics, err := proto2.PageGetLayoutMetrics{}.Call(p)
	if err != nil {
		return nil, err
	}

	if metrics.CSSContentSize == nil || metrics.CSSVisualViewport == nil {
		return nil, errors.New("failed to get css content size")
	}

	viewpointHeight := metrics.CSSVisualViewport.ClientHeight
	contentHeight := metrics.CSSContentSize.Height

	var (
		scrollTop float64
		images    []utils2.ImgWithBox
	)

	for {
		clip := &proto2.PageViewport{
			X:     0,
			Y:     scrollTop,
			Width: metrics.CSSVisualViewport.ClientWidth,
			Scale: 1,
		}

		scrollY := viewpointHeight - (opt.FixedTop + opt.FixedBottom)
		if scrollTop+viewpointHeight > contentHeight {
			clip.Height = contentHeight - scrollTop
		} else {
			clip.Height = scrollY
			if scrollTop != 0 {
				clip.Y += opt.FixedTop
			}
		}

		req := &proto2.PageCaptureScreenshot{
			Format:                opt.Format,
			Quality:               opt.Quality,
			Clip:                  clip,
			FromSurface:           false,
			CaptureBeyondViewport: false,
			OptimizeForSpeed:      false,
		}

		shot, err := req.Call(p)
		if err != nil {
			return nil, err
		}

		images = append(images, utils2.ImgWithBox{Img: shot.Data})

		scrollTop += scrollY
		if scrollTop >= contentHeight {
			break
		}

		err = p.Mouse.Scroll(0, scrollY, 1)
		if err != nil {
			return nil, fmt.Errorf("scroll error: %w", err)
		}

		err = p.WaitDOMStable(opt.WaitPerScroll, 0)
		if err != nil {
			return nil, fmt.Errorf("WaitDOMStable error: %w", err)
		}
	}

	var imgOption *utils2.ImgOption
	if opt.Quality != nil {
		imgOption = &utils2.ImgOption{
			Quality: *opt.Quality,
		}
	}

	bs, err := utils2.SplicePngVertical(images, opt.Format, imgOption)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// CaptureDOMSnapshot Returns a document snapshot, including the full DOM tree of the root node
// (including iframes, template contents, and imported documents) in a flattened array,
// as well as layout and white-listed computed style information for the nodes.
// Shadow DOM in the returned DOM tree is flattened.
// `Documents` The nodes in the DOM tree. The DOMNode at index 0 corresponds to the root document.
// `Strings` Shared string table that all string properties refer to with indexes.
// Normally use `Strings` is enough.
func (p *rodPage) CaptureDOMSnapshot() (domSnapshot *proto2.DOMSnapshotCaptureSnapshotResult, err error) {
	_ = proto2.DOMSnapshotEnable{}.Call(p)

	snapshot, err := proto2.DOMSnapshotCaptureSnapshot{
		ComputedStyles:                 []string{},
		IncludePaintOrder:              true,
		IncludeDOMRects:                true,
		IncludeBlendedBackgroundColors: true,
		IncludeTextColorOpacities:      true,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// PDF prints page as PDF.
func (p *rodPage) PDF(req *proto2.PagePrintToPDF) (*StreamReader, error) {
	req.TransferMode = proto2.PagePrintToPDFTransferModeReturnAsStream

	res, err := req.Call(p)
	if err != nil {
		return nil, err
	}

	return NewStreamReader(p, res.Stream), nil
}

// GetResource content by the url. Such as image, css, html, etc.
// Use the [proto.PageGetResourceTree] to list all the resources.
func (p *rodPage) GetResource(url string) ([]byte, error) {
	res, err := proto2.PageGetResourceContent{
		FrameID: p.FrameID,
		URL:     url,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	data := res.Content

	var bin []byte
	if res.Base64Encoded {
		bin, err = base64.StdEncoding.DecodeString(data)
		utils2.E(err)
	} else {
		bin = []byte(data)
	}

	return bin, nil
}
