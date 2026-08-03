package pdf

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type ChromeConfig struct {
	ExecutablePath    string
	NoSandbox         bool
	AllowJavaScript   bool
	AllowedAssetHosts []string
}

type ChromeRenderer struct {
	allocatorCancel context.CancelFunc
	browserCancel   context.CancelFunc
	browserContext  context.Context
	allowJavaScript bool
	allowedHosts    map[string]struct{}
	closeOnce       sync.Once
}

func NewChromeRenderer(ctx context.Context, cfg ChromeConfig) (*ChromeRenderer, error) {
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	options = append(options,
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	if cfg.ExecutablePath != "" {
		options = append(options, chromedp.ExecPath(cfg.ExecutablePath))
	}
	if cfg.NoSandbox {
		options = append(options,
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-setuid-sandbox", true),
		)
	}

	allocatorContext, allocatorCancel := chromedp.NewExecAllocator(ctx, options...)
	browserContext, browserCancel := chromedp.NewContext(allocatorContext)
	if err := chromedp.Run(browserContext); err != nil {
		browserCancel()
		allocatorCancel()
		return nil, err
	}

	allowedHosts := make(map[string]struct{}, len(cfg.AllowedAssetHosts))
	for _, host := range cfg.AllowedAssetHosts {
		allowedHosts[strings.ToLower(strings.TrimSpace(host))] = struct{}{}
	}

	return &ChromeRenderer{
		allocatorCancel: allocatorCancel,
		browserCancel:   browserCancel,
		browserContext:  browserContext,
		allowJavaScript: cfg.AllowJavaScript,
		allowedHosts:    allowedHosts,
	}, nil
}

func (r *ChromeRenderer) Render(ctx context.Context, request Request) ([]byte, error) {
	options, err := request.Options.Normalize()
	if err != nil {
		return nil, err
	}

	tabContext, tabCancel := chromedp.NewContext(r.browserContext)
	defer tabCancel()
	stop := context.AfterFunc(ctx, tabCancel)
	defer stop()

	var output []byte
	actions := make([]chromedp.Action, 0, 10)
	if len(r.allowedHosts) == 0 {
		actions = append(actions,
			network.Enable(),
			network.SetBlockedURLs([]string{"http://*", "https://*", "file://*", "ftp://*", "ws://*", "wss://*"}),
		)
	} else {
		r.intercept(tabContext)
		actions = append(actions, fetch.Enable())
	}
	actions = append(actions,
		emulation.SetScriptExecutionDisabled(!r.allowJavaScript),
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(actionContext context.Context) error {
			tree, err := page.GetFrameTree().Do(actionContext)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(tree.Frame.ID, request.HTML).Do(actionContext)
		}),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if r.allowJavaScript {
		actions = append(actions, waitActions(options)...)
	} else if delay := time.Duration(options.RenderDelay); delay > 0 {
		actions = append(actions, chromedp.Sleep(delay))
	}
	actions = append(actions, chromedp.ActionFunc(func(actionContext context.Context) error {
		params := page.PrintToPDF().
			WithLandscape(options.Landscape).
			WithPrintBackground(*options.PrintBackground).
			WithPreferCSSPageSize(*options.PreferCSSPageSize).
			WithScale(options.Scale).
			WithPaperWidth(mmToInches(options.WidthMM)).
			WithPaperHeight(mmToInches(options.HeightMM)).
			WithMarginTop(mmToInches(options.Margins.TopMM)).
			WithMarginRight(mmToInches(options.Margins.RightMM)).
			WithMarginBottom(mmToInches(options.Margins.BottomMM)).
			WithMarginLeft(mmToInches(options.Margins.LeftMM)).
			WithPageRanges(options.PageRanges).
			WithHeaderTemplate(options.HeaderTemplate).
			WithFooterTemplate(options.FooterTemplate).
			WithDisplayHeaderFooter(options.HeaderTemplate != "" || options.FooterTemplate != "").
			WithGenerateTaggedPDF(options.TaggedPDF).
			WithGenerateDocumentOutline(options.DocumentOutline)
		buffer, _, err := params.Do(actionContext)
		if err != nil {
			return err
		}
		output = buffer
		return nil
	}))

	if err := chromedp.Run(tabContext, actions...); err != nil {
		return nil, err
	}
	return output, nil
}

func (r *ChromeRenderer) Ready(ctx context.Context) error {
	tabContext, tabCancel := chromedp.NewContext(r.browserContext)
	defer tabCancel()
	stop := context.AfterFunc(ctx, tabCancel)
	defer stop()
	return chromedp.Run(tabContext, chromedp.Navigate("about:blank"))
}

func (r *ChromeRenderer) Close() {
	r.closeOnce.Do(func() {
		r.browserCancel()
		r.allocatorCancel()
	})
}

func (r *ChromeRenderer) intercept(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(event any) {
		paused, ok := event.(*fetch.EventRequestPaused)
		if !ok {
			return
		}
		go r.handleRequest(ctx, paused)
	})
}

func (r *ChromeRenderer) handleRequest(ctx context.Context, event *fetch.EventRequestPaused) {
	target := chromedp.FromContext(ctx)
	executorContext := cdp.WithExecutor(ctx, target.Target)
	if r.allowURL(event.Request.URL) {
		_ = fetch.ContinueRequest(event.RequestID).Do(executorContext)
		return
	}
	_ = fetch.FailRequest(event.RequestID, network.ErrorReasonBlockedByClient).Do(executorContext)
}

func (r *ChromeRenderer) allowURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "about", "data", "blob":
		return true
	case "http", "https":
		host := strings.ToLower(parsed.Hostname())
		if ip := net.ParseIP(host); ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
			return false
		}
		_, ok := r.allowedHosts[host]
		return ok
	default:
		return false
	}
}

func waitActions(options Options) []chromedp.Action {
	actions := make([]chromedp.Action, 0, 2)
	if options.WaitUntil == "fonts" || options.WaitUntil == "assets" {
		actions = append(actions, chromedp.Evaluate(`document.fonts ? document.fonts.ready.then(() => true) : true`, nil))
	}
	if options.WaitUntil == "assets" {
		actions = append(actions, chromedp.Evaluate(`Promise.all(Array.from(document.images).map(image => image.complete ? true : new Promise(resolve => { image.addEventListener('load', resolve, { once: true }); image.addEventListener('error', resolve, { once: true }) }))).then(() => true)`, nil))
	}
	if delay := time.Duration(options.RenderDelay); delay > 0 {
		actions = append(actions, chromedp.Sleep(delay))
	}
	return actions
}
