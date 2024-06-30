module client

go 1.20

replace gopack/tlv => ../../../gopack/tlv/

replace gopack/tagrpc => ../../../gopack/tagrpc/

replace gopack/jsonrpc => ../../../gopack/jsonrpc/

replace internal/service => ../../internal/service

replace internal/telemetry => ../../internal/telemetry

replace internal/typedef => ../../internal/typedef

replace internal/utils => ../../internal/utils

replace pkg => ../../pkg/

replace gopack/xbyte => ../../../gopack/xbyte/

require (
	gopack/tagrpc v0.0.0-00010101000000-000000000000
	gopack/tlv v0.0.0
	gopack/xbyte v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	gopack/jsonrpc v0.0.0-00010101000000-000000000000 // indirect
	internal/service v0.0.0-00010101000000-000000000000 // indirect
	internal/telemetry v0.0.0-00010101000000-000000000000 // indirect
	internal/typedef v0.0.0-00010101000000-000000000000 // indirect
	internal/utils v0.0.0-00010101000000-000000000000 // indirect
	pkg v0.0.0-00010101000000-000000000000 // indirect
)
