package main

import (
	"bytes"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"rsautil"
	"tag"
	"time"
	"typedef"

	"gopack/tagrpc"
	"gopack/xbyte"
)

var (
	SN   string = "014223586595610"
	err  error
	info typedef.GenericInfo

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	hubUDPAddr string = "192.168.1.150:2000"
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

	///-------------------------------------------
	tcpAddr, err := net.ResolveTCPAddr("tcp", "192.168.1.150:8083")
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	conn, err := tagrpc.DialTCP(nil, tcpAddr)
	if err != nil {
		fmt.Println("DialTCP:", err)
		return
	}

	configureTcp(conn)
	fmt.Printf("Подключился к серверу %s\n", conn.Tcp.RemoteAddr())
	go func(*tagrpc.TCPConn) {
		info.Busy = true
		defer func() {
			info.Busy = false
		}()
		for {
			err = conn.Update(time.Second * 60)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}(conn)

	///-------------------------------------------
	UDPAddr, err := net.ResolveUDPAddr("udp", hubUDPAddr)
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

	SNBytes := [16]byte{}
	copy(SNBytes[:], []byte(SN))
	deviceInfo := typedef.GenericInfo{SN: SNBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	info = deviceInfo
	for {
		telemetry, err := xbyte.StructToByte(deviceInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(UDPAddr, uint16(3073), telemetry)
		if err != nil {
			fmt.Println("UdpWrite:", err)
			continue
		}
		// fmt.Println(n)
		time.Sleep(time.Second * 5)
	}
}

// TCP
func configureTcp(conn *tagrpc.TCPConn) {
	conn.HandleFunc(1, remoteErr)
	conn.HandleFunc(2, rsaSetup)
	conn.HandleFunc(3, sendGenericInfo)
	conn.HandleFunc(1026, connectToServer)
	conn.HandleFunc(2053, executeJsonRPC)
}

func remoteErr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return errors.New(fmt.Sprint("remoteErr: ", string(val)))
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

func sendGenericInfo(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	telemetry, err := xbyte.StructToByte(info)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(3, telemetry)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	return
}

func connectToServer(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	if info.Busy {
		return
	}

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
	err = n.Response(1026, []byte("OK"))
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	go func(*tagrpc.TCPConn) {
		info.Busy = true
		defer func() {
			info.Busy = false
		}()
		for {
			err = conn.Update(time.Second * 60)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}(conn)

	return
}

func executeJsonRPC(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	val = bytes.TrimRightFunc(val, func(r rune) bool {
		return r == 0
	})

	body := bytes.NewReader(val)
	resp, err := http.Post("http://localhost/ubus", "application/json", body)
	if err != nil {
		err = fmt.Errorf("%s %s", "POST", err.Error())
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("%s %s", "ReadAll", err.Error())
		return
	}

	n.Response(2053, bodyBytes)
	return
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
	if info.Busy {
		return
	}

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
	go func(*tagrpc.TCPConn) {
		for {
			err = conn.Update(time.Second * 100)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}(conn)
	return
}
