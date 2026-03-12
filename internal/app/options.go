package app

type Options struct {
	Account string
	Mailbox string
	Limit   int
	Offset  int
	Format  string
	UID     uint32
}

type AccountDefaults struct {
	Mailbox  string
	PageSize int
	Format   string
}

type Request struct {
	Mailbox   string
	Limit     int
	Offset    int
	Format    string
	DetailUID uint32
}

func BuildRequest(options Options, defaults AccountDefaults) Request {
	mailbox := defaults.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}

	limit := defaults.PageSize
	if limit == 0 {
		limit = 20
	}

	format := defaults.Format
	if format == "" {
		format = "plain"
	}

	if options.Mailbox != "" {
		mailbox = options.Mailbox
	}
	if options.Limit > 0 {
		limit = options.Limit
	}
	if options.Format != "" {
		format = options.Format
	}

	offset := 0
	if options.Offset > 0 {
		offset = options.Offset
	}

	return Request{
		Mailbox:   mailbox,
		Limit:     limit,
		Offset:    offset,
		Format:    format,
		DetailUID: options.UID,
	}
}
