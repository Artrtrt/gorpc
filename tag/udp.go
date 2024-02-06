package tag

import (
	"fmt"
	"net"
)

type Udp struct {
	conn   *TagConn
	handle map[uint16]HandleFunc
}

type HandleFunc func(*Udp, uint16, []byte) error

func NewUdp(addr string) (*Udp, error) {
	conn, err := ListenUDP(addr)
	if err != nil {
		err = fmt.Errorf("NewConn: %s", err)
		return nil, err
	}

	return &Udp{conn: conn, handle: make(map[uint16]HandleFunc)}, nil
}

func (u *Udp) Handle(tag uint16, handlefunc HandleFunc) {
	u.handle[tag] = handlefunc
}

func (u *Udp) ReadLoop() error {
	for {
		addr, tag, val, err := u.Read()
		if err != nil {
			err = fmt.Errorf("tlv: %s", err)
			return err
		}

		_, ok := u.handle[tag]
		if !ok {
			u.Write(addr, 1, []byte("Unknown tag"))
			continue
		}

		if tag == 1 {
			err = fmt.Errorf("remote err: %s", val)
			return err
		}

		err = u.handle[tag](u, tag, val)
		if err != nil {
			err = fmt.Errorf("%s", err)
			return err
		}
	}
}

func (u *Udp) Read() (addr *net.UDPAddr, tag uint16, val []byte, err error) {
	return u.conn.Read()
}

func (u *Udp) Write(addr *net.UDPAddr, tag uint16, val []byte) (int, error) {
	return u.conn.Write(addr, tag, val)
}

func (u *Udp) Close() error {
	return u.conn.Close()
}
