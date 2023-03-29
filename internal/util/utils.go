package util

import (
	"context"
	"net"
	"net/http"
	"time"

	goipam "github.com/metal-stack/go-ipam"
	"github.com/metal-stack/go-ipam/api/v1/apiv1connect"
	"github.com/metal-stack/go-ipam/pkg/service"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	TestIPAMClientAddr = "http://localhost:9090"
)

func NewTestIPAMServer() *http.Server {
	zlog := zap.NewNop()
	ipam := goipam.New(context.Background())
	mux := http.NewServeMux()
	mux.Handle(apiv1connect.NewIpamServiceHandler(service.New(zlog.Sugar(), ipam)))

	server := &http.Server{
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 1 * time.Minute,
	}
	return server
}

/* maybe we can use a generic version in the future...
func ToStringSlice[S fmt.Stringer](items []S) (result []string) {
*/

func IPNetSliceToStringSlice(items []net.IPNet) (result []string) {
	for _, i := range items {
		result = append(result, i.String())
	}
	return
}
