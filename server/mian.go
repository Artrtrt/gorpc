package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"gopack/jsonrpc"
	"net"
	"net/http"
	"rsautil"
	"tcp"
)

var (
	err        error
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	addr       string = "localhost:8082"
	httpAddr   string = ":8083"
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

func httpServer(conn *tcp.RsaConn) {
	http.Handle("/", http.FileServer(http.Dir("./dist")))
	http.ListenAndServe(httpAddr, http.HandlerFunc(handleRPC))
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

	go httpServer()
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		fmt.Println("ResolveTCPAddr:", err)
		return
	}

	tpcLr, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		fmt.Println("ListenTCP:", err)
		return
	}

	defer tpcLr.Close()

	for {
		conn, err := tpcLr.AcceptTCP()
		if err != nil {
			fmt.Println("AcceptTCP:", err)
			continue
		}

		go func() {
			clientPublicKey, err := tcp.RsaKeyExchange(conn, publicKey)
			if err != nil {
				conn.Close()
				fmt.Println("RsaKeyExchange:", err)
				return
			}

			rsaConn := tcp.NewRsaConn(clientPublicKey, privateKey, conn)
			defer rsaConn.Close()
			fmt.Println(conn.RemoteAddr().String())
			requestch := make(chan []byte)
			responsech := make(chan []byte)
			go httpServer(requestch, responsech)

			for {
				tag, val, err := rsaConn.Read()
				if err != nil {
					fmt.Println("Conn read:", err)
					return
				}

				fmt.Println(tag, val)
			}
		}()
	}
}
