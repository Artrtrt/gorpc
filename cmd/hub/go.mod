module hub

go 1.20

replace gopack/xbyte => ../../../gopack/xbyte/

replace gopack/tlv => ../../../gopack/tlv/

replace gopack/tagrpc => ../../../gopack/tagrpc/

replace gopack/jsonrpc => ../../../gopack/jsonrpc/

replace internal => ../../internal/

replace pkg => ../../pkg/

replace sqlctrl => ../../../go_mod_sqlctrl-1.0.0/

require gopack/xbyte v0.0.0

require gopack/tlv v0.0.0

require github.com/mattn/go-sqlite3 v1.14.22

require gopack/tagrpc v0.0.0-00010101000000-000000000000

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	internal v0.0.0-00010101000000-000000000000 // indirect
	modernc.org/gc/v3 v3.0.0-20240107210532-573471604cb6 // indirect
	modernc.org/libc v1.49.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/sqlite v1.29.8 // indirect
	modernc.org/strutil v1.2.0 // indirect
	modernc.org/token v1.1.0 // indirect
	pkg v0.0.0-00010101000000-000000000000 // indirect
	sqlctrl v0.0.0-00010101000000-000000000000 // indirect
)
