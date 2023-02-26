package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
)

var (
	port = flag.Int("port", 3000, "The server port")

	mutex = &sync.RWMutex{}

	streams = map[string]map[string]http.ResponseWriter{}
)

func Stream(w http.ResponseWriter, r *http.Request) {
	streamId := r.Header.Get("StreamId")
	n := 0
	err := r.Context().Err()

	log.Printf("Stream %s created for connection %s", streamId, r.RemoteAddr)

	if streamId != "" {
		buf := make([]byte, 4*1024)

		for err == nil {
			n, err = r.Body.Read(buf)

			if err == nil && n > 0 {
				mutex.Lock()

				writers := streams[streamId]

				for _, writer := range writers {
					writer.Write(buf[:n])

					if f, ok := writer.(http.Flusher); ok {
						f.Flush()
					}
				}

				mutex.Unlock()
			}
		}
	} else {
		err = fmt.Errorf("could not create stream, StreamId doesn't exist in the header")
	}

	log.Println(err)
}

func Capture(w http.ResponseWriter, r *http.Request) {
	streamId := r.Header.Get("StreamId")
	connectionId := r.RemoteAddr

	if streamId != "" && connectionId != "" {
		mutex.Lock()

		_, ok := streams[streamId]

		if !ok {
			streams[streamId] = map[string]http.ResponseWriter{}
		}

		streams[streamId][connectionId] = w

		mutex.Unlock()

		log.Printf("Connection %s registered\n", connectionId)

		<-r.Context().Done()

		mutex.Lock()

		delete(streams[streamId], connectionId)

		mutex.Unlock()

		log.Printf("Connection %s unregistered\n", connectionId)
	} else {
		log.Printf("Connection %s couldn't be registered for stream %s", connectionId, streamId)
	}
}

func main() {
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/stream", Stream)
	mux.HandleFunc("/capture", Capture)

	fs := http.FileServer(http.Dir("./public"))

	mux.Handle(
		"/",
		fs,
	)

	addr := fmt.Sprintf("localhost:%d", *port)

	log.Printf("Listening %s", addr)

	log.Fatal(
		http.ListenAndServeTLS(
			addr,
			"server.crt",
			"server.key",
			mux,
		),
	)
}
