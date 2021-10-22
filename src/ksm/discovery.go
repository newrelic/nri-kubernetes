package ksm

type discoverer struct{}

type Discoverer interface {
	Discover() ([]string, error)
}

func NewDiscoverer() (Discoverer, error) {
	return &discoverer{}, nil
}

func (d *discoverer) Discover() ([]string, error) {
	return []string{"localhost:8080"}, nil
}
