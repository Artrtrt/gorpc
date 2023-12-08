package main

import (
	"bytes"
	"crypto/rsa"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopack/rsautil"
	"gopack/tlv"
	"gopack/xbyte"

	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	DeviceInfo DeviceInfo
	publickey  *rsa.PublicKey
	Time       int64
	Sent       bool
	toConnTPC  bool
}

type DeviceInfo struct {
	Mac    [32]byte
	Uptime int64
}

var (
	err        error
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	tcpAddr    *net.TCPAddr
	storage    = make(map[[32]byte]Storage)
)

func httpServer() (err error) {
	http.HandleFunc("/hub", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			mac := r.FormValue("mac")
			if mac == "" {
				fmt.Fprint(rw, "Пустой mac")
				return
			}

			macBytes := [32]byte{}
			copy(macBytes[:], []byte(mac))
			_, ok := storage[macBytes]
			if !ok {
				fmt.Fprint(rw, "Такого устройства нет")
				return
			}

			device := storage[macBytes]
			if time.Now().Unix()-device.Time > 120 {
				fmt.Fprint(rw, "Устройство не доступно")
				return
			}

			payload := storage[macBytes]
			payload.toConnTPC = true
			storage[macBytes] = payload
		}
	})
	http.ListenAndServe(":8081", nil)
	return
}

func handleUDPConn(conn *net.UDPConn) {
	var buf bytes.Buffer
	rw := tlv.NewReadWriter(&buf)
	for {
		data := make([]byte, 1024)
		_, raddr, err := conn.ReadFromUDP(data)
		if err != nil {
			fmt.Println("ReadFromUDP", err)
			continue
		}

		_, err = buf.Write(data)
		if err != nil {
			fmt.Println("Write", err)
			continue
		}
		tag, val, err := rw.Read()
		if err != nil {
			fmt.Println("Tlv", err)
			continue
		}

		buf.Reset() // Иначе буфер остается заполнен нулями
		switch tag {
		case 1:
			fmt.Printf("Ошибка %s. От устройства %s", string(val), raddr.String())
		case 3073:
			deviceInfo := DeviceInfo{}
			err = xbyte.ByteToStruct(val, &deviceInfo)
			if err != nil {
				fmt.Println("ByteToStruct", err)
				continue
			}

			time := time.Now().Unix()
			_, ok := storage[deviceInfo.Mac]
			if ok {
				payload := storage[deviceInfo.Mac]
				payload.Time = time
				storage[deviceInfo.Mac] = payload
				if payload.toConnTPC {
					err = rw.Write(1025, []byte(tcpAddr.String()))
					if err != nil {
						fmt.Println("Tlv", err)
						continue
					}

					_, err = conn.WriteToUDP(buf.Bytes(), raddr)
					if err != nil {
						fmt.Println("Conn write", err)
						continue
					}

					buf.Reset()
				}
				// fmt.Printf("Данные о роутере %s обновились\n", string(deviceInfo.Mac[:]))
			} else {
				storage[deviceInfo.Mac] = Storage{
					deviceInfo, nil, time, false, false,
				}
				// fmt.Printf("Роутер %s добавлен\n", deviceInfo.Mac)
			}
		default:
			err = rw.Write(1, []byte("Unknown tag"))
			if err != nil {
				fmt.Println("Tlv", err)
				continue
			}

			_, err = conn.WriteToUDP(buf.Bytes(), raddr)
			if err != nil {
				fmt.Println("Conn write", err)
				return
			}
			buf.Reset()
		}
	}
}

func handleTCPConn(conn *net.TCPConn, clientPublicKey *rsa.PublicKey) {
	rw := tlv.NewReadWriter(conn)
	for {
		tag, val, err := rw.Read()
		if err != nil {
			time.Sleep(time.Second * 5)
			return
		}

		switch tag {
		case 1:
			fmt.Printf("Ошибка %s. От устройства %s", string(val), conn.RemoteAddr())
		case 3073:
			telemetry, err := rsautil.DecryptPKCS1(privateKey, val)
			if err != nil {
				fmt.Println("DecryptPKCS1", err)
				continue
			}

			deviceInfo := DeviceInfo{}
			err = xbyte.ByteToStruct(telemetry, &deviceInfo)
			if err != nil {
				fmt.Println("ByteToStruct", err)
				continue
			}

			info, ok := storage[deviceInfo.Mac]
			if ok {
				info.toConnTPC = false
				// server, err := lessBusyServer() и в ответ его адресс
				encData, err := rsautil.EncryptPKCS1(clientPublicKey, []byte("hello world"))
				if err != nil {
					fmt.Println("EncryptPKCS1", err)
					continue
				}

				err = rw.Write(1026, encData)
				if err != nil {
					fmt.Println("Tlv", err)
					continue
				}
			}

			err = conn.Close()
			if err != nil {
				fmt.Println(err)
			}

		default:
			rw.Write(1, []byte("Unknown tag"))
		}
	}
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

	tcpAddr, err = net.ResolveTCPAddr("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("ResolveTCPAddr", err)
		return
	}

	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		fmt.Println("ListenTCP", err)
		return
	}

	defer tcpListener.Close()
	addr, err := net.ResolveUDPAddr("udp", "localhost:2000")
	if err != nil {
		fmt.Println("ResolveUDPAddr", err)
		return
	}

	udpListener, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("ListenUDP", err)
		return
	}

	defer udpListener.Close()
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

	go func() {
		for {
			conn, err := tcpListener.AcceptTCP()
			if err != nil {
				fmt.Println("AcceptTCP", err)
				continue
			}

			go func() {
				defer conn.Close()
				if conn == nil {
					fmt.Println("No connection")
					return
				}

				clientPublicKey, err := rsaSetup(conn, publicKey)
				if err != nil {
					fmt.Println("rsaSetup", err)
					conn.Close()
					return
				}

				go handleTCPConn(conn, clientPublicKey)
			}()
		}
	}()

	handleUDPConn(udpListener)
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

	for key, value := range storage {
		if value.Sent {
			continue
		}

		_, err := db.Exec(fmt.Sprintf("INSERT INTO deviceInfo (mac) VALUES ('%s');",
			value.DeviceInfo.Mac,
		))
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				value.Sent = true
				storage[key] = value
				continue
			}

			err = fmt.Errorf("%s %s", "INSERT", err)
			return err
		}

		value.Sent = true
		storage[key] = value
	}
	// fmt.Println(storage)
	return
}
