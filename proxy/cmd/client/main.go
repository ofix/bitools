package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"proxy/core"
)

func main() {
	proto := flag.String("proto", "socks5", "socks5 / connect")
	serverAddr := flag.String("server", "", "remote server addr, leave empty when -discover=true")
	transportName := flag.String("transport", "http2", "http2 / ssh2 / quic")
	discover := flag.Bool("discover", false, "auto discover server via multicast")
	discoverTimeout := flag.Int("discover-timeout", 5, "discover timeout seconds")

	// ssh 参数
	sshUser := flag.String("ssh-user", "", "ssh username")
	sshKeyPath := flag.String("ssh-key", "", "ssh private key path")
	sshPass := flag.String("ssh-pass", "", "ssh password")

	flag.Parse()

	var targetAddr string

	if *discover {
		fmt.Printf("Start multicast discover, timeout: %d s\n", *discoverTimeout)
		finder, err := core.NewFinder()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Create finder failed: %v\n", err)
			os.Exit(1)
		}
		defer finder.Close()

		annCh := make(chan *core.TunnelAnnounce, 1)
		errCh := make(chan error, 1)

		go func() {
			ann, err := finder.WaitAnnounce()
			if err != nil {
				errCh <- err
				return
			}
			annCh <- ann
		}()

		select {
		case <-time.After(time.Duration(*discoverTimeout) * time.Second):
			fmt.Fprintf(os.Stderr, "Discover timeout, no server found\n")
			os.Exit(1)
		case err := <-errCh:
			fmt.Fprintf(os.Stderr, "Discover error: %v\n", err)
			os.Exit(1)
		case ann := <-annCh:
			// 根据所选transport取对应地址
			switch *transportName {
			case "http2":
				targetAddr = ann.HTTP2Addr
			case "ssh2":
				targetAddr = ann.SSH2Addr
			case "quic":
				targetAddr = ann.QUICAddr
			default:
				fmt.Fprintf(os.Stderr, "unknown transport: %s\n", *transportName)
				os.Exit(1)
			}
			fmt.Printf("✅ Found tunnel server: %s\n", targetAddr)
		}
	} else {
		if *serverAddr == "" {
			fmt.Fprintf(os.Stderr, "Must set -server or enable -discover=true\n")
			os.Exit(1)
		}
		targetAddr = *serverAddr
		fmt.Printf("Use manual server addr: %s\n", targetAddr)
	}

	// 建立隧道 transport
	var t core.Transport
	var err error
	switch *transportName {
	case "http2":
		t, err = core.NewHTTP2Transport(targetAddr)
	case "ssh2":
		t, err = core.NewSSH2Transport(targetAddr, *sshUser, *sshKeyPath, *sshPass)
	case "quic":
		t, err = core.NewQUICTransport(targetAddr)
	default:
		fmt.Fprintf(os.Stderr, "unknown transport %s\n", *transportName)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Create transport failed: %v\n", err)
		os.Exit(1)
	}

	// 启动本地代理（socks5 / connect）
	port, err := core.ServeProxy(*proto, t)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Start local proxy failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Local %s proxy listening on :%d\n", *proto, port)

	// 阻塞
	select {}
}
