package main

import (
	"crypto/rsa"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os/exec"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/service"
	"internal/typedef"
	rsautil "internal/utils"
	udprpc "pkg/tagrpc"
)

type TrpcClientHubHandler struct {
	service.TrpcDefaultHandler
	service.ConnectToServer
}

type TrpcClientServerHandler struct {
	service.TrpcDefaultHandler
}

var (
	err         error
	genericInfo *typedef.GenericInfo
	privateKey  *rsa.PrivateKey

	hubUDPAddr string = "192.168.1.150:2000"
)

type SystemBoardInfo struct {
	Manufacturer string
	Product      string
	Hostname     string
	Serial       string
	Release      struct {
		Revision string
		Version  string
	}
}

func GetSystemBoardInfo() (info SystemBoardInfo, err error) {
	cmd := exec.Command("ubus", "call", "system", "board")
	byteArr, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("%s %s", "Command", err.Error())
		return
	}

	err = json.Unmarshal([]byte(byteArr), &info)
	if err != nil {
		return SystemBoardInfo{}, err
	}

	return
}

func GetDeviceUptime() (uptime float32, err error) {
	cmd := exec.Command("cat", "/proc/uptime")
	byteArr, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("%s %s", "Command", err.Error())
		return
	}

	bits := binary.LittleEndian.Uint32(byteArr)
	uptime = math.Float32frombits(bits)
	return
}

func main() {
	// publicKey, err = rsautil.PemToPublicKey("public.pem")
	// if err != nil {
	// 	fmt.Println("PemToPublicKey:", err)
	// 	return
	// }

	privateKey, err = rsautil.PemToPrivateKey("private.pem")
	if err != nil {
		fmt.Println("PemToPublicKey:", err)
		return
	}

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

	defer udp.Close()
	go configureUdp(udp)

	systemBoard, err := GetSystemBoardInfo()
	if err != nil {
		fmt.Println("GetSystemBoardInfo:", err)
		return
	}

	fmt.Println(systemBoard.Serial)
	var snBytes [16]byte
	copy(snBytes[:], systemBoard.Serial)
	genericInfo = &typedef.GenericInfo{SN: snBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	for {
		telemetry, err := xbyte.StructToByte(genericInfo)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(UDPAddr, uint16(3073), telemetry)
		if err != nil {
			fmt.Println("UdpWrite:", err)
			continue
		}
		// fmt.Println(n)
		time.Sleep(time.Second * 5)
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
	}

	client := TrpcClientHubHandler{
		TrpcDefaultHandler: trpcDefault,
		ConnectToServer: service.ConnectToServer{
			TrpcDefaultHandler: trpcDefault,
			ExecuteJsonRPC:     service.ExecuteJsonRPC{},
		},
	}
	conn.HandleFunc(service.TagRemoteErr, client.RemoteErr.Handler)
	conn.HandleFunc(service.TagRsaSetup, client.RsaSetup.Handler)
	conn.HandleFunc(service.TagSendGenericInfo, client.SendGenericInfo.Handler)
	conn.HandleFunc(service.TagConnectToServer, client.ConnectToServer.Handler)

	fmt.Println("Подключился к хабу")
	go func(*tagrpc.TCPConn) {
		for {
			err = conn.Update(time.Second * 60)
			if err != nil {
				conn.Close()
				fmt.Printf("Отключился от хаба. Ошибка: %s \n", err.Error())
				return
			}
		}
	}(conn)
	return
}
