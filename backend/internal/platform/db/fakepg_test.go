package db

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
)

// fakePG is a PostgreSQL server that answers one query, "SELECT 1", on any number of connections, over the
// extended protocol the pool uses (Parse, Describe, Bind, Execute, Sync) and the simple one (a ping). It lets
// the pool's behavior with canceled contexts be tested without a database; delay is how long each Execute
// takes, so that a cancel can land while the client waits for the answer.
type fakePG struct {
	ln    net.Listener
	delay time.Duration
	// readDelay is how long the server waits before it reads the next message, so that a big write blocks.
	readDelay time.Duration
	wg        sync.WaitGroup
}

func newFakePG(t *testing.T, delay time.Duration) *fakePG {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakePG{ln: ln, delay: delay}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			f.wg.Add(1)
			go func() {
				defer f.wg.Done()
				f.serve(c)
			}()
		}
	}()
	t.Cleanup(func() { _ = ln.Close(); f.wg.Wait() })
	return f
}

// URL is the connection string of the fake server.
func (f *fakePG) URL() string {
	return "postgres://u@" + f.ln.Addr().String() + "/d?sslmode=disable"
}

var oneColumn = &pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("n"), DataTypeOID: 23, DataTypeSize: 4, TypeModifier: -1}}}

func (f *fakePG) serve(c net.Conn) {
	defer c.Close()
	be := pgproto3.NewBackend(c, c)
	for {
		msg, err := be.ReceiveStartupMessage()
		if err != nil {
			return
		}
		if _, ok := msg.(*pgproto3.SSLRequest); ok {
			if _, err := c.Write([]byte("N")); err != nil {
				return
			}
			continue
		}
		if _, ok := msg.(*pgproto3.CancelRequest); ok {
			return // a cancel request is answered by closing
		}
		break
	}
	be.Send(&pgproto3.AuthenticationOk{})
	be.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "13.0.0"})
	be.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if be.Flush() != nil {
		return
	}
	for {
		time.Sleep(f.readDelay)
		msg, err := be.Receive()
		if err != nil {
			return
		}
		switch msg.(type) {
		case *pgproto3.Parse:
			_ = 0
			be.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Describe:
			be.Send(&pgproto3.ParameterDescription{})
			be.Send(oneColumn)
		case *pgproto3.Bind:
			be.Send(&pgproto3.BindComplete{})
		case *pgproto3.Execute:
			jitter := time.Duration(rand.IntN(6000)) * time.Microsecond //nolint:gosec // test jitter
			time.Sleep(f.delay + jitter)
			be.Send(&pgproto3.DataRow{Values: [][]byte{{0, 0, 0, 1}}})
			be.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
		case *pgproto3.Close:
			be.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Sync:
			be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		case *pgproto3.Query:
			be.Send(&pgproto3.EmptyQueryResponse{})
			be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		case *pgproto3.Terminate:
			return
		}
		if err := be.Flush(); err != nil && !errors.Is(err, io.EOF) {
			return
		}
	}
}
