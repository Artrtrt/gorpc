package main

import (
	"bytes"
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"tag"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"rsautil"
	"typedef"
)

type ConnectStorage map[*tagrpc.TCPConn]string

func (s ConnectStorage) Find(SN string) *tagrpc.TCPConn {
	for conn, val := range s {
		if val == SN {
			return conn
		}
	}

	return nil
}

var (
	SN     string = "014223586595611"
	err    error
	server *typedef.Server

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	hubUDPAddr           string = "192.168.1.150:2000"
	tcpAddr              string = "192.168.1.150:8083"
	httpAddr             string = "localhost:8084"
	hubConn              *tagrpc.TCPConn
	wantToConnectStorage = make(map[[16]byte]typedef.GenericInfo)
	connectStorage       = ConnectStorage{}
)

func handleCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, SN")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func httpServer() {
	http.Handle("/", handleCORS(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		magic := r.Header.Get("SN")
		if SN == "" {
			rw.Write([]byte("SN не должен быть пустым"))
			return
		}

		SN := MagicSNTransform(magic)
		conn := connectStorage.Find(SN)
		if conn == nil {
			rw.Write([]byte("Устройство не найдено"))
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			rw.Write([]byte(err.Error()))
			return
		}

		resp, err := conn.Execute(2053, buf)
		if err != nil {
			rw.Write([]byte(err.Error()))
			return
		}

		_, err = rw.Write(bytes.TrimRightFunc(resp, func(r rune) bool {
			return r == 0
		}))

		if err != nil {
			rw.Write([]byte(err.Error()))
			return
		}
	})))
	http.ListenAndServe(httpAddr, nil)
}

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

	addr, err := net.ResolveTCPAddr("tcp", tcpAddr)
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	tcpLr, err := tagrpc.ListenTCP(addr)
	if err != nil {
		fmt.Println("ListenTCP:", err)
		return
	}

	defer tcpLr.Close()
	tcpLr.HandleFunc(1, remoteErr)
	go acceptTcp(tcpLr)
	go httpServer()
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

	go configureUdp(udp)
	SNBytes := [16]byte{}
	tcpAddrBytes := [32]byte{}
	httpAddrBytes := [32]byte{}
	copy(SNBytes[:], []byte(SN))
	copy(tcpAddrBytes[:], []byte(tcpAddr))
	copy(httpAddrBytes[:], []byte(httpAddr))
	serverInfo := &typedef.GenericInfo{SN: SNBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	server = typedef.NewServer(tcpAddrBytes, httpAddrBytes, 100, serverInfo)

	for {
		info, err := xbyte.StructToByte(server.Info)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(UDPAddr, uint16(2049), info)
		if err != nil {
			fmt.Println("UdpWrite:", err)
			continue
		}

		// fmt.Println(n)
		time.Sleep(time.Second * 5)
	}
}

// TCP
func acceptTcp(lr *tagrpc.TCPListener) {
	for {
		conn, err := lr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP:", err)
			continue
		}

		raddr := conn.Tcp.RemoteAddr().String()
		fmt.Println("Подключился ", raddr)

		go func(*tagrpc.TCPConn) {
			for {
				err = conn.Update(time.Second * 60)
				if err != nil {
					conn.Close()
					delete(connectStorage, conn)
					fmt.Printf("Отключился %s. Ошибка: %s \n", raddr, err.Error())
					break
				}
			}
		}(conn)

		go func(*tagrpc.TCPConn) {
			dst, err := xbyte.RsaPublicToByte(publicKey)
			if err != nil {
				fmt.Println("RsaPublicToByte", err)
				return
			}

			response, err := conn.Execute(2, dst)
			if err != nil {
				fmt.Println("Execute", err)
				return
			}

			clientPublicKey, err := xbyte.ByteToRsaPublic(response)
			if err != nil {
				fmt.Println("ByteToRsaPublic", err)
				return
			}

			conn.Codec = tagrpc.NewRsaCodec(privateKey, clientPublicKey)

			response, err = conn.Execute(3, []byte{})
			if err != nil {
				fmt.Println("Execute", err)
				return
			}

			var genericInfo typedef.GenericInfo
			err = xbyte.ByteToStruct(response, &genericInfo)
			if err != nil {
				fmt.Println("ByteToStruct:", err)
				return
			}

			_, ok := wantToConnectStorage[genericInfo.SN]
			if !ok {
				conn.Write(1, []byte("Unknown device"))
				conn.Close()
				return
			}

			delete(wantToConnectStorage, genericInfo.SN)
			connectStorage[conn] = byteArrToString(genericInfo.SN[:])
			err = hubConn.Request(2051, response)
			if err != nil {
				fmt.Println("Request", err)
				return
			}
		}(conn)
	}
}

func remoteErr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return errors.New(fmt.Sprint("remoteErr:", string(val)))
}

func receiveDeviceInfo(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	defer n.Response(1027, []byte("OK"))
	fmt.Println("agsfd")
	var deviceInfo typedef.GenericInfo
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	_, ok := wantToConnectStorage[deviceInfo.SN]
	if !ok {
		wantToConnectStorage[deviceInfo.SN] = deviceInfo
	}

	return
}

func configureTcpForHub(conn *tagrpc.TCPConn) {
	conn.HandleFunc(1, remoteErr)
	conn.HandleFunc(2, rsaSetup)
	conn.HandleFunc(3, sendGenericInfo)
	conn.HandleFunc(1027, receiveDeviceInfo)
	conn.HandleFunc(1029, sendServerInfo)
}

func rsaSetup(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	hubPublicKey, err := xbyte.ByteToRsaPublic(val)
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

	n.Codec = tagrpc.NewRsaCodec(privateKey, hubPublicKey)
	return
}

func sendGenericInfo(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	telemetry, err := xbyte.StructToByte(server.Info)
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

func sendServerInfo(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	serverInfo, err := xbyte.StructToByte(server.Control.ServerInfo)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(1029, serverInfo)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

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
	if server.Info.Busy {
		return
	}

	hubAddr, err := net.ResolveTCPAddr("tcp", string(val))
	if err != nil {
		err = fmt.Errorf("ResolveTCPAddr: %s", err)
		return
	}

	hubConn, err = tagrpc.DialTCP(nil, hubAddr)
	if err != nil {
		err = fmt.Errorf("DialTCP: %s", err)
		return
	}

	configureTcpForHub(hubConn)
	fmt.Println("Подключился к хабу")
	go func(*tagrpc.TCPConn) {
		server.Info.Busy = true
		defer func() {
			server.Info.Busy = false
		}()

		for {
			err = hubConn.Update(time.Second * 60)
			if err != nil {
				fmt.Println("trpc:", err)
				return
			}
		}
	}(hubConn)

	return
}

// Вынести в utils
func byteArrToString(arr []byte) string {
	return string(bytes.TrimRightFunc(arr, func(r rune) bool {
		return r == 0
	}))
}

func MagicSNTransform(SN string) string {
	runes := []rune(SN)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
