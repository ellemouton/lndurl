package lndurl

type PayResponse struct {
	// Callback is the URL from LN SERVICE which will accept the pay request
	// parameters
	Callback string `json:"callback"`

	// MaxSendable is the max amount LN SERVICE is willing to receive
	MaxSendable string `json:"maxSendable"`

	// MinSendable is the min amount LN SERVICE is willing to receive, can
	// not be less than 1 or more than `maxSendable`
	MinSendable string `json:"minSendable"`

	// Metadata json which must be presented as raw string here, this is
	// required to pass signature verification at a later step.
	Metadata [][2]string `json:"metadata"`

	// Type of LNURL
	Tag Type `json:"tag"`
}

type InvoiceResponse struct {
	// PayRequest is a bech32-serialized lightning invoice.
	PayRequest string

	// Routes an empty array.
	Routes []string
}

type Type string

const (
	TypePayRequest = "payRequest"
)

type Error struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}
