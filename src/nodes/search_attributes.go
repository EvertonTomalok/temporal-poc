package nodes

import (
	"go.temporal.io/sdk/temporal"
)

// Search attribute keys for client-answered tracking
var (
	// ClientAnsweredField is a boolean search attribute indicating if client has answered
	ClientAnsweredField = temporal.NewSearchAttributeKeyBool("ClientAnswered")
	// ClientAnsweredAtField is a datetime search attribute for when client answered
	ClientAnsweredAtField = temporal.NewSearchAttributeKeyTime("ClientAnsweredAt")
)
