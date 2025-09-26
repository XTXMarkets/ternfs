// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package log

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

const (
	XMON_CREATE int32 = 0x4
	XMON_UPDATE int32 = 0x5
	XMON_CLEAR  int32 = 0x3
)

type XmonAppType string

const (
	XMON_NEVER    XmonAppType = "restech_eggsfs.never"
	XMON_DAYTIME  XmonAppType = "restech_eggsfs.daytime"
	XMON_CRITICAL XmonAppType = "restech_eggsfs.critical"
)

var appTypes = []XmonAppType{XMON_NEVER, XMON_DAYTIME, XMON_CRITICAL}

type XmonTroll struct {
	AppInstance string
	AppType     XmonAppType
}

type xmonRequest struct {
	troll       XmonTroll
	msgType     int32
	alertId     int64
	quietPeriod time.Duration
	binnable    bool
	message     string
	file        string
	line        int
	logLevel    LogLevel
}

type Xmon struct {
	hostname              string
	parent                XmonTroll
	children              []XmonTroll
	xmonAddr              string
	requests              chan xmonRequest
	nonBinnableAlertsSema *semaphore.Weighted
	onlyLogging           bool
	printQuietAlerts      bool
}

type XmonConfig struct {
	// If this is true, the alerts won't actually be sent,
	// but it'll still log alert creation etc
	OnlyLogging bool
	Addr        string
	// This is the "default" app instance/app type. We then
	// implicitly create instances for all the other app types,
	// and with the same app instance, so that we can send
	// alerts of different severity
	AppInstance      string
	AppType          XmonAppType
	PrintQuietAlerts bool
}

func (x *Xmon) packString(buf *bytes.Buffer, s string) {
	if len(s) >= 1<<16 {
		panic(fmt.Errorf("string too long (%v)", len(s)))
	}
	binary.Write(buf, binary.BigEndian, uint16(len(s)))
	buf.Write([]byte(s))
}

func (x *Xmon) readString(r io.Reader) (string, error) {
	var len uint16
	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return "", err
	}
	s := make([]byte, len)
	if _, err := io.ReadFull(r, s); err != nil {
		return "", err
	}
	return string(s), nil
}

const heartbeatIntervalSecs uint8 = 5

// sends both the main logon and the child logons.
func (x *Xmon) packLogon(buf *bytes.Buffer) {
	buf.Reset()

	// Name 	Type 	Description
	// Magic 	int16 	always 'T'
	// Version 	int32 	version number (latest version is 4)
	// Message type 	int32 	0x0
	// (unused) 	int32 	must be 0
	// Hostname 	string 	your hostname
	// Heartbeat interval 	byte 	in seconds, 0 for default (5s))
	// (unused) 	byte 	must be 0
	// (unused) 	int16 	must be 0
	// App type 	string 	defines rota and schedule
	// App inst 	string 	arbitrary identifier
	// Heap size 	int64 	(deprecated)
	// Num threads 	int32 	(deprecated)
	// Num errors 	int32 	(deprecated)
	// Mood 	int32 	current mood
	// Details JSON 	string 	(unused)
	binary.Write(buf, binary.BigEndian, int16('T')) // magic
	binary.Write(buf, binary.BigEndian, int32(4))   // version
	binary.Write(buf, binary.BigEndian, int32(0x0)) // message type
	binary.Write(buf, binary.BigEndian, int32(0x0)) // unused
	x.packString(buf, x.hostname)                   // hostname
	buf.Write([]byte{heartbeatIntervalSecs})        // interval
	buf.Write([]byte{0})                            // unused
	binary.Write(buf, binary.BigEndian, int16(0))   // unused
	x.packString(buf, string(x.parent.AppType))     // app type
	x.packString(buf, x.parent.AppInstance)         // app instance
	binary.Write(buf, binary.BigEndian, int64(0))   // heap size
	binary.Write(buf, binary.BigEndian, int32(0))   // num threads
	binary.Write(buf, binary.BigEndian, int32(0))   // num errors
	binary.Write(buf, binary.BigEndian, int32(0))   // happy mood
	x.packString(buf, "")                           // details

	for _, child := range x.children {
		// Message type	int32	0x100
		// (unused)	int32	must be 0
		// Hostname	string	your hostname
		// (unused)	byte	must be 0
		// (unused)	byte	must be 0
		// (unused)	int16	must be 0
		// App type	string	defines rota and schedule
		// App inst	string	arbitrary identifier
		// Heap size	int64	(deprecated)
		// Num threads	int32	(deprecated)
		// Num errors	int32	(deprecated)
		// Mood	int32	current mood
		// Details JSON	string	(unused)
		binary.Write(buf, binary.BigEndian, int32(0x100)) // msg type
		binary.Write(buf, binary.BigEndian, int32(0))     // must be 0
		x.packString(buf, x.hostname)                     // hostname
		buf.Write([]byte{0})                              // unused
		buf.Write([]byte{0})                              // unused
		binary.Write(buf, binary.BigEndian, int16(0))     // unused
		x.packString(buf, string(child.AppType))          // app type
		x.packString(buf, child.AppInstance)              // app instance
		binary.Write(buf, binary.BigEndian, int64(0))     // heap size
		binary.Write(buf, binary.BigEndian, int32(0))     // num threads
		binary.Write(buf, binary.BigEndian, int32(0))     // num errors
		binary.Write(buf, binary.BigEndian, int32(0))     // happy mood
		x.packString(buf, "")                             // details
	}
}

func (x *Xmon) packUpdate(buf *bytes.Buffer) {
	buf.Reset()
	// Message type 	int32 	0x1
	// (unused) 	int32 	must be 0
	// Heap size 	int64 	(deprecated)
	// Num threads 	int32 	(deprecated)
	// Num errors 	int32 	(deprecated)
	// Mood 	int32 	updated mood
	binary.Write(buf, binary.BigEndian, int32(0x1))
	binary.Write(buf, binary.BigEndian, int32(0))
	binary.Write(buf, binary.BigEndian, int64(0))
	binary.Write(buf, binary.BigEndian, int32(0))
	binary.Write(buf, binary.BigEndian, int32(0))
	binary.Write(buf, binary.BigEndian, int32(0)) // happy mood
}

func (x *Xmon) packRequest(buf *bytes.Buffer, req *xmonRequest) {
	if req.alertId < 0 {
		panic(fmt.Errorf("bad alert id %v", req.alertId))
	}
	buf.Reset()
	hasMessage := req.msgType == XMON_CREATE || req.msgType == XMON_UPDATE
	if x.parent == req.troll { // normal
		binary.Write(buf, binary.BigEndian, req.msgType)
		binary.Write(buf, binary.BigEndian, req.alertId)
		if hasMessage {
			binary.Write(buf, binary.BigEndian, req.binnable)
			x.packString(buf, req.message)
		}
	} else {
		req.msgType |= 0x100
		binary.Write(buf, binary.BigEndian, req.msgType)
		x.packString(buf, string(req.troll.AppType))
		x.packString(buf, req.troll.AppInstance)
		x.packString(buf, fmt.Sprintf("%v", req.alertId))
		if hasMessage {
			binnable := int32(0)
			if req.binnable {
				binnable = 1
			}
			binary.Write(buf, binary.BigEndian, binnable)
			x.packString(buf, req.message)
			x.packString(buf, "") // json (unused)
		}
	}
}

const maxBinnableAlerts int = 20
const maxNonBinnableAlerts int = 300

// alert in quiet period
type quietAlert struct {
	troll      XmonTroll
	quietUntil time.Time
	message    string
}

func (x *Xmon) run(log *Logger) {
	buffer := bytes.NewBuffer([]byte{})
	requestsCap := uint64(4096)
	requestsMask := requestsCap - 1
	requests := make([]xmonRequest, requestsCap)
	requestsHead := uint64(0)
	requestsTail := uint64(0)
	var binnableAlerts map[int64]struct{}
	if !x.onlyLogging {
		binnableAlerts = make(map[int64]struct{})
	}
	// alerts in quiet period
	quietAlerts := make(map[int64]*quietAlert)

	var conn *net.TCPConn
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	var err error

Reconnect:
	if x.onlyLogging {
		log.Info("using xmon only for logging")
	} else {
		// close previous conn
		if conn != nil {
			conn.Close()
			conn = nil
		}
		// delay if we're recovering after an error
		if err != nil {
			log.Info("about to reconnect to Xmon (%v) after err %v, waiting for 1 sec first", x.xmonAddr, err)
			time.Sleep(time.Second)
		} else {
			log.Info("connecting to Xmon (%v)", x.xmonAddr)
		}
		// connect
		var someConn net.Conn
		someConn, err = net.Dial("tcp", x.xmonAddr)
		if err != nil {
			log.ErrorNoAlert("could not connect to xmon, will reconnect: %v", err)
			goto Reconnect
		}
		conn = someConn.(*net.TCPConn)
		// send logon message(s)
		x.packLogon(buffer)
		if _, err = buffer.WriteTo(conn); err != nil {
			log.ErrorNoAlert("could not logon to xmon, will reconnect: %v", err)
			goto Reconnect
		}
		log.Info("connected to xmon, logon sent")
	}
	// we're in, start loop
	gotHeartbeatAt := time.Time{}
	lastReadAttempt := time.Time{}
	for {
		// reconnect when too much time has passed
		if !x.onlyLogging && !gotHeartbeatAt.Equal(time.Time{}) && time.Since(gotHeartbeatAt) > time.Second*2*time.Duration(heartbeatIntervalSecs) {
			log.Info("heartbeat deadline has passed, will reconnect")
			gotHeartbeatAt = time.Time{}
			goto Reconnect
		}

		// fetch a bunch of requests and then send them out
		if x.onlyLogging || !gotHeartbeatAt.Equal(time.Time{}) {
			now := time.Now()
			// unquiet alerts that are due
			for aid, alert := range quietAlerts {
				if alert.quietUntil.Before(now) {
					delete(quietAlerts, aid)
					requests[requestsTail&requestsMask] = xmonRequest{
						troll:    alert.troll,
						msgType:  XMON_CREATE,
						alertId:  aid,
						binnable: false,
						message:  alert.message,
					}
					requestsTail++
				}
			}
			// fill in requests from channel
			for requestsTail-requestsHead < requestsCap {
				select {
				case requests[requestsTail&requestsMask] = <-x.requests:
					requestsTail++
				default:
					goto NoMoreRequests
				}
			}
		NoMoreRequests:
			// send them out
			for requestsHead < requestsTail {
				req := &requests[requestsHead&requestsMask]
				switch req.msgType {
				case XMON_CREATE:
					if req.quietPeriod > 0 {
						if req.binnable {
							panic(fmt.Errorf("got alert create with quietPeriod=%v, but it is binnable", req.quietPeriod))
						}
						level := DEBUG
						if x.printQuietAlerts {
							level = req.logLevel
						}
						log.LogLocation(level, req.file, req.line, "quiet non-binnable alertId=%v message=%q quietPeriod=%v troll=%+v, will wait", req.alertId, req.message, req.quietPeriod, req.troll)
						quietAlerts[req.alertId] = &quietAlert{
							message:    req.message,
							quietUntil: now.Add(req.quietPeriod),
							troll:      req.troll,
						}
						goto SkipRequest
					} else if req.binnable && !x.onlyLogging && len(binnableAlerts) > maxBinnableAlerts {
						log.LogLocation(ERROR, req.file, req.line, "skipping create alert, alertId=%v binnable=%v message=%q troll=%+v, too many already", req.alertId, req.binnable, req.message, req.troll)
						// if we don't have the "too many alerts" alert, create it
						if _, ok := binnableAlerts[tooManyAlertsAlertId]; !ok {
							req = &xmonRequest{
								msgType:  XMON_CREATE,
								alertId:  tooManyAlertsAlertId,
								message:  "too many alerts, alerts dropped",
								binnable: true,
								troll:    x.parent, // it's unique per application
							}
						} else {
							goto SkipRequest
						}
					} else {
						log.LogLocation(req.logLevel, req.file, req.line, "creating alert, binnableAlerts=%v alertId=%v binnable=%v troll=%+v message=%q", len(binnableAlerts), req.alertId, req.binnable, req.troll, req.message)
					}
				case XMON_UPDATE:
					if req.binnable {
						panic(fmt.Errorf("unexpected update to non-binnable alert"))
					}
					quiet := quietAlerts[req.alertId]
					if quiet != nil {
						log.LogLocation(DEBUG, req.file, req.line, "skipping update alertId=%v message=%q troll=%+v since it's quiet until %v", req.alertId, req.message, req.troll, quiet.quietUntil)
						quiet.message = req.message
						goto SkipRequest
					}
					log.LogLocation(req.logLevel, req.file, req.line, "updating alert, alertId=%v binnable=%v message=%q troll=%+v", req.alertId, req.binnable, req.message, req.troll)
				case XMON_CLEAR:
					if req.binnable {
						panic(fmt.Errorf("unexpected clear to non-binnable alert"))
					}
					quiet := quietAlerts[req.alertId]
					if quiet != nil {
						log.LogLocation(DEBUG, req.file, req.line, "skipping clear alertId=%v lastMessage=%q troll=%+v since it's quiet until %v", req.alertId, req.message, req.troll, quiet.quietUntil)
						delete(quietAlerts, req.alertId)
						goto SkipRequest
					}
					log.LogLocation(INFO, req.file, req.line, "sending clear alert, alertId=%v lastMessage=%v troll=%+v", req.alertId, req.message, req.troll)
				default:
					panic(fmt.Errorf("bad req type %v", req.msgType))
				}
				if !x.onlyLogging {
					x.packRequest(buffer, req)
					if _, err = buffer.WriteTo(conn); err != nil {
						log.ErrorNoAlert("could not write request to xmon: %v", err)
						// note that we haven't removed the request from the ring buffer yet
						goto Reconnect
					}
					if (req.msgType == XMON_CREATE || req.msgType == (XMON_CREATE|0x100)) && req.binnable {
						binnableAlerts[req.alertId] = struct{}{}
					}
				}
			SkipRequest:
				requestsHead++
			}
		}
		// read all responses, not waiting too long
		if x.onlyLogging {
			time.Sleep(time.Millisecond * 100)
		} else {
			for {
				currentReadAttempt := time.Now()
				if lastReadAttempt.Equal(time.Time{}) {
					lastReadAttempt = currentReadAttempt
				}
				if currentReadAttempt.Sub(lastReadAttempt) > time.Second*2 {
					log.ErrorNoAlert("large delay between atempts to read xmon connection %v ", currentReadAttempt.Sub(lastReadAttempt))
				}
				if err = conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100)); err != nil {
					log.ErrorNoAlert("could not set deadline: %v", err)
					goto Reconnect
				}
				lastReadAttempt = currentReadAttempt
				var respType int32
				if err = binary.Read(conn, binary.BigEndian, &respType); err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() { // nothing to read
						break
					}
					log.ErrorNoAlert("could not read request from xmon: %v", err)
					goto Reconnect
				}
				switch respType {
				case 0x0:
					if gotHeartbeatAt.Equal(time.Time{}) {
						log.Info("got first xmon heartbeat, we're in")
					}
					gotHeartbeatAt = time.Now()
					x.packUpdate(buffer)
					if _, err = buffer.WriteTo(conn); err != nil {
						log.ErrorNoAlert("cold not send update to xmon: %v", err)
						goto Reconnect
					}
				case 0x1: // alert binned
					// we need to wait for the rest
					if err = conn.SetReadDeadline(time.Time{}); err != nil {
						log.ErrorNoAlert("could not set deadline: %v", err)
						goto Reconnect
					}
					var alertId int64
					if err = binary.Read(conn, binary.BigEndian, &alertId); err != nil {
						log.ErrorNoAlert("could not read alert id: %v", err)
						goto Reconnect
					}
					delete(binnableAlerts, alertId)
					log.Info("UI cleared alert alertId=%v", alertId)
				case 0x101: // child alert binned
					// we need to wait for the rest
					if err = conn.SetReadDeadline(time.Time{}); err != nil {
						log.ErrorNoAlert("could not set deadline: %v", err)
						goto Reconnect
					}
					// we don't care about which child this is, alert ids are unique anyway
					if _, err = x.readString(conn); err != nil {
						log.ErrorNoAlert("could not read child app type: %v", err)
						goto Reconnect
					}
					if _, err = x.readString(conn); err != nil {
						log.ErrorNoAlert("could not read child app instance: %v", err)
						goto Reconnect
					}
					var alertIdStr string
					if alertIdStr, err = x.readString(conn); err != nil {
						log.ErrorNoAlert("could not read child alert id: %v", err)
						goto Reconnect
					}
					var alertId int64
					if alertId, err = strconv.ParseInt(alertIdStr, 0, 64); err != nil {
						log.ErrorNoAlert("could not pares child alert id: %v", err)
						goto Reconnect
					}
					delete(binnableAlerts, alertId)
					log.Info("UI cleared alert alertId=%v", alertId)
				default:
					panic(fmt.Errorf("bad message type %v", respType))
				}
			}
		}
	}
}

func NewXmon(log *Logger, config *XmonConfig) (*Xmon, error) {
	var x *Xmon
	if config.OnlyLogging {
		x = &Xmon{
			requests:         make(chan xmonRequest, 4096),
			printQuietAlerts: config.PrintQuietAlerts,
			onlyLogging:      true,
		}
	} else {
		{
			found := false
			for _, appType := range appTypes {
				if appType == config.AppType {
					found = true
				}
			}
			if !found {
				panic(fmt.Errorf("unknown app type %q", config.AppType))
			}
		}
		if config.AppInstance == "" {
			panic(fmt.Errorf("empty app instance"))
		}
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		hostname = strings.Split(hostname, ".")[0]
		x = &Xmon{
			parent: XmonTroll{
				AppType:     config.AppType,
				AppInstance: config.AppInstance + "@" + hostname,
			},
			hostname:              hostname,
			requests:              make(chan xmonRequest, 4096),
			printQuietAlerts:      config.PrintQuietAlerts,
			children:              []XmonTroll{},
			nonBinnableAlertsSema: semaphore.NewWeighted(int64(maxNonBinnableAlerts)),
		}
		for _, appType := range appTypes {
			if appType == config.AppType {
				continue
			}
			x.children = append(x.children, XmonTroll{
				AppType:     appType,
				AppInstance: config.AppInstance + "@" + hostname,
			})
		}
		x.xmonAddr = config.Addr
	}
	go x.run(log)
	return x, nil
}

type XmonNCAlert struct {
	alertId     int64 // -1 if we haven't got one yet
	appType     XmonAppType
	lastMessage string
	quietPeriod time.Duration
}

func (x *XmonNCAlert) SetAppType(appType XmonAppType) {
	x.appType = appType
}

func (x *Xmon) NewNCAlert(quietPeriod time.Duration) *XmonNCAlert {
	return &XmonNCAlert{
		alertId:     -1,
		quietPeriod: quietPeriod,
	}
}

const tooManyAlertsAlertId = int64(0)

var alertIdCount = int64(1)

func xmonRaiseStack(
	log *Logger,
	xmon *Xmon,
	appType XmonAppType,
	calldepth int,
	alertId *int64,
	binnable bool,
	quietPeriod time.Duration,
	format string,
	v ...any,
) string {
	if appType == "" {
		appType = xmon.parent.AppType
	}
	troll := XmonTroll{
		AppInstance: xmon.parent.AppInstance,
		AppType:     appType,
	}
	logLevel := ERROR
	if appType == XMON_NEVER {
		logLevel = INFO
	}
	file, line := getFileLine(1 + calldepth)
	message := fmt.Sprintf("%s:%d "+format, append([]any{file, line}, v...)...)
	if binnable || quietPeriod == 0 {
		log.LogLocation(logLevel, file, line, "%s", message)
	}
	if *alertId < 0 {
		*alertId = atomic.AddInt64(&alertIdCount, 1)
		if !binnable && !xmon.onlyLogging {
			err := xmon.nonBinnableAlertsSema.Acquire(context.Background(), 1)
			if err != nil {
				panic(err)
			}
		}
		xmon.requests <- xmonRequest{
			troll:       troll,
			msgType:     XMON_CREATE,
			alertId:     *alertId,
			quietPeriod: quietPeriod,
			binnable:    binnable,
			message:     message,
			file:        file,
			line:        line,
			logLevel:    logLevel,
		}
	} else {
		xmon.requests <- xmonRequest{
			troll:       troll,
			msgType:     XMON_UPDATE,
			alertId:     *alertId,
			quietPeriod: quietPeriod,
			binnable:    binnable,
			message:     message,
			file:        file,
			line:        line,
			logLevel:    logLevel,
		}
	}
	return message
}

func (x *Xmon) RaiseStack(log *Logger, xmon *Xmon, appType XmonAppType, calldepth int, format string, v ...any) {
	alertId := int64(-1)
	xmonRaiseStack(log, x, appType, 1+calldepth, &alertId, true, 0, format, v...)
}

func (x *Xmon) Raise(log *Logger, xmon *Xmon, appType XmonAppType, format string, v ...any) {
	alertId := int64(-1)
	xmonRaiseStack(log, x, appType, 1, &alertId, true, 0, format, v...)
}

func (a *XmonNCAlert) RaiseStack(log *Logger, xmon *Xmon, calldepth int, format string, v ...any) {
	a.lastMessage = xmonRaiseStack(log, xmon, a.appType, 1+calldepth, &a.alertId, false, a.quietPeriod, format, v...)
}

func (a *XmonNCAlert) Raise(log *Logger, xmon *Xmon, format string, v ...any) {
	a.lastMessage = xmonRaiseStack(log, xmon, a.appType, 1, &a.alertId, false, a.quietPeriod, format, v...)
}

func (a *XmonNCAlert) Clear(log *Logger, xmon *Xmon) {
	if a.alertId < 0 {
		return
	}
	troll := XmonTroll{
		AppInstance: xmon.parent.AppInstance,
		AppType:     a.appType,
	}
	if troll.AppType == "" {
		troll.AppType = xmon.parent.AppType
	}
	if !xmon.onlyLogging {
		xmon.nonBinnableAlertsSema.Release(1)
	}
	xmon.requests <- xmonRequest{
		troll:   troll,
		msgType: XMON_CLEAR,
		alertId: a.alertId,
		message: a.lastMessage,
	}
	a.alertId = -1
}
