package tcp

import (
	"crypto/rsa"
	"fmt"
	"gopack/rsautil"
	"gopack/tlv"
	"net"
)

type RsaConn struct {
	rPublicKey *rsa.PublicKey
	privateKey *rsa.PrivateKey
	conn       *net.TCPConn
}

func NewRsaConn(rPublicKey *rsa.PublicKey, privateKey *rsa.PrivateKey, conn *net.TCPConn) *RsaConn {
	return &RsaConn{
		rPublicKey: rPublicKey,
		privateKey: privateKey,
		conn:       conn,
	}
}

func (c *RsaConn) Write(tag uint16, val []byte) (err error) {
	w := tlv.NewWriter(c.conn)
	enc, err := rsautil.EncryptPKCS1(c.rPublicKey, val)
	if err != nil {
		fmt.Println("EncryptPKCS1", err)
		return
	}

	err = w.Write(tag, enc)
	if err != nil {
		fmt.Println("Tlv", err)
		return
	}

	return
}

func (c *RsaConn) Read() (tag uint16, val []byte, err error) {
	r := tlv.NewReader(c.conn)
	tag, enc, err := r.Read()
	if err != nil {
		fmt.Println("Tlv", err)
		return
	}

	val, err = rsautil.DecryptPKCS1(c.privateKey, enc)
	if err != nil {
		fmt.Println("DecryptPKCS1", err)
		return
	}

	return
}

func (c *RsaConn) Close() error {
	return c.conn.Close()
}
