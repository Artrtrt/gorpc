package main

import (
	"crypto/rsa"
	"database/sql"
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

func handleTCPConn(conn *tagrpc.TCPConn) {
	for {
		tag, val, err := conn.Read()
		if err != nil {
			return
		}

		switch tag {
		case 1:
			fmt.Printf("Device err response: %s", string(val))
		case 3075:
			deviceInfo := typedef.DeviceInfo{}
			err = xbyte.ByteToStruct(val, &deviceInfo)
			if err != nil {
				fmt.Println("ByteToStruct", err)
				continue
			}

			info, ok := deviceStorage[deviceInfo.Mac]
			if !ok {
				conn.Write(1, []byte("Unknown device"))
				continue
			}

			info.ToConnTCP = false
			deviceStorage[deviceInfo.Mac] = info
			addr, err := lessBusyServer(serverStorage)
			if err != nil {
				conn.Write(1, []byte(fmt.Sprintf("lessBusyServer %s", err)))
				fmt.Println("lessBusyServer", err)
				continue
			}

			if addr[0] == 0 {
				fmt.Println("Wrong server addr")
				continue
			}

			err = conn.Write(1026, addr[:])
			if err != nil {
				fmt.Println("Conn.Write", err)
				continue
			}

			err = conn.Close()
			if err != nil {
				fmt.Println("Conn.Close", err)
				return
			}

			return
		case 2050:
			deviceInfo := typedef.DeviceInfo{}
			err = xbyte.ByteToStruct(val, &deviceInfo)
			if err != nil {
				fmt.Println("ByteToStruct", err)
				continue
			}

			fmt.Println("Сообщить клиенту адрес куда подключаться")
		case 2051:

		default:
			conn.Write(1, []byte("Unknown tag"))
		}
	}
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

	udp, err := tag.NewUdp("localhost:2000")
	if err != nil {
		fmt.Println("NewUdp:", err)
		return
	}

	defer udp.Close()
	go configureUdp(udp)

	fmt.Println("Слухает")
	go httpServer()

	// go func() {
	// 	for {
	// 		err := toSql(dbChan)

	// 		if err != nil {
	// 			fmt.Println("toSql", err)
	// 		}

	// 		time.Sleep(time.Second * 10)
	// 	}
	// }()

	acceptTcp(tcpLr)
}

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

func acceptTcp(lr *tagrpc.TCPListener) {
	for {
		tcpconn, err := lr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP", err)
			continue
		}
		go func() {
			clientPublicKey, err := tcp.RsaKeyExchange(tcpconn, publicKey)
			if err != nil {
				tcpconn.Close()
				fmt.Println("RsaKeyExchange", err)
				return
			}

			tcpconn.Codec = tagrpc.NewRsaCodec(privateKey, clientPublicKey)
			defer tcpconn.Close()
			fmt.Println("Подключился " + tcpconn.Tcp.RemoteAddr().String())
			handleTCPConn(tcpconn)
		}()
	}
}

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
