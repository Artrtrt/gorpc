module server

go 1.20

replace tcp => ../tcp/

replace typedef => ../typedef/

replace gopack/xbyte => ../../gopack/xbyte/

replace gopack/tlv => ../../gopack/tlv/

replace gopack/jsonrpc => ../../gopack/jsonrpc/

replace gopack/tagrpc => ../../gopack/tagrpc/

replace rsautil => ../rsautil/

replace tag => ../tag/

require tcp v0.0.0-00010101000000-000000000000

require (
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tagrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tlv v0.0.0-00010101000000-000000000000 // indirect
	gopack/xbyte v0.0.0-00010101000000-000000000000 // indirect
	rsautil v0.0.0-00010101000000-000000000000 // indirect
	tag v0.0.0-00010101000000-000000000000 // indirect
	typedef v0.0.0-00010101000000-000000000000 // indirect
)
