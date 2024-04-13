package debugserver

import (
	"flag"
	"io"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"

	lager "code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
)

const (
	DebugFlag = "debugAddr"
)

type DebugServerConfig struct {
	DebugAddress string `json:"debug_address"`
}

func AddFlags(flags *flag.FlagSet) {
	flags.String(
		DebugFlag,
		"",
		"host:port for serving pprof debugging info",
	)
}

func DebugAddress(flags *flag.FlagSet) string {
	dbgFlag := flags.Lookup(DebugFlag)
	if dbgFlag == nil {
		return ""
	}

	return dbgFlag.Value.String()
}

func Runner(address string, sink *lager.ReconfigurableSink) ifrit.Runner {
	return http_server.New(address, Handler(sink))
}

func Run(address string, sink *lager.ReconfigurableSink) (ifrit.Process, error) {
	p := ifrit.Invoke(Runner(address, sink))
	select {
	case <-p.Ready():
	case err := <-p.Wait():
		return nil, err
	}
	return p, nil
}

func Handler(sink *lager.ReconfigurableSink) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/log-level", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		level, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		switch string(level) {
		case "debug", "DEBUG", "d", strconv.Itoa(int(lager.DEBUG)):
			sink.SetMinLevel(lager.DEBUG)
		case "info", "INFO", "i", strconv.Itoa(int(lager.INFO)):
			sink.SetMinLevel(lager.INFO)
		case "error", "ERROR", "e", strconv.Itoa(int(lager.ERROR)):
			sink.SetMinLevel(lager.ERROR)
		case "fatal", "FATAL", "f", strconv.Itoa(int(lager.FATAL)):
			sink.SetMinLevel(lager.FATAL)
		}
	}))
	mux.Handle("/block-profile-rate", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_rate, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		rate, err := strconv.Atoi(string(_rate))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		if rate <= 0 {
			runtime.SetBlockProfileRate(0)
		} else {
			runtime.SetBlockProfileRate(rate)
		}
	}))
	mux.Handle("/mutex-profile-fraction", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_rate, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		rate, err := strconv.Atoi(string(_rate))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		if rate <= 0 {
			runtime.SetMutexProfileFraction(0)
		} else {
			runtime.SetMutexProfileFraction(rate)
		}
	}))

	return mux
}
