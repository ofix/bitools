package core

import (
	"context"
	"golang.org/x/crypto/ssh"
)

type SSH2Transport struct {
	client *ssh.Client
}

func NewSSH2Transport(addr, user, keyPath, pass string) (*SSH2Transport, error) {
	var auth []ssh.AuthMethod
	if keyPath != "" {
		// load private key
		b, _ := os.ReadFile(keyPath)
		signer, _ := ssh.ParsePrivateKey(b)
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if pass != "" {
		auth = append(auth, ssh.Password(pass))
	}
	conf := &ssh.ClientConfig{User: user, Auth: auth, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	cli, err := ssh.Dial("tcp", addr, conf)
	if err != nil {
		return nil, err
	}
	return &SSH2Transport{client: cli}, nil
}

func (s *SSH2Transport) OpenStream(ctx context.Context) (Stream, error) {
	ch, req, err := s.client.OpenChannel("tunnel", nil)
	if err != nil {
		return nil, err
	}
	go func() { <-req }()
	return &sshStream{ch: ch}, nil
}
func (s *SSH2Transport) AcceptStream(ctx context.Context) (Stream, error) { return nil, nil }
func (s *SSH2Transport) Close() error { return s.client.Close() }

type sshStream struct{ ch ssh.Channel }
func (s *sshStream) Read(b []byte) (int, error)  { return s.ch.Read(b) }
func (s *sshStream) Write(b []byte) (int, error) { return s.ch.Write(b) }
func (s *sshStream) Close() error                { return s.ch.Close() }
