package daytripper

import (
	"crypto/tls"
	"github.com/swedishborgie/daytripper/har"
	"net/http/httptrace"
	"strings"
	"time"
)

type timingsTracker struct {
	report     *tripReport
	startTimes struct {
		blocked  time.Time
		dns      time.Time
		connect  time.Time
		tls      time.Time
		send     time.Time
		response time.Time
		wait     time.Time
	}
}

func newTimingsTracker(report *tripReport) *timingsTracker {
	report.entry.StartedDateTime = time.Now()
	report.entry.Timings = &har.Timings{}
	return &timingsTracker{
		report: report,
	}
}

func (t *timingsTracker) GetTracker() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:              t.getConn,
		GotConn:              t.gotConn,
		GotFirstResponseByte: t.gotFirstResponseByte,
		DNSStart:             t.dnsStart,
		DNSDone:              t.dnsDone,
		ConnectStart:         t.connectStart,
		ConnectDone:          t.connectDone,
		TLSHandshakeStart:    t.tlsHandshakeStart,
		TLSHandshakeDone:     t.tlsHandshakeDone,
		WroteRequest:         t.wroteRequest,
	}
}

func (t *timingsTracker) getConn(_ string) {
	t.startTimes.blocked = time.Now()
}
func (t *timingsTracker) gotConn(_ httptrace.GotConnInfo) {
	t.report.entry.Timings.Blocked = har.DurationMS(time.Since(t.startTimes.blocked))
	// Set this here, in the case of pooled connections, this might be the step before send starts.
	// This will get overwritten later if there are further steps.
	t.startTimes.send = time.Now()

}
func (t *timingsTracker) gotFirstResponseByte() {
	t.report.entry.Timings.Wait = har.DurationMS(time.Since(t.startTimes.wait))
	t.startTimes.response = time.Now()
}

func (t *timingsTracker) dnsStart(_ httptrace.DNSStartInfo) {
	t.startTimes.dns = time.Now()
}

func (t *timingsTracker) dnsDone(_ httptrace.DNSDoneInfo) {
	t.report.entry.Timings.DNS = har.DurationMS(time.Since(t.startTimes.dns))
}

func (t *timingsTracker) connectStart(_ string, addr string) {
	portIdx := strings.LastIndex(addr, ":")

	if portIdx < 0 {
		t.report.entry.ServerIPAddress = addr
	} else {
		t.report.entry.ServerIPAddress = addr[0:portIdx]
		t.report.entry.Connection = addr[portIdx+1:]
	}
	t.startTimes.connect = time.Now()
}

func (t *timingsTracker) connectDone(_ string, _ string, _ error) {
	t.report.entry.Timings.Connect = har.DurationMS(time.Since(t.startTimes.connect))
	// In the case of HTTP connections, TLS will get skipped.
	t.startTimes.send = time.Now()
}

func (t *timingsTracker) tlsHandshakeStart() {
	t.startTimes.tls = time.Now()
}

func (t *timingsTracker) tlsHandshakeDone(tls.ConnectionState, error) {
	t.report.entry.Timings.SSL = har.DurationMS(time.Since(t.startTimes.tls))
	t.startTimes.send = time.Now()
}

func (t *timingsTracker) wroteRequest(_ httptrace.WroteRequestInfo) {
	t.report.entry.Timings.Send = har.DurationMS(time.Since(t.startTimes.send))
	t.startTimes.wait = time.Now()
}

func (t *timingsTracker) responseRead() {
	t.report.entry.Timings.Receive = har.DurationMS(time.Since(t.startTimes.response))
}
