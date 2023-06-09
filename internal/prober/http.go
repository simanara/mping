package prober

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	HTTP  ProbeType = "http"
	HTTPS ProbeType = "https"
)

type HTTPProber struct {
	client  *http.Client
	targets []string
	config  *HTTPConfig
}

func NewHTTPProber(targets []string, cfg *HTTPConfig) *HTTPProber {
	return &HTTPProber{
		client:  &http.Client{},
		targets: targets,
		config:  cfg,
	}
}

func (p *HTTPProber) sent(r chan *Event, t string) {
	r <- &Event{
		Target: t,
		Result: SENT,
	}
}

func (p *HTTPProber) timeout(r chan *Event, target string, now time.Time, err error) {
	r <- &Event{
		Target:   target,
		Result:   TIMEOUT,
		SentTime: now,
		Rtt:      time.Since(now),
		Message:  "timeout",
	}
}

func (p *HTTPProber) failed(r chan *Event, target string, now time.Time, err error) {
	r <- &Event{
		Target:   target,
		Result:   FAILED,
		SentTime: now,
		Rtt:      time.Since(now),
		Message:  err.Error(),
	}
}

func (p *HTTPProber) probe(r chan *Event, target string) {
	now := time.Now()
	p.sent(r, target)
	resp, err := p.client.Get(target)
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			p.timeout(r, target, now, err)
		} else {
			p.failed(r, target, now, err)
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.failed(r, target, now, err)
		return
	}
	if p.config.ExpectCode != 0 && p.config.ExpectCode != resp.StatusCode {
		p.failed(r, target, now, fmt.Errorf("status code: %d != %d", p.config.ExpectCode, resp.StatusCode))
	} else if p.config.ExpectBody != "" && p.config.ExpectBody != strings.TrimRight(string(body), "\n") {
		p.failed(r, target, now, errors.New("invalid body"))
	} else {
		r <- &Event{
			Target:   target,
			Result:   SUCCESS,
			SentTime: now,
			Rtt:      time.Since(now),
		}
	}
}

func (p *HTTPProber) Start(r chan *Event, interval, timeout time.Duration) error {
	p.client.Timeout = timeout
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			for _, target := range p.targets {
				go p.probe(r, target)
			}
		}
	}
}