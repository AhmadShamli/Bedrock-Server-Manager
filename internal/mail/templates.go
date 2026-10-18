package mail

import (
	"bytes"
	"fmt"
	"html"
	"strings"
)

func renderBaseHTML(title, previewText, contentHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <!--[if mso]>
  <style type="text/css">
    body, table, td {font-family: Arial, Helvetica, sans-serif !important;}
  </style>
  <![endif]-->
</head>
<body style="margin: 0; padding: 0; background-color: #090d16; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #f3f4f6; -webkit-font-smoothing: antialiased;">
  <div style="display: none; max-height: 0px; overflow: hidden; opacity: 0;">
    %s
  </div>
  <table width="100%%" border="0" cellpadding="0" cellspacing="0" style="background-color: #090d16; padding: 32px 16px;">
    <tr>
      <td align="center">
        <table width="100%%" border="0" cellpadding="0" cellspacing="0" style="max-width: 580px; background-color: #111827; border: 1px solid #1f293d; border-radius: 12px; overflow: hidden; box-shadow: 0 10px 25px rgba(0,0,0,0.5);">
          <!-- Header -->
          <tr>
            <td style="padding: 24px 32px; background-color: #131c2e; border-bottom: 1px solid #1f293d;">
              <table width="100%%" border="0" cellpadding="0" cellspacing="0">
                <tr>
                  <td align="left">
                    <span style="font-size: 18px; font-weight: bold; letter-spacing: -0.5px; color: #10b981; font-family: monospace;">⚡ BEDROCK SERVER MANAGER</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Main Content -->
          <tr>
            <td style="padding: 32px;">
              %s
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 20px 32px; background-color: #0d131f; border-top: 1px solid #1f293d; text-align: center; font-size: 12px; color: #6b7280; font-family: monospace;">
              Bedrock Server Manager &bull; Automated System Notification
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, html.EscapeString(title), html.EscapeString(previewText), contentHTML)
}

// PasswordResetTemplate generates HTML and plaintext bodies for password reset.
func PasswordResetTemplate(username, resetURL string) (string, string) {
	title := "Reset Your Password"
	preview := "Instructions to reset your Bedrock Server Manager account password."

	contentHTML := fmt.Sprintf(`
      <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 700; color: #f9fafb;">Reset Your Password</h2>
      <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #9ca3af;">
        Hello <strong style="color: #e5e7eb;">%s</strong>,
      </p>
      <p style="margin: 0 0 24px 0; font-size: 14px; line-height: 1.6; color: #9ca3af;">
        We received a request to reset the password for your Bedrock Server Manager account. Click the button below to choose a new password:
      </p>
      <div style="text-align: center; margin: 32px 0;">
        <a href="%s" style="background-color: #10b981; color: #042f2e; font-weight: 700; font-size: 14px; text-decoration: none; padding: 12px 28px; border-radius: 8px; display: inline-block; font-family: monospace;">RESET PASSWORD</a>
      </div>
      <p style="margin: 0 0 16px 0; font-size: 12px; line-height: 1.5; color: #6b7280;">
        This password reset link will expire in <strong style="color: #9ca3af;">15 minutes</strong> and can only be used once. If you did not make this request, you can safely ignore this email; your password will remain unchanged.
      </p>
      <div style="margin-top: 24px; padding-top: 16px; border-top: 1px solid #1f293d;">
        <p style="margin: 0; font-size: 11px; line-height: 1.4; color: #6b7280; word-break: break-all;">
          If the button does not work, copy and paste this URL into your browser:<br>
          <a href="%s" style="color: #10b981; text-decoration: none;">%s</a>
        </p>
      </div>`, html.EscapeString(username), html.EscapeString(resetURL), html.EscapeString(resetURL), html.EscapeString(resetURL))

	textBody := fmt.Sprintf("Reset Your Password - Bedrock Server Manager\n\n"+
		"Hello %s,\n\n"+
		"We received a request to reset your password. Use the link below to set a new password:\n\n"+
		"%s\n\n"+
		"This link will expire in 15 minutes. If you did not request this, please ignore this email.\n",
		username, resetURL)

	return renderBaseHTML(title, preview, contentHTML), textBody
}

// EmailVerificationTemplate generates HTML and plaintext bodies for email verification.
func EmailVerificationTemplate(username, verifyURL string) (string, string) {
	title := "Verify Your Email Address"
	preview := "Confirm your email address to activate your Bedrock Server Manager account."

	contentHTML := fmt.Sprintf(`
      <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 700; color: #f9fafb;">Verify Your Email Address</h2>
      <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #9ca3af;">
        Welcome to Bedrock Server Manager, <strong style="color: #e5e7eb;">%s</strong>!
      </p>
      <p style="margin: 0 0 24px 0; font-size: 14px; line-height: 1.6; color: #9ca3af;">
        Please verify your email address to complete your registration and activate your dashboard access:
      </p>
      <div style="text-align: center; margin: 32px 0;">
        <a href="%s" style="background-color: #10b981; color: #042f2e; font-weight: 700; font-size: 14px; text-decoration: none; padding: 12px 28px; border-radius: 8px; display: inline-block; font-family: monospace;">VERIFY EMAIL ADDRESS</a>
      </div>
      <p style="margin: 0 0 16px 0; font-size: 12px; line-height: 1.5; color: #6b7280;">
        This verification link will expire in <strong style="color: #9ca3af;">24 hours</strong>. If you did not register for an account, please disregard this message.
      </p>
      <div style="margin-top: 24px; padding-top: 16px; border-top: 1px solid #1f293d;">
        <p style="margin: 0; font-size: 11px; line-height: 1.4; color: #6b7280; word-break: break-all;">
          Direct Link:<br>
          <a href="%s" style="color: #10b981; text-decoration: none;">%s</a>
        </p>
      </div>`, html.EscapeString(username), html.EscapeString(verifyURL), html.EscapeString(verifyURL), html.EscapeString(verifyURL))

	textBody := fmt.Sprintf("Verify Your Email Address - Bedrock Server Manager\n\n"+
		"Welcome %s,\n\n"+
		"Please verify your email address using the link below:\n\n"+
		"%s\n\n"+
		"This link will expire in 24 hours.\n",
		username, verifyURL)

	return renderBaseHTML(title, preview, contentHTML), textBody
}

// WelcomeEmailTemplate generates HTML and plaintext bodies for account welcome.
func WelcomeEmailTemplate(username, planName, dashboardURL string) (string, string) {
	title := "Welcome to Bedrock Server Manager"
	preview := fmt.Sprintf("Your account is ready. Welcome to Bedrock Server Manager, %s!", username)

	contentHTML := fmt.Sprintf(`
      <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 700; color: #f9fafb;">Welcome to the Platform!</h2>
      <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #9ca3af;">
        Hello <strong style="color: #e5e7eb;">%s</strong>,
      </p>
      <p style="margin: 0 0 24px 0; font-size: 14px; line-height: 1.6; color: #9ca3af;">
        Your Bedrock Server Manager account has been successfully created and configured with the <strong style="color: #10b981;">%s</strong> plan.
      </p>
      <div style="text-align: center; margin: 32px 0;">
        <a href="%s" style="background-color: #10b981; color: #042f2e; font-weight: 700; font-size: 14px; text-decoration: none; padding: 12px 28px; border-radius: 8px; display: inline-block; font-family: monospace;">ACCESS DASHBOARD</a>
      </div>`, html.EscapeString(username), html.EscapeString(planName), html.EscapeString(dashboardURL))

	textBody := fmt.Sprintf("Welcome to Bedrock Server Manager\n\n"+
		"Hello %s,\n\n"+
		"Your account is now ready with plan: %s.\n"+
		"Login at: %s\n",
		username, planName, dashboardURL)

	return renderBaseHTML(title, preview, contentHTML), textBody
}

// SystemAlertTemplate generates HTML and plaintext bodies for administrative alerts.
func SystemAlertTemplate(alertType, alertTitle, alertDescription string, fields map[string]string, dashboardURL string) (string, string) {
	preview := fmt.Sprintf("[%s] %s", alertType, alertTitle)

	var fieldsHTML bytes.Buffer
	var fieldsText strings.Builder

	if len(fields) > 0 {
		fieldsHTML.WriteString(`<table width="100%" border="0" cellpadding="8" cellspacing="0" style="background-color: #1a2333; border: 1px solid #2d3748; border-radius: 8px; margin: 20px 0; font-family: monospace; font-size: 12px;">`)
		for k, v := range fields {
			fieldsHTML.WriteString(fmt.Sprintf(`
        <tr>
          <td width="35%%" style="color: #9ca3af; border-bottom: 1px solid #243044; padding: 8px 12px;">%s</td>
          <td width="65%%" style="color: #e5e7eb; font-weight: bold; border-bottom: 1px solid #243044; padding: 8px 12px;">%s</td>
        </tr>`, html.EscapeString(k), html.EscapeString(v)))
			fieldsText.WriteString(fmt.Sprintf("  * %s: %s\n", k, v))
		}
		fieldsHTML.WriteString(`</table>`)
	}

	btnHTML := ""
	if dashboardURL != "" {
		btnHTML = fmt.Sprintf(`
      <div style="text-align: center; margin: 28px 0 12px 0;">
        <a href="%s" style="background-color: #374151; color: #f3f4f6; font-weight: 600; font-size: 13px; text-decoration: none; padding: 10px 22px; border-radius: 6px; display: inline-block; font-family: monospace; border: 1px solid #4b5563;">VIEW IN DASHBOARD</a>
      </div>`, html.EscapeString(dashboardURL))
	}

	contentHTML := fmt.Sprintf(`
      <div style="display: inline-block; padding: 4px 10px; border-radius: 6px; background-color: #374151; color: #f3f4f6; font-size: 11px; font-weight: bold; font-family: monospace; text-transform: uppercase; margin-bottom: 12px;">
        %s
      </div>
      <h2 style="margin: 0 0 12px 0; font-size: 18px; font-weight: 700; color: #f9fafb;">%s</h2>
      <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #cbd5e1;">
        %s
      </p>
      %s
      %s`,
		html.EscapeString(alertType),
		html.EscapeString(alertTitle),
		html.EscapeString(alertDescription),
		fieldsHTML.String(),
		btnHTML,
	)

	textBody := fmt.Sprintf("[%s] %s\n\n%s\n\n%s\nDashboard: %s\n",
		alertType, alertTitle, alertDescription, fieldsText.String(), dashboardURL)

	return renderBaseHTML(alertTitle, preview, contentHTML), textBody
}

// TestEmailTemplate generates HTML and plaintext bodies for SMTP test notifications.
func TestEmailTemplate(serverTime string) (string, string) {
	title := "BSM SMTP Test Notification"
	preview := "Your Bedrock Server Manager SMTP mail delivery configuration is verified and working!"

	contentHTML := fmt.Sprintf(`
      <div style="display: inline-block; padding: 4px 10px; border-radius: 6px; background-color: #064e3b; color: #34d399; font-size: 11px; font-weight: bold; font-family: monospace; margin-bottom: 12px; border: 1px solid #059669;">
        ✓ SMTP CONNECTED
      </div>
      <h2 style="margin: 0 0 12px 0; font-size: 18px; font-weight: 700; color: #f9fafb;">SMTP Configuration Verified</h2>
      <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #cbd5e1;">
        This test notification confirms that your Bedrock Server Manager email delivery integration is successfully connected and capable of dispatching outgoing emails.
      </p>
      <table width="100%%" border="0" cellpadding="8" cellspacing="0" style="background-color: #1a2333; border: 1px solid #2d3748; border-radius: 8px; margin: 20px 0; font-family: monospace; font-size: 12px;">
        <tr>
          <td width="35%%" style="color: #9ca3af; border-bottom: 1px solid #243044; padding: 8px 12px;">Status</td>
          <td width="65%%" style="color: #34d399; font-weight: bold; border-bottom: 1px solid #243044; padding: 8px 12px;">Verified & Active</td>
        </tr>
        <tr>
          <td width="35%%" style="color: #9ca3af; padding: 8px 12px;">Server UTC Time</td>
          <td width="65%%" style="color: #e5e7eb; padding: 8px 12px;">%s</td>
        </tr>
      </table>`, html.EscapeString(serverTime))

	textBody := fmt.Sprintf("BSM SMTP Test Notification\n\n"+
		"Your Bedrock Server Manager SMTP mail delivery configuration is verified and working!\n"+
		"Status: Verified & Active\n"+
		"Server UTC Time: %s\n", serverTime)

	return renderBaseHTML(title, preview, contentHTML), textBody
}
