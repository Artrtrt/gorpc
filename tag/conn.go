package tag

import (
	"bytes"
	"fmt"
	"gopack/tlv"
	"net"
)

type TagConn struct {
	conn *net.UDPConn
}

func listenUDP(addr string) (*TagConn, error) {
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		err = fmt.Errorf("ResolveUDPAddr: %s", err)
		return nil, err
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		err = fmt.Errorf("ListenUDP: %s", err)
		return nil, err
	}

	return &TagConn{conn: conn}, nil
}

func (t *TagConn) read() (raddr *net.UDPAddr, tag uint16, val []byte, err error) {
	data := make([]byte, 1024)
	_, raddr, err = t.conn.ReadFromUDP(data)
	if err != nil {
		err = fmt.Errorf("readFromUDP: %s", err)
		return
	}

	var buf bytes.Buffer
	_, err = buf.Write(data)
	if err != nil {
		err = fmt.Errorf("buf write: %s", err)
		return
	}

	rw := tlv.NewReadWriter(&buf)
	tag, val, err = rw.Read()
	return raddr, tag, val, err
}

func (t *TagConn) write(raddr *net.UDPAddr, tag uint16, val []byte) (n int, err error) {
	var buf bytes.Buffer
	rw := tlv.NewReadWriter(&buf)
	err = rw.Write(tag, val)
	if err != nil {
		err = fmt.Errorf("buf write: %s", err)
		return
	}

	return t.conn.WriteToUDP(buf.Bytes(), raddr)
}

func (t *TagConn) close() error {
	return t.conn.Close()
}
