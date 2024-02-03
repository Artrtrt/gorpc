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
	err  error
	info typedef.DeviceInfo

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
			conn, err := InitConnect(string(val))
			if err != nil {
				fmt.Println("InitConnect", err)
				conn.Close()
				continue
			}
			fmt.Println("Подключился к хабу")
			go handleTCPConn(conn)
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

			conn, err := InitConnect(string(val))
			if err != nil {
				fmt.Println("InitConnect", err)
				conn.Close()
				continue
			}

			fmt.Printf("Подключился к серверу %s\n", conn.Tcp.RemoteAddr())
			go func() { // Дальше будет доделываться(это прием инфы от сервера)
				for {
					tag, val, err := conn.Read()
					if err != nil {
						fmt.Println("Conn read:", err)
						return
					}

					fmt.Println(tag, val)
				}
			}()
		default:
			conn.Write(1, []byte("Unknown tag"))
		}
	}
}

func InitConnect(addr string) (conn *tagrpc.TCPConn, err error) {
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

	serverPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
	if err != nil {
		err = fmt.Errorf("RsaKeyExchange: %s", err)
		return
	}

	conn.Codec = tagrpc.NewRsaCodec(privateKey, serverPublicKey)
	if err != nil {
		err = fmt.Errorf("dialTcp: %s", err)
		return
	}

	telemetry, err := xbyte.StructToByte(info)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = conn.Write(3075, telemetry)
	if err != nil {
		err = fmt.Errorf("conn.Write: %s", err)
		return
	}

	return
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

	macBytes := [32]byte{}
	copy(macBytes[:], []byte("AB:15:31:AA:93:26"))
	deviceInfo := typedef.DeviceInfo{Mac: macBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	info = deviceInfo
	sendData(conn, info)
}

func sendData(conn *net.UDPConn, deviceInfo typedef.DeviceInfo) {
	for {
		var reqBuf bytes.Buffer
		telemetry, err := xbyte.StructToByte(deviceInfo)
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
