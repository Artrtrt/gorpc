module server

go 1.20

replace gopack/xbyte => ../../../gopack/xbyte/

replace gopack/tlv => ../../../gopack/tlv/

replace gopack/jsonrpc => ../../../gopack/jsonrpc/

replace gopack/tagrpc => ../../../gopack/tagrpc/

replace internal/service => ../../internal/service

replace internal/telemetry => ../../internal/telemetry

replace internal/typedef => ../../internal/typedef

replace internal/utils => ../../internal/utils

replace pkg => ../../pkg/

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tagrpc v0.0.0-00010101000000-000000000000 // indirect
	gopack/tlv v0.0.0-00010101000000-000000000000 // indirect
	gopack/xbyte v0.0.0-00010101000000-000000000000 // indirect
	internal/service v0.0.0-00010101000000-000000000000 // indirect
	internal/telemetry v0.0.0-00010101000000-000000000000 // indirect
	internal/typedef v0.0.0-00010101000000-000000000000 // indirect
	internal/utils v0.0.0-00010101000000-000000000000 // indirect
	pkg v0.0.0-00010101000000-000000000000 // indirect
)
