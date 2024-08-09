module gorpc

go 1.20

replace gopack/xbyte => ../gopack/xbyte/

replace gopack/tlv => ../gopack/tlv/

replace gopack/tagrpc => ../gopack/tagrpc/

replace gopack/jsonrpc => ../gopack/jsonrpc/

require (
	github.com/google/uuid v1.6.0
	gopack/tagrpc v0.0.0-00010101000000-000000000000
	gopack/xbyte v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sync v0.8.0
	golang.org/x/sys v0.6.0 // indirect
)
