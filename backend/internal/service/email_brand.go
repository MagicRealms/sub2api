package service

import (
	"html"
	"strings"
)

func sunworldEmailFrame(siteName, title, content, accent string) string {
	if strings.TrimSpace(accent) == "" {
		accent = "#e8c778"
	}
	siteName = html.EscapeString(siteName)
	title = html.EscapeString(title)

	return `<!doctype html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { margin: 0; padding: 24px 12px; background: #f5f9f8; color: #30494b; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
    .button { display: inline-block; padding: 12px 20px; border-radius: 7px; background: #55a99b; color: #ffffff !important; font-weight: 700; text-decoration: none; }
    .muted { color: #738887; font-size: 13px; }
    @media only screen and (max-width: 620px) { body { padding: 0; } .sw-frame { border-radius: 0 !important; } .sw-pad { padding-left: 20px !important; padding-right: 20px !important; } }
  </style>
</head>
<body style="margin:0;padding:24px 12px;background:#f5f9f8;color:#30494b;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;">
    <tr><td align="center" style="padding:0 12px;">
      <table class="sw-frame" role="presentation" width="100%" cellspacing="0" cellpadding="0" style="width:100%;max-width:620px;border:1px solid #dfeae6;border-top:4px solid ` + accent + `;border-radius:14px;background:#ffffff;overflow:hidden;">
        <tr><td class="sw-pad" style="padding:22px 30px 24px;background:#edf7f3;color:#29484a;">
          <table role="presentation" cellspacing="0" cellpadding="0" style="border-collapse:collapse;">
            <tr>
              <td width="46" height="46" align="center" valign="middle" style="width:46px;height:46px;border:1px solid #dfeae6;border-radius:12px;background:#ffffff;">
                <img src="https://codex.magicrealms.top/sunworld-email-avatar.png" width="42" height="42" alt="太阳新世界" style="display:block;width:42px;height:42px;border:0;border-radius:10px;">
              </td>
              <td style="padding-left:12px;color:#29484a;font-size:13px;line-height:1.4;font-weight:700;">SUNWORLD<br><span style="color:#718987;font-size:11px;font-weight:500;">太阳新世界</span></td>
            </tr>
          </table>
          <h1 style="margin:20px 0 0;color:#29484a;font-size:23px;line-height:1.35;font-weight:700;">` + title + `</h1>
        </td></tr>
        <tr><td class="sw-pad" style="padding:27px 30px 29px;color:#344d4e;font-size:15px;line-height:1.75;">
          ` + content + `
        </td></tr>
        <tr><td class="sw-pad" style="padding:17px 30px;border-top:1px solid #e8efec;background:#f8fbfa;color:#738684;font-size:12px;line-height:1.65;">
          此邮件由 ` + siteName + ` 自动发送，请勿直接回复。<br>This is an automated message; please do not reply. Contact us through the support channel in your account.
        </td></tr>
      </table>
      <p style="margin:14px 0 0;color:#859694;font-size:11px;line-height:1.5;">SUNWORLD · 太阳新世界</p>
    </td></tr>
  </table>
</body>
</html>`
}
