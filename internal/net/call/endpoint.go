package call

import (
	"context"
	"fmt"
	"net"
	"strings"
)

type Endpoint interface {
	//Call(ctx context.Context, data []byte) ([]byte, error)
	//Close() error
	//Name() string

	Dial(ctx context.Context) (net.Conn, error)

	Address() string
}

func TCP(address string) NetEndpoint {
	return NetEndpoint{"tcp", address}
}

func Unix(address string) NetEndpoint {
	return NetEndpoint{"unix", address}
}

func UDP(address string) NetEndpoint {
	return NetEndpoint{"udp", address}
}

type NetEndpoint struct {
	Net  string // e.g., "tcp", "udp", "unix"
	Addr string // e.g., "localhost:8000"
}

func (n NetEndpoint) Dial(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, n.Net, n.Addr)
}

func (n NetEndpoint) Address() string {
	return fmt.Sprintf("%s://%s", n.Net, n.Addr)
}

func ParseNetEndpoint(endpoint string) (NetEndpoint, error) {
	n, addr, ok := strings.Cut(endpoint, "://")
	if !ok {
		return NetEndpoint{}, fmt.Errorf("%q does not have format <network>://<address>", endpoint)
	}
	return NetEndpoint{n, addr}, nil
}
