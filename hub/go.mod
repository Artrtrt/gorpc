module hub

go 1.20

replace tcp => ../tcp/

replace gopack/xbyte => ../../gopack/xbyte/

replace gopack/tlv => ../../gopack/tlv/

replace gopack/tagrpc => ../../gopack/tagrpc/

replace rsautil => ../rsautil/

require gopack/xbyte v0.0.0

require gopack/tlv v0.0.0

require rsautil v0.0.0

require github.com/mattn/go-sqlite3 v1.14.18

require tcp v0.0.0-00010101000000-000000000000

require (
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopack/tagrpc v0.0.0-00010101000000-000000000000 // indirect
)
