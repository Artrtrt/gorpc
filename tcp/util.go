package tcp

import (
	"crypto/rsa"
	"fmt"
	"gopack/tlv"
	"gopack/xbyte"
	"net"
)

func RsaKeyExchange(conn *net.TCPConn, publicKey *rsa.PublicKey) (rPublicKey *rsa.PublicKey, err error) {
	if conn == nil {
		return nil, fmt.Errorf("%s", "No connection")
	}

	if publicKey == nil {
		return nil, fmt.Errorf("%s", "No public key")
	}

	keyCh := make(chan *rsa.PublicKey, 1)
	errCh := make(chan error, 1)
	rw := tlv.NewReadWriter(conn)
	go func() {
		_, val, err := rw.Read()
		if err != nil {
			errCh <- fmt.Errorf("%s %s", "Read public key:", err)
			return
		}

		rPublicKey, err = xbyte.ByteToRsaPublic(val)
		if err != nil {
			errCh <- fmt.Errorf("%s %s", "ByteToRsaPublic:", err)
			return
		}

		keyCh <- rPublicKey
	}()

	dst, err := xbyte.RsaPublicToByte(publicKey)
	if err != nil {
		err = fmt.Errorf("%s %s", "RsaPublicToByte:", err)
		return
	}
	err = rw.Write(2, dst)
	if err != nil {
		err = fmt.Errorf("%s %s", "Send public key:", err)
		return
	}

	select {
	case rPublicKey = <-keyCh:
		return
	case err = <-errCh:
		return
	}
}
