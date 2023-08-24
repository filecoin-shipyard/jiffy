package jiffy

import (
	"net/url"
)

var (
	_ Offloader = (*httpOffloader)(nil)
)

type (
	Offloader interface {
		Offload(*Piece) (*Offload, error)
	}
	Offload struct {
		Type    string
		URL     *url.URL
		Headers map[string]string
	}
	httpOffloader struct {
		j *Jiffy
	}
)

func newHttpOffloader(j *Jiffy) (*httpOffloader, error) {
	return &httpOffloader{j: j}, nil
}

func (h *httpOffloader) Offload(_ *Piece) (*Offload, error) {
	// TODO implement
	// TODO add auth : "golang.org/x/oauth2"
	return &Offload{
		Type:    "http",
		URL:     nil,
		Headers: nil,
	}, nil
}
