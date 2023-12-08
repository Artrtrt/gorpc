package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"gopack/rsautil"
	"gopack/tlv"
	"net"
	"time"

	"gopack/xbyte"
)

type DeviceInfo struct {
	Mac    [32]byte
	Uptime int64
}

var (
	err       error
	telemetry []byte

	privateKey   *rsa.PrivateKey
	publicKey    *rsa.PublicKey
	hubPublicKey *rsa.PublicKey

	hubUDPAddr *net.UDPAddr
	hubTCPAddr *net.TCPAddr
	toConnCh   chan bool
	TcpEn      bool
)

func handleUDPConn(conn *net.UDPConn, ch chan bool) { // Должен перестать слушать, когда клиент подключается к роуетеру
	for {
		data := make([]byte, 1024)
		_, addr, err := conn.ReadFromUDP(data)
		if err != nil {
			fmt.Println("ReadFromUDP", err)
			continue
		}

		if addr.String() != hubUDPAddr.String() {
			fmt.Println("Unknown addr")
			continue
		}

		var buf bytes.Buffer
		_, err = buf.Write(data)
		if err != nil {
			fmt.Println("Write", err)
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
			if TcpEn {
				return
			}

			hubTCPAddr, err = net.ResolveTCPAddr("tcp", string(val))
			if err != nil {
				fmt.Println("ResolveTCPAddr", err)
				continue
			}

			fmt.Println("Пробует подключиться по tcp")
			toConnCh <- true
		case 1:
			fmt.Println("Hub err response:", string(val))
		default:
			rw.Write(1, []byte("Unknown tag"))
		}
	}
}

func handleTCPConn(rw tlv.ReadWriter) error {
	tag, val, err := rw.Read()
	if err != nil {
		return err
	}

	switch tag {
	case 1:
		fmt.Println("Hub err response:", string(val))
	case 1026:
		serverIp, err := rsautil.DecryptPKCS1(privateKey, val)
		if err != nil {
			return err
		}

		fmt.Println(string(serverIp))
		// connectToServer()

	default:
		rw.Write(1, []byte("Unknown tag"))
	}

	return nil
}

func main() {
	toConnCh = make(chan bool, 1)
	TcpEn = false

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

	laddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		fmt.Println("ResolveUDPAddr", err)
		return
	}

	hubUDPAddr, err = net.ResolveUDPAddr("udp", "localhost:2000")
	if err != nil {
		fmt.Println("ResolveUDPAddr", err)
		return
	}

	go func() { // tcp подключение к хабу для получения ip сервера
		for {
			<-toConnCh
			conn, err := net.DialTCP("tcp", nil, hubTCPAddr)
			if err != nil {
				fmt.Println("DialTCP", err)
				continue
			}

			TcpEn = true
			hubPublicKey, err = rsaSetup(conn, publicKey)
			if err != nil {
				fmt.Println("rsaSetup", err)
				conn.Close()
				TcpEn = false
				continue
			}

			enc, err := rsautil.EncryptPKCS1(hubPublicKey, telemetry)
			if err != nil {
				fmt.Println("EncryptPKCS1", err)
				conn.Close()
				TcpEn = false
				continue
			}

			rw := tlv.NewReadWriter(conn)
			err = rw.Write(3073, enc)
			if err != nil {
				fmt.Println("Conn write", err)
				conn.Close()
				TcpEn = false
				continue
			}

			go func() {
				rw := tlv.NewReadWriter(conn)
				for {
					err = handleTCPConn(rw)
					if err != nil {
						conn.Close()
						TcpEn = false
					}
				}
			}()
		}
	}()

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		fmt.Println("ListenUDP", err)
		return
	}

	defer conn.Close()
	go handleUDPConn(conn, toConnCh)

	for {
		var reqBuf bytes.Buffer
		macBytes := [32]byte{}
		copy(macBytes[:], []byte("AB:15:31:AA:93:26"))
		deviceInfo := DeviceInfo{macBytes, time.Now().Unix() - 1000}
		telemetry, err = xbyte.StructToByte(deviceInfo)
		if err != nil {
			fmt.Println("StructToByte", err)
			continue
		}
		rw := tlv.NewReadWriter(&reqBuf)
		err = rw.Write(uint16(3073), telemetry)
		if err != nil {
			fmt.Println("Tlv", err)
			continue
		}

		_, err = conn.WriteToUDP(reqBuf.Bytes(), hubUDPAddr)
		if err != nil {
			fmt.Println("WriteToUDP", err)
			continue
		}

		// fmt.Println(n)
		time.Sleep(time.Second * 5)
	}
}

func rsaSetup(conn *net.TCPConn, publicKey *rsa.PublicKey) (remotePublicKey *rsa.PublicKey, err error) {
	fmt.Println("asfgdhf")
	rw := tlv.NewReadWriter(conn)
	go func() {
		tag, val, err := rw.Read()
		if tag != 2 {
			rw.Write(1, []byte("First send public key"))
			return
		}

		if err != nil {
			fmt.Println("Tlv", err)
			return
		}

		remotePublicKey, err = xbyte.ByteToRsaPublic(val)
		if err != nil {
			fmt.Println("ByteToRsaPublic", err)
			return
		}
	}()

	dst, err := xbyte.RsaPublicToByte(publicKey)
	if err != nil {
		fmt.Println("RsaPublicToByte", err)
		return
	}
	err = rw.Write(2, dst)
	if err != nil {
		fmt.Println("Send public key", err)
		return
	}
	return
}
