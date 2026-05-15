package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/yuanting/MLSE/internal/debugview"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	openBrowser := flag.Bool("open", false, "open the debug page in a browser")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <input.go>\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Render Go source and MLSE formal instructions in a browser.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	snapshot, err := debugview.BuildSnapshot(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-debug: %v\n", err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-debug: listen %s: %v\n", *addr, err)
		os.Exit(1)
	}
	url := browserURL(listener.Addr())
	fmt.Fprintf(os.Stderr, "mlse-debug: serving %s\n", url)
	if *openBrowser {
		if err := openURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "mlse-debug: open browser: %v\n", err)
		}
	}

	server := &http.Server{
		Handler:           debugview.NewServer(snapshot),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "mlse-debug: serve: %v\n", err)
		os.Exit(1)
	}
}

func browserURL(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return "http://" + addr.String() + "/"
	}
	host := tcp.IP.String()
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, fmt.Sprint(tcp.Port)) + "/"
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
