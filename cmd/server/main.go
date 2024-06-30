package main

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/service"
	"internal/telemetry"
	"internal/typedef"
	"internal/utils"
	udprpc "pkg/tagrpc"

	"github.com/google/uuid"
)

type TrpcServerHubHandler struct {
	service.TrpcDefaultHandler
	service.ReceiveDeviceInfo
	service.SendServerInfo
}

type TrpcServerdeviceHandler struct {
	service.RemoteErr
}

var (
	privateKey        *rsa.PrivateKey
	genericInfo       *typedef.GenericInfo
	serverInfoControl *typedef.ServerInfoControl

	config  *typedef.Config
	hubConn *tagrpc.TCPConn

	storage = typedef.Storage{}
)

func handleCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func httpServer() {
	http.Handle("/", handleCORS(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		uuid := r.Header.Get("UUID")
		if uuid == "" {
			rw.Write([]byte("UUID не должен быть пустым"))
			return
		}

		info := storage.GetByUUID(uuid)
		if info == nil {
			rw.Write([]byte("Устройство не найдено"))
			return
		}

		if info.Conn == nil {
			rw.Write([]byte("Устройство не подключено к сереверу"))
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			rw.Write([]byte(err.Error()))
			return
		}

		resp, err := info.Conn.Execute(service.TagExecuteJsonRPC, buf)
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
	http.ListenAndServe(fmt.Sprintf("%s:%s", config.Ip, config.HttpPort), nil)
}

func main() {
	content, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Println("ReadFile:", err)
		return
	}

	err = json.Unmarshal(content, &config)
	if err != nil {
		fmt.Println("Unmarshal:", err)
		return
	}

	var (
		systemBoard typedef.SystemBoard
	)

	if config.AppLocal {
		var serial [64]byte
		copy(serial[:], []byte("014223586595694"))
		systemBoard = typedef.SystemBoard{
			Serial: serial,
		}
	} else {
		systemBoard, err = telemetry.GetSystemBoardInfo()
		if err != nil {
			fmt.Println("GetSystemBoardInfo:", err)
			return
		}
	}

	fmt.Println(string(systemBoard.Serial[:]))
	fmt.Println("Сервер запущен")
	genericInfo = &typedef.GenericInfo{SystemBoard: systemBoard}

	privateKey, err = utils.PemToPrivateKey("private.pem")
	if err != nil {
		fmt.Println("PemToPublicKey", err)
		return
	}

	tcpAddr := fmt.Sprintf("%s:%s", config.Ip, config.TcpPort)
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
	UDPAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", config.HubIp, config.HubUdpPort))
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
	copy(httpAddrBytes[:], []byte(fmt.Sprintf("%s:%s", config.Ip, config.HttpPort)))
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
					info := conn.Storage["info"].(*typedef.Info)
					delete(storage, info.GenericInfo.SystemBoard.Serial)
					fmt.Printf("Отключился %s. %s \n", raddr, err.Error())
					conn.Close()
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

			info, ok := storage[genericInfo.SystemBoard.Serial]
			if !ok {
				conn.Write(service.TagRemoteErr, []byte("Unknown device"))
				conn.Close()
				return
			}

			info.DevicePayload.ToConnTCP = false
			info.Conn = conn
			info.GenericInfo.UUID = uuid.New()
			conn.Storage["info"] = info

			devicePayload, err := xbyte.StructToByte(info.GenericInfo)
			if err != nil {
				fmt.Println("StructToByte:", err)
				return
			}

			err = hubConn.Request(service.TagDeviceConnected, devicePayload)
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

	trpcDefault := service.TrpcDefaultHandler{
		RemoteErr: service.RemoteErr{},
		RsaSetup: service.RsaSetup{
			PrivateKey: privateKey,
		},
		SendGenericInfo: service.SendGenericInfo{
			GenericInfo: genericInfo,
		},
		ChaCha20Setup: service.ChaCha20Setup{
			Secret: genericInfo.SystemBoard.Serial[:],
		},
	}

	server := TrpcServerHubHandler{
		TrpcDefaultHandler: trpcDefault,
		ReceiveDeviceInfo: service.ReceiveDeviceInfo{
			Storage: &storage,
		},
		SendServerInfo: service.SendServerInfo{
			ServerInfoControl: serverInfoControl,
		},
	}

	hubConn.HandleFunc(service.TagRemoteErr, server.RemoteErr.Handler)
	hubConn.HandleFunc(service.TagRsaSetup, server.RsaSetup.Handler)
	hubConn.HandleFunc(service.TagSendGenericInfo, server.SendGenericInfo.Handler)
	hubConn.HandleFunc(service.TagChaCha20Setup, server.ChaCha20Setup.Handler)
	hubConn.HandleFunc(service.TagSendInfoToServer, server.ReceiveDeviceInfo.Handler)
	hubConn.HandleFunc(service.TagGetServerInfo, server.SendServerInfo.Handler)
	fmt.Println("Подключился к хабу")
	go func(*tagrpc.TCPConn) {
		for {
			err = hubConn.Update(time.Second * 60)
			if err != nil {
				hubConn.Close()
				fmt.Printf("Отключился от хаба. %s\n", err.Error())
				return
			}
		}
	}(hubConn)

	return
}
