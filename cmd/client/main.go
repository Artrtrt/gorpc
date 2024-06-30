package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/service"
	"internal/telemetry"
	"internal/typedef"
	"internal/utils"
	udprpc "pkg/tagrpc"
)

type TrpcClientHubHandler struct {
	service.TrpcDefaultHandler
	service.ConnectToServer
}

var (
	privateKey  *rsa.PrivateKey
	genericInfo *typedef.GenericInfo
	config      *typedef.Config
)

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
		uptime      uint64
	)

	if config.AppLocal {
		var serial [64]byte
		copy(serial[:], []byte("014223586593338"))
		systemBoard = typedef.SystemBoard{
			Serial: serial,
		}
	} else {
		systemBoard, err = telemetry.GetSystemBoardInfo()
		if err != nil {
			fmt.Println("GetSystemBoardInfo:", err)
			return
		}

		uptime, err = telemetry.GetDeviceUptime()
		if err != nil {
			fmt.Println("GetUptime:", err)
			return
		}
	}

	fmt.Println(string(systemBoard.Serial[:]))
	fmt.Println("Клиент запущен")
	genericInfo = &typedef.GenericInfo{SystemBoard: systemBoard, Uptime: uptime, Busy: false}

	privateKey, err = utils.PemToPrivateKey("private.pem")
	if err != nil {
		fmt.Println("PemToPublicKey:", err)
		return
	}

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

	defer udp.Close()
	go configureUdp(udp)
	for {
		genericInfoByte, err := xbyte.StructToByte(genericInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(UDPAddr, service.TagSendDeviceInfoUdp, genericInfoByte)
		if err != nil {
			fmt.Println("UdpWrite:", err)
			continue
		}
		time.Sleep(time.Second * 5)
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
	if genericInfo.Busy {
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

	client := TrpcClientHubHandler{
		TrpcDefaultHandler: trpcDefault,
		ConnectToServer: service.ConnectToServer{
			TrpcDefaultHandler: trpcDefault,
			ExecuteJsonRPC: service.ExecuteJsonRPC{
				Endpoint: config.WebEndpoint,
			},
			GenericInfo: genericInfo,
		},
	}
	conn.HandleFunc(service.TagRemoteErr, client.RemoteErr.Handler)
	conn.HandleFunc(service.TagRsaSetup, client.RsaSetup.Handler)
	conn.HandleFunc(service.TagSendGenericInfo, client.SendGenericInfo.Handler)
	conn.HandleFunc(service.TagChaCha20Setup, client.ChaCha20Setup.Handler)
	conn.HandleFunc(service.TagConnectToServer, client.ConnectToServer.Handler)

	fmt.Println("Подключился к хабу")
	go func(*tagrpc.TCPConn) {
		for {
			err = conn.Update(time.Second * 60)
			if err != nil {
				conn.Close()
				fmt.Printf("Отключился от хаба. %s \n", err.Error())
				return
			}
		}
	}(conn)
	return
}
