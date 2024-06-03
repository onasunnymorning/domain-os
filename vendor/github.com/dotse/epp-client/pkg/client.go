// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package pkg

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"time"

	epplib "github.com/dotse/epp-lib"
)

// Client hold the information needed for the client to get and send messages
// to and from the server.
type Client struct {
	conn      *tls.Conn
	buf       epplib.MessageBuffer
	Greeting  string
	Templates Templates
}

// Connect connects to the server.
func Connect(addr string, config *tls.Config) (*Client, error) {
	conn, err := tls.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	c := &Client{conn: conn}

	// Wait for the greeting.
	r, err := epplib.MessageReader(c.conn, 0)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	c.Greeting = string(data)

	return c, nil
}

// SendCommand will write the request given on the
// server tls connection. Then read the response and return the result.
func (c *Client) SendCommand(request []byte) (string, error) {
	bb := bytes.NewBuffer(request)

	if _, err := c.buf.ReadFrom(bb); err != nil {
		return "", err
	}

	if err := c.buf.FlushTo(c.conn); err != nil {
		return "", err
	}

	// Wait for the response.
	r, err := epplib.MessageReader(c.conn, 0)
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(r)

	return string(b), err
}

// SendCommandUsingTemplate send a command to the epp server using
// templates and custom data.
func (c *Client) SendCommandUsingTemplate(template string, data any) (string, error) {
	bb := &bytes.Buffer{}

	if err := c.Templates.Execute(bb, template, data); err != nil {
		return "", err
	}

	return c.SendCommand(bb.Bytes())
}

// KeepAlive send hello commands to the epp server to keep the connection alive.
func (c *Client) KeepAlive(ctx context.Context) {
	ticker := time.NewTicker(300 * time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := c.SendCommandUsingTemplate("hello.xml", nil); err != nil {
					panic(err)
				}
			}
		}
	}()
}
