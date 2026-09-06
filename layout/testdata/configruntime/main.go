package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"testapp/config"
)

func main() {
	os.Clearenv()
	for key, value := range map[string]string{
		"HTTP_READ_TIMEOUT":          "banana",
		"QUEUE_POLL_INTERVAL":        "banana",
		"QUEUE_ADVISORY_LOCK_PREFIX": "2147483648",
		"TRACE_SAMPLE_RATE":          "50%",
	} {
		must(os.Setenv(key, value))
	}

	database, err := config.NewDatabase()
	must(err)
	assert(database.Host == config.DefaultDatabaseHost, "database defaults")
	fmt.Println("PASS: database config ignores unrelated invalid and missing settings")

	_, err = config.NewHTTP()
	assert(err != nil && strings.Contains(err.Error(), "HTTP_READ_TIMEOUT"), "HTTP parse error")
	_, err = config.NewAuth()
	assert(err != nil && strings.Contains(err.Error(), "TOKEN_SIGNING_KEY"), "auth required values")

	queueInsert, err := config.NewQueueInsert()
	must(err)
	_, err = config.NewQueueWorker(queueInsert)
	assert(
		err != nil &&
			strings.Contains(err.Error(), "QUEUE_POLL_INTERVAL") &&
			strings.Contains(err.Error(), "QUEUE_ADVISORY_LOCK_PREFIX"),
		"queue worker errors",
	)
	fmt.Println("PASS: each constructor validates only its own process requirements")

	app, err := config.NewApp()
	must(err)
	_, err = config.NewTelemetry(app)
	assert(err != nil && strings.Contains(err.Error(), "TRACE_SAMPLE_RATE"), "telemetry parse error")

	must(os.Setenv("SESSION_KEY", "00112233445566778899aabbccddeeff"))
	must(os.Setenv("SESSION_ENCRYPTION_KEY", "00112233445566778899aabbccddeeff"))
	session, err := config.NewSession(app)
	must(err)
	assert(len(session.AuthenticationKey) == 16 && session.MaxAge == config.DefaultSessionMaxAge, "session")

	must(os.Setenv("TOKEN_SIGNING_KEY", "secret"))
	must(os.Setenv("PEPPER", "pepper"))
	must(os.Setenv("PREVIOUS_PEPPERS", "old-one, ,old-two"))
	auth, err := config.NewAuth()
	must(err)
	assert(len(auth.PreviousPeppers) == 2, "auth pepper list")

	must(os.Setenv("EMAIL_PROVIDER", "mailpit"))
	mailIdentity, err := config.NewMail(app)
	must(err)
	assert(mailIdentity.DefaultSenderSignature == "noreply@localhost:8080", "mail sender identity")
	mail, err := config.NewMailTransport(app)
	must(err)
	assert(mail.Driver == config.MailpitDriver, "mail driver")

	must(os.Setenv("QUEUE_POLL_INTERVAL", "2s"))
	must(os.Setenv("QUEUE_ADVISORY_LOCK_PREFIX", "12"))
	queueWorker, err := config.NewQueueWorker(queueInsert)
	must(err)
	assert(queueWorker.Config.PollInterval == 2*time.Second, "queue worker override")
	fmt.Println("PASS: typed values and selected mail driver load successfully")

	must(os.Setenv("PROJECT_NAME", ""))
	_, err = config.NewApp()
	assert(err != nil, "a present empty string must not silently use the default")
	fmt.Println("PASS: present empty values remain explicit")
}

func assert(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
