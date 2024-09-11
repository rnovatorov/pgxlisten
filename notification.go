package pgxlisten

type Notification struct {
	ConnectionReset bool
	Payload         string
}
