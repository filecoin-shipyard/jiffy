package motion

import (
	"github.com/filecoin-shipyard/jiffy"
	"github.com/filecoin-shipyard/telefil"
)

type (
	// Option represents a configurable parameter in Store.
	Option  func(*options) error
	options struct {
		telefilOptions []telefil.Option
		jiffyOptions   []jiffy.Option
	}
)

func newOptions(o ...Option) (*options, error) {
	var opts options
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	return &opts, nil
}

// WithTelefilOptions sets the options used to instantiate a telefil.Telefil Filecoin chain API client.
func WithTelefilOptions(opts ...telefil.Option) Option {
	return func(o *options) error {
		o.telefilOptions = opts
		return nil
	}
}

// WithJiffyOptions sets the options used to instantiate the jiffy.Jiffy backing store.
func WithJiffyOptions(opts ...jiffy.Option) Option {
	return func(o *options) error {
		o.jiffyOptions = opts
		return nil
	}
}
