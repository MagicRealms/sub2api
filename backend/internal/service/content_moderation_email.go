package service

import (
	"fmt"
	"html"
	"strings"
	"time"
)

func buildContentModerationViolationEmailBody(siteName string, log *ContentModerationLog, cfg *ContentModerationConfig) string {
	if log == nil {
		return ""
	}
	userName := strings.TrimSpace(log.UserEmail)
	if userName == "" && log.UserID != nil {
		userName = fmt.Sprintf("UID %d", *log.UserID)
	}
	threshold := cfg.BanThreshold
	if threshold <= 0 {
		threshold = defaultContentModerationBanThreshold
	}
	statusBlock := ""
	if log.AutoBanned {
		statusBlock = `<p style="margin-top:20px;padding:14px 16px;border-left:3px solid #b65d50;background:#fff5f1;color:#87483f;font-weight:700;">账户当前处于封禁状态，所有 API 请求将被拒绝。</p>`
	}
	content := fmt.Sprintf(`<p>用户 <strong>%s</strong>，您的 API 请求触发了内容审核策略。请查看本次事件信息。</p>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;margin-top:18px;">
  <tr><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;">触发时间</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;text-align:right;">%s</td></tr>
  <tr><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;">所属分组</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;text-align:right;">%s</td></tr>
  <tr><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;">命中类别 / 分数</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;text-align:right;">%s / %.3f</td></tr>
  <tr><td style="padding:10px 12px;color:#687b79;">累计触发</td><td style="padding:10px 12px;text-align:right;">%d 次（封禁阈值 %d）</td></tr>
</table>%s
<p class="muted">请检查请求内容；如需协助或认为此结果有误，请通过平台支持渠道联系管理员。</p>`,
		html.EscapeString(userName),
		html.EscapeString(time.Now().Format("2006-01-02 15:04:05")),
		html.EscapeString(defaultContentModerationString(log.GroupName, "-")),
		html.EscapeString(defaultContentModerationString(log.HighestCategory, "-")),
		log.HighestScore,
		log.ViolationCount,
		threshold,
		statusBlock,
	)
	return sunworldEmailFrame(siteName, "账户风控提醒", content, "#b65d50")
}

func buildContentModerationAccountDisabledEmailBody(siteName string, log *ContentModerationLog, cfg *ContentModerationConfig) string {
	if log == nil {
		return ""
	}
	userName := strings.TrimSpace(log.UserEmail)
	if userName == "" && log.UserID != nil {
		userName = fmt.Sprintf("UID %d", *log.UserID)
	}
	threshold := cfg.BanThreshold
	if threshold <= 0 {
		threshold = defaultContentModerationBanThreshold
	}
	content := fmt.Sprintf(`<p>用户 <strong>%s</strong>，您的账户因在统计周期内多次触发内容审核规则，已被系统自动禁用。</p>
<h2 style="margin:22px 0 8px;color:#284a47;font-size:16px;">封禁详情</h2>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;margin-top:18px;">
  <tr><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;">禁用时间</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;text-align:right;">%s</td></tr>
  <tr><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;">所属分组</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;text-align:right;">%s</td></tr>
  <tr><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;">命中类别 / 分数</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;text-align:right;">%s / %.3f</td></tr>
  <tr><td style="padding:10px 12px;color:#687b79;">累计触发</td><td style="padding:10px 12px;text-align:right;">%d 次（阈值 %d）</td></tr>
</table>
<p style="margin-top:20px;padding:14px 16px;border-left:3px solid #b65d50;background:#fff5f1;color:#87483f;font-weight:700;">账户当前处于封禁状态，所有 API 请求将被拒绝</p>
<p class="muted">如需申诉或申请恢复，请通过平台支持渠道联系管理员。</p>`,
		html.EscapeString(userName),
		html.EscapeString(time.Now().Format("2006-01-02 15:04:05")),
		html.EscapeString(defaultContentModerationString(log.GroupName, "-")),
		html.EscapeString(defaultContentModerationString(log.HighestCategory, "-")),
		log.HighestScore,
		log.ViolationCount,
		threshold,
	)
	return sunworldEmailFrame(siteName, "账户已被自动禁用", content, "#a95147")
}

func defaultContentModerationString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// buildCyberPolicyNoticeEmailBody 是 cyber_policy 通知邮件的内置兜底正文。
func buildCyberPolicyNoticeEmailBody(siteName string, log *ContentModerationLog) string {
	if log == nil {
		return ""
	}
	userName := strings.TrimSpace(log.UserEmail)
	if userName == "" && log.UserID != nil {
		userName = fmt.Sprintf("UID %d", *log.UserID)
	}
	content := fmt.Sprintf(`<p>用户 <strong>%s</strong>，您的请求被上游服务商的网络安全策略拦截。</p>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="border-collapse:collapse;margin-top:18px;table-layout:fixed;">
  <tr><td style="width:120px;padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;vertical-align:top;">触发时间</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;overflow-wrap:anywhere;">%s</td></tr>
  <tr><td style="width:120px;padding:10px 12px;border-bottom:1px solid #e5eeea;color:#687b79;vertical-align:top;">模型</td><td style="padding:10px 12px;border-bottom:1px solid #e5eeea;overflow-wrap:anywhere;">%s</td></tr>
  <tr><td style="width:120px;padding:10px 12px;color:#687b79;vertical-align:top;">上游说明</td><td style="padding:10px 12px;overflow-wrap:anywhere;word-break:break-all;white-space:pre-wrap;">%s</td></tr>
</table>
<p class="muted">如认为系误判，可调整请求内容后重试，或联系平台支持。</p>`,
		html.EscapeString(userName),
		html.EscapeString(log.CreatedAt.Format("2006-01-02 15:04:05")),
		html.EscapeString(defaultContentModerationString(log.Model, "-")),
		html.EscapeString(defaultContentModerationString(log.Error, "-")),
	)
	return sunworldEmailFrame(siteName, "网络安全策略提醒", content, "#b65d50")
}
