package main

import (
	"crypto/rsa"
	"encoding/csv"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sqlctrl"
	"syscall"
	"time"

	"gopack/jsonrpc"
	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/service"
	"internal/typedef"
	"internal/utils"
	udprpc "pkg/tagrpc"

	_ "modernc.org/sqlite"
)

type JrpcHubHandler struct {
	service.ReceiveSN
}

type TrpcHubServerHandler struct {
	service.RemoteErr
	service.SendClientHttpAddr
}

type TrpcHubDeviceHandler struct {
	service.RemoteErr
}

var (
	err        error
	privateKey *rsa.PrivateKey

	udpAddr  string = "192.168.1.163:2000"
	tpcAddr  string = "192.168.1.163:8080"
	httpAddr string = "localhost:8081"

	serverList []string
	storage    = typedef.Storage{}
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
	hub := JrpcHubHandler{
		ReceiveSN: service.ReceiveSN{
			Storage: &storage,
		},
	}

	server := jsonrpc.NewServer()
	var req, resp string
	server.HandleFunc(service.MethodSendSN, hub.ReceiveSN.Handler, req, resp)
	mux := http.NewServeMux()
	mux.Handle("/hub/", handleCORS(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			server.ServeHTTP(rw, r)
		}

		if r.Method == http.MethodGet {
			http.StripPrefix("/hub/", http.FileServer(http.Dir("./static/hub"))).ServeHTTP(rw, r)
		}
	})))

	mux.Handle("/api/ubus/", handleCORS(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/api/ubus/", handleCORS(http.FileServer(http.Dir("./static/webui")))).ServeHTTP(rw, r)
	})))

	http.ListenAndServe(httpAddr, mux)
}

func main() {
	privateKey, err = utils.PemToPrivateKey("private.pem")
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

	db, err := sqlctrl.NewDatabase("sqlite", "./test.db")
	if err != nil {
		fmt.Println("NewDatabase", err)
		return
	}

	deviceInfoTable, err := sqlctrl.NewTable("DeviceInfo", typedef.ToSql{})
	if err != nil {
		fmt.Println("NewTable", err)
		return
	}

	if !db.CheckExistTable(deviceInfoTable) {
		err = db.CreateTable(deviceInfoTable)
		if err != nil {
			fmt.Println("CreateTable", err)
			return
		}
	}

	exit := make(chan os.Signal, 1)
	dbChan := make(chan bool, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exit
		_, err = toSql(db, deviceInfoTable, dbChan)
		if err != nil {
			fmt.Println("toSql", err)
		}

		os.Exit(1)
	}()

	go func() {
		for {
			_, err = toSql(db, deviceInfoTable, dbChan)
			if err != nil {
				fmt.Println("toSql", err)
			}

			time.Sleep(time.Hour)
		}
	}()

	udp, err := udprpc.NewUdp(udpAddr)
	if err != nil {
		fmt.Println("NewUdp:", err)
		return
	}

	defer udp.Close()
	go configureUdp(udp)
	go httpServer()
	fmt.Println("Хаб запущен")

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
	acceptTcp(tcpLr)
}

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
					info := conn.Storage["info"].(*typedef.Info)
					if info.Type == "server" {
						delete(storage, info.GenericInfo.SystemBoard.Serial)
					}

					conn.Close()
					fmt.Printf("Отключился %s \n", addr)
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

			responsePublicKey, err := conn.Execute(service.TagRsaSetup, dst)
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

			responseGenericInfo, err := conn.Execute(service.TagSendGenericInfo, []byte{})
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

			serial := genericInfo.SystemBoard.Serial
			info := storage[serial]
			if !info.MatchGenericInfo(genericInfo) {
				fmt.Println("Not match genericInfo:", err)
				return
			}

			// _, err = conn.Execute(service.TagChaCha20Setup, []byte{})
			// if err != nil {
			// 	fmt.Println("Execute", err)
			// 	return
			// }

			// conn.Codec = tagrpc.NewChaCha20Codec(serial[:], serial[:])
			conn.Storage["info"] = info

			if info.WhitelistContainsServer(serverList) {
				err := serverConnect(conn, info)
				if err != nil {
					conn.Request(1, []byte(err.Error()))
				}
			} else if storage.RouterExist(serial) {
				err = deviceConnect(conn, info)
				if err != nil {
					info.DevicePayload.ToConnTCP = false
					info.DevicePayload.ErrChan <- err
					conn.Request(1, []byte(err.Error()))
				}
				conn.Close()
			} else {
				conn.Close()
			}
		}(conn)
	}
}

func serverConnect(conn *tagrpc.TCPConn, info *typedef.Info) (err error) {
	hub := TrpcHubServerHandler{
		RemoteErr: service.RemoteErr{},
		SendClientHttpAddr: service.SendClientHttpAddr{
			HttpAddr: httpAddr,
			Storage:  &storage,
		},
	}

	conn.HandleFunc(service.TagRemoteErr, hub.RemoteErr.Handler)
	conn.HandleFunc(service.TagDeviceConnected, hub.SendClientHttpAddr.Handler)
	serverInfo, err := getServerInfo(conn)
	if err != nil {
		fmt.Println("GetServerInfo", err.Error())
		return
	}

	fmt.Println(string(serverInfo.TcpAddr[:]))
	info.ServerInfo = &serverInfo
	info.Conn = conn
	for {
		err = updateServerInfo(info.GenericInfo.SystemBoard.Serial)
		if err != nil {
			fmt.Println("updateServerInfo:", err)
			return
		}
		time.Sleep(time.Second * 20)
	}
}

func deviceConnect(conn *tagrpc.TCPConn, info *typedef.Info) (err error) {
	hub := TrpcHubDeviceHandler{
		RemoteErr: service.RemoteErr{},
	}

	conn.HandleFunc(service.TagRemoteErr, hub.RemoteErr.Handler)
	serverConn, serverAddr, err := storage.LessBusyServer()
	if err != nil {
		err = fmt.Errorf("%s: %s", "LessBusyServer", err.Error())
		return
	}

	genericInfoBytes, err := xbyte.StructToByte(info.GenericInfo)
	if err != nil {
		err = fmt.Errorf("%s: %s", "StructToByte", err.Error())
		return
	}

	_, err = serverConn.Execute(service.TagSendInfoToServer, genericInfoBytes)
	if err != nil {
		err = fmt.Errorf("%s: %s", "Execute", err.Error())
		return
	}

	_, err = conn.Execute(service.TagConnectToServer, []byte(serverAddr[:]))
	if err != nil {
		err = fmt.Errorf("%s: %s", "Execute", err.Error())
		return
	}

	info.DevicePayload.ToConnTCP = false
	return
}

func getServerInfo(conn *tagrpc.TCPConn) (serverInfo typedef.ServerInfo, err error) {
	response, err := conn.Execute(service.TagGetServerInfo, []byte{})
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

func updateServerInfo(serial [64]byte) (err error) {
	device, ok := storage[serial]
	if !ok {
		return
	}

	serverInfo, err := getServerInfo(device.Conn)
	if err != nil {
		err = fmt.Errorf("%s %s", "getServerInfo", err.Error())
		return
	}

	device.ServerInfo.ConnectionCount = serverInfo.ConnectionCount
	return
}

// UDP
func configureUdp(udp *udprpc.Udp) {
	udp.HandleFunc(service.TagSendServerInfoUdp, receiveServerInfo)
	udp.HandleFunc(service.TagSendDeviceInfoUdp, receiveDeviceInfo)

	for {
		err := udp.ReadAndExec()
		if err != nil {
			fmt.Println("udp readAndExec:", err)
			continue
		}
	}
}

func receiveServerInfo(u *udprpc.Udp, tag uint16, val []byte) (err error) {
	genericInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &genericInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	info := &typedef.Info{
		Type:        "server",
		GenericInfo: &genericInfo,
		ServerInfo:  &typedef.ServerInfo{},
	}

	if !info.WhitelistContainsServer(serverList) {
		_, err = u.Write(u.Raddr, service.TagRemoteErr, []byte("Unknown device"))
		return
	}

	serial := genericInfo.SystemBoard.Serial
	_, ok := storage[serial]
	if ok {
		return
	}

	storage[serial] = info
	_, err = u.Write(u.Raddr, service.TagConnectToHub, []byte(tpcAddr))
	if err != nil {
		err = fmt.Errorf("UdpWrite: %s", err)
		return
	}

	return
}

func receiveDeviceInfo(u *udprpc.Udp, tag uint16, val []byte) (err error) {
	genericInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &genericInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	time := uint64(time.Now().Unix())
	info, ok := storage[genericInfo.SystemBoard.Serial]
	if ok {
		info.DevicePayload.Time = time
		info.GenericInfo.Busy = genericInfo.Busy
		info.GenericInfo.Uptime = genericInfo.Uptime
		if info.DevicePayload.ToConnTCP && !info.GenericInfo.Busy {
			_, err = u.Write(u.Raddr, service.TagConnectToHub, []byte(tpcAddr))
			if err != nil {
				err = fmt.Errorf("UdpWrite: %s", err)
				return
			}
		}
	} else {
		storage[genericInfo.SystemBoard.Serial] = &typedef.Info{
			Type:        "router",
			GenericInfo: &genericInfo,
			DevicePayload: &typedef.DevicePayload{
				Time: time, ToConnTCP: false, HttpAddrChan: make(chan string, 1), ErrChan: make(chan error, 1),
			},
		}
	}

	return
}

func toSql(db *sqlctrl.DataBase, table *sqlctrl.Table, mutex chan bool) (lastId int64, err error) {
	mutex <- true
	defer func() {
		<-mutex
	}()

	var rowArr []typedef.ToSql
	for _, val := range storage {
		if val.SentToDB {
			continue
		}

		var row typedef.ToSql
		err = utils.StructFieldsToString(val.GenericInfo.SystemBoard, &row)
		if err != nil {
			err = fmt.Errorf("%s: %s", "StructFieldsToString", err)
			return
		}

		row.Type = val.Type
		val.SentToDB = true
		rowArr = append(rowArr, row)
	}

	if len(rowArr) == 0 {
		return
	}

	lastId, err = db.InsertValue(table, rowArr)
	if err != nil {
		err = fmt.Errorf("%s: %s", "InsertValue", err)
		return
	}

	return
}
