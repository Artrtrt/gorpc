package main

import (
	"bytes"
	"crypto/rsa"
	"errors"
	"fmt"
	"gopack/tagrpc"
	"gopack/xbyte"
	"net"
	"rsautil"
	"tag"
	"time"
	"typedef"
)

var (
	err  error
	info typedef.GenericInfo

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	hubUDPAddr *net.UDPAddr
)

func main() {
	publicKey, err = rsautil.PemToPublicKey("public.pem")
	if err != nil {
		fmt.Println("PemToPublicKey:", err)
		return
	}

	privateKey, err = rsautil.PemToPrivateKey("private.pem")
	if err != nil {
		fmt.Println("PemToPublicKey:", err)
		return
	}

	hubUDPAddr, err = net.ResolveUDPAddr("udp", "localhost:2000")
	if err != nil {
		fmt.Println("ResolveUDPAddr:", err)
		return
	}

	udp, err := tag.NewUdp(":0")
	if err != nil {
		fmt.Println("NewUdp:", err)
		return
	}

	defer udp.Close()
	go configureUdp(udp)

	macBytes := [32]byte{}
	copy(macBytes[:], []byte("AB:15:31:AA:93:26"))
	deviceInfo := typedef.GenericInfo{Mac: macBytes, Uptime: time.Now().Unix() - 1000}
	info = deviceInfo
	sendData(udp, info)
}

// TCP
func configureTcp(conn *tagrpc.TCPConn) {
	conn.HandleFunc(1, remoteErr)
	conn.HandleFunc(2, rsaSetup)
	conn.HandleFunc(1026, connectToServer)
	conn.HandleFunc(1028, sendGenericInfo)
}

func rsaSetup(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	rPublicKey, err := xbyte.ByteToRsaPublic(val)
	if err != nil {
		err = fmt.Errorf("%s %s", "ByteToRsaPublic:", err)
		return
	}

	dst, err := xbyte.RsaPublicToByte(publicKey)
	if err != nil {
		err = fmt.Errorf("%s %s", "RsaPublicToByte:", err)
		return
	}

	err = n.Response(2, dst)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	n.Codec = tagrpc.NewRsaCodec(privateKey, rPublicKey)
	return
}

func connectToServer(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	val = bytes.TrimRightFunc(val, func(r rune) bool {
		return r == 0
	})

	tcpAddr, err := net.ResolveTCPAddr("tcp", string(val))
	if err != nil {
		err = fmt.Errorf("ResolveTCPAddr: %s", err)
		return
	}

	conn, err := tagrpc.DialTCP(nil, tcpAddr)
	if err != nil {
		err = fmt.Errorf("DialTCP: %s", err)
		return
	}

	configureTcp(conn)
	fmt.Printf("Подключился к серверу %s\n", conn.Tcp.RemoteAddr())
	for {
		err = conn.Update(time.Second * 60)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func sendGenericInfo(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	telemetry, err := xbyte.StructToByte(info)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(1028, telemetry)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	return
}

func remoteErr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return errors.New(fmt.Sprint("remoteErr: ", string(val)))
}

// UDP
func configureUdp(udp *tag.Udp) {
	udp.HandleFunc(1025, connectToHub)

	for {
		err := udp.ReadAndExec()
		if err != nil {
			fmt.Println("udp readAndExec:", err)
			continue
		}
	}
}

func connectToHub(u *tag.Udp, tag uint16, val []byte) (err error) {
	hubAddr, err := net.ResolveTCPAddr("tcp", string(val))
	if err != nil {
		err = fmt.Errorf("ResolveTCPAddr: %s", err)
		return
	}

	conn, err := tagrpc.DialTCP(nil, hubAddr)
	if err != nil {
		err = fmt.Errorf("DialTCP: %s", err)
		return
	}

	configureTcp(conn)
	fmt.Println("Подключился к хабу")
	go func() {
		for {
			err = conn.Update(time.Second * 100)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}()
	return
}

func sendData(udp *tag.Udp, deviceInfo typedef.GenericInfo) {
	for {
		telemetry, err := xbyte.StructToByte(deviceInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(hubUDPAddr, uint16(3073), telemetry)
		if err != nil {
			fmt.Println("UdpWrite:", err)
			continue
		}
		// fmt.Println(n)
		time.Sleep(time.Second * 5)
	}
}
