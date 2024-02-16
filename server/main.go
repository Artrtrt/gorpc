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
	"tcp"
	"time"
	"typedef"
)

// type Server struct {
// 	Control *typedef.ServerInfoControl
// 	Info    *typedef.GenericInfo
// }

// func NewServer(addr [32]byte, connectionLimit uint32, info *typedef.GenericInfo) *Server {
// 	return &Server{
// 		Control: typedef.NewServerInfoControl(addr, connectionLimit),
// 		Info:    info,
// 	}
// }

var (
	err  error
	info typedef.GenericInfo

	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	addr                 string = "localhost:8082"
	hubUDPAddr           *net.UDPAddr
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

func httpServer(port int, raddr string) {
	// ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	// if err != nil {
	// 	fmt.Println("Listen:", err)
	// 	return
	// }

	// http.FileServer(http.Dir("./dist"))
	// fmt.Println("Веб интерфейс:", ln.Addr())
	// err = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	request, err := io.ReadAll(r.Body)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}

	// 	response, err := conn.Execute(1000, request)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}

	// 	w.Write(response)
	// }))

	// if err != nil {
	// 	fmt.Println("Serve:", err)
	// 	return
	// }
	fmt.Println(port)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./dist")))
	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	// http.ListenAndServe(httpAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	request, err := io.ReadAll(r.Body)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}

	// 	response, err := conn.Execute(1000, request)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}

	// 	w.Write(response)
	// }))
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
	configureTcp(tcpLr)
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
	macBytes := [32]byte{}
	copy(macBytes[:], []byte("AB:15:31:AA:93:27"))
	serverInfo := typedef.GenericInfo{Mac: macBytes, Uptime: time.Now().Unix() - 1000}
	info = serverInfo
	// control := typedef.NewServerInfoControl(serverInfo, 1000)
	for {
		info, err := xbyte.StructToByte(info)
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

		go func(conn *tagrpc.TCPConn) {
			for {
				err = conn.Update(time.Second * 60)
				if err != nil {
					fmt.Println(err)
					conn.Close()
					break
				}
			}
		}(conn)

		go func(conn *tagrpc.TCPConn) {
			raddr := conn.Tcp.RemoteAddr().String()
			clientPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
			if err != nil {
				fmt.Println("RsaKeyExchange:", err)
				return
			}

			conn.Codec = tagrpc.NewRsaCodec(privateKey, clientPublicKey)
			fmt.Println("Подключился ", raddr)

			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				fmt.Println("Ошибка при открытии порта:", err)
				return
			}
			defer listener.Close()

			port := listener.Addr().(*net.TCPAddr).Port
			go httpServer(port, raddr)

		}(conn)
	}
}

func configureTcp(lr *tagrpc.TCPListener) {
	lr.HandleFunc(1, remoteErr)
	lr.HandleFunc(1027, receiveDeviceInfo)
	lr.HandleFunc(3074, acceptDevice)
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

	_, ok := wantToConnectStorage[deviceInfo.Mac]
	if ok {
		return
	}

	wantToConnectStorage[deviceInfo.Mac] = deviceInfo
	return
}

func acceptDevice(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	var deviceInfo typedef.GenericInfo
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	_, ok := wantToConnectStorage[deviceInfo.Mac]
	if !ok {
		fmt.Println("Такого нет")
		n.Close()
		return
	}

	fmt.Println("Все норм")
	return
}

func configureTcpForHub(conn *tagrpc.TCPConn) {
	conn.HandleFunc(1, remoteErr)
	conn.HandleFunc(2, rsaSetup)
	conn.HandleFunc(1028, sendInfo)
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

func sendInfo(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	telemetry, err := xbyte.StructToByte(info)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(1028, telemetry)
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

	configureTcpForHub(conn)
	fmt.Println("Подключился к хабу")
	for {
		err = conn.Update(time.Second * 60)
		if err != nil {
			err = fmt.Errorf("trpc: %s", err)
			break
		}
	}

	return
}

// dir, _ := os.Open("./dist")
// defer dir.Close()
// files, err := dir.Readdir(-1)
// if err != nil {
// 	fmt.Println(err)
// 	return
// }

// var fileName string
// for _, file := range files {
// 	match, err := regexp.MatchString("app\\.*\\.js", file.Name())
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// 	if match {
// 		fileName = file.Name()
// 	}
// }

// byteContent, err := ioutil.ReadFile(fmt.Sprintf("./dist/js/%s", fileName))
// if err != nil {
// 	fmt.Println(err)
// 	return
// }

// fmt.Sprintf(string(byteContent), raddr)
