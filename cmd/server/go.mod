module server

go 1.20

replace gopack/xbyte => ../../../gopack/xbyte/

replace gopack/tlv => ../../../gopack/tlv/

replace gopack/jsonrpc => ../../../gopack/jsonrpc/

replace gopack/tagrpc => ../../../gopack/tagrpc/

replace internal => ../../internal/

replace pkg => ../../pkg/

require (
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tagrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tlv v0.0.0-00010101000000-000000000000 // indirect
	gopack/xbyte v0.0.0-00010101000000-000000000000 // indirect
	internal v0.0.0-00010101000000-000000000000 // indirect
	pkg v0.0.0-00010101000000-000000000000 // indirect
)
