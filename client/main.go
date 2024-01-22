package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"net"
	"tcp"
	"time"

	"gopack/tlv"
	"gopack/xbyte"
	"rsautil"
)

type DeviceInfo struct {
	Mac    [32]byte
	Uptime int64
}

var (
	err       error
	telemetry []byte

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	hubUDPAddr *net.UDPAddr
)

func handleUDPConn(conn *net.UDPConn) { // Должен перестать слушать, когда клиент подключается к роуетеру или серверу
	for {
		data := make([]byte, 1024)
		_, addr, err := conn.ReadFromUDP(data)
		if err != nil {
			fmt.Println("ReadFromUDP:", err)
			continue
		}

		if addr.String() != hubUDPAddr.String() {
			fmt.Println("Unknown addr")
			continue
		}

		var buf bytes.Buffer
		_, err = buf.Write(data)
		if err != nil {
			fmt.Println("Buf write:", err)
			continue
		}

		rw := tlv.NewReadWriter(&buf)
		tag, val, err := rw.Read()
		if err != nil {
			fmt.Println("Tlv", err)
			continue
		}

		buf.Reset()
		switch tag {
		case 1025:
			fmt.Println("Подключается к хабу")
			hubAddr, err := net.ResolveTCPAddr("tcp", string(val))
			if err != nil {
				fmt.Println("ResolveTCPAddr:", err)
				continue
			}

			conn, err := net.DialTCP("tcp", nil, hubAddr)
			if err != nil {
				fmt.Println("DialTCP:", err)
				continue
			}

			hubPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
			if err != nil {
				conn.Close()
				fmt.Println("RsaKeyExchange:", err)
				continue
			}

			rsaConn := tcp.NewRsaConn(hubPublicKey, privateKey, conn)
			err = rsaConn.Write(3073, telemetry)
			if err != nil {
				rsaConn.Close()
				fmt.Println("RSAConn:", err)
				continue
			}

			go handleTCPConn(rsaConn)
		case 1:
			fmt.Println("Hub err response:", string(val))
		default:
			rw.Write(1, []byte("Unknown tag"))
		}
	}
}

func handleTCPConn(conn *tcp.RsaConn) {
	for {
		tag, val, err := conn.Read()
		if err != nil {
			conn.Close()
			return
		}

		switch tag {
		case 1:
			fmt.Println("Hub err response:", string(val))
		case 1026:
			go connectToServer(string(val))
			conn.Close()
			return
		default:
			conn.Write(1, []byte("Unknown tag"))
		}
	}
}

func connectToServer(addr string) {
	fmt.Println("Подключается к северу")
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("DialTCP:", err)
		return
	}

	serverPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
	if err != nil {
		conn.Close()
		fmt.Println("RsaKeyExchange:", err)
		return
	}

	rsaConn := tcp.NewRsaConn(serverPublicKey, privateKey, conn)
	defer rsaConn.Close()

	err = rsaConn.Write(3073, telemetry)
	if err != nil {
		fmt.Println("RSAConn:", err)
		return
	}
}

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

	laddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		fmt.Println("ResolveUDPAddr:", err)
		return
	}

	hubUDPAddr, err = net.ResolveUDPAddr("udp", "localhost:2000")
	if err != nil {
		fmt.Println("ResolveUDPAddr:", err)
		return
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		fmt.Println("ListenUDP:", err)
		return
	}

	defer conn.Close()
	go handleUDPConn(conn)

	for {
		var reqBuf bytes.Buffer
		macBytes := [32]byte{}
		copy(macBytes[:], []byte("AB:15:31:AA:93:26"))
		deviceInfo := DeviceInfo{macBytes, time.Now().Unix() - 1000}
		telemetry, err = xbyte.StructToByte(deviceInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}
		rw := tlv.NewReadWriter(&reqBuf)
		err = rw.Write(uint16(3073), telemetry)
		if err != nil {
			fmt.Println("Tlv:", err)
			continue
		}

		_, err = conn.WriteToUDP(reqBuf.Bytes(), hubUDPAddr)
		if err != nil {
			fmt.Println("WriteToUDP:", err)
			continue
		}

		// fmt.Println(n)
		time.Sleep(time.Second * 5)
	}
}
