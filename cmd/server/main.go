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
	"internal/telemetry"
	"internal/typedef"
	"internal/utils"
	udprpc "pkg/tagrpc"
)

type TrpcServerHubHandler struct {
	service.RemoteErr
	service.RsaSetup
	service.SendGenericInfo
	service.ReceiveDeviceInfo
	service.SendServerInfo
}

type TrpcServerdeviceHandler struct {
	service.RemoteErr
}

type ConnectStorage map[*tagrpc.TCPConn]string

func (s ConnectStorage) Find(Serial string) *tagrpc.TCPConn {
	for conn, val := range s {
		if val == Serial {
			return conn
		}
	}

	return nil
}

var (
	privateKey        *rsa.PrivateKey
	genericInfo       *typedef.GenericInfo
	serverInfoControl *typedef.ServerInfoControl

	tcpAddr    string = "192.168.1.1:8083"
	httpAddr   string = "192.168.1.1:8084"
	hubUDPAddr string = "192.168.1.163:2000"
	hubConn    *tagrpc.TCPConn

	wantToConnectStorage = make(map[[64]byte]typedef.GenericInfo)
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
		serial := r.Header.Get("SN")
		if serial == "" {
			rw.Write([]byte("Serial не должен быть пустым"))
			return
		}

		conn := connectStorage.Find(serial)
		if conn == nil {
			rw.Write([]byte("Устройство не найдено"))
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			rw.Write([]byte(err.Error()))
			return
		}

		resp, err := conn.Execute(service.TagExecuteJsonRPC, buf)
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
	systemBoard, err := telemetry.GetSystemBoardInfo()
	if err != nil {
		fmt.Println("GetSystemBoardInfo:", err)
		return
	}

	fmt.Println(string(systemBoard.Serial[:]))
	genericInfo = &typedef.GenericInfo{SystemBoard: systemBoard}

	privateKey, err = utils.PemToPrivateKey("private.pem")
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
	tcpAddrBytes := [32]byte{}
	httpAddrBytes := [32]byte{}
	copy(tcpAddrBytes[:], []byte(tcpAddr))
	copy(httpAddrBytes[:], []byte(httpAddr))
	serverInfoControl = typedef.NewServerInfoControl(tcpAddrBytes, httpAddrBytes, 100)

	for {
		info, err := xbyte.StructToByte(genericInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(UDPAddr, service.TagSendServerInfoUdp, info)
		if err != nil {
			fmt.Println("UdpWrite:", err)
			continue
		}

		time.Sleep(time.Second * 5)
	}
}

func acceptTcp(lr *tagrpc.TCPListener) {
	for {
		conn, err := lr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP:", err)
			continue
		}

		raddr := conn.Tcp.RemoteAddr().String()
		server := TrpcServerdeviceHandler{
			RemoteErr: service.RemoteErr{},
		}

		conn.HandleFunc(service.TagRemoteErr, server.RemoteErr.Handler)
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
			dst, err := xbyte.RsaPublicToByte(&privateKey.PublicKey)
			if err != nil {
				fmt.Println("RsaPublicToByte", err)
				return
			}

			response, err := conn.Execute(service.TagRsaSetup, dst)
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

			response, err = conn.Execute(service.TagSendGenericInfo, []byte{})
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

			_, ok := wantToConnectStorage[genericInfo.SystemBoard.Serial]
			if !ok {
				conn.Write(service.TagRemoteErr, []byte("Unknown device"))
				conn.Close()
				return
			}

			delete(wantToConnectStorage, genericInfo.SystemBoard.Serial)
			connectStorage[conn] = utils.ByteArrToString(genericInfo.SystemBoard.Serial[:])
			err = hubConn.Request(service.TagDeviceConnected, response)
			if err != nil {
				fmt.Println("Request", err)
				return
			}
		}(conn)
	}
}

func configureUdp(udp *udprpc.Udp) {
	udp.HandleFunc(service.TagConnectToHub, connectToHub)

	for {
		err := udp.ReadAndExec()
		if err != nil {
			fmt.Println("udp readAndExec:", err)
			continue
		}
	}
}

func connectToHub(u *udprpc.Udp, tag uint16, val []byte) (err error) {
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

	server := TrpcServerHubHandler{
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
	hubConn.HandleFunc(service.TagSendInfoToServer, server.ReceiveDeviceInfo.Handler)
	hubConn.HandleFunc(service.TagGetServerInfo, server.SendServerInfo.Handler)
	fmt.Println("Подключился к хабу")
	go func(*tagrpc.TCPConn) {
		for {
			err = hubConn.Update(time.Second * 60)
			if err != nil {
				hubConn.Close()
				fmt.Printf("Отключился от хаба. Ошибка: %s\n", err.Error())
				return
			}
		}
	}(hubConn)

	return
}
