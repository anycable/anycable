package node_mocks

import (
	node "github.com/anycable/anycable-go/node"
)

func SessionMatcher(expected *node.Session) func(actual interface{}) bool {
	return func(actual interface{}) bool {
		act, ok := actual.(*node.Session)

		if !ok {
			return false
		}

		return expected.GetID() == act.GetID()
	}
}
