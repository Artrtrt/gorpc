module hub

go 1.20

replace typedef => ../../internal/typedef/

replace tag => ../../internal/tag/

replace gopack/xbyte => ../../../gopack/xbyte/

replace gopack/tlv => ../../../gopack/tlv/

replace gopack/tagrpc => ../../../gopack/tagrpc/

replace gopack/jsonrpc => ../../../gopack/jsonrpc/

replace utils => ../../internal/utils/

require gopack/xbyte v0.0.0

require gopack/tlv v0.0.0

require github.com/mattn/go-sqlite3 v1.14.19

require gopack/tagrpc v0.0.0-00010101000000-000000000000

require (
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	tag v0.0.0-00010101000000-000000000000 // indirect
	typedef v0.0.0-00010101000000-000000000000 // indirect
	utils v0.0.0-00010101000000-000000000000 // indirect
)
