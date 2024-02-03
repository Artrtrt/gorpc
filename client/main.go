package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"net"
	"tcp"
	"time"
	"typedef"

	"gopack/tagrpc"
	"gopack/tlv"
	"gopack/xbyte"
	"rsautil"
)

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
			hubAddr, err := net.ResolveTCPAddr("tcp", string(val))
			if err != nil {
				fmt.Println("ResolveTCPAddr:", err)
				continue
			}

			tcpconn, err := tagrpc.DialTCP(nil, hubAddr)
			if err != nil {
				fmt.Println("DialTCP:", err)
				continue
			}

			hubPublicKey, err := tcp.RsaKeyExchange(tcpconn, publicKey)
			if err != nil {
				conn.Close()
				fmt.Println("RsaKeyExchange:", err)
				continue
			}

			rsacodec := tagrpc.NewRsaCodec(privateKey, hubPublicKey)
			tcpconn.Codec = rsacodec
			err = tcpconn.Write(3073, telemetry)
			if err != nil {
				tcpconn.Close()
				fmt.Println("tcpconn:", err)
				continue
			}

			fmt.Println("Подключился к хабу")
			go handleTCPConn(tcpconn)
		case 1:
			fmt.Println("Hub err response:", string(val))
		default:
			rw.Write(1, []byte("Unknown tag"))
		}
	}
}

func handleTCPConn(conn *tagrpc.TCPConn) {
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
			val = bytes.TrimRightFunc(val, func(r rune) bool {
				return r == 0
			})
			go connectToServer(string(val))
			conn.Close()
			return
		default:
			conn.Write(1, []byte("Unknown tag"))
		}
	}
}

func connectToServer(addr string) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	conn, err := tagrpc.DialTCP(nil, tcpAddr)
	if err != nil {
		fmt.Println("DialTCP:", err)
		return
	}

	raddr := conn.Tcp.RemoteAddr().String()
	defer func() {
		fmt.Printf("Соединение с сервером %s разорвано\n", raddr)
		conn.Close()
	}()

	serverPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
	if err != nil {
		fmt.Println("RsaKeyExchange:", err)
		return
	}

	conn.Codec = tagrpc.NewRsaCodec(privateKey, serverPublicKey)
	err = conn.Write(3073, telemetry)
	if err != nil {
		fmt.Println("RSAConn:", err)
		return
	}

	fmt.Printf("Подключился к северу %s\n", raddr)
	for {
		tag, val, err := conn.Read()
		if err != nil {
			fmt.Println("Conn read:", err)
			return
		}

		fmt.Println(tag, val)
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
		deviceInfo := typedef.DeviceInfo{macBytes, time.Now().Unix() - 1000}
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
