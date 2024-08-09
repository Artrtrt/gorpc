package components

import "gopack/tagrpc"

type ComponentInterface interface {
	SetHandler(tag int, handler HandlerInterface)
}

type HandlerInterface interface {
	Run(n *tagrpc.Node, tag uint16, val []byte) (err error)
}
