package endpoint

import (
	"context"
	"crypto/tls"
	"time"

	"golang.org/x/net/quic"
)

type DefaultEndpoint struct {
	Address    string
	Attributes map[string]string
	conn       *quic.Conn
}

func NewDefaultEndpoint(ctx context.Context, address string, kvs ...string) (*DefaultEndpoint, error) {
	de := &DefaultEndpoint{Address: address}

	if len(kvs) > 0 && len(kvs)%2 == 0 {
		attributes := make(map[string]string, len(kvs)/2)
		for i := 0; i < len(kvs); i += 2 {
			attributes[kvs[i]] = kvs[i+1]
		}
		de.Attributes = attributes
	}

	quicEndpoint := &quic.Endpoint{}
	conn, err := quicEndpoint.Dial(ctx, "udp", de.Address, &quic.Config{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"akasar-rpc"},
		},
		MaxStreamReadBufferSize:  1024 * 1024 * 4,
		MaxStreamWriteBufferSize: 1024 * 1024 * 4,
		MaxConnReadBufferSize:    1024 * 1024 * 4,
		RequireAddressValidation: false,
		StatelessResetKey:        [32]byte{},
		HandshakeTimeout:         time.Second * 3,
		MaxIdleTimeout:           time.Minute * 5,
		KeepAlivePeriod:          time.Second * 30,
		QLogLogger:               nil,
	})

	if err != nil {
		return nil, err
	}

	de.conn = conn

	return de, nil
}

func (e *DefaultEndpoint) Call(ctx context.Context, data []byte) ([]byte, error) {
	stream, err := e.conn.NewStream(ctx)
	if err != nil {
		return nil, err
	}

	left := len(data)
	for {
		n, err := stream.Write(data)
		if err != nil {
			return nil, err
		}
		left -= n

		if left <= 0 {
			break
		}
		data = data[n:]
	}

	stream.Flush()

	stream.CloseWrite()

	return nil, nil
}

func (e *DefaultEndpoint) Name() string {
	return e.Address
}

func (e *DefaultEndpoint) Close() error {
	return e.conn.Close()
}
