package service

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"gopack/tagrpc"
	"internal/typedef"
)

const (
	TagSendDeviceInfoUdp = 3073
)

type ConnectToServer struct {
	TrpcDefaultHandler
	ExecuteJsonRPC
	*typedef.GenericInfo
}

func (data ConnectToServer) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	defer n.Response(TagConnectToServer, []byte("OK"))

	if data.GenericInfo.Busy {
		return
	}

	val = bytes.TrimRightFunc(val, func(r rune) bool {
		return r == 0
	})

	output, err := n.Codec.Decode(val)
	if err != nil {
		err = fmt.Errorf("Encode: %s", err)
		return
	}

	fmt.Println(string(output))
	tcpAddr, err := net.ResolveTCPAddr("tcp", string(val))
	if err != nil {
		err = fmt.Errorf("ResolveTCPAddr: %s", err)
		return
	}

	conn, err := tagrpc.DialTCP(nil, tcpAddr)
	if err != nil {
		err = fmt.Errorf("DialTCP: %s", err)
		return
	}

	conn.HandleFunc(TagRemoteErr, data.TrpcDefaultHandler.RemoteErr.Handler)
	conn.HandleFunc(TagRsaSetup, data.TrpcDefaultHandler.RsaSetup.Handler)
	conn.HandleFunc(TagSendGenericInfo, data.TrpcDefaultHandler.SendGenericInfo.Handler)
	conn.HandleFunc(TagExecuteJsonRPC, data.ExecuteJsonRPC.Handler)
	fmt.Printf("Подключился к серверу %s\n", conn.Tcp.RemoteAddr())

	go func(*tagrpc.TCPConn) {
		data.GenericInfo.Busy = true
		for {
			err = conn.Update(time.Second * 60)
			if err != nil {
				data.GenericInfo.Busy = false
				fmt.Println(err)
				return
			}
		}
	}(conn)

	return
}

type ExecuteJsonRPC struct {
	Endpoint string
}

func (data ExecuteJsonRPC) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	val = bytes.TrimRightFunc(val, func(r rune) bool {
		return r == 0
	})

	body := bytes.NewReader(val)
	resp, err := http.Post(data.Endpoint, "application/json", body)
	if err != nil {
		err = fmt.Errorf("%s %s", "POST", err.Error())
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("%s %s", "ReadAll", err.Error())
		return
	}

	n.Response(TagExecuteJsonRPC, bodyBytes)
	return
}
