package main

import (
	"bytes"
	"crypto/rsa"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"tag"
	"time"

	"gopack/jsonrpc"
	"gopack/tagrpc"
	"gopack/xbyte"
	"rsautil"
	"typedef"

	_ "github.com/mattn/go-sqlite3"
)

type DevicePayload struct {
	GenericInfo  typedef.GenericInfo
	Time         int64
	SentToDB     bool
	ToConnTCP    bool
	HttpAddrChan chan string
}

type DeviceStorage map[[16]byte]DevicePayload

// FIXIT
func (s DeviceStorage) Contains(SN [16]byte) bool {
	for _, payload := range s {
		if payload.GenericInfo.SN == SN {
			return true
		}
	}

	return false
}

type ServerPayload struct {
	GenericInfo *typedef.GenericInfo
	ServerInfo  *typedef.ServerInfo
}

type ServerStorage map[*tagrpc.TCPConn]ServerPayload

// FIXIT
func (s ServerStorage) Contains(SN [16]byte) bool {
	for _, payload := range s {
		if payload.GenericInfo.SN == SN {
			return true
		}
	}

	return false
}

func (s ServerStorage) lessBusyServer() (conn *tagrpc.TCPConn, addr [32]byte, err error) {
	if len(s) == 0 {
		err = fmt.Errorf("%s", "No servers")
		return
	}

	maxFreeConn := 0
	for key, server := range s {
		freeConn := server.ServerInfo.ConnectionLimit - server.ServerInfo.ConnectionCount
		if freeConn > uint32(maxFreeConn) {
			addr = server.ServerInfo.TcpAddr
			conn = key
			maxFreeConn = int(freeConn)
		}
	}
	return
}

var (
	err        error
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	udpAddr  string = "192.168.1.150:2000"
	tpcAddr  string = "192.168.1.150:8080"
	httpAddr string = "localhost:8081"

	serverList    []string
	deviceStorage = DeviceStorage{}
	serverStorage = ServerStorage{}
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
	server := jsonrpc.NewServer()
	var req, resp string
	server.HandleFunc("sendSN", receiveSN, req, resp)
	mux := http.NewServeMux()
	mux.Handle("/hub/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			server.ServeHTTP(rw, r)
		}

		if r.Method == http.MethodGet {
			http.StripPrefix("/hub/", http.FileServer(http.Dir("./static/hub"))).ServeHTTP(rw, r)
		}
	}))

	mux.Handle("/api/ubus/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/api/ubus/", http.FileServer(http.Dir("./static/webui"))).ServeHTTP(rw, r)
	}))

	http.ListenAndServe(httpAddr, mux)
}

func receiveSN(req interface{}) (resp interface{}, err error) {
	if req == "" {
		err = errors.New("SN не может быть пустым")
		return
	}

	SNBytes := [16]byte{}
	copy(SNBytes[:], []byte(req.(string)))
	_, ok := deviceStorage[SNBytes]
	if !ok {
		err = errors.New("Запрашиваемое устройство не найдено")
		return
	}

	device := deviceStorage[SNBytes]
	if time.Now().Unix()-device.Time > 120 {
		err = errors.New("Устройство не доступно")
		return
	}

	payload := deviceStorage[SNBytes]
	if payload.ToConnTCP {
		err = errors.New("Устройство занято")
		return
	}

	payload.ToConnTCP = true
	deviceStorage[SNBytes] = payload

	select {
	case <-time.After(time.Second * 20):
		payload.ToConnTCP = false
		deviceStorage[SNBytes] = payload
		err = errors.New("Устройство не доступно")
		return
	case resp = <-deviceStorage[SNBytes].HttpAddrChan:
	}

	return
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

	file, err := os.Open("serverlist.csv")
	if err != nil {
		fmt.Println("Open", err)
		return
	}

	defer file.Close()
	serverList, err = csv.NewReader(file).Read()
	if err != nil {
		fmt.Println("Read", err)
		return
	}

	// dbChan := make(chan bool, 1)
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exit
		// Выгрузка данных в бд
		os.Exit(1)
	}()

	udp, err := tag.NewUdp(udpAddr)
	if err != nil {
		fmt.Println("NewUdp:", err)
		return
	}

	defer udp.Close()
	go configureUdp(udp)
	go httpServer()
	fmt.Println("Слухает")

	tcpAddr, err := net.ResolveTCPAddr("tcp", tpcAddr)
	if err != nil {
		fmt.Println("ResolveTCPAddr", err)
		return
	}

	tcpLr, err := tagrpc.ListenTCP(tcpAddr)
	if err != nil {
		fmt.Println("ListenTCP", err)
		return
	}

	defer tcpLr.Close()
	tcpLr.HandleFunc(1, remoteErr)
	acceptTcp(tcpLr)
}

// TCP
func acceptTcp(lr *tagrpc.TCPListener) {
	for {
		conn, err := lr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP", err)
			continue
		}
		addr := conn.Tcp.RemoteAddr().String()
		fmt.Println("Подключился " + addr)
		go func(*tagrpc.TCPConn) {
			for {
				err = conn.Update(time.Second * 60)
				if err != nil {
					_, ok := serverStorage[conn]
					if ok {
						delete(serverStorage, conn)
					}
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

			responsePublicKey, err := conn.Execute(2, dst)
			if err != nil {
				fmt.Println("Execute", err)
				return
			}

			clientPublicKey, err := xbyte.ByteToRsaPublic(responsePublicKey)
			if err != nil {
				fmt.Println("ByteToRsaPublic", err)
				return
			}

			conn.Codec = tagrpc.NewRsaCodec(privateKey, clientPublicKey)

			responseGenericInfo, err := conn.Execute(3, []byte{})
			if err != nil {
				fmt.Println("Execute", err)
				return
			}

			var genericInfo typedef.GenericInfo
			err = xbyte.ByteToStruct(responseGenericInfo, &genericInfo)
			if err != nil {
				fmt.Println("ByteToStruct:", err)
				return
			}

			if contains(serverList, byteArrToString(genericInfo.SN[:])) {
				if serverStorage.Contains(genericInfo.SN) {
					return
				}

				serverStorage[conn] = ServerPayload{&genericInfo, nil}
				configureTcpForServer(conn)

				for {
					err = updateServerInfo(conn)
					if err != nil {
						fmt.Println("updateServerInfo:", err)
						return
					}
					time.Sleep(time.Second * 20)
				}
			} else if deviceStorage.Contains(genericInfo.SN) {
				serverConn, serverAddr, err := serverStorage.lessBusyServer()
				if err != nil {
					conn.Request(1, []byte(err.Error()))
					fmt.Println("lessBusyServer:", err)
					return
				}

				fmt.Println(serverConn)
				_, err = serverConn.Execute(1027, responseGenericInfo)
				if err != nil {
					fmt.Println("Request:", err)
					return
				}

				_, err = conn.Execute(1026, []byte(serverAddr[:]))
				if err != nil {
					fmt.Println("Execute:", err)
					return
				}
				tmp := deviceStorage[genericInfo.SN]
				tmp.ToConnTCP = false
				deviceStorage[genericInfo.SN] = tmp
				conn.Close()
			} else {
				conn.Close()
			}
		}(conn)
	}
}

func getServerInfo(conn *tagrpc.Node) (serverInfo typedef.ServerInfo, err error) {
	response, err := conn.Execute(1029, []byte{})
	if err != nil {
		fmt.Println("Execute", err)
		return
	}

	err = xbyte.ByteToStruct(response, &serverInfo)
	if err != nil {
		fmt.Println("ByteToStruct:", err)
		return
	}

	return
}

func updateServerInfo(conn *tagrpc.TCPConn) (err error) {
	_, ok := serverStorage[conn]
	if !ok {
		return errors.New("server not connect")
	}

	serverInfo, err := getServerInfo(conn.Node)
	if err != nil {
		err = fmt.Errorf("%s %s", "getServerInfo", err.Error())
		return
	}

	server := serverStorage[conn]
	server.ServerInfo = &serverInfo
	serverStorage[conn] = server
	return
}

func configureTcpForServer(conn *tagrpc.TCPConn) {
	conn.HandleFunc(2051, sendClientHttpAddr)
}

func remoteErr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return errors.New(fmt.Sprint("remoteErr:", string(val)))
}

func sendClientHttpAddr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	SN := MagicSNTransform(byteArrToString(deviceInfo.SN[:]))
	serverInfo, err := getServerInfo(n)
	if err != nil {
		err = fmt.Errorf("getServerInfo: %s", err)
		return
	}

	addr := "http://" + httpAddr + "/api/ubus/" + "?SN=" + SN + "&endpoint=http://" + byteArrToString(serverInfo.HttpAddr[:])
	deviceStorage[deviceInfo.SN].HttpAddrChan <- addr
	return
}

// UDP
func configureUdp(udp *tag.Udp) {
	udp.HandleFunc(2049, receiveGenericServerInfo)
	udp.HandleFunc(3073, receiveGenericDeviceInfo)

	for {
		err := udp.ReadAndExec()
		if err != nil {
			fmt.Println("udp readAndExec:", err)
			continue
		}
	}
}

func receiveGenericServerInfo(u *tag.Udp, tag uint16, val []byte) (err error) {
	serverInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &serverInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	if !contains(serverList, byteArrToString(serverInfo.SN[:])) {
		_, err = u.Write(u.Raddr, 1, []byte("Unknown device"))
		return
	}

	if serverInfo.Busy {
		return
	}

	_, err = u.Write(u.Raddr, 1025, []byte(tpcAddr))
	if err != nil {
		err = fmt.Errorf("UdpWrite: %s", err)
		return
	}

	return
}

func receiveGenericDeviceInfo(u *tag.Udp, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	time := time.Now().Unix()
	_, ok := deviceStorage[deviceInfo.SN]
	if ok {
		data := deviceStorage[deviceInfo.SN]
		data.Time = time
		data.GenericInfo = deviceInfo
		deviceStorage[deviceInfo.SN] = data
		if data.ToConnTCP && !deviceInfo.Busy {
			_, err = u.Write(u.Raddr, 1025, []byte(tpcAddr))
			if err != nil {
				err = fmt.Errorf("UdpWrite: %s", err)
				return
			}
		}
		// fmt.Printf("Данные о роутере %s обновились\n", string(deviceInfo.SN[:]))
	} else {
		deviceStorage[deviceInfo.SN] = DevicePayload{
			deviceInfo, time, false, false, make(chan string, 1),
		}
		// fmt.Printf("Роутер %s добавлен\n", deviceInfo.SN)
	}

	return
}

// Вынести в utils
func byteArrToString(arr []byte) string {
	return string(bytes.TrimRightFunc(arr, func(r rune) bool {
		return r == 0
	}))
}

func contains(arr []string, str string) bool {
	for _, val := range arr {
		if strings.Contains(val, str) {
			return true
		}
	}
	return false
}

func MagicSNTransform(SN string) string {
	runes := []rune(SN)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
