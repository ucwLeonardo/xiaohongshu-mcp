package browser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// Browser wraps a go-rod browser with Chrome 132+ headless compatibility.
//
// Chrome 132 removed --headless=old and the new headless mode rejects
// --no-startup-window ("Multiple targets are not supported in headless mode").
// This type configures go-rod's launcher directly, setting only --headless
// without --no-startup-window.
type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

// Close closes the browser and cleans up launcher resources.
func (b *Browser) Close() {
	defer b.launcher.Cleanup()
	if err := b.browser.Close(); err != nil {
		logrus.Warnf("failed to close browser: %v", err)
	}
}

// NewPage creates a new page with stealth mode enabled.
func (b *Browser) NewPage() (*rod.Page, error) {
	return stealth.Page(b.browser)
}

type browserConfig struct {
	binPath string
}

// Option configures browser creation.
type Option func(*browserConfig)

// WithBinPath sets a custom Chrome/Chromium binary path.
func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

// maskProxyCredentials masks username and password in proxy URL for safe logging.
func maskProxyCredentials(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.User == nil {
		return proxyURL
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword("***", "***")
	} else {
		u.User = url.User("***")
	}
	return u.String()
}

// NewBrowser creates a browser instance with Chrome 132+ headless compatibility.
func NewBrowser(headless bool, options ...Option) (*Browser, error) {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	l := launcher.New().
		Set("no-sandbox")

	// Set headless WITHOUT --no-startup-window (Chrome 132+ fix).
	// go-rod's Headless(true) sets both --headless and --no-startup-window,
	// which Chrome 132+ rejects. We set --headless directly instead.
	if headless {
		l = l.Set("headless")
		l = l.Delete("no-startup-window")
	}

	if cfg.binPath != "" {
		l = l.Bin(cfg.binPath)
	}

	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	debugURL, err := l.Launch()
	if err != nil {
		l.Cleanup()
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	b := rod.New().ControlURL(debugURL)
	if err := b.Connect(); err != nil {
		l.Cleanup()
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	// Load cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cks []*proto.NetworkCookie
		if err := json.Unmarshal(data, &cks); err != nil {
			logrus.Warnf("failed to unmarshal cookies: %v", err)
		} else if err := b.SetCookies(proto.CookiesToParams(cks)); err != nil {
			logrus.Warnf("failed to set cookies: %v", err)
		} else {
			logrus.Debugf("loaded cookies from file successfully")
		}
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return &Browser{
		browser:  b,
		launcher: l,
	}, nil
}
