package main

import (
	"crypto/rsa"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"tag"
	"tcp"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"rsautil"
	"typedef"

	_ "github.com/mattn/go-sqlite3"
)

type DeviceStorage struct {
	DeviceInfo typedef.DeviceInfo
	Time       int64
	SentToDB   bool
	ToConnTCP  bool
}

type ServerStorage struct {
	ServerInfo typedef.ServerInfo
}

var (
	err           error
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	addr          string = "localhost:8080"
	httpAddr      string = ":8081"
	deviceStorage        = make(map[[32]byte]DeviceStorage)
	serverStorage        = make(map[[32]byte]typedef.ServerInfo)
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
			if payload.DeviceInfo.Busy {
				fmt.Fprint(rw, "Устройство занято")
				return
			} else {
				payload.ToConnTCP = true
				payload.DeviceInfo.Busy = true
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

	dbChan := make(chan bool, 1)
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exit
		toSql(dbChan)
		os.Exit(1)
	}()

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
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
	configureTcp(tcpLr)

	udp, err := tag.NewUdp("localhost:2000")
	if err != nil {
		fmt.Println("NewUdp:", err)
		return
	}

	defer udp.Close()
	go configureUdp(udp)
	fmt.Println("Слухает")

	// go func() {
	// 	for {
	// 		err := toSql(dbChan)

	// 		if err != nil {
	// 			fmt.Println("toSql", err)
	// 		}

	// 		time.Sleep(time.Second * 10)
	// 	}
	// }()

	go httpServer()
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

		go func() {
			clientPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
			if err != nil {
				fmt.Println("RsaKeyExchange", err)
				return
			}

			conn.Codec = tagrpc.NewRsaCodec(privateKey, clientPublicKey)
			fmt.Println("Подключился " + conn.Tcp.RemoteAddr().String())
			for {
				err = conn.Update(100000000000000)
				if err != nil {
					fmt.Println(err)
					return
				}
			}
		}()
	}
}

func configureTcp(lr *tagrpc.TCPListener) {
	lr.HandleFunc(1, remoteErr)
	lr.HandleFunc(2050, sendClientHttpAddr)
	lr.HandleFunc(2051, closeDeviceConnection)
	lr.HandleFunc(3075, sendServerAddr)
}

func remoteErr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return errors.New(fmt.Sprint("remoteErr:", string(val)))
}

func sendClientHttpAddr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.DeviceInfo{}
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

func sendServerAddr(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.DeviceInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	info, ok := deviceStorage[deviceInfo.Mac]
	if !ok {
		n.Write(1, []byte("Unknown device"))
		return
	}

	info.ToConnTCP = false
	deviceStorage[deviceInfo.Mac] = info
	addr, err := lessBusyServer(serverStorage)
	if err != nil {
		n.Write(1, []byte(fmt.Sprintf("lessBusyServer %s", err)))
		err = fmt.Errorf("lessBusyServer: %s", err)
		return
	}

	if addr[0] == 0 {
		err = fmt.Errorf("wrong server addr")
		return
	}

	err = n.Write(1026, addr[:])
	if err != nil {
		err = fmt.Errorf("tagrpc Write: %s", err)
		return
	}

	err = n.Close()
	if err != nil {
		fmt.Println("tagrpc Close", err)
		return
	}

	return
}

func lessBusyServer(storage map[[32]byte]typedef.ServerInfo) (serverAddr [32]byte, err error) {
	if len(storage) == 0 {
		err = fmt.Errorf("%s", "No servers")
		return
	}

	maxFreeConnections := 0
	for addr, server := range storage {
		if (server.ConnectionLimit - server.ConnectionCount) > uint32(maxFreeConnections) {
			serverAddr = addr
		}
	}
	return
}

// UDP
func configureUdp(udp *tag.Udp) {
	udp.Handle(2049, receiveServerInfo)
	udp.Handle(3073, receiveDeviceInfo)

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

func receiveServerInfo(u *tag.Udp, tag uint16, val []byte) (err error) {
	serverInfo := typedef.ServerInfo{}
	err = xbyte.ByteToStruct(val, &serverInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	_, ok := serverStorage[serverInfo.Addr]
	if ok {
		data := serverStorage[serverInfo.Addr]
		data.ConnectionCount = serverInfo.ConnectionCount
		data.ConnectionLimit = serverInfo.ConnectionLimit
	} else {
		serverStorage[serverInfo.Addr] = typedef.ServerInfo{
			Addr: serverInfo.Addr, ConnectionCount: serverInfo.ConnectionCount, ConnectionLimit: serverInfo.ConnectionLimit,
		}
	}

	return
}

func receiveDeviceInfo(u *tag.Udp, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.DeviceInfo{}
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
		data.DeviceInfo = deviceInfo
		deviceStorage[deviceInfo.Mac] = data
		if data.ToConnTCP {
			_, err = u.Write(u.Raddr, 1025, []byte(addr))
			if err != nil {
				err = fmt.Errorf("UdpWrite: %s", err)
				return
			}
		}
		// fmt.Printf("Данные о роутере %s обновились\n", string(deviceInfo.Mac[:]))
	} else {
		deviceStorage[deviceInfo.Mac] = DeviceStorage{
			deviceInfo, time, false, false,
		}
		// fmt.Printf("Роутер %s добавлен\n", deviceInfo.Mac)
	}

	return
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
			value.DeviceInfo.Mac,
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
