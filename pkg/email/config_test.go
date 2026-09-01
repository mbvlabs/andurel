package email

import "testing"

func TestDefaultMailpitConfigValidate(t *testing.T) {
	if err := DefaultMailpitConfig().Validate(); err != nil {
		t.Fatalf("DefaultMailpitConfig().Validate() = %v, want nil", err)
	}
}

func TestMailpitConfigValidateRequiresHostAndPort(t *testing.T) {
	tests := []MailpitConfig{
		{Host: "", Port: "1025"},
		{Host: "127.0.0.1", Port: ""},
	}

	for _, config := range tests {
		if err := config.Validate(); err == nil {
			t.Fatalf("Validate() = nil, want error for %#v", config)
		}
	}
}

func TestNewMailpitUsesDefaults(t *testing.T) {
	client, err := NewMailpit()
	if err != nil {
		t.Fatalf("NewMailpit() = %v, want nil error", err)
	}
	if client == nil {
		t.Fatal("NewMailpit() returned nil client")
	}
}

func TestNewMailpitAppliesConfig(t *testing.T) {
	client, err := NewMailpit(WithMailpitConfig(MailpitConfig{
		Host: "mailpit.test",
		Port: "2525",
	}))
	if err != nil {
		t.Fatalf("NewMailpit() = %v, want nil error", err)
	}
	if client.host != "mailpit.test" || client.port != "2525" {
		t.Fatalf("client = %#v, want host mailpit.test and port 2525", client)
	}
}

func TestSendTransactionalRequiresRecipient(t *testing.T) {
	err := SendTransactional(t.Context(), TransactionalData{
		From:     "from@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hi</p>",
	}, &Mailpit{host: "127.0.0.1", port: "1025"})
	if !IsValidationError(err) {
		t.Fatalf("SendTransactional() = %v, want validation error", err)
	}
}
