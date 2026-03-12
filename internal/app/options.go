package app

type Options struct {
	Mailbox string
	Limit   int
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

	return Request{
		Mailbox:   mailbox,
		Limit:     limit,
		Format:    format,
		DetailUID: options.UID,
	}
}
