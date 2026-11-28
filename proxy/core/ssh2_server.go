package core

import (
	"golang.org/x/crypto/ssh"
	"net"
)

func ServeSSH2(listen string, signer ssh.Signer) error {
	conf := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	conf.AddHostKey(signer)
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	for {
		conn, _ := l.Accept()
		sshConn, chans, reqs, err := ssh.NewServerConn(conn, conf)
		if err != nil {
			continue
		}
		go ssh.DiscardRequests(reqs)
		go func() {
			for ch := range chans {
				if ch.ChannelType() != "tunnel" {
					ch.Reject(ssh.UnknownChannelType, "unknown")
					continue
				}
				ch.Accept()
				stream := &sshStream{ch: ch}
				go func(s Stream) {
					for {
						f, err := DecodeFrame(s)
						if err != nil {
							return
						}
						_ = EncodeFrame(s, f)
					}
				}(stream)
			}
		}()
	}
}
