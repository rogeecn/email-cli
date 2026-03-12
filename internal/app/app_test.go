package app

import (
	"context"
	"errors"
	"testing"

	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
)

type fakeAccountLoader struct {
	cfg config.Config
	err error
}

func (f fakeAccountLoader) Load() (config.Config, error) {
	return f.cfg, f.err
}

type fakeMailService struct {
	listMailbox  string
	listLimit    int
	listOffset   int
	getMailbox   string
	getUID       uint32
	listResult   ListResult
	detailResult mail.Detail
	listErr      error
	getErr       error
}

func (f *fakeMailService) ListRecent(_ context.Context, account config.AccountConfig, mailbox string, limit int, offset int) (ListResult, error) {
	_ = account
	f.listMailbox = mailbox
	f.listLimit = limit
	f.listOffset = offset
	if f.listErr != nil {
		return ListResult{}, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeMailService) GetByUID(_ context.Context, account config.AccountConfig, mailbox string, uid uint32) (mail.Detail, error) {
	_ = account
	f.getMailbox = mailbox
	f.getUID = uid
	if f.getErr != nil {
		return mail.Detail{}, f.getErr
	}
	return f.detailResult, nil
}

func TestApplicationRunListMode(t *testing.T) {
	loader := fakeAccountLoader{cfg: config.Config{
		DefaultAccount: "personal",
		Accounts: map[string]config.AccountConfig{
			"personal": {
				Provider: "qq",
				Defaults: config.DefaultOptions{Mailbox: "INBOX", PageSize: 20, Format: "plain"},
			},
		},
	}}
	service := &fakeMailService{listResult: ListResult{Summaries: []mail.Summary{{UID: 7, Subject: "Hello"}}, Total: 1}}
	application := New(loader, service)

	result, err := application.Run(context.Background(), Options{Offset: 5})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Mode != ModeList {
		t.Fatalf("Mode = %q, want %q", result.Mode, ModeList)
	}
	if service.listMailbox != "INBOX" || service.listLimit != 20 || service.listOffset != 5 {
		t.Fatalf("list called with mailbox=%q limit=%d offset=%d", service.listMailbox, service.listLimit, service.listOffset)
	}
	if len(result.Summaries) != 1 || result.Summaries[0].UID != 7 {
		t.Fatalf("unexpected list result: %+v", result.Summaries)
	}
	if result.ListMetadata.Total != 1 || result.ListMetadata.Limit != 20 || result.ListMetadata.Offset != 5 {
		t.Fatalf("unexpected list metadata: %+v", result.ListMetadata)
	}
}

func TestApplicationRunDetailMode(t *testing.T) {
	loader := fakeAccountLoader{cfg: config.Config{
		DefaultAccount: "personal",
		Accounts: map[string]config.AccountConfig{
			"personal": {
				Provider: "gmail",
				Defaults: config.DefaultOptions{Mailbox: "INBOX", PageSize: 20, Format: "plain"},
			},
		},
	}}
	service := &fakeMailService{detailResult: mail.Detail{Summary: mail.Summary{UID: 99, Subject: "World"}}}
	application := New(loader, service)

	result, err := application.Run(context.Background(), Options{UID: 99})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Mode != ModeDetail {
		t.Fatalf("Mode = %q, want %q", result.Mode, ModeDetail)
	}
	if service.getMailbox != "INBOX" || service.getUID != 99 {
		t.Fatalf("detail called with mailbox=%q uid=%d", service.getMailbox, service.getUID)
	}
	if result.Detail.UID != 99 {
		t.Fatalf("unexpected detail result: %+v", result.Detail)
	}
}

func TestApplicationPropagatesServiceErrors(t *testing.T) {
	expectedErr := errors.New("uid not found")
	loader := fakeAccountLoader{cfg: config.Config{
		DefaultAccount: "personal",
		Accounts: map[string]config.AccountConfig{
			"personal": {
				Provider: "qq",
				Defaults: config.DefaultOptions{Mailbox: "INBOX", PageSize: 20, Format: "plain"},
			},
		},
	}}
	service := &fakeMailService{getErr: expectedErr}
	application := New(loader, service)

	_, err := application.Run(context.Background(), Options{UID: 9})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Run error = %v, want %v", err, expectedErr)
	}
}
