package email

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

var (
	_ TransactionalSender = (*SMTP)(nil)
	_ MarketingSender     = (*SMTP)(nil)
)

// SMTPTLSMode selects how an SMTP connection is encrypted.
type SMTPTLSMode string

const (
	// SMTPTLSStartTLS upgrades a plaintext connection with STARTTLS. It is the
	// default and the right choice for submission on port 587.
	SMTPTLSStartTLS SMTPTLSMode = "starttls"
	// SMTPTLSImplicit dials TLS directly, as submission on port 465 expects.
	SMTPTLSImplicit SMTPTLSMode = "tls"
	// SMTPTLSNone sends plaintext. Only use it for a local relay.
	SMTPTLSNone SMTPTLSMode = "none"
)

// SMTPConfig configures a production SMTP relay.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	TLS      SMTPTLSMode
	Timeout  time.Duration
}

// Validate verifies SMTP configuration. Username and password must be supplied
// together, and plaintext authentication is refused outside a local relay.
func (config SMTPConfig) Validate() error {
	if strings.TrimSpace(config.Host) == "" {
		return fmt.Errorf("email: smtp host cannot be empty")
	}
	if strings.TrimSpace(config.Port) == "" {
		return fmt.Errorf("email: smtp port cannot be empty")
	}

	switch config.TLS {
	case "", SMTPTLSStartTLS, SMTPTLSImplicit, SMTPTLSNone:
	default:
		return fmt.Errorf("email: unsupported smtp TLS mode %q", config.TLS)
	}

	if (config.Username == "") != (config.Password == "") {
		return fmt.Errorf("email: smtp username and password must be set together")
	}

	if config.TLS == SMTPTLSNone && config.Username != "" && !isLocalHost(config.Host) {
		return fmt.Errorf("email: smtp credentials require TLS unless the host is local")
	}

	return nil
}

// SMTP sends email through a production SMTP relay.
type SMTP struct {
	config    SMTPConfig
	tlsConfig *tls.Config
}

// NewSMTP constructs an SMTP sender from the supplied configuration.
func NewSMTP(config SMTPConfig) (*SMTP, error) {
	if config.TLS == "" {
		config.TLS = SMTPTLSStartTLS
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("email: create smtp sender: %w", err)
	}

	return &SMTP{config: config}, nil
}

func (sender *SMTP) SendTransactional(
	ctx context.Context,
	payload TransactionalPayload,
) error {
	outbound := outboundMessage{
		From:        payload.From,
		To:          nonEmpty(payload.To),
		Cc:          payload.Cc,
		Bcc:         payload.Bcc,
		ReplyTo:     payload.ReplyTo,
		Subject:     payload.Subject,
		HTMLBody:    payload.HTMLBody,
		TextBody:    payload.TextBody,
		Attachments: payload.Attachments,
	}

	message, err := buildMessage(outbound)
	if err != nil {
		return err
	}

	return sender.send(ctx, payload.From, outbound.recipients(), message)
}

func (sender *SMTP) SendMarketing(
	ctx context.Context,
	payload MarketingPayload,
) error {
	outbound := outboundMessage{
		From:     payload.From,
		To:       payload.To,
		ReplyTo:  payload.ReplyTo,
		Subject:  payload.Subject,
		HTMLBody: payload.HTMLBody,
		TextBody: payload.TextBody,
	}

	message, err := buildMessage(outbound)
	if err != nil {
		return err
	}

	return sender.send(ctx, payload.From, outbound.recipients(), message)
}

func (sender *SMTP) send(
	ctx context.Context,
	from string,
	recipients []string,
	message []byte,
) error {
	return sendMail(ctx, sendOptions{
		address:    net.JoinHostPort(sender.config.Host, sender.config.Port),
		host:       sender.config.Host,
		tlsMode:    sender.config.TLS,
		username:   sender.config.Username,
		password:   sender.config.Password,
		timeout:    sender.config.Timeout,
		tlsConfig:  sender.tlsConfig,
		from:       from,
		recipients: recipients,
		message:    message,
	})
}

// nonEmpty keeps an optional single recipient out of the address headers and
// the SMTP envelope.
func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}

	return []string{value}
}

type sendOptions struct {
	address    string
	host       string
	tlsMode    SMTPTLSMode
	username   string
	password   string
	timeout    time.Duration
	tlsConfig  *tls.Config
	from       string
	recipients []string
	message    []byte
}

// plainAuth sends PLAIN credentials. It replaces smtp.PlainAuth because a
// client wrapped in TLS before smtp.NewClient still reports itself as
// plaintext to that helper, which refuses to authenticate.
type plainAuth struct {
	username string
	password string
	host     string
}

func (auth *plainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server.Name != auth.host {
		return "", nil, fmt.Errorf("email: smtp server name mismatch")
	}

	return "PLAIN", []byte("\x00" + auth.username + "\x00" + auth.password), nil
}

func (auth *plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("email: unexpected SMTP authentication challenge")
	}

	return nil, nil
}

func sendMail(ctx context.Context, options sendOptions) error {
	dialer := net.Dialer{Timeout: options.timeout}

	conn, err := dialer.DialContext(ctx, "tcp", options.address)
	if err != nil {
		return smtpError(ctx, err)
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else if options.timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(options.timeout))
	}

	tlsConfig := options.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			ServerName: options.host,
			MinVersion: tls.VersionTLS12,
		}
	}

	if options.tlsMode == SMTPTLSImplicit {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return smtpError(ctx, err)
		}
		conn = tlsConn
	}

	// Closing the connection interrupts a read that is waiting on the server,
	// which is the only way to abandon an SMTP conversation when the context
	// has no deadline.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	client, err := smtp.NewClient(conn, options.host)
	if err != nil {
		return smtpError(ctx, err)
	}
	defer func() { _ = client.Close() }()

	if options.tlsMode == SMTPTLSStartTLS {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			return PermanentError{Err: fmt.Errorf("email: smtp server does not support STARTTLS")}
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return smtpError(ctx, err)
		}
	}

	if options.username != "" {
		if options.tlsMode == SMTPTLSNone && !isLocalHost(options.host) {
			return PermanentError{Err: fmt.Errorf("email: smtp credentials require TLS unless the host is local")}
		}

		auth := &plainAuth{
			username: options.username,
			password: options.password,
			host:     options.host,
		}
		if err := client.Auth(auth); err != nil {
			return smtpError(ctx, err)
		}
	}

	if err := client.Mail(options.from); err != nil {
		return smtpError(ctx, err)
	}

	for _, recipient := range options.recipients {
		if err := client.Rcpt(recipient); err != nil {
			return smtpError(ctx, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return smtpError(ctx, err)
	}

	if _, err := writer.Write(options.message); err != nil {
		_ = writer.Close()
		return smtpError(ctx, err)
	}

	if err := writer.Close(); err != nil {
		return smtpError(ctx, err)
	}

	if err := client.Quit(); err != nil {
		return smtpError(ctx, err)
	}

	return nil
}

// smtpError prefers the context's error because a cancelled conversation
// usually surfaces as a closed-connection error from net/smtp.
func smtpError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return TemporaryError{Err: ctxErr}
	}

	return classifySMTPError(err)
}

// classifySMTPError maps SMTP replies and transport failures onto the retry
// contract callers already understand.
func classifySMTPError(err error) error {
	if err == nil {
		return nil
	}

	if isCertificateError(err) {
		return PermanentError{Err: err}
	}

	if reply, ok := errors.AsType[*textproto.Error](err); ok {
		if reply.Code >= 500 && reply.Code < 600 {
			return PermanentError{Err: err}
		}
	}

	return TemporaryError{Err: err}
}

// isCertificateError reports whether err is a TLS certificate verification
// failure. These reflect misconfigured trust or hostnames, so retrying them
// cannot succeed.
func isCertificateError(err error) bool {
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.HostnameError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.CertificateInvalidError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return true
	}

	return false
}

func isLocalHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}

	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}
