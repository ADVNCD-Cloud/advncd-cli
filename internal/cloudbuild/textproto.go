package cloudbuild

import "net/textproto"

type textprotoMIMEHeader map[string][]string

func (h textprotoMIMEHeader) Set(k, v string) {
	h[k] = []string{v}
}

func (h textprotoMIMEHeader) std() textproto.MIMEHeader {
	return textproto.MIMEHeader(h)
}