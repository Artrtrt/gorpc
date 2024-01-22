module server

go 1.20

replace tcp => ../tcp/

replace gopack/xbyte => ../../gopack/xbyte/

replace gopack/tlv => ../../gopack/tlv/

replace gopack/jsonrpc => ../../gopack/jsonrpc/

replace rsautil => ../rsautil/

require tcp v0.0.0-00010101000000-000000000000

require (
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tlv v0.0.0-00010101000000-000000000000 // indirect
	gopack/xbyte v0.0.0-00010101000000-000000000000 // indirect
	rsautil v0.0.0-00010101000000-000000000000 // indirect
)
