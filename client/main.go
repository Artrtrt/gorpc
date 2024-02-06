package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"gopack/tagrpc"
	"gopack/xbyte"
	"net"
	"rsautil"
	"tag"
	"tcp"
	"time"
	"typedef"
)

var (
	err  error
	info typedef.DeviceInfo

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	hubUDPAddr *net.UDPAddr
)

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
	configureUdp(udp)

	macBytes := [32]byte{}
	copy(macBytes[:], []byte("AB:15:31:AA:93:26"))
	deviceInfo := typedef.DeviceInfo{Mac: macBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	info = deviceInfo
	sendData(udp, info)
}

func configureUdp(udp *tag.Udp) {
	udp.Handle(1025, connectToHub)

	go func() {
		err := udp.ReadLoop()
		if err != nil {
			udp.Close()
			fmt.Println("ReadLoop:", err)
			return
		}
	}()
}

func connectToHub(u *tag.Udp, tag uint16, val []byte) error {
	conn, err := InitConnect(string(val))
	if err != nil {
		err = fmt.Errorf("InitConnect: %s", err)
		conn.Close()
		return err
	}

	fmt.Println("Подключился к хабу")
	go handleTCPConn(conn)
	return nil
}

func sendData(udp *tag.Udp, deviceInfo typedef.DeviceInfo) {
	for {
		telemetry, err := xbyte.StructToByte(deviceInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		udp.Write(hubUDPAddr, uint16(3073), telemetry)
		// fmt.Println(n)
		time.Sleep(time.Second * 5)
	}
}
