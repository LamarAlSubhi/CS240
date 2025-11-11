package gossip

import (
	"encoding/json"
	"net"
)

type Transport interface {
	Listen(bind string, handler func(Msg, string)) error
	Send(addr string, m Msg) error
	Close() error
}

type UDPTransport struct{ conn *net.UDPConn }

func NewUDPTransport() *UDPTransport { return &UDPTransport{} }

func (t *UDPTransport) Listen(bind string, handler func(Msg, string)) error {
	addr, err := net.ResolveUDPAddr("udp", bind)
	if err != nil {
		return err
	}
	t.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, raddr, err := t.conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			var m Msg
			if json.Unmarshal(buf[:n], &m) == nil {
				handler(m, raddr.String())
			}
		}
	}()
	return nil
}

func (t *UDPTransport) Send(addr string, m Msg) error {
	b, _ := json.Marshal(m)
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	_, err = t.conn.WriteToUDP(b, raddr)
	return err
}

func (t *UDPTransport) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
