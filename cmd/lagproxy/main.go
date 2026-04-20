//go:build dev

package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

func main() {
	listen := flag.String("listen", ":8081", "Address to listen on")
	target := flag.String("target", ":8080", "Address to proxy to (the real server)")
	latency := flag.Int("latency", 75, "One-way latency in milliseconds")
	jitter := flag.Int("jitter", 10, "Random jitter in milliseconds (±)")
	flag.Parse()

	log.Printf("Lag proxy: %s -> %s (latency=%dms, jitter=±%dms, RTT~%dms)",
		*listen, *target, *latency, *jitter, *latency*2)

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConn(conn, *target, *latency, *jitter)
	}
}

func handleConn(client net.Conn, targetAddr string, latencyMs, jitterMs int) {
	defer client.Close()

	jitterDur := time.Duration(randJitter(jitterMs)) * time.Millisecond
	time.Sleep(time.Duration(latencyMs)*time.Millisecond + jitterDur)

	backend, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		log.Printf("dial backend error: %v", err)
		return
	}
	defer backend.Close()

	log.Printf("proxying %s <-> %s", client.RemoteAddr(), backend.RemoteAddr())

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		relayWithLatency(client, backend, latencyMs, jitterMs)
	}()

	go func() {
		defer wg.Done()
		relayWithLatency(backend, client, latencyMs, jitterMs)
	}()

	wg.Wait()
}

func relayWithLatency(src, dst net.Conn, latencyMs, jitterMs int) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("read error: %v", err)
			}
			return
		}

		jitterDur := time.Duration(randJitter(jitterMs)) * time.Millisecond
		time.Sleep(time.Duration(latencyMs)*time.Millisecond + jitterDur)

		_, err = dst.Write(buf[:n])
		if err != nil {
			log.Printf("write error: %v", err)
			return
		}
	}
}

func randJitter(jitterMs int) int {
	if jitterMs <= 0 {
		return 0
	}
	return rand.Intn(2*jitterMs+1) - jitterMs
}
