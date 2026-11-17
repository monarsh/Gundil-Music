package main

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Android has no /etc/resolv.conf, so Go's built-in resolver (used in this
// cgo-free build) would ask 127.0.0.1:53 and fail. Use the nameservers Termux
// configures, falling back to public DNS.
func init() {
	servers := []string{}
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}
	if f, err := os.Open(filepath.Join(prefix, "etc", "resolv.conf")); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) >= 2 && fields[0] == "nameserver" && net.ParseIP(fields[1]) != nil {
				servers = append(servers, net.JoinHostPort(fields[1], "53"))
			}
		}
		f.Close()
	}
	servers = append(servers, "8.8.8.8:53", "1.1.1.1:53")
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			var err error
			for _, s := range servers {
				var c net.Conn
				if c, err = d.DialContext(ctx, network, s); err == nil {
					return c, nil
				}
			}
			return nil, err
		},
	}
}
