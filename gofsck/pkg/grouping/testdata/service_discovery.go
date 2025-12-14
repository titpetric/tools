package model

type ServiceDiscovery struct{}

func (s *ServiceDiscovery) Set() string {
	return "Discovery"
}
