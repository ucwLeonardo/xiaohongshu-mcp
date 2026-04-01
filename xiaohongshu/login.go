package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	time.Sleep(1 * time.Second)

	exists, _, err := pp.Has(`.main-container .user .link-wrapper .channel`)
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}

	if !exists {
		return false, errors.Wrap(err, "login status element not found")
	}

	return true, nil
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists {
		// 已经登录，直接返回
		return nil
	}

	// 等待扫码成功提示或者登录完成
	// 这里我们等待登录成功的元素出现，这样更简单可靠
	pp.MustElement(".main-container .user .link-wrapper .channel")

	return nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists {
		return "", true, nil
	}

	// 获取二维码图片
	src, err := pp.MustElement(".login-container .qrcode-img").Attribute("src")
	if err != nil {
		return "", false, errors.Wrap(err, "get qrcode src failed")
	}
	if src == nil || len(*src) == 0 {
		return "", false, errors.New("qrcode src is empty")
	}

	return *src, false, nil
}

func (a *LoginAction) WaitForLogin(ctx context.Context) bool {
	pp := a.page.Context(ctx)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			el, err := pp.Element(".main-container .user .link-wrapper .channel")
			if err == nil && el != nil {
				return true
			}
		}
	}
}

// SubmitVerificationCode fills the SMS verification code input on the
// login page and clicks the submit button. This is used when XHS requires
// additional SMS verification after QR code scan.
func (a *LoginAction) SubmitVerificationCode(ctx context.Context, code string) error {
	pp := a.page.Context(ctx)
	start := time.Now()
	logrus.Infof("[VerifyCode] START code_len=%d page_url=%s", len(code), a.page.MustInfo().URL)

	// Try multiple CSS selectors for the verification code input
	inputSelectors := []string{
		`input[placeholder*="验证码"]`,
		`input[placeholder*="code"]`,
		`input[type="tel"]`,
		`input[type="number"]`,
		`input[type="text"]`,
	}

	var filled bool
	for _, sel := range inputSelectors {
		logrus.Infof("[VerifyCode] trying input selector: %s", sel)
		el, err := pp.Timeout(2 * time.Second).Element(sel)
		if err != nil || el == nil {
			logrus.Infof("[VerifyCode]   not found or error: %v", err)
			continue
		}
		logrus.Infof("[VerifyCode]   FOUND, filling code")
		_ = el.SelectAllText()
		if err := el.Input(code); err != nil {
			logrus.Warnf("[VerifyCode]   input failed: %v", err)
			continue
		}
		filled = true
		break
	}

	if !filled {
		logrus.Errorf("[VerifyCode] FAIL no input found, duration=%dms", time.Since(start).Milliseconds())
		return errors.New("未找到验证码输入框")
	}

	// Primary approach: press Enter to submit (works regardless of button type/text)
	logrus.Infof("[VerifyCode] pressing Enter to submit")
	if err := pp.Keyboard.Press(input.Enter); err != nil {
		logrus.Warnf("[VerifyCode] Enter key failed: %v", err)
	} else {
		logrus.Infof("[VerifyCode] DONE pressed Enter, duration=%dms", time.Since(start).Milliseconds())
		return nil
	}

	// Fallback: try to click submit buttons by text
	buttonTexts := []string{"确定", "确认", "登录", "验证", "提交"}
	for _, text := range buttonTexts {
		logrus.Infof("[VerifyCode] trying button text: %s", text)
		// Search all clickable elements, not just <button>
		el, err := pp.Timeout(1 * time.Second).ElementR("*", text)
		if err != nil || el == nil {
			continue
		}
		if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
			logrus.Warnf("[VerifyCode]   click failed: %v", err)
			continue
		}
		logrus.Infof("[VerifyCode] DONE clicked element=%q duration=%dms", text, time.Since(start).Milliseconds())
		return nil
	}

	logrus.Warnf("[VerifyCode] DONE code filled but no submit method worked, duration=%dms", time.Since(start).Milliseconds())
	return nil
}
