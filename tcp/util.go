package tcp

import (
	"crypto/rsa"
	"fmt"
	"gopack/tagrpc"
	"gopack/xbyte"
	"net"
)

func InitConnect(addr string, publicKey *rsa.PublicKey, privateKey *rsa.PrivateKey) (conn *tagrpc.TCPConn, err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		err = fmt.Errorf("ResolveTCPAddr: %s", err)
		return
	}

	conn, err = tagrpc.DialTCP(nil, tcpAddr)
	if err != nil {
		err = fmt.Errorf("DialTCP: %s", err)
		return
	}

	serverPublicKey, err := RsaKeyExchange(conn, publicKey)
	if err != nil {
		err = fmt.Errorf("RsaKeyExchange: %s", err)
		return
	}

	conn.Codec = tagrpc.NewRsaCodec(privateKey, serverPublicKey)
	// telemetry, err := xbyte.StructToByte(defInfo)
	// if err != nil {
	// 	err = fmt.Errorf("StructToByte: %s", err)
	// 	return
	// }

	// err = conn.Write(tag, telemetry)
	// if err != nil {
	// 	err = fmt.Errorf("conn.Write: %s", err)
	// 	return
	// }

	return
}

func RsaKeyExchange(conn *tagrpc.TCPConn, publicKey *rsa.PublicKey) (rPublicKey *rsa.PublicKey, err error) {
	if conn == nil {
		return nil, fmt.Errorf("%s", "No connection")
	}

	if publicKey == nil {
		return nil, fmt.Errorf("%s", "No public key")
	}

	keyCh := make(chan *rsa.PublicKey, 1)
	errCh := make(chan error, 1)
	go func() {
		_, val, err := conn.Read()
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
	err = conn.Write(2, dst)
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
