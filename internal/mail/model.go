package mail

type Summary struct {
	UID     uint32   `json:"uid" yaml:"uid"`
	Date    string   `json:"date" yaml:"date"`
	From    string   `json:"from" yaml:"from"`
	To      []string `json:"to" yaml:"to"`
	Subject string   `json:"subject" yaml:"subject"`
	Seen    bool     `json:"seen" yaml:"seen"`
}

type Attachment struct {
	Name        string `json:"name" yaml:"name"`
	ContentType string `json:"content_type" yaml:"content_type"`
	Size        int64  `json:"size" yaml:"size"`
}

type Detail struct {
	Summary     `json:",inline" yaml:",inline"`
	CC          []string          `json:"cc,omitempty" yaml:"cc,omitempty"`
	BCC         []string          `json:"bcc,omitempty" yaml:"bcc,omitempty"`
	Flags       []string          `json:"flags,omitempty" yaml:"flags,omitempty"`
	TextBody    string            `json:"text_body,omitempty" yaml:"text_body,omitempty"`
	HTMLBody    string            `json:"html_body,omitempty" yaml:"html_body,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty" yaml:"attachments,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}
