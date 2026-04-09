// Event bus layer
package eventbus

// Bus entry point
type Bus struct {
	Client *Client
	Pub    *Publisher
	Sub    *Subscriber
}

// New bus
func New(redisURL string) (*Bus, error) {
	c, err := NewClient(redisURL)
	if err != nil {
		return nil, err
	}
	return &Bus{
		Client: c,
		Pub:    NewPublisher(c),
		Sub:    NewSubscriber(c),
	}, nil
}

// Close bus
func (b *Bus) Close() error {
	return b.Client.Close()
}
