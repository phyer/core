module github.com/phyer/core

replace (
	v5sdk_go/config => ./submodules/okex/config
	v5sdk_go/rest => ./submodules/okex/rest
	v5sdk_go/utils => ./submodules/okex/utils
	v5sdk_go/ws => ./submodules/okex/ws
)

go 1.21

require (
	github.com/bitly/go-simplejson v0.5.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/phyer/texus v0.0.0-20241207132635-0e7fb63f8196
	github.com/sirupsen/logrus v1.9.3
	v5sdk_go/rest v0.0.0-00010101000000-000000000000
	v5sdk_go/ws v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	v5sdk_go/config v0.0.0-00010101000000-000000000000 // indirect
	v5sdk_go/utils v0.0.0-00010101000000-000000000000 // indirect
)
