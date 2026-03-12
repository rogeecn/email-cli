package imap

import (
	"context"

	"github.com/rogeecn/email-cli/internal/app"
	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
)

type Client interface {
	ListRecent(ctx context.Context, mailbox string, limit int, offset int) (app.ListResult, error)
	GetByUID(ctx context.Context, mailbox string, uid uint32) (mail.Detail, error)
}

type Service struct {
	client Client
}

func NewService(client Client) Service {
	return Service{client: client}
}

func (s Service) ListRecent(ctx context.Context, account config.AccountConfig, mailbox string, limit int, offset int) (app.ListResult, error) {
	_ = account
	return s.client.ListRecent(ctx, mailbox, limit, offset)
}

func (s Service) GetByUID(ctx context.Context, account config.AccountConfig, mailbox string, uid uint32) (mail.Detail, error) {
	_ = account
	return s.client.GetByUID(ctx, mailbox, uid)
}
