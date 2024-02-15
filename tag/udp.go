package tag

import (
	"errors"
	"fmt"
	"net"
)

type Udp struct {
	Raddr  *net.UDPAddr
	conn   *TagConn
	handle map[uint16]HandleFunc
}

type HandleFunc func(*Udp, uint16, []byte) error

func NewUdp(addr string) (*Udp, error) {
	conn, err := listenUDP(addr)
	if err != nil {
		err = fmt.Errorf("ListenUDP: %s", err)
		return nil, err
	}

	return &Udp{
			Raddr:  nil,
			conn:   conn,
			handle: make(map[uint16]HandleFunc)},
		nil
}

func (u *Udp) HandleFunc(tag uint16, handle HandleFunc) {
	u.handle[tag] = handle
}

func (u *Udp) ReadAndExec() (err error) {
	tag, val, err := u.Read()

	if tag == 1 {
		err = fmt.Errorf("remote err: %s", val)
		return
	}

	_, ok := u.handle[tag]
	if !ok {
		u.Write(u.Raddr, 1, []byte("unknown tag"))
		return
	}

	err = u.handle[tag](u, tag, val)
	if err != nil {
		u.Write(u.Raddr, 1, []byte(err.Error()))
		err = fmt.Errorf("%s", err)
		return
	}

	return
}

func (u *Udp) Execute(tag uint16, value []byte) (err error) {
	if u.Raddr == nil {
		return errors.New("remote addr is nil")
	}

	if tag == 1 {
		err = fmt.Errorf("remote err: %s", value)
		return
	}

	_, ok := u.handle[tag]
	if !ok {
		u.Write(u.Raddr, 1, []byte("unknown tag"))
		return
	}

	err = u.handle[tag](u, tag, value)
	if err != nil {
		u.Write(u.Raddr, 1, []byte(err.Error()))
		err = fmt.Errorf("%s", err)
		return
	}

	return
}

func (u *Udp) Read() (tag uint16, val []byte, err error) {
	addr, tag, val, err := u.conn.read()
	u.Raddr = addr
	return
}

func (u *Udp) Write(addr *net.UDPAddr, tag uint16, val []byte) (int, error) {
	return u.conn.write(addr, tag, val)
}

func (u *Udp) Close() error {
	return u.conn.close()
}
