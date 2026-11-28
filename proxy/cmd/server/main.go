package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"proxy/core"
)

func main() {
	transportName := flag.String("transport", "http2", "http2 / ssh2 / quic")
	listen := flag.String("listen", "0.0.0.0:8443", "listen addr")
	autoCert := flag.Bool("auto-cert", true, "auto generate tls cert/ssh hostkey (memory, no file)")
	flag.Parse()

	// 填充公告信息：只填当前启动的这个transport的地址
	var h2Addr, sshAddr, quicAddr string
	switch *transportName {
	case "http2":
		h2Addr = *listen
	case "ssh2":
		sshAddr = *listen
	case "quic":
		quicAddr = *listen
	}

	// 启动组播公告
	ann := core.NewAnnouncer(h2Addr, sshAddr, quicAddr)
	if err := ann.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start multicast announcer failed: %v\n", err)
		return
	}
	fmt.Println("multicast announce running, interval 2s")

	// 信号监听，优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nreceived stop signal, closing announcer...")
		ann.Stop()
		os.Exit(0)
	}()

	var err error
	switch *transportName {
	case "http2":
		if !*autoCert {
			fmt.Fprintf(os.Stderr, "http2 requires auto-cert=true for auto generation\n")
			return
		}
		cert, err := core.GenTLSCert()
		if err != nil {
			fmt.Fprintf(os.Stderr, "gen tls cert failed: %v\n", err)
			return
		}
		err = core.ServeHTTP2(*listen, cert)
		fmt.Println("http2 server exit:", err)

	case "ssh2":
		signer, err := core.GenSSHHostKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "gen ssh host key failed: %v\n", err)
			return
		}
		err = core.ServeSSH2(*listen, signer)
		fmt.Println("ssh2 server exit:", err)

	case "quic":
		cert, err := core.GenTLSCert()
		if err != nil {
			fmt.Fprintf(os.Stderr, "gen tls cert failed: %v\n", err)
			return
		}
		err = core.ServeQUIC(*listen, cert)
		fmt.Println("quic server exit:", err)
	}
	// 隧道退出时也要停公告器
	ann.Stop()
}
