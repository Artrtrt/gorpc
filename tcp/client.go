package tcp

import (
	"crypto/rsa"
	"fmt"
	"gopack/rsautil"
	"gopack/tlv"
	"gopack/xbyte"
	"net"
)

type TCP struct {
	RPublicKey *rsa.PublicKey
	Raddr      *net.TCPAddr
	PublicKey  *rsa.PublicKey
	Conn       *net.TCPConn
	Rw         tlv.ReadWriter
}

func NewTCP(publicKey *rsa.PublicKey, raddr *net.TCPAddr) *TCP {
	return &TCP{
		RPublicKey: nil,
		Raddr:      raddr,
		PublicKey:  publicKey,
		Conn:       nil,
		Rw:         nil,
	}
}

func (tcp *TCP) Dial() (err error) {
	conn, err := net.DialTCP("tcp", nil, tcp.Raddr)
	if err != nil {
		err = fmt.Errorf("DialTCP %s", err)
		return
	}

	tcp.Rw = tlv.NewReadWriter(conn)
	tcp.Conn = conn

	err = tcp.rsaSetup()
	if err != nil {
		conn.Close()
		err = fmt.Errorf("rsaSetup %s", err)
		return
	}

	return
}

func (tcp *TCP) Close() error {
	return tcp.Conn.Close()
}

func (tcp *TCP) rsaSetup() (err error) {
	readCh := make(chan bool, 1)
	go func() {
		defer func() {
			readCh <- true
		}()

		_, val, err := tcp.Rw.Read()
		if err != nil {
			fmt.Println("Read public key", err)
			return
		}

		tcp.RPublicKey, err = xbyte.ByteToRsaPublic(val)
		if err != nil {
			fmt.Println("ByteToRsaPublic", err)
			return
		}
	}()

	dst, err := xbyte.RsaPublicToByte(tcp.PublicKey)
	if err != nil {
		fmt.Println("RsaPublicToByte", err)
		return
	}
	err = tcp.Rw.Write(2, dst)
	if err != nil {
		fmt.Println("Send public key", err)
		return
	}

	<-readCh
	return
}

func (tcp *TCP) Write(tag uint16, val []byte) (err error) {
	enc, err := rsautil.EncryptPKCS1(tcp.RPublicKey, val)
	if err != nil {
		fmt.Println("EncryptPKCS1", err)
		return
	}

	err = tcp.Rw.Write(tag, enc)
	if err != nil {
		fmt.Println("Tlv", err)
		return
	}

	return
}
