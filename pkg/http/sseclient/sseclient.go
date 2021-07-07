package sseclient

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
)

// Event represents a Server-Sent Event
type Event struct {
	Name string
	Data []byte
}

// Stream opens a connection to a stream of server sent events
func Stream(rawurl string) (chan Event, error) {
	resp, err := get(rawurl)
	if err != nil {
		return nil, err
	}

	events := make(chan Event)
	reader := bufio.NewReader(resp.Body)
	go loop(reader, events)
	return events, nil
}

func loop(reader *bufio.Reader, events chan Event) {
	ev := Event{}
	var buf bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during resp.Body read:%s\n", err)

			close(events)
		}

		switch {
		// name of event
		case hasPrefix(line, "event: "):
			ev.Name = string(line[7 : len(line)-1])
		case hasPrefix(line, "event:"):
			ev.Name = string(line[6 : len(line)-1])

		// event data
		case hasPrefix(line, "data: "):
			buf.Write(line[6:])
		case hasPrefix(line, "data:"):
			buf.Write(line[5:])

		// end of event
		case bytes.Equal(line, []byte("\n")):
			ev.Data = buf.Bytes()
			buf.Reset()
			events <- ev
			ev = Event{}

		default:
			fmt.Fprintf(os.Stderr, "Error: len:%d\n%s", len(line), line)

			close(events)
		}
	}
}

func get(rawurl string) (*http.Response, error) {
	resp, err := http.Get(rawurl)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got response status code %d\n", resp.StatusCode)
	}
	return resp, nil
}

func hasPrefix(s []byte, prefix string) bool {
	return bytes.HasPrefix(s, []byte(prefix))
}
