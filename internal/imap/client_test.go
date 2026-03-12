package imap

import (
	"context"
	"errors"
	"testing"

	"github.com/rogeecn/email-cli/internal/app"
	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
)

type fakeClient struct {
	listMailbox  string
	listLimit    int
	listOffset   int
	getMailbox   string
	getUID       uint32
	listResult   app.ListResult
	detailResult mail.Detail
	listErr      error
	getErr       error
}

func (f *fakeClient) ListRecent(_ context.Context, mailbox string, limit int, offset int) (app.ListResult, error) {
	f.listMailbox = mailbox
	f.listLimit = limit
	f.listOffset = offset
	if f.listErr != nil {
		return app.ListResult{}, f.listErr
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
		listResult:   app.ListResult{Summaries: []mail.Summary{{UID: 1, Subject: "hello"}}, Total: 1},
		detailResult: mail.Detail{Summary: mail.Summary{UID: 1, Subject: "hello"}},
	}
	service := NewService(client)

	listResult, err := service.ListRecent(context.Background(), config.AccountConfig{}, "INBOX", 20, 5)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if client.listMailbox != "INBOX" || client.listLimit != 20 || client.listOffset != 5 {
		t.Fatalf("ListRecent delegated wrong arguments: mailbox=%q limit=%d offset=%d", client.listMailbox, client.listLimit, client.listOffset)
	}
	if len(listResult.Summaries) != 1 || listResult.Summaries[0].UID != 1 || listResult.Total != 1 {
		t.Fatalf("unexpected list result: %+v", listResult)
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

	_, err := service.ListRecent(context.Background(), config.AccountConfig{}, "INBOX", 20, 0)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("ListRecent error = %v, want %v", err, expectedErr)
	}

	_, err = service.GetByUID(context.Background(), config.AccountConfig{}, "INBOX", 9)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("GetByUID error = %v, want %v", err, expectedErr)
	}
}
