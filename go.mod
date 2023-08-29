module github.com/nexodus-io/nexodus

go 1.20

require (
	github.com/Nerzal/gocloak/v13 v13.5.0
	github.com/ahmetb/dlog v0.0.0-20170105205344-4fb5f8204f26
	github.com/briandowns/spinner v1.23.0
	github.com/bufbuild/connect-go v1.8.0
	github.com/bytedance/gopkg v0.0.0-20221122125632-68358b8ecec6
	github.com/cockroachdb/cockroach-go/v2 v2.3.3
	github.com/coreos/go-oidc/v3 v3.6.0
	github.com/cucumber/godog v0.12.6
	github.com/docker/docker v24.0.5-0.20230714235725-36e9e796c6fc+incompatible // 24.0 branch
	github.com/envoyproxy/go-control-plane v0.11.0
	github.com/fatih/color v1.15.0
	github.com/gin-contrib/cors v1.4.0
	github.com/gin-contrib/zap v0.1.0
	github.com/gin-gonic/gin v1.9.1
	github.com/go-gormigrate/gormigrate/v2 v2.0.2
	github.com/go-jose/go-jose/v3 v3.0.0
	github.com/go-session/redis/v3 v3.1.0
	github.com/go-session/session/v3 v3.2.1
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/securecookie v1.1.1
	github.com/itchyny/gojq v0.12.13
	github.com/libp2p/go-reuseport v0.3.0
	github.com/metal-stack/go-ipam v1.11.6
	github.com/natefinch/atomic v1.0.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/pion/stun v0.6.0
	github.com/redis/go-redis/v9 v9.0.2
	github.com/sirupsen/logrus v1.9.2
	github.com/stretchr/testify v1.8.4
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	github.com/swaggo/swag v1.16.1
	github.com/testcontainers/testcontainers-go v0.21.0
	github.com/uptrace/opentelemetry-go-extra/otelgorm v0.2.0
	github.com/urfave/cli/v2 v2.25.3
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/zsais/go-gin-prometheus v0.1.0
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.41.1
	go.opentelemetry.io/otel v1.15.1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.15.1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.15.1
	go.opentelemetry.io/otel/sdk v1.15.1
	go.opentelemetry.io/otel/trace v1.15.1
	go.uber.org/zap v1.24.0
	golang.org/x/net v0.13.0
	golang.org/x/oauth2 v0.8.0
	golang.org/x/sys v0.10.0
	golang.org/x/term v0.10.0
	golang.zx2c4.com/wireguard v0.0.0-20230325221338-052af4a8072b
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230215201556-9c5414ab4bde
	gorm.io/driver/postgres v1.5.0
	gorm.io/driver/sqlite v1.4.4
	gorm.io/gorm v1.24.7-0.20230306060331-85eaf9eeda11
	gvisor.dev/gvisor v0.0.0-20221203005347-703fd9b7fbc0
	k8s.io/api v0.27.4
	k8s.io/apimachinery v0.27.4
	k8s.io/client-go v0.27.4
)

require github.com/natefinch/pie v0.0.0-20170715172608-9a0d72014007

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/agnivade/levenshtein v1.1.1 // indirect
	github.com/ahmetalpbalkan/dlog v0.0.0-20170105205344-4fb5f8204f26 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cncf/xds/go v0.0.0-20230310173818-32f1caf87195 // indirect
	github.com/containerd/containerd v1.6.19 // indirect
	github.com/cpuguy83/dockercfg v0.3.1 // indirect
	github.com/cucumber/gherkin-go/v19 v19.0.3 // indirect
	github.com/cucumber/messages-go/v16 v16.0.1 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.10.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-redis/redis/v8 v8.11.4 // indirect
	github.com/go-resty/resty/v2 v2.7.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-memdb v1.3.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/itchyny/timefmt-go v0.1.5 // indirect
	github.com/jackc/pgx/v5 v5.3.0 // indirect
	github.com/josharian/native v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/genetlink v1.2.0 // indirect
	github.com/mdlayher/netlink v1.6.2 // indirect
	github.com/mdlayher/socket v0.2.3 // indirect
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2 // indirect
	github.com/opencontainers/runc v1.1.5 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pion/dtls/v2 v2.2.7 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/transport/v2 v2.2.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/uptrace/opentelemetry-go-extra/otelsql v0.2.0 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yashtewari/glob-intersection v0.1.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.15.1 // indirect
	go.opentelemetry.io/otel/metric v0.38.1 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/exp v0.0.0-20230510235704-dd950f8aeaea // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/avast/retry-go/v4 v4.3.3 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/spec v0.20.7 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmoiron/sqlx v1.3.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lib/pq v1.10.9
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/montanaflynn/stats v0.7.0 // indirect
	github.com/open-policy-agent/opa v0.50.2
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	go.etcd.io/etcd/api/v3 v3.5.7 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.7 // indirect
	go.etcd.io/etcd/client/v3 v3.5.7 // indirect
	go.mongodb.org/mongo-driver v1.11.2 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	go4.org/netipx v0.0.0-20230125063823-8449b0a6169f // indirect
	golang.org/x/crypto v0.11.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.11.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20230306155012-7f2fa6fef1f4
	google.golang.org/grpc v1.55.0
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/metal-stack/go-ipam => github.com/dave-tucker/go-ipam v0.0.0-20230220173413-c25c56b2408b

replace golang.zx2c4.com/wireguard => github.com/nexodus-io/wireguard-go v0.0.0-20230407202523-3eab17a590b0 // 0.0.20230420 tag
