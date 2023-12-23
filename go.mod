module github.com/clash-dev/clash

go 1.21.3

require (
	github.com/Dreamacro/protobytes v0.0.0-20230911123819-0bbf144b9b9a
	github.com/adrg/xdg v0.4.0
	github.com/clash-dev/clash/api v0.0.0
	github.com/dlclark/regexp2 v1.10.0
	github.com/go-chi/chi/v5 v5.0.10
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/render v1.0.3
	github.com/gofrs/uuid/v5 v5.0.0
	github.com/gorilla/websocket v1.5.1
	github.com/insomniacslk/dhcp v0.0.0-20231126010706-b0416c0f187a
	github.com/kr/pretty v0.3.1
	github.com/mdlayher/netlink v1.7.2
	github.com/miekg/dns v1.1.57
	github.com/oschwald/geoip2-golang v1.9.0
	github.com/samber/lo v1.39.0
	github.com/spf13/cobra v1.8.0
	github.com/square/exit v1.1.0
	github.com/stretchr/testify v1.8.4
	github.com/vishvananda/netlink v1.2.1-beta.2.0.20230420174744-55c8b9515a01
	go.etcd.io/bbolt v1.3.8
	go.uber.org/atomic v1.11.0
	go.uber.org/automaxprocs v1.5.3
	golang.org/x/crypto v0.16.0
	golang.org/x/net v0.19.0
	golang.org/x/sync v0.5.0
	golang.org/x/sys v0.15.0
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mdlayher/socket v0.5.0 // indirect
	github.com/oschwald/maxminddb-golang v1.12.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/u-root/uio v0.0.0-20230305220412-3e8cd9d6bf63 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20231127185646-65229373498e // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231127180814-3a041ad873d4 // indirect

)

replace github.com/clash-dev/clash/api => ./api
