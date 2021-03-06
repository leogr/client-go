package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/falcosecurity/client-go/pkg/api/output"
	"github.com/falcosecurity/client-go/pkg/api/version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is a wrapper for the gRPC connection
// it allows to connect to a Falco gRPC server.
// It is created using the function NewForConfig(config *Config) .
type Client struct {
	conn                 *grpc.ClientConn
	versionServiceClient version.ServiceClient
	outputServiceClient  output.ServiceClient
}

// Config is the configuration definition for connecting to a Falco gRPC server.
type Config struct {
	Hostname       string
	Port           uint16
	CertFile       string
	KeyFile        string
	CARootFile     string
	UnixSocketPath string
}

const targetFormat = "%s:%d"

// NewForConfig is used to create a new Falco gRPC client.
func NewForConfig(config *Config) (*Client, error) {
	if len(config.UnixSocketPath) > 0 {
		return newUnixSocketClient(config)
	}
	return newNetworkClient(config)
}

func newUnixSocketClient(config *Config) (*Client, error) {
	dialOption := grpc.WithInsecure()
	conn, err := grpc.Dial(config.UnixSocketPath, dialOption)
	if err != nil {
		return nil, fmt.Errorf("error dialing server: %v", err)
	}
	return &Client{
		conn: conn,
	}, nil
}

func newNetworkClient(config *Config) (*Client, error) {
	certificate, err := tls.LoadX509KeyPair(
		config.CertFile,
		config.KeyFile,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading the X.509 key pair: %v", err)
	}

	certPool := x509.NewCertPool()
	rootCA, err := ioutil.ReadFile(config.CARootFile)
	if err != nil {
		return nil, fmt.Errorf("error reading the CA Root file certificate: %v", err)
	}

	ok := certPool.AppendCertsFromPEM(rootCA)
	if !ok {
		return nil, fmt.Errorf("error appending the root CA to the certificate pool")
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		ServerName:   config.Hostname,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	})

	dialOption := grpc.WithTransportCredentials(transportCreds)
	conn, err := grpc.Dial(fmt.Sprintf(targetFormat, config.Hostname, config.Port), dialOption)
	if err != nil {
		return nil, fmt.Errorf("error dialing server: %v", err)
	}

	return &Client{
		conn: conn,
	}, nil
}

// Output is the client for Falco Outputs.
// When using it you can use `subscribe()` to receive a stream of Falco output events.
func (c *Client) Output() (output.ServiceClient, error) {
	if err := c.checkConn(); err != nil {
		return nil, err
	}
	if c.outputServiceClient == nil {
		c.outputServiceClient = output.NewServiceClient(c.conn)
	}
	return c.outputServiceClient, nil
}

// Version it the client for Falco Version API.
// When using it you can use `version()` to receive the Falco version.
func (c *Client) Version() (version.ServiceClient, error) {
	if err := c.checkConn(); err != nil {
		return nil, err
	}
	if c.versionServiceClient == nil {
		c.versionServiceClient = version.NewServiceClient(c.conn)
	}
	return c.versionServiceClient, nil
}

// Close the connection to the falco gRPC server.
func (c *Client) Close() error {
	if err := c.checkConn(); err != nil {
		return err
	}
	return c.conn.Close()
}

func (c *Client) checkConn() error {
	if c.conn == nil {
		return fmt.Errorf("missing connection for the current client")
	}
	return nil
}
