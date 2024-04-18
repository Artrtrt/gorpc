module client

go 1.20

replace gopack/tlv => ../../../gopack/tlv/

replace gopack/tagrpc => ../../../gopack/tagrpc/

replace gopack/jsonrpc => ../../../gopack/jsonrpc/

replace internal => ../../internal/

replace pkg => ../../pkg/

replace gopack/xbyte => ../../../gopack/xbyte/

require (
	gopack/tagrpc v0.0.0-00010101000000-000000000000
	gopack/tlv v0.0.0
	gopack/xbyte v0.0.0
)

require (
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	internal v0.0.0-00010101000000-000000000000 // indirect
	pkg v0.0.0-00010101000000-000000000000 // indirect
)
