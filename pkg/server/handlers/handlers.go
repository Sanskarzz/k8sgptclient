package handlers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientHandler holds the Kubernetes client
type ClientHandler struct {
	Client client.Client
}

// NewClientHandler creates a new ClientHandler
func NewClientHandler(client client.Client) *ClientHandler {
	return &ClientHandler{
		Client: client,
	}
}
