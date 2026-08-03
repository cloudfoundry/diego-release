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

type ReconfigurableSinkInterface interface {
	SetMinLevel(level lager.LogLevel)
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

// Run starts the debug server with the provided address and log controller.
// Run() -> runProcess() -> Runner() -> http_server.New() -> Handler()
func Run(address string, zapCtrl zapLogLevelController) (ifrit.Process, error) {
	return runProcess(address, &LagerAdapter{zapCtrl})
}

// runProcess starts the debug server and returns the process.
// It invokes the Runner with the provided address and log controller.
func runProcess(address string, zapCtrl zapLogLevelController) (ifrit.Process, error) {
	p := ifrit.Invoke(Runner(address, zapCtrl))
	select {
	case <-p.Ready():
	case err := <-p.Wait():
		return nil, err
	}
	return p, nil
}

// Runner creates an ifrit.Runner for the debug server with the provided address and log controller.
func Runner(address string, zapCtrl zapLogLevelController) ifrit.Runner {
	return http_server.New(address, Handler(zapCtrl))
}

func Handler(zapCtrl zapLogLevelController) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/log-level", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the log level from the request body.
		level, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		// Validate the log level request.
		var normalizedLevel string
		if normalizedLevel, err = validateAndNormalize(w, r, level); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Convert the log level to lager.LogLevel.
		if normalizedLevel == "warn" {
			// Note that zapcore.WarnLevel is not directly supported by lager.
			// And lager does not have a separate WARN level, it uses INFO for warnings.
			// So to set the minimum level to "warn" we send an Invalid log level of 99,
			// which hits the default case in the SetMinLevel method.
			// This is a workaround to ensure that the log level is set correctly.
			zapCtrl.SetMinLevel(lager.LogLevel(99))
		} else {
			lagerLogLevel, err := lager.LogLevelFromString(normalizedLevel)
			if err != nil {
				http.Error(w, "Invalid log level: "+err.Error(), http.StatusBadRequest)
				return
			}
			zapCtrl.SetMinLevel(lagerLogLevel)
		}
		// Respond with a success message.
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("/log-level was invoked with Level: " + normalizedLevel + "\n"))
		if normalizedLevel == "fatal" {
			w.Write([]byte("Note: Fatal logs are reported as error logs in the Gorouter logs.\n"))
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
			// #nosec G104 - ignore errors writing http response to avoid spamming logs during  DoS
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
			// #nosec G104 - ignore errors writing http response to avoid spamming logs during  DoS
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
