module client

go 1.20

replace gopack/tlv => ../../gopack/tlv/

replace gopack/tagrpc => ../../gopack/tagrpc/

replace rsautil => ../rsautil/

replace tcp => ../tcp/

require gopack/tlv v0.0.0

replace gopack/xbyte => ../../gopack/xbyte/

require (
	gopack/xbyte v0.0.0
	rsautil v0.0.0-00010101000000-000000000000
	tcp v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopack/tagrpc v0.0.0-00010101000000-000000000000 // indirect
)
