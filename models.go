package pshandler

type inbound struct {
	Action  string `json:"action"`
	Topic   string `json:"topic"`
	Message []byte `json:"message"`
}

type outbound struct {
	Message []byte        `json:"message"`
	Error   outboundError `json:"error"`
}

type outboundError struct {
	Message             string `json:"message"`
	InternalServerError bool   `json:"internalServerError"`
}
