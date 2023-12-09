package main

import (
	"crypto/rsa"
	"fmt"
	"gopack/rsautil"
	"net"
	"tcp"
)

var (
	err        error
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	addr       string = "localhost:8082"
)

func main() {
	publicKey, err = rsautil.PemToPublicKey("public.pem")
	if err != nil {
		fmt.Println("PemToPublicKey", err)
		return
	}

	privateKey, err = rsautil.PemToPrivateKey("private.pem")
	if err != nil {
		fmt.Println("PemToPublicKey", err)
		return
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	tpcLr, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		fmt.Println("ListenTCP:", err)
		return
	}

	defer tpcLr.Close()

	for {
		conn, err := tpcLr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP:", err)
			continue
		}

		go func() {
			clientPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
			if err != nil {
				conn.Close()
				fmt.Println("RsaKeyExchange:", err)
				return
			}

			rsaConn := tcp.NewRsaConn(clientPublicKey, privateKey, conn)
			defer rsaConn.Close()

			tag, val, err := rsaConn.Read()
			if err != nil {
				fmt.Println("Conn read:", err)
				return
			}

			fmt.Println(tag, val)
		}()
	}
}
