package imap

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
)

func TestReadOnlyWireCommands(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	if err := clientConn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	commands := make(chan string, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		fmt.Fprint(serverConn, "* PREAUTH [CAPABILITY IMAP4rev1] ready\r\n")
		scanner := bufio.NewScanner(serverConn)
		for scanner.Scan() {
			line := scanner.Text()
			commands <- line
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			if fields[1] == "EXAMINE" || fields[1] == "SELECT" {
				fmt.Fprintf(serverConn, "* 0 EXISTS\r\n* OK [UIDVALIDITY 1] valid\r\n%s OK [READ-ONLY] selected\r\n", fields[0])
			} else {
				fmt.Fprintf(serverConn, "%s OK done\r\n", fields[0])
			}
		}
	}()
	inner := imapclient.New(clientConn, nil)
	defer inner.Close()
	wrapped := wrappedClient{inner: inner}
	if _, err := wrapped.Select("INBOX").Wait(); err != nil {
		t.Fatal(err)
	}
	if err := wrapped.Fetch([]uint32{1}, true).Close(); err != nil {
		t.Fatal(err)
	}
	inner.Close()
	<-done
	close(commands)
	var all []string
	for line := range commands {
		all = append(all, line)
	}
	wire := strings.Join(all, "\n")
	if !strings.Contains(wire, "EXAMINE") || strings.Contains(wire, " SELECT ") || !strings.Contains(wire, "BODY.PEEK[]") {
		t.Fatalf("read operations may mutate mailbox:\n%s", wire)
	}
}
