package main

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"gopack/jsonrpc"
	"gopack/tagrpc"
	"gopack/xbyte"
	"net"
	"net/http"
	"rsautil"
	"tag"
	"time"
	"typedef"
)

type Server struct {
	Control *typedef.ServerInfoControl
	Info    *typedef.GenericInfo
}

func NewServer(addr [32]byte, connectionLimit uint32, info *typedef.GenericInfo) *Server {
	return &Server{
		Control: typedef.NewServerInfoControl(addr, connectionLimit),
		Info:    info,
	}
}

var (
	SN     string = "014223586595611"
	err    error
	server *Server

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	addr                 string = "localhost:8083"
	hubUDPAddr           *net.UDPAddr
	hubConn              *tagrpc.TCPConn
	wantToConnectStorage = make(map[[32]byte]typedef.GenericInfo)
)

func handleRPC(w http.ResponseWriter, r *http.Request) {
	var request jsonrpc.Request
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client := jsonrpc.NewClient(jsonrpc.NewClientTransportHttp("http://192.168.1.1/ubus"))
	response := client.RawRequest(request)
	byteResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(byteResponse)
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

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	tcpLr, err := tagrpc.ListenTCP(tcpAddr)
	if err != nil {
		fmt.Println("ListenTCP:", err)
		return
	}

	defer tcpLr.Close()
	tcpLr.HandleFunc(1, remoteErr)
	go acceptTcp(tcpLr)
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

	go configureUdp(udp)
	SNBytes := [32]byte{}
	addrBytes := [32]byte{}
	copy(SNBytes[:], []byte(SN))
	copy(addrBytes[:], []byte(addr))
	serverInfo := &typedef.GenericInfo{SN: SNBytes, Uptime: time.Now().Unix() - 1000, Busy: false}
	server = NewServer(addrBytes, 100, serverInfo)

	for {
		info, err := xbyte.StructToByte(server.Info)
		if err != nil {
			fmt.Println("StructToByte:", err)
			continue
		}

		_, err = udp.Write(hubUDPAddr, uint16(2049), info)
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
					fmt.Printf("Отключился %s. Ошибка: %s \n", addr, err.Error())
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

	err = n.Response(1027, []byte("OK"))
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
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
