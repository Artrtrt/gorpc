package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/service"
	"internal/typedef"
	rsautil "internal/utils"
	udprpc "pkg/tagrpc"
)

type TrpcServerHandler struct {
	service.RemoteErr
	service.RsaSetup
	service.SendGenericInfo
	service.ReceiveDeviceInfo
	service.SendServerInfo
}

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
	SN                string = "014223586595611"
	err               error
	genericInfo       *typedef.GenericInfo
	serverInfoControl *typedef.ServerInfoControl

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
	go acceptTcp(tcpLr)
	go httpServer()
	UDPAddr, err := net.ResolveUDPAddr("udp", hubUDPAddr)
	if err != nil {
		fmt.Println("ResolveUDPAddr:", err)
		return
	}

	udp, err := udprpc.NewUdp(":0")
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
	genericInfo = &typedef.GenericInfo{SN: SNBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	serverInfoControl = typedef.NewServerInfoControl(tcpAddrBytes, httpAddrBytes, 100)

	for {
		info, err := xbyte.StructToByte(genericInfo)
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

func configureUdp(udp *udprpc.Udp) {
	udp.HandleFunc(1025, connectToHub)

	for {
		err := udp.ReadAndExec()
		if err != nil {
			fmt.Println("udp readAndExec:", err)
			continue
		}
	}
}

func connectToHub(u *udprpc.Udp, tag uint16, val []byte) (err error) {
	if genericInfo.Busy {
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

	server := TrpcServerHandler{
		RemoteErr: service.RemoteErr{},
		RsaSetup: service.RsaSetup{
			PrivateKey: privateKey,
		},
		SendGenericInfo: service.SendGenericInfo{
			GenericInfo: genericInfo,
		},
		ReceiveDeviceInfo: service.ReceiveDeviceInfo{
			WantToConnectStorage: wantToConnectStorage,
		},
		SendServerInfo: service.SendServerInfo{
			ServerInfoControl: serverInfoControl,
		},
	}

	hubConn.HandleFunc(service.TagRemoteErr, server.RemoteErr.Handler)
	hubConn.HandleFunc(service.TagRsaSetup, server.RsaSetup.Handler)
	hubConn.HandleFunc(service.TagSendGenericInfo, server.SendGenericInfo.Handler)
	hubConn.HandleFunc(service.TagReceiveDeviceInfo, server.ReceiveDeviceInfo.Handler)
	hubConn.HandleFunc(service.TagSendServerInfo, server.SendServerInfo.Handler)
	fmt.Println("Подключился к хабу")
	go func(*tagrpc.TCPConn) {
		genericInfo.Busy = true
		defer func() {
			genericInfo.Busy = false
		}()

		for {
			err = hubConn.Update(time.Second * 60)
			if err != nil {
				hubConn.Close()
				fmt.Printf("Отключился от хаба. Ошибка: %s \n", err.Error())
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
