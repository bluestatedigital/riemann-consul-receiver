package main

import (
    "os"
    "os/signal"
    "syscall"
    "fmt"
    "time"
    
    log "github.com/Sirupsen/logrus"
    flags "github.com/jessevdk/go-flags"

    "github.com/armon/consul-api"
    "github.com/amir/raidman"
)

// http://technosophos.com/2014/06/11/compile-time-string-in-go.html
var version string = "undef"

type Options struct {
    Debug          bool   `env:"DEBUG"           long:"debug"                                            description:"enable debug logging"`
    LogFile        string `env:"LOG_FILE"        long:"log-file"                                         description:"JSON log file path"`
    RiemannHost    string `env:"RIEMANN_HOST"    long:"riemann-host" required:"true"                     description:"Riemann host"`
    RiemannPort    int    `env:"RIEMANN_PORT"    long:"riemann-port"                 default:"5555"      description:"Riemann port"`
    Proto          string `env:"RIEMANN_PROTO"   long:"proto"                        default:"udp"       description:"protocol to use when sending Riemann events"`
    ConsulHost     string `env:"CONSUL_HOST"     long:"consul-host"                  default:"127.0.0.1" description:"Consul host"`
    ConsulPort     int    `env:"CONSUL_PORT"     long:"consul-port"                  default:"8500"      description:"Consul port"`
    UpdateInterval string `env:"UPDATE_INTERVAL" long:"interval"                     default:"1m"        description:"how frequently to post events to Riemann"`
    LockDelay      string `env:"LOCK_DELAY"      long:"lock-delay"                   default:"15s"       description:"lock delay after session invalidation"`
    PrintVersion   bool   `                      long:"version"                                          description:"display version and exit"`
}

func sendHealthResults(riemann RiemannClient, healthResults []HealthCheck, updateInterval time.Duration, nodeName, dc string) error {
    for _, healthCheck := range healthResults {
        // {
        //   "Name": "Service 'client-youngaustria' check",
        //   "ServiceName": "client-youngaustria",
        //   "Node": "web-fwork-gen-028.us-east-1.aws.prod.bsdinternal.com"
        //   "CheckID": "service:client-youngaustria",
        //   "ServiceID": "client-youngaustria",
        //   "Output": "TTL expired",
        //   "Notes": "",
        //   "Status": "critical",
        // },

        // Riemann event TTL: A floating-point time, in seconds, that
        // this event is considered valid for
        eventTtl := float32((updateInterval * 3) / time.Second)
        
        // convert Consul status to Riemann state
        state := map[string]string{
            "passing":  "ok",
            "warning":  "warning",
            "critical": "critical",
        }[healthCheck.Status]
        
        // there may be multiple services with the same name on a given host;
        // these must have different serviceIds. there are also checks that
        // aren't associated with a specific service.  service-specific checks
        // have an id of "service:<serviceId>".
        evt := &raidman.Event{
            Ttl:         eventTtl,
            Time:        time.Now().Unix(),
            Tags:        append(healthCheck.Tags, "consul"),
            Host:        healthCheck.Node,
            State:       state,
            Service:     healthCheck.CheckID,
            Description: healthCheck.Output,
            Attributes:  map[string]string{
                "reporting_node": nodeName,
                "datacenter":     dc,
                "notes":          healthCheck.Notes,
            },
        }
        
        err := riemann.Send(evt)
        
        if err != nil {
            return err
        }
    }
    
    return nil
}

func mainLoop(
    lockWatcher    *LockWatcher,
    healthChecker  *HealthChecker,
    riemannHost    string,
    riemannPort    int,
    riemannProto   string,
    updateInterval time.Duration,
    nodeName       string,
    dc             string,
    done           chan<- interface{},
) {
    // indicate to caller when this routine is done; just close channel so the
    // write doesn't block
    defer func() { close(done) }()
    
    // (attempt to) capture stack trace on a panic
    defer recoverAndLog("mainLoop")
    
    // used to notify when lock has been lost; it'll just get closed
    var lockWatchChan <-chan interface{}
    
    // receives HealthCheck results
    var healthResultsChan <-chan []HealthCheck
    
    // control channel for the health results checker
    var healthResultsAbort chan interface{}

    // the riemann client
    var riemann RiemannClient

    keepGoing := true
    haveLock := false
    
    for keepGoing {
        // @todo update health check only when don't have lock or when health
        // results are processed successfully.
        err := lockWatcher.UpdateHealthCheck()
        checkError("unable to submit health check", err)

        if ! haveLock {
            log.Debug("acquiring lock")
            
            // don't have lock; attempt to acquire it. AcquireLock() blocks.
            haveLock, err = lockWatcher.AcquireLock()
            checkError("error acquiring lock", err)
            
            if haveLock {
                log.Info("acquired lock")
                
                // connect to Riemann
                riemannAddr := fmt.Sprintf("%s:%d", riemannHost, riemannPort)
                log.Infof("connecting to Riemann at %s via %s", riemannAddr, riemannProto)

                riemann, err = raidman.Dial(riemannProto, riemannAddr)
                
                if err != nil {
                    log.Errorf("unable to connect to Riemann: %v", err)
                    lockWatcher.ReleaseLock()
                    haveLock = false
                } else {
                    log.Info("connected")

                    // get notified when we lose our lock
                    lockWatchChan = lockWatcher.WatchLock()
                    
                    // start retrieving health results
                    healthResultsAbort = make(chan interface{})
                    healthResultsChan = healthChecker.WatchHealthResults(healthResultsAbort)
                }
            } else {
                log.Debug("could not acquire lock")
            }
        }
        
        if haveLock {
            // AcquireLock blocks for the updateInterval period.  we only have
            // channels to read from if we've got the lock.
            
            select {
                // wait for the lock to be lost
                case <-lockWatchChan:
                    log.Warn("lost lock")
                    
                    haveLock = false
                    
                    healthResultsAbort <- nil
                    healthResultsAbort = nil
                    log.Debug("commanded health results watcher to stop")
                    
                    lockWatchChan = nil
                    
                    riemann.Close()
                    riemann = nil
                
                case healthResults, more := <-healthResultsChan:
                    // channel closed if there was an error retrieving the health
                    // results. also check that we still have the lock, as the
                    // health results channel could still have results.
                    log.Debug("got health results")

                    if more && haveLock {
                        log.Debug("processing health results")
                        err := sendHealthResults(riemann, healthResults, updateInterval, nodeName, dc)
                        
                        if err != nil {
                            log.Errorf("error sending event to Riemann: %v", err)
                            
                            lockWatcher.ReleaseLock()
                        }
                    } else {
                        // lost lock or error occurred retrieving health results
                        lockWatcher.ReleaseLock()
                    }

                case <-time.After(updateInterval):
                    // timeout
            }
        }
    }
}

func main() {
    var opts Options
    
    _, err := flags.Parse(&opts)
    if err != nil {
        os.Exit(1)
    }
    
    if opts.PrintVersion {
        fmt.Printf("Version: %s\n", version)
        os.Exit(0)
    }
    
    // parse UpdateInterval and LockDelay before setting up logging
    updateInterval, err := time.ParseDuration(opts.UpdateInterval)
    checkError(fmt.Sprintf("invalid update interval %s", opts.UpdateInterval), err)
    
    lockDelay, err := time.ParseDuration(opts.LockDelay)
    checkError(fmt.Sprintf("invalid lock delay %s", opts.LockDelay), err)
    
    if opts.Debug {
        // Only log the warning severity or above.
        log.SetLevel(log.DebugLevel)
    }
    
    if opts.LogFile != "" {
        logFp, err := os.OpenFile(opts.LogFile, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0600)
        checkError(fmt.Sprintf("error opening %s", opts.LogFile), err)
        
        defer logFp.Close()
        
        // ensure panic output goes to log file
        // https://code.google.com/p/go/issues/detail?id=325
        syscall.Dup2(int(logFp.Fd()), 1)
        syscall.Dup2(int(logFp.Fd()), 2)
        
        // log as JSON
        log.SetFormatter(&log.JSONFormatter{})
        
        // send output to file
        log.SetOutput(logFp)
    }
    
    // connect to Consul
    consulConfig := consulapi.DefaultConfig()
    consulConfig.Address = fmt.Sprintf("%s:%d", opts.ConsulHost, opts.ConsulPort)
    log.Infof("connecting to Consul at %s", consulConfig.Address)

    consul, err := consulapi.NewClient(consulConfig)
    checkError("unable to create consul client", err)
    
    // need dc and node name for riemann event attributes
    agentInfo, err := consul.Agent().Self()
    checkError("unable to retrieve agent info", err)

    nodeName := agentInfo["Config"]["NodeName"].(string)
    dc := agentInfo["Config"]["Datacenter"].(string)

    lockWatcher, err := NewLockWatcher(
        consul.Agent(),
        consul.Session(),
        consul.KV(),
        consul.Health(),
        updateInterval,
        lockDelay,
        "riemann-consul-receiver",
        "services/riemann-consul-receiver",
    )
    
    checkError("unable to initialize consul receiver", err)
    
    healthChecker := NewHealthChecker(consul.Health(), consul.Catalog(), updateInterval)
    
    err = lockWatcher.RegisterService()
    checkError("unable to register service", err)
    
    _, err = lockWatcher.InitSession()
    checkError("unable to init session", err)
    
    // destroy the session when the process exits
    defer lockWatcher.DestroySession()
    
    // receive OS signals so we can cleanly shut down
    // use syscall signals because os only provides Interrupt and Kill
    signalChan := make(chan os.Signal)
    signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

    log.Debug("starting main loop")

    done := make(chan interface{})
    go mainLoop(lockWatcher, healthChecker, opts.RiemannHost, opts.RiemannPort, opts.Proto, updateInterval, nodeName, dc, done)
    
    // Block until a signal is received or mainLoop crashes
    select {
        case <-signalChan:
        case <-done:
    }
}
