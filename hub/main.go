package main

import (
	"bytes"
	"crypto/rsa"
	"database/sql"
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

	"gopack/tagrpc"
	"gopack/xbyte"
	"rsautil"
	"typedef"

	_ "github.com/mattn/go-sqlite3"
)

type DevicePayload struct {
	GenericInfo typedef.GenericInfo
	Time        int64
	SentToDB    bool
	ToConnTCP   bool
}

type ServerPayload struct {
	GenericInfo *typedef.GenericInfo
	ServerInfo  *typedef.ServerInfo
}

var (
	err        error
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey

	addrStr  string = "localhost:8080"
	httpAddr string = ":8081"

	serverList    []string
	deviceStorage = make(map[[32]byte]DevicePayload)
	serverStorage = make(map[*tagrpc.TCPConn]ServerPayload)
)

func httpServer() {
	http.HandleFunc("/hub", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			mac := r.FormValue("mac")
			if mac == "" {
				fmt.Fprint(rw, "Пустой mac")
				return
			}

			macBytes := [32]byte{}
			copy(macBytes[:], []byte(mac))
			_, ok := deviceStorage[macBytes]
			if !ok {
				fmt.Fprint(rw, "Такого устройства нет")
				return
			}

			device := deviceStorage[macBytes]
			if time.Now().Unix()-device.Time > 120 {
				fmt.Fprint(rw, "Устройство не доступно")
				return
			}

			payload := deviceStorage[macBytes]
			if payload.ToConnTCP {
				fmt.Fprint(rw, "Устройство занято")
				return
			} else {
				payload.ToConnTCP = true
				deviceStorage[macBytes] = payload
			}
		}
	})
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

	dbChan := make(chan bool, 1)
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exit
		toSql(dbChan)
		os.Exit(1)
	}()

	udp, err := tag.NewUdp("localhost:2000")
	if err != nil {
		fmt.Println("NewUdp:", err)
		return
	}

	defer udp.Close()
	go configureUdp(udp)
	go httpServer()
	fmt.Println("Слухает")

	tcpAddr, err := net.ResolveTCPAddr("tcp", addrStr)
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
	// go func() {
	// 	for {
	// 		err := toSql(dbChan)

	// 		if err != nil {
	// 			fmt.Println("toSql", err)
	// 		}

	// 		time.Sleep(time.Second * 10)
	// 	}
	// }()
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
		go func(conn *tagrpc.TCPConn) {
			for {
				err = conn.Update(time.Second * 60)
				if err != nil {
					_, ok := serverStorage[conn]
					if ok {
						delete(serverStorage, conn)
					}
					fmt.Println("trpc", err)
					conn.Close()
					fmt.Println("Отключился " + addr)
					break
				}
			}
		}(conn)

		go func(conn *tagrpc.TCPConn) {
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

			response, err = conn.Execute(1028, []byte{})
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

			if contains(serverList, byteArrToString(genericInfo.Mac[:])) {
				if serverStorageContains(serverStorage, genericInfo.Mac) {
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
			} else if deviceStorageContains(deviceStorage, genericInfo.Mac) {
				serverAddr, err := lessBusyServer(serverStorage)
				if err != nil {
					conn.Request(1, []byte(err.Error()))
					fmt.Println("lessBusyServer:", err)
					return
				}

				response, err = conn.Execute(1026, serverAddr[:])
				if err != nil {
					fmt.Println("lessBusyServer:", err)
					return
				}

				fmt.Println(string(response))
				conn.Close()
			} else {
				conn.Close()
			}
		}(conn)
	}
}

func updateServerInfo(conn *tagrpc.TCPConn) (err error) {
	_, ok := serverStorage[conn]
	if !ok {
		return errors.New("server not connect")
	}

	response, err := conn.Execute(1029, []byte{})
	if err != nil {
		fmt.Println("Execute", err)
		return
	}

	var serverInfo typedef.ServerInfo
	err = xbyte.ByteToStruct(response, &serverInfo)
	if err != nil {
		fmt.Println("ByteToStruct:", err)
		return
	}

	server := serverStorage[conn]
	server.ServerInfo = &serverInfo
	serverStorage[conn] = server
	return
}

func configureTcpForServer(conn *tagrpc.TCPConn) {
	conn.HandleFunc(2051, sendClientHttpAddr)
	conn.HandleFunc(2052, closeDeviceConnection)
}

func remoteErr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return errors.New(fmt.Sprint("remoteErr:", string(val)))
}

// func acceptServer(n *tagrpc.Node, tag uint16, val []byte) (err error) {
// 	serverInfo := typedef.GenericInfo{}
// 	err = xbyte.ByteToStruct(val, &serverInfo)
// 	if err != nil {
// 		err = fmt.Errorf("ByteToStruct: %s", err)
// 		return
// 	}

// 	if !contains(serverList, byteArrToString(serverInfo.Mac[:])) {
// 		n.Close()
// 	}

// 	if serverStorageContains(serverStorage, serverInfo.Mac) {
// 		n.Write(1, []byte("Device already connect"))
// 		return
// 	}

// 	resp, err := n.Execute(1029, nil)
// 	if err != nil {
// 		err = fmt.Errorf("execute: %s", err)
// 		return
// 	}

// 	fmt.Println(resp)
// 	return
// }

func sendClientHttpAddr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	fmt.Println("Сообщить клиенту адрес куда подключаться")
	return
}

func closeDeviceConnection(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	fmt.Println("hello")
	return
}

// FIXIT
func deviceStorageContains(storage map[[32]byte]DevicePayload, mac [32]byte) bool {
	for _, payload := range storage {
		if payload.GenericInfo.Mac == mac {
			return true
		}
	}

	return false
}

func serverStorageContains(storage map[*tagrpc.TCPConn]ServerPayload, mac [32]byte) bool {
	for _, payload := range storage {
		if payload.GenericInfo.Mac == mac {
			return true
		}
	}

	return false
}

func lessBusyServer(storage map[*tagrpc.TCPConn]ServerPayload) (serverAddr [32]byte, err error) {
	if len(storage) == 0 {
		err = fmt.Errorf("%s", "No servers")
		return
	}

	maxFreeConn := 0
	for _, server := range storage {
		freeConn := server.ServerInfo.ConnectionLimit - server.ServerInfo.ConnectionCount
		if freeConn > uint32(maxFreeConn) {
			serverAddr = server.ServerInfo.Addr
			maxFreeConn = int(freeConn)
		}
	}
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

	if !contains(serverList, byteArrToString(serverInfo.Mac[:])) {
		_, err = u.Write(u.Raddr, 1, []byte("Unknown device"))
		return
	}

	_, err = u.Write(u.Raddr, 1025, []byte(addrStr))
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
	_, ok := deviceStorage[deviceInfo.Mac]
	if ok {
		data := deviceStorage[deviceInfo.Mac]
		data.Time = time
		data.GenericInfo = deviceInfo
		deviceStorage[deviceInfo.Mac] = data
		if data.ToConnTCP {
			_, err = u.Write(u.Raddr, 1025, []byte(addrStr))
			if err != nil {
				err = fmt.Errorf("UdpWrite: %s", err)
				return
			}
		}
		// fmt.Printf("Данные о роутере %s обновились\n", string(deviceInfo.Mac[:]))
	} else {
		deviceStorage[deviceInfo.Mac] = DevicePayload{
			deviceInfo, time, false, false,
		}
		// fmt.Printf("Роутер %s добавлен\n", deviceInfo.Mac)
	}

	return
}

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

// -----------------------------
func deviceByMac() {
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		err = fmt.Errorf("%s %s", "sql.Open", err)
		return
	}

	err = db.Ping()
	if err != nil {
		err = fmt.Errorf("%s %s", "Ping", err)
		return
	}

	defer db.Close()
	db.Exec("SELECT")
}

func toSql(dbChan chan bool) (err error) {
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		err = fmt.Errorf("%s %s", "sql.Open", err)
		return
	}

	err = db.Ping()
	if err != nil {
		err = fmt.Errorf("%s %s", "Ping", err)
		return
	}

	defer db.Close()

	dbChan <- true

	defer func() {
		<-dbChan
	}()

	for key, value := range deviceStorage {
		if value.SentToDB {
			continue
		}

		_, err := db.Exec(fmt.Sprintf("INSERT INTO deviceInfo (mac) VALUES ('%s');",
			value.GenericInfo.Mac,
		))
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				value.SentToDB = true
				deviceStorage[key] = value
				continue
			}

			err = fmt.Errorf("%s %s", "INSERT", err)
			return err
		}

		value.SentToDB = true
		deviceStorage[key] = value
	}
	// fmt.Println(storage)
	return
}
