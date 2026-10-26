package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/mbvlabs/andurel/pkg/email"
	"github.com/riverqueue/river"
)

type transactionalSenderFunc func(context.Context, email.TransactionalPayload) error

func (f transactionalSenderFunc) SendTransactional(
	ctx context.Context,
	payload email.TransactionalPayload,
) error {
	return f(ctx, payload)
}

type marketingSenderFunc func(context.Context, email.MarketingPayload) error

func (f marketingSenderFunc) SendMarketing(
	ctx context.Context,
	payload email.MarketingPayload,
) error {
	return f(ctx, payload)
}

func TestEmailJobKindsStayStable(t *testing.T) {
	if got := (SendTransactionalEmailArgs{}).Kind(); got != "send_transactional_email" {
		t.Fatalf("transactional kind = %q", got)
	}
	if got := (SendMarketingEmailArgs{}).Kind(); got != "send_marketing_email" {
		t.Fatalf("marketing kind = %q", got)
	}
}

func TestAddEmailWorkersRegistersBothWorkers(t *testing.T) {
	workers := river.NewWorkers()
	transactional := transactionalSenderFunc(func(context.Context, email.TransactionalPayload) error {
		return nil
	})
	marketing := marketingSenderFunc(func(context.Context, email.MarketingPayload) error {
		return nil
	})

	AddEmailWorkers(workers, transactional, marketing)

	if err := river.AddWorkerSafely[SendTransactionalEmailArgs](
		workers,
		NewSendTransactionalEmailWorker(transactional),
	); err == nil {
		t.Fatal("transactional email worker was not registered")
	}
	if err := river.AddWorkerSafely[SendMarketingEmailArgs](
		workers,
		NewSendMarketingEmailWorker(marketing),
	); err == nil {
		t.Fatal("marketing email worker was not registered")
	}
}

func TestSendTransactionalCancelsNonRetryableErrors(t *testing.T) {
	permanent := email.PermanentError{Err: errors.New("mailbox does not exist")}
	err := SendTransactional(
		t.Context(),
		email.TransactionalData{To: "user@example.com", From: "app@example.com", Subject: "Hi", HTMLBody: "<p>Hi</p>"},
		transactionalSenderFunc(func(context.Context, email.TransactionalPayload) error {
			return permanent
		}),
	)

	if _, ok := errors.AsType[*river.JobCancelError](err); !ok {
		t.Fatalf("SendTransactional error = %v, want a cancelled job", err)
	}
	if !errors.Is(err, permanent) {
		t.Fatalf("SendTransactional dropped the sender error: %v", err)
	}
}

func TestSendTransactionalReturnsRetryableErrors(t *testing.T) {
	temporary := email.TemporaryError{Err: errors.New("mail server unavailable")}
	err := SendTransactional(
		t.Context(),
		email.TransactionalData{To: "user@example.com", From: "app@example.com", Subject: "Hi", HTMLBody: "<p>Hi</p>"},
		transactionalSenderFunc(func(context.Context, email.TransactionalPayload) error {
			return temporary
		}),
	)

	if _, ok := errors.AsType[*river.JobCancelError](err); ok {
		t.Fatalf("SendTransactional cancelled a retryable error: %v", err)
	}
	if !errors.Is(err, temporary) {
		t.Fatalf("SendTransactional error = %v, want the sender error", err)
	}
}
