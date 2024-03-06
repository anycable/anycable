package identity

import (
	"fmt"

	"github.com/anycable/anycable-go/common"
)

// PublicIdentifier identifies all clients and use their sid as the only identifier
type PublicIdentifier struct {
}

var _ Identifier = (*PublicIdentifier)(nil)

func NewPublicIdentifier() *PublicIdentifier {
	return &PublicIdentifier{}
}

func (pi *PublicIdentifier) Identify(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	return &common.ConnectResult{
		Identifier:    publicIdentifiers(sid),
		Transmissions: []string{actionCableWelcomeMessage(sid)},
		Status:        common.SUCCESS,
	}, nil
}

func publicIdentifiers(sid string) string {
	return fmt.Sprintf(`{"sid":"%s"}`, sid)
}
