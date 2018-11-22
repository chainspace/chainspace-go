module chainspace.io/prototype

require (
	cloud.google.com/go v0.27.0 // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20170702084017-28f7e881ca57 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.11 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc
	github.com/cenkalti/backoff v2.0.0+incompatible // indirect
	github.com/dchest/siphash v1.2.0 // indirect
	github.com/dgraph-io/badger v1.5.3
	github.com/dgryski/go-farm v0.0.0-20180109070241-2de33835d102 // indirect
	github.com/docker/distribution v2.6.2+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.3.3 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/gin-contrib/cors v0.0.0-20181008113111-488de3ec974f
	github.com/gin-contrib/sse v0.0.0-20170109093832-22d885f9ecc7 // indirect
	github.com/gin-gonic/gin v1.3.0
	github.com/go-openapi/jsonreference v0.17.2 // indirect
	github.com/go-openapi/spec v0.17.2 // indirect
	github.com/gogo/protobuf v1.1.1
	github.com/golang/protobuf v1.2.0
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2 // indirect
	github.com/grandcat/zeroconf v0.0.0-20180329153754-df75bb3ccae1
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.3 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/miekg/dns v1.0.8 // indirect
	github.com/minio/highwayhash v0.0.0-20180501080913-85fc8a2dacad
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.8.0 // indirect
	github.com/rs/cors v1.5.0
	github.com/sirupsen/logrus v1.2.0 // indirect
	github.com/stretchr/testify v1.2.2
	github.com/swaggo/gin-swagger v1.0.0
	github.com/swaggo/swag v1.4.0
	github.com/tav/golly v0.0.0-20180823113506-ad032321f11e
	github.com/ugorji/go/codec v0.0.0-20181120210156-7d13b37dbec6 // indirect
	golang.org/x/crypto v0.0.0-20180904163835-0709b304e793
	golang.org/x/net v0.0.0-20181005035420-146acd28ed58
	golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be
	golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2 // indirect
	golang.org/x/tools v0.0.0-20181117154741-2ddaf7f79a09 // indirect
	google.golang.org/api v0.0.0-20180906000440-49a9310a9145
	google.golang.org/appengine v1.1.0
	google.golang.org/grpc v1.16.0 // indirect
	gopkg.in/go-playground/validator.v8 v8.18.2 // indirect
	gopkg.in/yaml.v2 v2.2.1
	gotest.tools v2.1.0+incompatible // indirect
)

replace github.com/docker/docker v1.13.1 => github.com/docker/engine v0.0.0-20180905181431-7485ef7e46e2

replace github.com/docker/distribution v2.6.2+incompatible => github.com/docker/distribution v2.7.0-rc.0.0.20181002220433-1cb4180b1a5b+incompatible
