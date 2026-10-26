package email

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSMTP struct {
	listener       net.Listener
	dataReply      string
	mailReply      string
	rcptReply      string
	stallAfterEHLO bool

	mu         sync.Mutex
	messages   []string
	recipients []string
	auths      []string
}

func newFakeSMTP(t *testing.T, tlsConfig *tls.Config) *fakeSMTP {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}

	server := &fakeSMTP{listener: listener}
	t.Cleanup(func() { _ = listener.Close() })

	go server.serve()

	return server
}

func (server *fakeSMTP) serve() {
	conn, err := server.listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	protocol := textproto.NewConn(conn)
	defer func() { _ = protocol.Close() }()

	_ = protocol.PrintfLine("220 fake ESMTP")

	for {
		line, err := protocol.ReadLine()
		if err != nil {
			return
		}

		command := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(command, "EHLO"):
			if server.stallsAfterEHLO() {
				// Wait for the client to give up instead of sending a reply.
				_, _ = protocol.ReadLine()
				return
			}
			_ = protocol.PrintfLine("250-fake")
			_ = protocol.PrintfLine("250-AUTH PLAIN")
			_ = protocol.PrintfLine("250 OK")
		case strings.HasPrefix(command, "HELO"):
			_ = protocol.PrintfLine("250 fake")
		case strings.HasPrefix(command, "AUTH PLAIN"):
			server.mu.Lock()
			server.auths = append(server.auths, line)
			server.mu.Unlock()
			_ = protocol.PrintfLine("235 2.7.0 Authentication successful")
		case strings.HasPrefix(command, "MAIL FROM:"):
			if server.mailReply != "" {
				_ = protocol.PrintfLine("%s", server.mailReply)
				continue
			}
			_ = protocol.PrintfLine("250 2.1.0 OK")
		case strings.HasPrefix(command, "RCPT TO:"):
			if start := strings.Index(line, "<"); start >= 0 {
				if end := strings.LastIndex(line, ">"); end > start {
					server.mu.Lock()
					server.recipients = append(server.recipients, line[start+1:end])
					server.mu.Unlock()
				}
			}
			if server.rcptReply != "" {
				_ = protocol.PrintfLine("%s", server.rcptReply)
				continue
			}
			_ = protocol.PrintfLine("250 2.1.5 OK")
		case command == "DATA":
			if server.dataReply != "" {
				_ = protocol.PrintfLine("%s", server.dataReply)
				continue
			}
			_ = protocol.PrintfLine("354 End data with <CR><LF>.<CR><LF>")
			data, err := protocol.ReadDotBytes()
			if err != nil {
				return
			}
			server.mu.Lock()
			server.messages = append(server.messages, string(data))
			server.mu.Unlock()
			_ = protocol.PrintfLine("250 2.0.0 OK")
		case command == "QUIT":
			_ = protocol.PrintfLine("221 2.0.0 Bye")
			return
		default:
			_ = protocol.PrintfLine("250 OK")
		}
	}
}

func (server *fakeSMTP) hostPort(t *testing.T) (string, string) {
	t.Helper()

	host, port, err := net.SplitHostPort(server.listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}

	return host, port
}

func (server *fakeSMTP) snapshot() (messages, recipients, auths []string) {
	server.mu.Lock()
	defer server.mu.Unlock()

	return append([]string(nil), server.messages...),
		append([]string(nil), server.recipients...),
		append([]string(nil), server.auths...)
}

func (server *fakeSMTP) stallNextEHLO() {
	server.mu.Lock()
	defer server.mu.Unlock()

	server.stallAfterEHLO = true
}

func (server *fakeSMTP) stallsAfterEHLO() bool {
	server.mu.Lock()
	defer server.mu.Unlock()

	return server.stallAfterEHLO
}

func TestSMTPConfigValidate(t *testing.T) {
	tests := []struct {
		name   string
		config SMTPConfig
		want   string
	}{
		{
			name:   "missing host",
			config: SMTPConfig{Port: "587"},
			want:   "host cannot be empty",
		},
		{
			name:   "missing port",
			config: SMTPConfig{Host: "smtp.example.com"},
			want:   "port cannot be empty",
		},
		{
			name:   "username without password",
			config: SMTPConfig{Host: "smtp.example.com", Port: "587", Username: "user"},
			want:   "username and password must be set together",
		},
		{
			name:   "password without username",
			config: SMTPConfig{Host: "smtp.example.com", Port: "587", Password: "secret"},
			want:   "username and password must be set together",
		},
		{
			name:   "unknown TLS mode",
			config: SMTPConfig{Host: "smtp.example.com", Port: "587", TLS: "ssl"},
			want:   "unsupported smtp TLS mode",
		},
		{
			name: "credentials over plaintext remote host",
			config: SMTPConfig{
				Host:     "smtp.example.com",
				Port:     "25",
				Username: "user",
				Password: "secret",
				TLS:      SMTPTLSNone,
			},
			want: "credentials require TLS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, test.want)
			}

			if _, err := NewSMTP(test.config); err == nil {
				t.Fatalf("NewSMTP() = nil error, want error for %#v", test.config)
			}
		})
	}
}

func TestNewSMTPDefaultsToStartTLS(t *testing.T) {
	client, err := NewSMTP(SMTPConfig{Host: "smtp.example.com", Port: "587"})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	if client.config.TLS != SMTPTLSStartTLS {
		t.Fatalf("TLS = %q, want %q", client.config.TLS, SMTPTLSStartTLS)
	}
}

func TestSMTPDeliversPlainMessage(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	})
	if err != nil {
		t.Fatalf("SendTransactional() = %v, want nil error", err)
	}

	messages, recipients, auths := server.snapshot()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	if len(auths) != 0 {
		t.Fatalf("auths = %v, want none", auths)
	}
	if len(recipients) != 1 || recipients[0] != "person@example.com" {
		t.Fatalf("recipients = %v, want person@example.com", recipients)
	}

	message, err := mail.ReadMessage(strings.NewReader(messages[0]))
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if got := message.Header.Get("From"); got != "noreply@example.com" {
		t.Fatalf("From = %q, want noreply@example.com", got)
	}
	if got := message.Header.Get("To"); got != "person@example.com" {
		t.Fatalf("To = %q, want person@example.com", got)
	}
	if got := message.Header.Get("Date"); got == "" {
		t.Fatal("Date header is empty")
	}
	if got := message.Header.Get("Message-ID"); !strings.HasSuffix(got, "@example.com>") {
		t.Fatalf("Message-ID = %q, want domain example.com", got)
	}
}

func TestSMTPSendsAuthWhenCredentialsAreSet(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{
		Host:     host,
		Port:     port,
		Username: "user",
		Password: "secret",
		TLS:      SMTPTLSNone,
	})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("SendTransactional() = %v, want nil error", err)
	}

	_, _, auths := server.snapshot()
	if len(auths) != 1 {
		t.Fatalf("auths = %v, want one AUTH command", auths)
	}

	want := "AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00user\x00secret"))
	if auths[0] != want {
		t.Fatalf("auth = %q, want %q", auths[0], want)
	}
}

func TestSMTPMapsRepliesToTypedErrors(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		wantTmp bool
	}{
		{name: "4xx is temporary", reply: "451 4.3.0 try again later", wantTmp: true},
		{name: "5xx is permanent", reply: "554 5.7.1 rejected", wantTmp: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newFakeSMTP(t, nil)
			server.dataReply = test.reply
			host, port := server.hostPort(t)

			client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
			if err != nil {
				t.Fatalf("NewSMTP() = %v, want nil error", err)
			}

			err = client.SendTransactional(t.Context(), TransactionalPayload{
				To:       "person@example.com",
				From:     "noreply@example.com",
				Subject:  "Hello",
				HTMLBody: "<p>Hello</p>",
			})
			if err == nil {
				t.Fatal("SendTransactional() = nil, want error")
			}

			if test.wantTmp {
				if _, ok := errors.AsType[TemporaryError](err); !ok {
					t.Fatalf("error = %T, want TemporaryError", err)
				}
				if !IsRetryable(err) {
					t.Fatalf("IsRetryable(%v) = false, want true", err)
				}
				return
			}

			if _, ok := errors.AsType[PermanentError](err); !ok {
				t.Fatalf("error = %T, want PermanentError", err)
			}
			if IsRetryable(err) {
				t.Fatalf("IsRetryable(%v) = true, want false", err)
			}
		})
	}
}

func TestSMTPKeepsBccOutOfHeaders(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		Bcc:      []string{"hidden@example.com"},
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("SendTransactional() = %v, want nil error", err)
	}

	messages, recipients, _ := server.snapshot()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	if len(recipients) != 2 ||
		recipients[0] != "person@example.com" ||
		recipients[1] != "hidden@example.com" {
		t.Fatalf("recipients = %v, want both To and Bcc addresses", recipients)
	}

	if strings.Contains(messages[0], "Bcc:") {
		t.Fatalf("message contains a Bcc header:\n%s", messages[0])
	}
	if strings.Contains(messages[0], "hidden@example.com") {
		t.Fatalf("message leaks the Bcc address:\n%s", messages[0])
	}
}

func TestSMTPDeliversAttachments(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	content := []byte("attachment body")
	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
		Attachments: []Attachment{{
			Name:        "notes.txt",
			Content:     content,
			ContentType: "text/plain",
		}},
	})
	if err != nil {
		t.Fatalf("SendTransactional() = %v, want nil error", err)
	}

	messages, _, _ := server.snapshot()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}

	message, err := mail.ReadMessage(strings.NewReader(messages[0]))
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("content type = %q, want multipart/mixed", mediaType)
	}

	reader := multipart.NewReader(message.Body, params["boundary"])
	if _, err := reader.NextRawPart(); err != nil {
		t.Fatalf("next alternative part: %v", err)
	}

	part, err := reader.NextRawPart()
	if err != nil {
		t.Fatalf("next attachment part: %v", err)
	}

	disposition, dispositionParams, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parse attachment disposition: %v", err)
	}
	if disposition != "attachment" {
		t.Fatalf("disposition = %q, want attachment", disposition)
	}
	if dispositionParams["filename"] != "notes.txt" {
		t.Fatalf("filename = %q, want notes.txt", dispositionParams["filename"])
	}

	encoded, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(string(encoded))))
	if err != nil {
		t.Fatalf("decode attachment: %v", err)
	}
	if string(decoded) != string(content) {
		t.Fatalf("attachment content = %q, want %q", decoded, content)
	}
}

func TestSMTPEncodesNonASCIIHeaders(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		From:     "Ünïcode Sender <noreply@example.com>",
		Subject:  "Héllo wörld",
		HTMLBody: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("SendTransactional() = %v, want nil error", err)
	}

	messages, _, _ := server.snapshot()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}

	message, err := mail.ReadMessage(strings.NewReader(messages[0]))
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	subject := message.Header.Get("Subject")
	if !strings.HasPrefix(subject, "=?utf-8?") {
		t.Fatalf("Subject = %q, want RFC 2047 encoding", subject)
	}

	decoder := new(mime.WordDecoder)
	decoded, err := decoder.DecodeHeader(subject)
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}
	if decoded != "Héllo wörld" {
		t.Fatalf("decoded subject = %q, want Héllo wörld", decoded)
	}

	from := message.Header.Get("From")
	if !strings.HasPrefix(from, "=?utf-8?") {
		t.Fatalf("From = %q, want encoded display name", from)
	}
}

func TestSMTPHonoursContextCancellation(t *testing.T) {
	t.Run("before dialing", func(t *testing.T) {
		server := newFakeSMTP(t, nil)
		host, port := server.hostPort(t)

		client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
		if err != nil {
			t.Fatalf("NewSMTP() = %v, want nil error", err)
		}

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err = client.SendTransactional(ctx, TransactionalPayload{
			To:       "person@example.com",
			From:     "noreply@example.com",
			Subject:  "Hello",
			HTMLBody: "<p>Hello</p>",
		})
		assertContextCancelled(t, err)
	})

	t.Run("during conversation", func(t *testing.T) {
		server := newFakeSMTP(t, nil)
		server.stallNextEHLO()
		host, port := server.hostPort(t)

		client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSNone})
		if err != nil {
			t.Fatalf("NewSMTP() = %v, want nil error", err)
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		timer := time.AfterFunc(100*time.Millisecond, cancel)
		defer timer.Stop()

		err = client.SendTransactional(ctx, TransactionalPayload{
			To:       "person@example.com",
			From:     "noreply@example.com",
			Subject:  "Hello",
			HTMLBody: "<p>Hello</p>",
		})
		assertContextCancelled(t, err)
	})
}

func assertContextCancelled(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("SendTransactional() = nil, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if !IsRetryable(err) {
		t.Fatalf("IsRetryable(%v) = false, want true", err)
	}
}

func TestSMTPImplicitTLS(t *testing.T) {
	server := newFakeSMTP(t, testTLSConfig(t))
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{
		Host:     host,
		Port:     port,
		Username: "user",
		Password: "secret",
		TLS:      SMTPTLSImplicit,
	})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}
	client.tlsConfig = &tls.Config{
		InsecureSkipVerify: true, // self-signed test certificate
		MinVersion:         tls.VersionTLS12,
	}

	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("SendTransactional() = %v, want nil error", err)
	}

	messages, _, auths := server.snapshot()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	if len(auths) != 1 {
		t.Fatalf("auths = %v, want one AUTH command", auths)
	}
}

func TestSMTPTreatsCertificateVerificationFailuresAsPermanent(t *testing.T) {
	server := newFakeSMTP(t, testTLSConfig(t))
	host, port := server.hostPort(t)

	client, err := NewSMTP(SMTPConfig{Host: host, Port: port, TLS: SMTPTLSImplicit})
	if err != nil {
		t.Fatalf("NewSMTP() = %v, want nil error", err)
	}

	err = client.SendTransactional(t.Context(), TransactionalPayload{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
	})
	if err == nil {
		t.Fatal("SendTransactional() = nil, want certificate verification error")
	}
	if _, ok := errors.AsType[PermanentError](err); !ok {
		t.Fatalf("error = %T, want PermanentError", err)
	}
	if IsRetryable(err) {
		t.Fatalf("IsRetryable(%v) = true, want false", err)
	}
}

func TestClassifySMTPErrorTreatsCertificateFailuresAsPermanent(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "unknown authority",
			err:  x509.UnknownAuthorityError{},
		},
		{
			name: "hostname mismatch",
			err:  x509.HostnameError{Host: "smtp.example.com"},
		},
		{
			name: "invalid certificate",
			err:  x509.CertificateInvalidError{Reason: x509.Expired},
		},
		{
			name: "certificate verification wrapper",
			err: &tls.CertificateVerificationError{
				Err: x509.UnknownAuthorityError{},
			},
		},
		{
			name: "wrapped certificate verification error",
			err: fmt.Errorf("tls handshake: %w", &tls.CertificateVerificationError{
				Err: x509.HostnameError{Host: "smtp.example.com"},
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classified := classifySMTPError(test.err)
			if _, ok := errors.AsType[PermanentError](classified); !ok {
				t.Fatalf("classifySMTPError(%v) = %T, want PermanentError", test.err, classified)
			}
			if IsRetryable(classified) {
				t.Fatalf("IsRetryable(%v) = true, want false", classified)
			}
		})
	}
}

func testTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificate, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certificate},
			PrivateKey:  key,
		}},
		MinVersion: tls.VersionTLS12,
	}
}
