package imap

import (
	"context"
	"errors"
	"testing"

	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
)

type fakeClient struct {
	listMailbox  string
	listLimit    int
	getMailbox   string
	getUID       uint32
	listResult   []mail.Summary
	detailResult mail.Detail
	listErr      error
	getErr       error
}

func (f *fakeClient) ListRecent(_ context.Context, mailbox string, limit int) ([]mail.Summary, error) {
	f.listMailbox = mailbox
	f.listLimit = limit
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeClient) GetByUID(_ context.Context, mailbox string, uid uint32) (mail.Detail, error) {
	f.getMailbox = mailbox
	f.getUID = uid
	if f.getErr != nil {
		return mail.Detail{}, f.getErr
	}
	return f.detailResult, nil
}

func TestServiceDelegatesListAndDetailCalls(t *testing.T) {
	client := &fakeClient{
		listResult:   []mail.Summary{{UID: 1, Subject: "hello"}},
		detailResult: mail.Detail{Summary: mail.Summary{UID: 1, Subject: "hello"}},
	}
	service := NewService(client)

	summaries, err := service.ListRecent(context.Background(), config.AccountConfig{}, "INBOX", 20)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if client.listMailbox != "INBOX" || client.listLimit != 20 {
		t.Fatalf("ListRecent delegated wrong arguments: mailbox=%q limit=%d", client.listMailbox, client.listLimit)
	}
	if len(summaries) != 1 || summaries[0].UID != 1 {
		t.Fatalf("unexpected list result: %+v", summaries)
	}

	detail, err := service.GetByUID(context.Background(), config.AccountConfig{}, "INBOX", 1)
	if err != nil {
		t.Fatalf("GetByUID returned error: %v", err)
	}
	if client.getMailbox != "INBOX" || client.getUID != 1 {
		t.Fatalf("GetByUID delegated wrong arguments: mailbox=%q uid=%d", client.getMailbox, client.getUID)
	}
	if detail.UID != 1 {
		t.Fatalf("unexpected detail result: %+v", detail)
	}
}

func TestServicePropagatesClientErrors(t *testing.T) {
	expectedErr := errors.New("imap unavailable")
	service := NewService(&fakeClient{listErr: expectedErr, getErr: expectedErr})

	_, err := service.ListRecent(context.Background(), config.AccountConfig{}, "INBOX", 20)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("ListRecent error = %v, want %v", err, expectedErr)
	}

	_, err = service.GetByUID(context.Background(), config.AccountConfig{}, "INBOX", 9)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("GetByUID error = %v, want %v", err, expectedErr)
	}
}
