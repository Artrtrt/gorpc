package main

import (
	"crypto/rsa"
	"encoding/json"
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

var (
	err                  error
	privateKey           *rsa.PrivateKey
	publicKey            *rsa.PublicKey
	addr                 string = "localhost:8082"
	hubUDPAddr           *net.UDPAddr
	wantToConnectStorage = make(map[[32]byte]typedef.DeviceInfo)
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

func handleTCP(conn *tagrpc.TCPConn) {
	for {
		tag, val, err := conn.Read()
		if err != nil {
			return
		}

		switch tag {
		case 1:
			fmt.Println("Hub err response:", string(val))
		case 1027:
			var deviceInfo typedef.DeviceInfo
			err = xbyte.ByteToStruct(val, &deviceInfo)
			if err != nil {
				fmt.Println("ByteToStruct", err)
				continue
			}

			_, ok := wantToConnectStorage[deviceInfo.Mac]
			if ok {
				continue
			}

			wantToConnectStorage[deviceInfo.Mac] = deviceInfo
		case 3075:
			var deviceInfo typedef.DeviceInfo
			err = xbyte.ByteToStruct(val, &deviceInfo)
			if err != nil {
				fmt.Println("ByteToStruct", err)
				continue
			}

			_, ok := wantToConnectStorage[deviceInfo.Mac]
			if ok {
				continue
			}
		}
	}
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
	var byteAddr [32]byte
	copy(byteAddr[:], []byte(addr))
	control := typedef.NewServerInfoControl(byteAddr, 1000)
	sendData(udp, control.ServerInfo)
}

func acceptTcp(lr *tagrpc.TCPListener) {
	for {
		conn, err := lr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP:", err)
			continue
		}

		go func() {
			raddr := conn.Tcp.RemoteAddr().String()
			defer func() {
				fmt.Printf("Соединение с %s разорвано\n", raddr)
				conn.Close()
			}()

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

			for {
				tag, val, err := conn.Read()
				if err != nil {
					fmt.Println("Conn read:", err)
					return
				}
				// var deviceInfo DeviceInfo
				// xbyte.ByteToStruct(val, &deviceInfo)
				// fmt.Println(string(deviceInfo.Mac[:]))
				// fmt.Println(deviceInfo.Uptime)

				fmt.Println(tag, val)
			}
		}()
	}
}

func sendData(udp *tag.Udp, info typedef.ServerInfo) {
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

func configureUdp(udp *tag.Udp) {
	for {
		tag, val, err := udp.Read()
		if err != nil {
			fmt.Println("udp read:", err)
			continue
		}

		err = udp.Execute(tag, val)
		if err != nil {
			fmt.Println("udp execute:", err)
			continue
		}
	}
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
