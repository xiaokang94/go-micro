package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/micro/go-micro/v2/client"
	grpcClient "github.com/micro/go-micro/v2/client/grpc"
	"github.com/micro/go-micro/v2/router"
	"github.com/micro/go-micro/v2/registry/memory"
	"github.com/micro/go-micro/v2/server"
	grpcServer "github.com/micro/go-micro/v2/server/grpc"
)

type testHandler struct{}

func (t *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"hello": "world"}`))
}

func TestHTTPProxy(t *testing.T) {
	c, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	addr := c.Addr().String()

	url := fmt.Sprintf("http://%s", addr)

	testCases := []struct {
		// http endpoint to call e.g /foo/bar
		httpEp string
		// rpc endpoint called e.g Foo.Bar
		rpcEp string
		// should be an error
		err bool
	}{
		{"/", "Foo.Bar", false},
		{"/", "Foo.Baz", false},
		{"/helloworld", "Hello.World", true},
	}

	// handler
	http.Handle("/", new(testHandler))

	// new proxy
	p := NewSingleHostProxy(url)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := memory.NewRegistry()
	rtr := router.NewRouter(router.Registry(reg))

	// new micro service
	srv := grpcServer.NewServer(
		server.Name("foobar"),
		server.Registry(reg),
		server.WithRouter(p),
	)

	cli := grpcClient.NewClient(
		client.Router(rtr),
	)

	// run service
	go http.Serve(c, nil)

	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	time.Sleep(time.Second)
	for _, test := range testCases {
		req := cli.NewRequest("foobar", test.rpcEp, map[string]string{"foo": "bar"}, client.WithContentType("application/json"))
		var rsp map[string]string
		err := cli.Call(ctx, req, &rsp)
		if err != nil && test.err == false {
			t.Fatal(err)
		}
		if v := rsp["hello"]; v != "world" {
			t.Fatalf("Expected hello world got %s from %s", v, test.rpcEp)
		}
	}
}
