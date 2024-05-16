package streams

import (
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/datastreams"
	"github.com/smartcontractkit/chainlink-common/pkg/values"
)

type Codec struct {
}

func (c Codec) UnwrapValid(wrapped values.Value, allowedSigners [][]byte, minRequiredSignatures int) ([]datastreams.FeedReport, error) {
	dest := []datastreams.FeedReport{}
	err := wrapped.UnwrapTo(&dest)
	// TODO (KS-196): validate reports
	return dest, err
}

func (c Codec) Wrap(reports []datastreams.FeedReport) (values.Value, error) {
	return values.Wrap(reports)
}

func NewCodec() Codec {
	return Codec{}
}
