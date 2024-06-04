package main

import (
	"fmt"
	"net"
	"time"

	epp "github.com/dotse/epp-lib"
)

func main() {
	commandMux := &epp.CommandMux{}

	// commandMux.BindGreeting(funcThatHandlesGreetingCommand)
	// commandMux.Bind(
	// 	epp.NewXMLPathBuilder().
	// 		AddOrphan("//hello", epp.NamespaceIETFEPP10.String()).String(),
	// 	funcThatHandlesHelloCommand,
	// )
	// commandMux.BindCommand("info", epp.NamespaceIETFContact10.String(),
	// 	funcTharHandlesContactInfoCommand,
	// )

	server := &epp.Server{
		HandleCommand: commandMux.Handle,
		Greeting:      commandMux.GetGreeting,
		// TLSConfig: tls.Config{
		// 	Certificates: []tls.Certificate{tlsCert},
		// 	ClientAuth:   tls.RequireAnyClientCert,
		// 	MinVersion:   tls.VersionTLS12,
		// },
		Timeout:      time.Hour,
		IdleTimeout:  350 * time.Second,
		WriteTimeout: 2 * time.Minute,
		ReadTimeout:  10 * time.Second,
		// Logger:         log.New(os.Stdout, "", 0), // whichever log you prefer that fit the interface
		MaxMessageSize: 1000,
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: 700,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Listening on port 700")

	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}
